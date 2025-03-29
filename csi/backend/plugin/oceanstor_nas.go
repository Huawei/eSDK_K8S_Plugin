/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/helper"
	xuanwuV1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume/creator"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/version"
)

const (
	// HyperMetroVstorePairActive defines active status for hyper metro vstore
	HyperMetroVstorePairActive = "0"
	// HyperMetroVstorePairLinkStatusConnected defines connected status for hyper metro vstore
	HyperMetroVstorePairLinkStatusConnected = "1"
	// HyperMetroDomainActive defines active status for hyper metro domain
	HyperMetroDomainActive = "1"
	// HyperMetroDomainRunningStatusNormal defines normal status for hyper metro domain running status
	HyperMetroDomainRunningStatusNormal = "0"

	// ConsistentSnapshotsSpecification defines consistent snapshot limits
	ConsistentSnapshotsSpecification = "128"
)

var supportConsistentSnapshotsMinVersion = "6.1.6"

// OceanstorNasPlugin implements storage StoragePlugin interface
type OceanstorNasPlugin struct {
	OceanstorPlugin
	portals       []string
	vStorePairID  string
	metroDomainID string

	nasHyperMetro       volume.NASHyperMetro
	metroRemotePlugin   *OceanstorNasPlugin
	replicaRemotePlugin *OceanstorNasPlugin
}

func init() {
	RegPlugin("oceanstor-nas", &OceanstorNasPlugin{})
}

// NewPlugin used to create new plugin
func (p *OceanstorNasPlugin) NewPlugin() StoragePlugin {
	return &OceanstorNasPlugin{}
}

// Init used to init the plugin
func (p *OceanstorNasPlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	var exist bool
	p.vStorePairID, exist = config["metrovStorePairID"].(string)
	if exist {
		log.AddContext(ctx).Infof("The metro vStorePair ID is %s", p.vStorePairID)
	}

	var protocol string
	var err error
	protocol, p.portals, err = verifyProtocolAndPortals(parameters, constants.OceanStorNas)
	if err != nil {
		log.AddContext(ctx).Errorf("check parameter failed, err: %v", err)
		return err
	}

	err = p.init(ctx, config, keepLogin)
	if err != nil {
		log.AddContext(ctx).Errorf("init oceanstor nas failed, config: %+v, parameters: %+v err: %v",
			config, parameters, err)
		return err
	}

	// only Dorado V6 6.1.7 and later versions support this nfs+ feature.
	versionComp := version.CompareVersions(p.cli.GetStorageVersion(), constants.MinVersionSupportNfsPlus)
	if protocol == ProtocolNfsPlus && (!p.product.IsDoradoV6OrV7() || (p.product.IsDoradoV6() && versionComp == -1)) {
		p.Logout(ctx)

		return errors.New("current storage version doesn't support nfs+")
	}

	return nil
}

func (p *OceanstorNasPlugin) checkNfsPlusPortalsFormat(portals []string) bool {
	var portalsTypeIP bool
	var portalsTypeDomain bool

	for _, portal := range portals {
		ip := net.ParseIP(portal)
		if ip != nil {
			portalsTypeIP = true
			if portalsTypeDomain {
				return false
			}
		} else {
			portalsTypeDomain = true
			if portalsTypeIP {
				return false
			}
		}
	}

	return true
}

func (p *OceanstorNasPlugin) getNasObj() *volume.NAS {
	var metroRemoteCli client.OceanstorClientInterface

	if p.metroRemotePlugin != nil {
		metroRemoteCli = p.metroRemotePlugin.cli
	}

	return volume.NewNAS(p.cli, metroRemoteCli, p.product, p.nasHyperMetro, p.isLogicPortRunningOnOwnSite())
}

