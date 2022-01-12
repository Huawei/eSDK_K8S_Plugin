package plugin

import (
	"errors"
	"fmt"
	"storage/oceanstor/client"
	"storage/oceanstor/volume"
	"utils"
	"utils/log"
)

const (
	HYPER_METRO_VSTORE_PAIR_ACTIVE                = "0"
	HYPER_METRO_VSTORE_PAIR_LINK_STATUS_CONNECTED = "1"
	HYPER_METRO_DOMAIN_ACTIVE                     = "1"
	HYPER_METRO_DOMAIN_RUNNING_STATUS_NORMAL      = "0"
)

type OceanstorNasPlugin struct {
	OceanstorPlugin
	portal       string
	vStorePairID string

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
		return errors.New("protocol must be provided and be nfs for oceanstor-nas backend")
	}

	portals, exist := parameters["portals"].([]interface{})
	if !exist || len(portals) == 0 {
		return errors.New("portals must be provided for oceanstor-nas backend")
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

	return volume.NewNAS(p.cli, metroRemoteCli, replicaRemoteCli, p.product)
}

func (p *OceanstorNasPlugin) CreateVolume(name string, parameters map[string]interface{}) (utils.Volume, error) {
	size, ok := parameters["size"].(int64)
	if !ok || !utils.IsCapacityAvailable(size, SectorSize) {
		msg := fmt.Sprintf("Create Volume: the capacity %d is not an integer multiple of 512.", size)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	params := p.getParams(name, parameters)
	nas := p.getNasObj()
	volObj, err := nas.Create(params)
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

func (p *OceanstorNasPlugin) DeleteVolume(name string) error {
	nas := p.getNasObj()
	return nas.Delete(name)
}

func (p *OceanstorNasPlugin) ExpandVolume(name string, size int64) (bool, error) {
	if !utils.IsCapacityAvailable(size, SectorSize) {
		msg := fmt.Sprintf("Expand Volume: the capacity %d is not an integer multiple of 512.", size)
		log.Errorln(msg)
		return false, errors.New(msg)
	}
	newSize := utils.TransVolumeCapacity(size, SectorSize)
	nas := p.getNasObj()
	return false, nas.Expand(name, newSize)
}

func (p *OceanstorNasPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	return p.fsStageVolume(name, p.portal, parameters)
}

func (p *OceanstorNasPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	return p.unstageVolume(name, parameters)
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

func (p *OceanstorNasPlugin) NodeExpandVolume(string, string) error {
	return nil
}

func (p *OceanstorNasPlugin) CreateSnapshot(fsName, snapshotName string) (map[string]interface{}, error) {
	nas := p.getNasObj()

	snapshotName = utils.GetFSSnapshotName(snapshotName)
	snapshot, err := nas.CreateSnapshot(fsName, snapshotName)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (p *OceanstorNasPlugin) DeleteSnapshot(snapshotParentId, snapshotName string) error {
	nas := p.getNasObj()

	snapshotName = utils.GetFSSnapshotName(snapshotName)
	err := nas.DeleteSnapshot(snapshotParentId, snapshotName)
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

	return capabilities, nil
}

func (p *OceanstorNasPlugin) updateHyperMetroCapability(capabilities map[string]interface{}) error {
	if capabilities["SupportMetro"] != true {
		return nil
	}

	if p.metroRemotePlugin == nil || p.vStorePairID == "" {
		capabilities["SupportMetro"] = false
		return nil
	}

	vStorePair, err := p.cli.GetvStorePairByID(p.vStorePairID)
	if err != nil {
		return err
	}

	if p.product == "DoradoV6" && vStorePair != nil {
		fsHyperMetroDomain, err := p.cli.GetFSHyperMetroDomain(vStorePair["DOMAINNAME"].(string))
		if err != nil {
			return err
		}

		if fsHyperMetroDomain == nil ||
			fsHyperMetroDomain["CONFIGROLE"] != HYPER_METRO_DOMAIN_ACTIVE ||
			fsHyperMetroDomain["RUNNINGSTATUS"] != HYPER_METRO_DOMAIN_RUNNING_STATUS_NORMAL ||
			vStorePair["LOCALVSTORENAME"] != p.cli.GetvStoreName() {
			capabilities["SupportMetro"] = false
		}
	} else {
		if vStorePair == nil ||
			vStorePair["ACTIVEORPASSIVE"] != HYPER_METRO_VSTORE_PAIR_ACTIVE ||
			vStorePair["LINKSTATUS"] != HYPER_METRO_VSTORE_PAIR_LINK_STATUS_CONNECTED ||
			vStorePair["LOCALVSTORENAME"] != p.cli.GetvStoreName() {
			capabilities["SupportMetro"] = false
		}
	}

	return nil
}

func (p *OceanstorNasPlugin) updateReplicationCapability(capabilities map[string]interface{}) error {
	if capabilities["SupportReplication"] == true && p.replicaRemotePlugin == nil {
		capabilities["SupportReplication"] = false
	}
	return nil
}

