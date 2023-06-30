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

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/storage/oceanstor/volume"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	HYPER_METRO_VSTORE_PAIR_ACTIVE                = "0"
	HYPER_METRO_VSTORE_PAIR_LINK_STATUS_CONNECTED = "1"
	HYPER_METRO_DOMAIN_ACTIVE                     = "1"
	HYPER_METRO_DOMAIN_RUNNING_STATUS_NORMAL      = "0"

	ConsistentSnapshotsSpecification = "128"
)

var supportConsistentSnapshotsVersions = []string{"6.1.6"}

type OceanstorNasPlugin struct {
	OceanstorPlugin
	portal        string
	vStorePairID  string
	metroDomainID string

	nasHyperMetro       volume.NASHyperMetro
	metroRemotePlugin   *OceanstorNasPlugin
	replicaRemotePlugin *OceanstorNasPlugin
}

func init() {
	RegPlugin("oceanstor-nas", &OceanstorNasPlugin{})
}

func (p *OceanstorNasPlugin) NewPlugin() Plugin {
	return &OceanstorNasPlugin{}
}

func (p *OceanstorNasPlugin) Init(config, parameters map[string]interface{}, keepLogin bool) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || protocol != "nfs" {
		return errors.New("protocol must be provided and be \"nfs\" for oceanstor-nas backend")
	}

	portals, exist := parameters["portals"].([]interface{})
	if !exist || len(portals) != 1 {
		return errors.New("portals must be provided for oceanstor-nas backend and just support one portal")
	}

	err := p.init(config, keepLogin)
	if err != nil {
		return err
	}

	p.portal = portals[0].(string)
	p.vStorePairID, exist = config["metrovStorePairID"].(string)
	if exist {
		log.Infof("The metro vStorePair ID is %s", p.vStorePairID)
	}

	return nil
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

func (p *OceanstorNasPlugin) QueryVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	params := p.getParams(ctx, name, parameters)
	nas := p.getNasObj()
	return nas.Query(ctx, name, params)
}

func (p *OceanstorNasPlugin) DeleteVolume(ctx context.Context, name string) error {
	nas := p.getNasObj()
	return nas.Delete(ctx, name)
}

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

func (p *OceanstorNasPlugin) UpdatePoolCapabilities(poolNames []string) (map[string]interface{}, error) {
	return p.updatePoolCapabilities(poolNames, "2")
}

func (p *OceanstorNasPlugin) UpdateReplicaRemotePlugin(remote Plugin) {
	p.replicaRemotePlugin = remote.(*OceanstorNasPlugin)
}

func (p *OceanstorNasPlugin) UpdateMetroRemotePlugin(remote Plugin) {
	p.metroRemotePlugin = remote.(*OceanstorNasPlugin)
}

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

func (p *OceanstorNasPlugin) DeleteSnapshot(ctx context.Context, snapshotParentId, snapshotName string) error {
	nas := p.getNasObj()

	snapshotName = utils.GetFSSnapshotName(snapshotName)
	err := nas.DeleteSnapshot(ctx, snapshotParentId, snapshotName)
	if err != nil {
		return err
	}

	return nil
}

func (p *OceanstorNasPlugin) UpdateBackendCapabilities() (map[string]interface{}, map[string]interface{}, error) {
	capabilities, specifications, err := p.OceanstorPlugin.UpdateBackendCapabilities()
	if err != nil {
		return nil, nil, err
	}

	err = p.updateHyperMetroCapability(capabilities)
	if err != nil {
		return nil, nil, err
	}

	err = p.updateReplicationCapability(capabilities)
	if err != nil {
		return nil, nil, err
	}

	err = p.updateNFS4Capability(capabilities)
	if err != nil {
		return nil, nil, err
	}

	// update the SupportConsistentSnapshot capability and specification
	err = p.updateConsistentSnapshotCapability(capabilities, specifications)
	if err != nil {
		return nil, nil, err
	}

	return capabilities, specifications, nil
}