// CreateVolume used to create volume
func (p *OceanstorNasPlugin) CreateVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	if p.metroRemotePlugin == nil {
		if err := p.assertLogicPortRunOnOwnSite(ctx); err != nil {
			return nil, err
		}
	}

	params := getParams(ctx, name, parameters)
	params["metroDomainID"] = p.metroDomainID
	nas := p.getNasObj()
	volObj, err := nas.Create(ctx, params)
	if err != nil {
		return nil, err
	}

	return volObj, nil
}

func (p *OceanstorNasPlugin) getClient() (client.OceanstorClientInterface, client.OceanstorClientInterface) {
	var replicaRemoteCli client.OceanstorClientInterface
	if p.replicaRemotePlugin != nil {
		replicaRemoteCli = p.replicaRemotePlugin.cli
	}
	return p.cli, replicaRemoteCli
}

// QueryVolume used to query volume
func (p *OceanstorNasPlugin) QueryVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	params := getParams(ctx, name, parameters)
	nas := p.getNasObj()
	return nas.Query(ctx, name, params)
}

// DeleteVolume used to delete volume
func (p *OceanstorNasPlugin) DeleteVolume(ctx context.Context, name string) error {
	if p.metroRemotePlugin == nil {
		if err := p.assertLogicPortRunOnOwnSite(ctx); err != nil {
			return err
		}
	}
	nas := p.getNasObj()
	return nas.Delete(ctx, name)
}

// ExpandVolume used to expand volume
func (p *OceanstorNasPlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	if p.metroRemotePlugin == nil {
		if err := p.assertLogicPortRunOnOwnSite(ctx); err != nil {
			return false, err
		}
	}
	nas := p.getNasObj()
	return false, nas.Expand(ctx, name, size)
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *OceanstorNasPlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	vStoreQuotaMap, err := p.getVstoreCapacity(ctx)
	if err != nil {
		log.AddContext(ctx).Debugf("get vstore capacity failed, err: %v", err)
		vStoreQuotaMap = map[string]interface{}{}
	}

	return p.updatePoolCapabilities(ctx, poolNames, vStoreQuotaMap, "2")
}

