/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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
	"errors"
	"fmt"
	"net"
	"strconv"

	xuanwuV1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/pkg/constants"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/storage/oceanstor/volume"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
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

var supportConsistentSnapshotsVersions = []string{"6.1.6"}

// OceanstorNasPlugin implements storage Plugin interface
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
func (p *OceanstorNasPlugin) NewPlugin() Plugin {
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
	protocol, p.portals, err = verifyProtocolAndPortals(parameters)
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

	if protocol == ProtocolNfsPlus && p.cli.GetStorageVersion() < constants.MinVersionSupportLabel {
		return errors.New("only oceanstor nas version gte 6.1.7 support nfs_plus")
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
	var metroRemoteCli client.BaseClientInterface
	var replicaRemoteCli client.BaseClientInterface

	if p.metroRemotePlugin != nil {
		metroRemoteCli = p.metroRemotePlugin.cli
	}
	if p.replicaRemotePlugin != nil {
		replicaRemoteCli = p.replicaRemotePlugin.cli
	}

	return volume.NewNAS(p.cli, metroRemoteCli, replicaRemoteCli, p.product, p.nasHyperMetro)
}

// CreateVolume used to create volume
func (p *OceanstorNasPlugin) CreateVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	size, ok := parameters["size"].(int64)
	if !ok || !utils.IsCapacityAvailable(size, SectorSize) {
		msg := fmt.Sprintf("Create Volume: the capacity %d is not an integer multiple of 512.", size)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	params := p.getParams(ctx, name, parameters)
	params["metroDomainID"] = p.metroDomainID
	nas := p.getNasObj()
	volObj, err := nas.Create(ctx, params)
	if err != nil {
		return nil, err
	}

	return volObj, nil
}

func (p *OceanstorNasPlugin) getClient() (client.BaseClientInterface, client.BaseClientInterface) {
	var replicaRemoteCli client.BaseClientInterface
	if p.replicaRemotePlugin != nil {
		replicaRemoteCli = p.replicaRemotePlugin.cli
	}
	return p.cli, replicaRemoteCli
}

// QueryVolume used to query volume
func (p *OceanstorNasPlugin) QueryVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	params := p.getParams(ctx, name, parameters)
	nas := p.getNasObj()
	return nas.Query(ctx, name, params)
}

// DeleteVolume used to delete volume
func (p *OceanstorNasPlugin) DeleteVolume(ctx context.Context, name string) error {
	nas := p.getNasObj()
	return nas.Delete(ctx, name)
}

// ExpandVolume used to expand volume
func (p *OceanstorNasPlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	if !utils.IsCapacityAvailable(size, SectorSize) {
		msg := fmt.Sprintf("Expand Volume: the capacity %d is not an integer multiple of 512.", size)
		log.AddContext(ctx).Errorln(msg)
		return false, errors.New(msg)
	}
	newSize := utils.TransVolumeCapacity(size, SectorSize)
	nas := p.getNasObj()
	return false, nas.Expand(ctx, name, newSize)
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
	if p.product != constants.OceanStorDoradoV6 || p.cli.GetvStoreName() == "" ||
		p.cli.GetStorageVersion() < constants.DoradoV615 {
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
		nasCapacityQuota, err = strconv.ParseInt(totalStr, 10, 64)
	}
	if freeStr, ok := vStore["nasFreeCapacityQuota"].(string); ok {
		nasFreeCapacityQuota, err = strconv.ParseInt(freeStr, 10, 64)
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
		string(xuanwuV1.FreeCapacity):  nasFreeCapacityQuota * 512,
		string(xuanwuV1.TotalCapacity): nasCapacityQuota * 512,
		string(xuanwuV1.UsedCapacity):  (nasCapacityQuota - nasFreeCapacityQuota) * 512,
	}, nil
}

// UpdateMetroRemotePlugin used to convert metroRemotePlugin to OceanstorSanPlugin
func (p *OceanstorNasPlugin) UpdateMetroRemotePlugin(ctx context.Context, remote Plugin) {
	var ok bool
	p.metroRemotePlugin, ok = remote.(*OceanstorNasPlugin)
	if !ok {
		log.AddContext(ctx).Warningf("convert metroRemotePlugin to OceanstorNasPlugin failed, data: %v", remote)
	}
}

// CreateSnapshot used to create snapshot
func (p *OceanstorNasPlugin) CreateSnapshot(ctx context.Context,
	fsName, snapshotName string) (map[string]interface{}, error) {
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

func (p *OceanstorNasPlugin) updateHyperMetroCapability(ctx context.Context,
	capabilities map[string]interface{}) error {
	if p.product == "DoradoV6" {
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
	if p.product == "DoradoV6" && vStorePair != nil {
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
	p.UpdateRemoteCapabilities(ctx, capabilities)
	return nil
}

func (p *OceanstorNasPlugin) updateConsistentSnapshotCapability(
	capabilities, specifications map[string]interface{}) error {
	var supportConsistentSnapshot bool
	if utils.StringContain(p.cli.GetStorageVersion(), supportConsistentSnapshotsVersions) {
		supportConsistentSnapshot = true
		specifications["ConsistentSnapshotLimits"] = ConsistentSnapshotsSpecification
	}
	capabilities["SupportConsistentSnapshot"] = supportConsistentSnapshot
	return nil
}

func (p *OceanstorNasPlugin) updateReplicationCapability(capabilities map[string]interface{}) error {
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

	if !nfsServiceSetting["SupportNFS3"] {
		capabilities["SupportNFS3"] = false
	}

	if nfsServiceSetting["SupportNFS4"] {
		capabilities["SupportNFS4"] = true
	}

	if nfsServiceSetting["SupportNFS41"] {
		capabilities["SupportNFS41"] = true
	}

	return nil
}

// UpdateRemoteCapabilities used to update remote storage capabilities
func (p *OceanstorNasPlugin) UpdateRemoteCapabilities(ctx context.Context, capabilities map[string]interface{}) {
	// update the hyperMetro remote backend capabilities
	if p.metroRemotePlugin == nil || p.metroRemotePlugin.cli == nil {
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
		return pkgUtils.Errorf(ctx, "Verify parameters: [%v] failed. \nparameters must be provided", config["parameters"])
	}

	_, _, err := verifyProtocolAndPortals(parameters)
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

	clientConfig, err := p.getNewClientConfig(ctx, param)
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
func (p *OceanstorNasPlugin) DeleteDTreeVolume(ctx context.Context, m map[string]interface{}) error {
	return errors.New("not implement")
}

// ExpandDTreeVolume used to expand DTree volume
func (p *OceanstorNasPlugin) ExpandDTreeVolume(ctx context.Context, m map[string]interface{}) (bool, error) {
	return false, errors.New("not implement")
}
