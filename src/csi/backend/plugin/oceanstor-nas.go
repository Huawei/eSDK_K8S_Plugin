package plugin

import (
	"errors"

	"github.com/Huawei/eSDK_K8S_Plugin/src/dev"
	"github.com/Huawei/eSDK_K8S_Plugin/src/storage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/src/storage/oceanstor/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
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
	portal, exist := parameters["portal"].(string)
	if !exist {
		return errors.New("portal must be provided for oceanstor-nas backend")
	}

	err := p.init(config, keepLogin)
	if err != nil {
		return err
	}

	p.portal = portal
	p.vStorePairID, _ = config["metrovStorePairID"].(string)

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

func (p *OceanstorNasPlugin) CreateVolume(name string, parameters map[string]interface{}) (string, error) {
	params := p.getParams(name, parameters)
	nas := p.getNasObj()

	err := nas.Create(params)
	if err != nil {
		return "", err
	}

	return params["name"].(string), nil
}

func (p *OceanstorNasPlugin) DeleteVolume(name string) error {
	nas := p.getNasObj()
	return nas.Delete(name)
}

func (p *OceanstorNasPlugin) ExpandVolume(name string, size int64) (bool, error) {
	newSize := utils.TransVolumeCapacity(size, 512)
	nas := p.getNasObj()
	return false, nas.Expand(name, newSize)
}

func (p *OceanstorNasPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	fsName := utils.GetFileSystemName(name)
	exportPath := p.portal + ":/" + fsName

	targetPath := parameters["targetPath"].(string)
	mountFlags := parameters["mountFlags"].(string)

	err := dev.MountFsDev(exportPath, targetPath, mountFlags)
	if err != nil {
		log.Errorf("Mount share %s to %s error: %v", exportPath, targetPath, err)
		return err
	}

	return nil
}

func (p *OceanstorNasPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	targetPath := parameters["targetPath"].(string)
	err := dev.Unmount(targetPath)
	if err != nil {
		log.Errorf("Unmount volume %s error: %v", name, err)
		return err
	}

	return nil
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