func (p *OceanstorNasPlugin) updateHyperMetroCapability(capabilities map[string]interface{}) error {
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

	vStorePair, err := p.cli.GetvStorePairByID(context.Background(), p.vStorePairID)
	if err != nil {
		return err
	}

	if p.product == "DoradoV6" && vStorePair != nil {
		fsHyperMetroDomain, err := p.cli.GetFSHyperMetroDomain(context.Background(),
			vStorePair["DOMAINNAME"].(string))
		if err != nil {
			return err
		}

		if fsHyperMetroDomain == nil ||
			fsHyperMetroDomain["RUNNINGSTATUS"] != HYPER_METRO_DOMAIN_RUNNING_STATUS_NORMAL {
			capabilities["SupportMetro"] = false
			return nil
		}

		p.nasHyperMetro = volume.NASHyperMetro{
			FsHyperMetroActiveSite: fsHyperMetroDomain["CONFIGROLE"] == HYPER_METRO_DOMAIN_ACTIVE,
			LocVStoreID:            vStorePair["LOCALVSTOREID"].(string),
			RmtVStoreID:            vStorePair["REMOTEVSTOREID"].(string),
		}
		p.metroDomainID = vStorePair["DOMAINID"].(string)
	} else {
		if vStorePair == nil ||
			vStorePair["ACTIVEORPASSIVE"] != HYPER_METRO_VSTORE_PAIR_ACTIVE ||
			vStorePair["LINKSTATUS"] != HYPER_METRO_VSTORE_PAIR_LINK_STATUS_CONNECTED ||
			vStorePair["LOCALVSTORENAME"] != p.cli.GetvStoreName() {
			capabilities["SupportMetro"] = false
		}
	}
	p.UpdateRemoteCapabilities(capabilities)
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

func (p *OceanstorNasPlugin) updateNFS4Capability(capabilities map[string]interface{}) error {
	if capabilities == nil {
		capabilities = make(map[string]interface{})
	}

	nfsServiceSetting, err := p.cli.GetNFSServiceSetting(context.Background())
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

func (p *OceanstorNasPlugin) UpdateRemoteCapabilities(capabilities map[string]interface{}) {
	// update the hyperMetro remote backend capabilities
	if p.metroRemotePlugin == nil || p.metroRemotePlugin.cli == nil {
		capabilities["SupportMetro"] = false
		return
	}

	features, err := p.metroRemotePlugin.cli.GetLicenseFeature(context.Background())
	if err != nil {
		log.Warningf("Get license feature error: %v", err)
		capabilities["SupportMetro"] = false
		return
	}

	capabilities["SupportMetro"] = capabilities["SupportMetro"].(bool) &&
		utils.IsSupportFeature(features, "HyperMetroNAS")
}

func (p *OceanstorNasPlugin) verifyOceanstorNasParam(ctx context.Context, config map[string]interface{}) error {
	parameters, exist := config["parameters"].(map[string]interface{})
	if !exist {
		msg := fmt.Sprintf("Verify parameters: [%v] failed. \nparameters must be provided", config["parameters"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	// verify protocol portals
	protocol, exist := parameters["protocol"].(string)
	if !exist || protocol != "nfs" {
		msg := fmt.Sprintf("Verify protocol: [%v] failed. \nProtocol must be provided and must be \"nfs\" for "+
			"oceanstor-nas backend\n", parameters["protocol"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	portals, exist := parameters["portals"].([]interface{})
	if !exist || len(portals) != 1 {
		msg := fmt.Sprintf("Verify portals: [%v] failed. \nportals must be provided for oceanstor-nas backend "+
			"and just support one portal\n", parameters["portals"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

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
	cli := client.NewClient(clientConfig)
	err = cli.ValidateLogin(ctx)
	if err != nil {
		return err
	}
	cli.Logout(ctx)

	return nil
}

func (p *OceanstorNasPlugin) DeleteDTreeVolume(ctx context.Context, m map[string]interface{}) error {
	return errors.New("not implement")
}

func (p *OceanstorNasPlugin) ExpandDTreeVolume(ctx context.Context, m map[string]interface{}) (bool, error) {
	return false, errors.New("not implement")
}