func (p *OceanstorNasPlugin) getVstoreCapacity(ctx context.Context) (map[string]interface{}, error) {
	// only Dorado V6 6.1.5 and later versions need to get vStore's capacity.
	if !p.product.IsDoradoV6OrV7() ||
		(p.product.IsDoradoV6() && version.CompareVersions(p.cli.GetStorageVersion(), constants.DoradoV615) == -1) ||
		p.cli.GetvStoreName() == "" {
		return map[string]interface{}{}, nil
	}
	vStore, err := p.cli.GetvStoreByName(ctx, p.cli.GetvStoreName())
	if err != nil {
		return nil, err
	}
	if vStore == nil {
		return nil, fmt.Errorf("not find vstore by name, name: %s", p.cli.GetvStoreName())
	}

	var nasCapacityQuota, nasFreeCapacityQuota int64

	if totalStr, ok := vStore["nasCapacityQuota"].(string); ok {
		nasCapacityQuota, err = strconv.ParseInt(totalStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
	}
	if freeStr, ok := vStore["nasFreeCapacityQuota"].(string); ok {
		nasFreeCapacityQuota, err = strconv.ParseInt(freeStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
	}
	if err != nil {
		log.AddContext(ctx).Warningf("parse vstore quota failed, error: %v", err)
		return nil, err
	}

	log.AddContext(ctx).Debugf("nasFreeCapacityQuota %v,nasCapacityQuota %v",
		nasFreeCapacityQuota, nasCapacityQuota)
	// if not set quota, nasCapacityQuota is 0, nasFreeCapacityQuota is -1
	if nasCapacityQuota == 0 || nasFreeCapacityQuota == -1 {
		return map[string]interface{}{}, nil
	}

	return map[string]interface{}{
		string(xuanwuV1.FreeCapacity):  nasFreeCapacityQuota * constants.AllocationUnitBytes,
		string(xuanwuV1.TotalCapacity): nasCapacityQuota * constants.AllocationUnitBytes,
		string(xuanwuV1.UsedCapacity):  (nasCapacityQuota - nasFreeCapacityQuota) * constants.AllocationUnitBytes,
	}, nil
}

// UpdateMetroRemotePlugin used to convert metroRemotePlugin to OceanstorSanPlugin
func (p *OceanstorNasPlugin) UpdateMetroRemotePlugin(ctx context.Context, remote StoragePlugin) {
	var ok bool
	p.metroRemotePlugin, ok = remote.(*OceanstorNasPlugin)
	if !ok {
		log.AddContext(ctx).Warningf("convert metroRemotePlugin to OceanstorNasPlugin failed, data: %v", remote)
	}
}

// CreateSnapshot used to create snapshot
func (p *OceanstorNasPlugin) CreateSnapshot(ctx context.Context,
	fsName, snapshotName string) (map[string]interface{}, error) {
	if p.metroRemotePlugin == nil {
		if err := p.assertLogicPortRunOnOwnSite(ctx); err != nil {
			return nil, err
		}
	}
	nas := p.getNasObj()

	snapshotName = utils.GetFSSnapshotName(snapshotName)
	snapshot, err := nas.CreateSnapshot(ctx, fsName, snapshotName)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// DeleteSnapshot used to delete snapshot
func (p *OceanstorNasPlugin) DeleteSnapshot(ctx context.Context, snapshotParentId, snapshotName string) error {
	if p.metroRemotePlugin == nil {
		if err := p.assertLogicPortRunOnOwnSite(ctx); err != nil {
			return err
		}
	}
	nas := p.getNasObj()

	snapshotName = utils.GetFSSnapshotName(snapshotName)
	err := nas.DeleteSnapshot(ctx, snapshotParentId, snapshotName)
	if err != nil {
		return err
	}

	return nil
}

// UpdateBackendCapabilities used to update backend capabilities
func (p *OceanstorNasPlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities, specifications, err := p.OceanstorPlugin.UpdateBackendCapabilities(ctx)
	if err != nil {
		return nil, nil, err
	}

	err = p.updateHyperMetroCapability(ctx, capabilities)
	if err != nil {
		return nil, nil, err
	}

	err = p.updateReplicationCapability(capabilities)
	if err != nil {
		return nil, nil, err
	}

	err = p.updateNFS4Capability(ctx, capabilities)
	if err != nil {
		return nil, nil, err
	}

	err = p.updateSmartThin(capabilities)
	if err != nil {
		return nil, nil, err
	}

	p.updateVStorePair(ctx, specifications)

	// update the SupportConsistentSnapshot capability and specification
	err = p.updateConsistentSnapshotCapability(capabilities, specifications)
	if err != nil {
		return nil, nil, err
	}

	return capabilities, specifications, nil
}

func (p *OceanstorNasPlugin) updateHyperMetroCapability(ctx context.Context, capabilities map[string]any) error {
	if capabilities == nil {
		return nil
	}

	if p.product.IsDoradoV6OrV7() {
		capabilities["SupportMetro"] = capabilities["SupportMetroNAS"]
	}
	delete(capabilities, "SupportMetroNAS")

	if capabilities["SupportMetro"] != true {
		return nil
	}

	if p.vStorePairID == "" {
		capabilities["SupportMetro"] = false
		return nil
	}

	vStorePair, err := p.cli.GetvStorePairByID(ctx, p.vStorePairID)
	if err != nil {
		return err
	}

	var ok bool
	if p.product.IsDoradoV6OrV7() && vStorePair != nil {
		fsHyperMetroDomain, err := p.cli.GetFSHyperMetroDomain(ctx,
			vStorePair["DOMAINNAME"].(string))
		if err != nil {
			return err
		}

		if fsHyperMetroDomain == nil ||
			fsHyperMetroDomain["RUNNINGSTATUS"] != HyperMetroDomainRunningStatusNormal {
			capabilities["SupportMetro"] = false
			return nil
		}

		p.nasHyperMetro = volume.NASHyperMetro{
			FsHyperMetroActiveSite: fsHyperMetroDomain["CONFIGROLE"] == HyperMetroDomainActive,
			LocVStoreID:            vStorePair["LOCALVSTOREID"].(string),
			RmtVStoreID:            vStorePair["REMOTEVSTOREID"].(string),
		}
		p.metroDomainID, ok = vStorePair["DOMAINID"].(string)
		if !ok {
			return fmt.Errorf("convert DOMAINID: %v to string failed", vStorePair["DOMAINID"])
		}
	} else {
		if vStorePair == nil ||
			vStorePair["ACTIVEORPASSIVE"] != HyperMetroVstorePairActive ||
			vStorePair["LINKSTATUS"] != HyperMetroVstorePairLinkStatusConnected ||
			vStorePair["LOCALVSTORENAME"] != p.cli.GetvStoreName() {
			capabilities["SupportMetro"] = false
		}
	}
	p.UpdateSupportMetroByRemoteLicense(ctx, capabilities)
	return nil
}

func (p *OceanstorNasPlugin) updateConsistentSnapshotCapability(capabilities, specifications map[string]any) error {
	if capabilities == nil {
		return nil
	}

	// only storage version gte DoradoV6 6.1.7 or DoradoV7 support consistent snapshot feature.
	versionComp := version.CompareVersions(p.cli.GetStorageVersion(), supportConsistentSnapshotsMinVersion)
	if p.product.IsDoradoV6() && versionComp != -1 || p.product.IsDoradoV7() {
		capabilities["SupportConsistentSnapshot"] = true
		if specifications != nil {
			specifications["ConsistentSnapshotLimits"] = ConsistentSnapshotsSpecification
		}
		return nil
	}

	capabilities["SupportConsistentSnapshot"] = false
	return nil
}

func (p *OceanstorNasPlugin) updateReplicationCapability(capabilities map[string]interface{}) error {
	if capabilities == nil {
		return nil
	}

	if capabilities["SupportReplication"] == true && p.replicaRemotePlugin == nil {
		capabilities["SupportReplication"] = false
	}
	return nil
}

func (p *OceanstorNasPlugin) updateNFS4Capability(ctx context.Context, capabilities map[string]interface{}) error {
	if capabilities == nil {
		capabilities = make(map[string]interface{})
	}

	nfsServiceSetting, err := p.cli.GetNFSServiceSetting(ctx)
	if err != nil {
		return err
	}

	// NFS3 is enabled by default.
	capabilities["SupportNFS3"] = true
	capabilities["SupportNFS4"] = false
	capabilities["SupportNFS41"] = false
	capabilities["SupportNFS42"] = false

	if !nfsServiceSetting["SupportNFS3"] {
		capabilities["SupportNFS3"] = false
	}

	if nfsServiceSetting["SupportNFS4"] {
		capabilities["SupportNFS4"] = true
	}

	if nfsServiceSetting["SupportNFS41"] {
		capabilities["SupportNFS41"] = true
	}

	if nfsServiceSetting["SupportNFS42"] {
		capabilities["SupportNFS42"] = true
	}

	return nil
}

// UpdateSupportMetroByRemoteLicense updates the support metro capability by remote storage license
func (p *OceanstorNasPlugin) UpdateSupportMetroByRemoteLicense(ctx context.Context,
	capabilities map[string]interface{}) {
	if capabilities == nil {
		return
	}

	if p.metroRemotePlugin == nil || p.metroRemotePlugin.cli == nil || !p.metroRemotePlugin.GetOnline() {
		capabilities["SupportMetro"] = false
		return
	}

	features, err := p.metroRemotePlugin.cli.GetLicenseFeature(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("Get license feature error: %v", err)
		capabilities["SupportMetro"] = false
		return
	}

	capabilities["SupportMetro"] = capabilities["SupportMetro"].(bool) &&
		utils.IsSupportFeature(features, "HyperMetroNAS")
}

func (p *OceanstorNasPlugin) verifyOceanstorNasParam(ctx context.Context, config map[string]interface{}) error {
	parameters, exist := config["parameters"].(map[string]interface{})
	if !exist {
		return pkgUtils.Errorf(ctx, "Verify parameters: [%v] failed. \nparameters must be provided",
			config["parameters"])
	}

	_, _, err := verifyProtocolAndPortals(parameters, constants.OceanStorNas)
	if err != nil {
		return pkgUtils.Errorf(ctx, "check nas parameter failed, err: %v", err)
	}

	return nil
}

// Validate used to validate OceanstorNasPlugin parameters
func (p *OceanstorNasPlugin) Validate(ctx context.Context, param map[string]interface{}) error {
	log.AddContext(ctx).Infoln("Start to validate OceanstorNasPlugin parameters.")

	err := p.verifyOceanstorNasParam(ctx, param)
	if err != nil {
		return err
	}

	clientConfig, err := getNewClientConfig(ctx, param)
	if err != nil {
		return err
	}

	// Login verification
	cli, err := client.NewClient(ctx, clientConfig)
	if err != nil {
		return err
	}

	err = cli.ValidateLogin(ctx)
	if err != nil {
		return err
	}
	cli.Logout(ctx)

	return nil
}

// DeleteDTreeVolume used to delete DTree volume
func (p *OceanstorNasPlugin) DeleteDTreeVolume(_ context.Context, _ string, _ string) error {
	return errors.New("fusion storage does not support DTree feature")
}

// ExpandDTreeVolume used to expand DTree volume
func (p *OceanstorNasPlugin) ExpandDTreeVolume(context.Context, string, string, int64) (bool, error) {
	return false, errors.New("fusion storage does not support DTree feature")
}

// ModifyVolume used to modify volume hyperMetro status
func (p *OceanstorNasPlugin) ModifyVolume(ctx context.Context, VolumeId string, modifyType pkgVolume.ModifyVolumeType,
	param map[string]string) error {

	log.AddContext(ctx).Infof("ModifyVolume, VolumeId: %s, param: %v", VolumeId, param)
	if err := p.canModify(); err != nil {
		log.AddContext(ctx).Infof("volume can't modify, cause by: %v", err)
		return err
	}

	if param == nil {
		param = make(map[string]string)
	}
	param["vStorePairID"] = p.vStorePairID

	var err error
	if modifyType == pkgVolume.Local2HyperMetro {
		err = p.modifyVolumeFromLocalToHyperMetro(ctx, VolumeId, param)
	} else if modifyType == pkgVolume.HyperMetro2Local {
		err = p.modifyVolumeFromHyperMetroToLocal(ctx, VolumeId, param)
	} else {
		errMsg := fmt.Sprintf("wrong modifyType: %v", modifyType)
		log.AddContext(ctx).Errorln(errMsg)
		return errors.New(errMsg)
	}

	return err
}

func (p *OceanstorNasPlugin) canModify() error {
	if p.metroRemotePlugin == nil || p.metroRemotePlugin.cli == nil {
		return fmt.Errorf("metro plugin not exist")
	}

	if p.cli.GetCurrentSiteWwn() == "" || p.metroRemotePlugin.cli.GetCurrentSiteWwn() == "" {
		return fmt.Errorf("backend: [%s] or [%s] wwn is empty, can't modify volume", p.name,
			p.metroRemotePlugin.name)
	}

	if p.cli.GetCurrentSiteWwn() == p.metroRemotePlugin.cli.GetCurrentSiteWwn() {
		return fmt.Errorf("backend: [%s] and [%s] is running on the same storage device", p.name,
			p.metroRemotePlugin.name)
	}

	if !p.isLogicPortRunningOnOwnSite() {
		return fmt.Errorf("local backend: [%s] is not running on own storage device", p.name)
	}

	if !p.metroRemotePlugin.isLogicPortRunningOnOwnSite() {
		return fmt.Errorf("metro backend: [%s] is not running on own storage device", p.metroRemotePlugin.name)
	}
	return nil
}

// GetLocal2HyperMetroParameters used to get local -> hyperMetro parameters
func (p *OceanstorNasPlugin) GetLocal2HyperMetroParameters(ctx context.Context, VolumeId string,
	parameters map[string]string) (map[string]interface{}, error) {

	// init param, SC parameters fill here after conversion.
	param := pkgUtils.ConvertMapString2MapInterface(parameters)
	param["hyperMetro"] = "true"

	backendName, volumeName := utils.SplitVolumeId(VolumeId)
	param["backend"] = helper.GetBackendName(backendName)

	ret := map[string]any{
		"name":     volumeName,
		"vstoreId": "0",
	}

	toLowerParams(param, ret)
	processBoolParams(ctx, param, ret)
	ret[creator.ModifyVolumeKey] = true
	log.AddContext(ctx).Infof("getParams finish, parameters: %v", ret)
	return ret, nil
}

func (p *OceanstorNasPlugin) modifyVolumeFromLocalToHyperMetro(ctx context.Context, VolumeId string,
	parameters map[string]string) error {
	log.AddContext(ctx).Infoln("modifyVolumeFromLocalToHyperMetro begin.")

	// get params
	param, err := p.GetLocal2HyperMetroParameters(ctx, VolumeId, parameters)
	if err != nil {
		errMsg := fmt.Sprintf("GetLocal2HyperMetroParameters failed, error: %v", err)
		log.AddContext(ctx).Errorln(errMsg)
		return err
	}
	log.AddContext(ctx).Infof("param: %v", param)

	// create hyperMetro fs
	nas := p.getNasObj()
	_, err = nas.Modify(ctx, param)
	if err != nil {
		errMsg := fmt.Sprintf("Create volume failed, volumeID: %v, error: %v.", VolumeId, err)
		log.AddContext(ctx).Errorln(errMsg)
		return err
	}

	log.AddContext(ctx).Infof("modify volume: %s from local to hyperMetro volume success.", VolumeId)
	return nil
}

func (p *OceanstorNasPlugin) deleteHyperMetroPair(ctx context.Context, VolumeId string) error {
	// use active cli to delete pair
	nas := p.getNasObj()
	activeCli := nas.GetActiveHyperMetroCli()
	_, volumeName := utils.SplitVolumeId(VolumeId)
	fsInfo, err := activeCli.GetFileSystemByName(ctx, volumeName)
	if err != nil {
		return fmt.Errorf("get filesystem by name failed, volumeName: %v, error: %w", volumeName, err)
	}

	if fsInfo["HYPERMETROPAIRIDS"] != nil {
		var hyperMetroIDs []string
		hyperMetroIdBytes := []byte(fsInfo["HYPERMETROPAIRIDS"].(string))
		err = json.Unmarshal(hyperMetroIdBytes, &hyperMetroIDs)
		if err != nil {
			return fmt.Errorf("unmarshal hyperMetroIdBytes failed, error: %w", err)
		}

		log.AddContext(ctx).Infof("Delete HyperMetro pair: %v, len(hyperMetroIDs): %d",
			hyperMetroIDs, len(hyperMetroIDs))
		if len(hyperMetroIDs) > 0 {
			_, err = nas.DeleteHyperMetro(ctx,
				map[string]interface{}{"hypermetroIDs": hyperMetroIDs},
				map[string]interface{}{"activeClient": activeCli})
			if err != nil {
				return fmt.Errorf("delete hypermetro pair failed, error: %w", err)
			}
		}
	}

	log.AddContext(ctx).Infoln("deleteHyperMetroPair success.")
	return nil
}

func (p *OceanstorNasPlugin) deleteRemoteFilesystem(ctx context.Context, VolumeId string) error {
	// No matter what the upper layer delivers, only the remote volume is deleted.
	if p.metroRemotePlugin == nil || p.metroRemotePlugin.cli == nil {
		return fmt.Errorf("p.metroRemotePlugin or p.metroRemotePlugin.cli is nil")
	}

	_, volumeName := utils.SplitVolumeId(VolumeId)
	remoteFsInfo, err := p.metroRemotePlugin.cli.GetFileSystemByName(ctx, volumeName)
	if err != nil {
		return fmt.Errorf("get filesystem by name failed, volumeName: %v, error: %w", volumeName, err)
	}
	if remoteFsInfo != nil {
		// 1. Delete share
		vStoreId, ok := remoteFsInfo["vstoreId"].(string)
		if !ok {
			vStoreId = SystemVStore
		}

		nas := p.getNasObj()
		err = nas.SafeDeleteShare(ctx, volumeName, vStoreId, p.metroRemotePlugin.cli)
		if err != nil {
			return fmt.Errorf("DeleteShare failed, volumeName: %s, vStoreId: %s, error: %w",
				volumeName, vStoreId, err)
		}

		// 2. Delete fs
		deleteParams := map[string]interface{}{"ID": remoteFsInfo["ID"], "vstoreId": remoteFsInfo["vstoreId"]}
		err = p.metroRemotePlugin.cli.SafeDeleteFileSystem(ctx, deleteParams)
		if err != nil {
			return fmt.Errorf("use backend [%s] deleteFileSystem failed, error: %w", p.metroRemotePlugin.name, err)
		}
	}

	log.AddContext(ctx).Infoln("deleteRemoteFilesystem success.")
	return nil
}

func (p *OceanstorNasPlugin) modifyVolumeFromHyperMetroToLocal(ctx context.Context, VolumeId string,
	parameters map[string]string) error {
	log.AddContext(ctx).Infoln("modifyVolumeFromHyperMetroToLocal begin.")

	err := p.deleteHyperMetroPair(ctx, VolumeId)
	if err != nil {
		return pkgUtils.Errorf(ctx, "deleteHyperMetroPair failed, error: %v", err)
	}

	err = p.deleteRemoteFilesystem(ctx, VolumeId)
	if err != nil {
		return pkgUtils.Errorf(ctx, "deleteRemoteFilesystem failed, error: %v", err)
	}

	log.AddContext(ctx).Infof("modify volume: %s from hyperMetro to local volume success.", VolumeId)
	return nil
}

func (p *OceanstorNasPlugin) assertLogicPortRunOnOwnSite(ctx context.Context) error {
	log.AddContext(ctx).Debugf("currentLifWwn: %s, currentSiteWwn: %s",
		p.cli.GetCurrentLifWwn(), p.cli.GetCurrentSiteWwn())

	if p.cli.GetCurrentLifWwn() == "" || p.cli.GetCurrentSiteWwn() == "" {
		return nil
	}

	if p.cli.GetCurrentLifWwn() != p.cli.GetCurrentSiteWwn() {
		return fmt.Errorf("logic port [%s] is not running on own site, currentSiteWwn: %s, currentLifWwn: %s",
			p.cli.GetCurrentLif(ctx), p.cli.GetCurrentSiteWwn(), p.cli.GetCurrentLifWwn())
	}

	return nil
}

func (p *OceanstorNasPlugin) isLogicPortRunningOnOwnSite() bool {
	// The homeSiteWwn of logical port only has value when it belongs to the hyper metro vStore.
	// so if the value of wwn is empty, the logical port is running on its own site by default.
	if p.cli == nil {
		return false
	}

	if p.cli.GetCurrentLifWwn() == "" || p.cli.GetCurrentSiteWwn() == "" {
		return true
	}

	return p.cli.GetCurrentLifWwn() == p.cli.GetCurrentSiteWwn()
}
