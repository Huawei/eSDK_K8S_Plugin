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
)

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
	var metroRemoteCli *client.Client
	var replicaRemoteCli *client.Client

	if p.metroRemotePlugin != nil {
		metroRemoteCli = p.metroRemotePlugin.cli
	}
	if p.replicaRemotePlugin != nil {
		replicaRemoteCli = p.replicaRemotePlugin.cli
	}

	return volume.NewNAS(p.cli, metroRemoteCli, replicaRemoteCli, p.product, p.nasHyperMetro)
}

func (p *OceanstorNasPlugin) CreateVolume(ctx context.Context,
	name string,
	parameters map[string]interface{}) (utils.Volume, error) {
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

func (p *OceanstorNasPlugin) getClient() (*client.Client, *client.Client) {
	var replicaRemoteCli *client.Client
	if p.replicaRemotePlugin != nil {
		replicaRemoteCli = p.replicaRemotePlugin.cli
	}
	return p.cli, replicaRemoteCli
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

func (p *OceanstorNasPlugin) StageVolume(ctx context.Context,
	name string,
	parameters map[string]interface{}) error {
	return p.fsStageVolume(ctx, name, p.portal, parameters)
}

func (p *OceanstorNasPlugin) UnstageVolume(ctx context.Context,
	name string,
	parameters map[string]interface{}) error {
	return p.unstageVolume(ctx, name, parameters)
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

func (p *OceanstorNasPlugin) NodeExpandVolume(context.Context, string, string, bool, int64) error {
	return nil
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

func (p *OceanstorNasPlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	capabilities, err := p.OceanstorPlugin.UpdateBackendCapabilities()
	if err != nil {
		return nil, err
	}

	err = p.updateHyperMetroCapability(capabilities)
	if err != nil {
		return nil, err
	}

	err = p.updateReplicationCapability(capabilities)
	if err != nil {
		return nil, err
	}

	err = p.updateNFS4Capability(capabilities)
	if err != nil {
		return nil, err
	}

	return capabilities, nil
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

func (p *OceanstorNasPlugin) updateReplicationCapability(capabilities map[string]interface{}) error {
	if capabilities["SupportReplication"] == true && p.replicaRemotePlugin == nil {
		capabilities["SupportReplication"] = false
	}
	return nil
}

func (p *OceanstorNasPlugin) updateNFS4Capability(capabilities map[string]interface{}) error {
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
