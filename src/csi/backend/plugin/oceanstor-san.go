package plugin

import (
	"connector"
	"encoding/json"
	"errors"
	"fmt"
	"proto"
	"reflect"
	"storage/oceanstor/attacher"
	"storage/oceanstor/client"
	"storage/oceanstor/volume"
	"utils"
	"utils/log"
)

const (
	HYPERMETROPAIR_RUNNING_STATUS_NORMAL = "1"
	HYPERMETROPAIR_RUNNING_STATUS_PAUSE  = "41"
)

type OceanstorSanPlugin struct {
	OceanstorPlugin
	protocol string
	portals  []string
	alua     map[string]interface{}

	replicaRemotePlugin *OceanstorSanPlugin
	metroRemotePlugin   *OceanstorSanPlugin
	locStorageOnline    bool
	rmtStorageOnline    bool
}

func init() {
	RegPlugin("oceanstor-san", &OceanstorSanPlugin{})
}

func (p *OceanstorSanPlugin) NewPlugin() Plugin {
	return &OceanstorSanPlugin{}
}

func (p *OceanstorSanPlugin) Init(config, parameters map[string]interface{}, keepLogin bool) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "iscsi" && protocol != "fc" && protocol != "roce" && protocol != "fc-nvme") {
		return errors.New("protocol must be provided as 'iscsi', 'fc', 'roce' or 'fc-nvme' for oceanstor-san backend")
	}

	p.alua, _ = parameters["ALUA"].(map[string]interface{})

	if protocol == "iscsi" || protocol == "roce" {
		portals, exist := parameters["portals"].([]interface{})
		if !exist {
			return errors.New("portals are required to configure for iSCSI or RoCE backend")
		}

		IPs, err := proto.VerifyIscsiPortals(portals)
		if err != nil {
			return err
		}

		p.portals = IPs
	}

	err := p.init(config, keepLogin)
	if err != nil {
		return err
	}

	if (protocol == "roce" || protocol == "fc-nvme") && p.product != "DoradoV6" {
		msg := fmt.Sprintf("The storage backend %s does not support NVME protocol", p.product)
		log.Errorln(msg)
		return errors.New(msg)
	}

	p.protocol = protocol
	p.locStorageOnline = true

	return nil
}

func (p *OceanstorSanPlugin) getSanObj() *volume.SAN {
	var metroRemoteCli *client.Client
	var replicaRemoteCli *client.Client

	if p.metroRemotePlugin != nil {
		metroRemoteCli = p.metroRemotePlugin.cli
	}
	if p.replicaRemotePlugin != nil {
		replicaRemoteCli = p.replicaRemotePlugin.cli
	}

	return volume.NewSAN(p.cli, metroRemoteCli, replicaRemoteCli, p.product)
}

func (p *OceanstorSanPlugin) CreateVolume(name string, parameters map[string]interface{}) (string, error) {
	params := p.getParams(name, parameters)
	san := p.getSanObj()

	err := san.Create(params)
	if err != nil {
		return "", err
	}

	return params["name"].(string), nil
}

func (p *OceanstorSanPlugin) DeleteVolume(name string) error {
	san := p.getSanObj()
	return san.Delete(name)
}

func (p *OceanstorSanPlugin) ExpandVolume(name string, size int64) (bool, error) {
	san := p.getSanObj()
	newSize := utils.TransVolumeCapacity(size, 512)
	isAttach, err := san.Expand(name, newSize)
	return isAttach, err
}

func (p *OceanstorSanPlugin) isHyperMetro(lun map[string]interface{}) bool {
	var rss map[string]string
	rssStr := lun["HASRSSOBJECT"].(string)
	json.Unmarshal([]byte(rssStr), &rss)

	return rss["HyperMetro"] == "TRUE"
}

func (p *OceanstorSanPlugin) metroHandler(localCli, metroCli *client.Client, lun, parameters map[string]interface{},
	method string) ([]reflect.Value, error) {
	localLunID := lun["ID"].(string)
	pair, err := localCli.GetHyperMetroPairByLocalObjID(localLunID)
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return nil, fmt.Errorf("hypermetro pair of LUN %s doesn't exist", localLunID)
	}

	if method == "ControllerDetach" || method == "NodeUnstage" {
		if pair["RUNNINGSTATUS"] != HYPERMETROPAIR_RUNNING_STATUS_NORMAL &&
			pair["RUNNINGSTATUS"] != HYPERMETROPAIR_RUNNING_STATUS_PAUSE {
			log.Warningf("hypermetro pair status of LUN %s is not normal or pause", localLunID)
		}
	} else {
		if pair["RUNNINGSTATUS"] != HYPERMETROPAIR_RUNNING_STATUS_NORMAL {
			log.Warningf("hypermetro pair status of LUN %s is not normal", localLunID)
		}
	}

	localAttacher := attacher.NewAttacher(p.product, localCli, p.protocol, "csi", p.portals, p.alua)
	remoteAttacher := attacher.NewAttacher(p.metroRemotePlugin.product, metroCli, p.metroRemotePlugin.protocol,
		"csi", p.metroRemotePlugin.portals, p.metroRemotePlugin.alua)

	metroAttacher := attacher.NewMetroAttacher(localAttacher, remoteAttacher, p.protocol)
	lunName := lun["NAME"].(string)
	out := utils.ReflectCall(metroAttacher, method, lunName, parameters)

	return out, nil
}

func (p *OceanstorSanPlugin) localHandler(localCli *client.Client, lun, parameters map[string]interface{},
	method string) ([]reflect.Value, error) {
	var localAttacher attacher.AttacherPlugin
	if p.rmtStorageOnline {
		localAttacher = attacher.NewAttacher(p.metroRemotePlugin.product, localCli, p.metroRemotePlugin.protocol,
			"csi", p.metroRemotePlugin.portals, p.metroRemotePlugin.alua)
	} else {
		localAttacher = attacher.NewAttacher(p.product, localCli, p.protocol, "csi", p.portals, p.alua)
	}

	lunName := lun["NAME"].(string)
	out := utils.ReflectCall(localAttacher, method, lunName, parameters)
	return out, nil
}

func (p *OceanstorSanPlugin) handler(localCli, metroCli *client.Client, lun, parameters map[string]interface{},
	method string) ([]reflect.Value, error) {
	var out []reflect.Value
	var err error

	if !p.isHyperMetro(lun) {
		return p.localHandler(localCli, lun, parameters, method)
	}

	if p.locStorageOnline && p.rmtStorageOnline {
		out, err = p.metroHandler(localCli, metroCli, lun, parameters, method)
	} else if p.locStorageOnline {
		log.Warningf("the lun %s is hyperMetro, but just the local storage is online", lun["NAME"].(string))
		out, err = p.localHandler(localCli, lun, parameters, method)
	} else if p.rmtStorageOnline {
		log.Warningf("the lun %s is hyperMetro, but just the remote storage is online", lun["NAME"].(string))
		out, err = p.localHandler(metroCli, lun, parameters, method)
	}

	return out, err
}

func (p *OceanstorSanPlugin) DetachVolume(name string, parameters map[string]interface{}) error {
	cli, metroCli := p.getClient()
	if p.locStorageOnline {
		defer cli.Logout()
	}
	if p.rmtStorageOnline {
		defer metroCli.Logout()
	}

	lunName := utils.GetLunName(name)
	lun, err := p.getLunInfo(cli, metroCli, lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return err
	}
	if lun == nil {
		log.Warningf("LUN %s to detach doesn't exist", lunName)
		return nil
	}

	var out []reflect.Value

	out, err = p.handler(p.cli, metroCli, lun, parameters, "ControllerDetach")
	if err != nil {
		return err
	}
	if len(out) != 2 {
		return fmt.Errorf("detach volume %s error", lunName)
	}

	result := out[1].Interface()
	if result != nil {
		return result.(error)
	}

	return nil
}

func (p *OceanstorSanPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	cli, metroCli := p.getClient()
	if p.locStorageOnline {
		defer cli.Logout()
	}
	if p.rmtStorageOnline {
		defer metroCli.Logout()
	}

	lunName := utils.GetLunName(name)
	lun, err := p.getLunInfo(cli, metroCli, lunName)
	if err != nil {
		return err
	}
	if lun == nil {
		return fmt.Errorf("LUN %s to stage doesn't exist", lunName)
	}

	var out []reflect.Value
	out, err = p.handler(cli, metroCli, lun, parameters, "NodeStage")
	if err != nil {
		return err
	}
	if len(out) != 2 {
		return fmt.Errorf("stage volume %s error", lunName)
	}

	result := out[1].Interface()
	if result != nil {
		return result.(error)
	}

	// If the request to stage is for volumeDevice of type Block and the devicePath
	// is provided then do not format and create FS and mount it.
    // Simply create a symlink to the devpath on the staging area
	if parameters["volumeMode"].(string) == "Block" {
		log.Infof("The request to stage raw block device")
		mountpoint := parameters["stagingPath"].(string)
		devpath := out[0].Interface().(string)
		err := utils.CreateSymlink(devpath, mountpoint)
		if nil != err {
			log.Errorf("Error in staging device")
			return err
		}
	}
	return p.lunStageVolume(name, out[0].Interface().(string), parameters)
}

func (p *OceanstorSanPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	err := p.unstageVolume(name, parameters)
	if err != nil {
		return err
	}

	cli, metroCli := p.getClient()
	if p.locStorageOnline {
		defer cli.Logout()
	}
	if p.rmtStorageOnline {
		defer metroCli.Logout()
	}

	lunName := utils.GetLunName(name)
	lun, err := p.getLunInfo(cli, metroCli, lunName)
	if err != nil {
		return err
	}
	if lun == nil {
		return nil
	}

	var out []reflect.Value

	out, err = p.handler(cli, metroCli, lun, parameters, "NodeUnstage")
	if err != nil {
		return err
	}
	if len(out) != 1 {
		return fmt.Errorf("unstage volume %s error", lunName)
	}

	result := out[0].Interface()
	if result != nil {
		return result.(error)
	}

	return nil
}

func (p *OceanstorSanPlugin) UpdatePoolCapabilities(poolNames []string) (map[string]interface{}, error) {
	return p.updatePoolCapabilities(poolNames, "1")
}

func (p *OceanstorSanPlugin) UpdateReplicaRemotePlugin(remote Plugin) {
	p.replicaRemotePlugin = remote.(*OceanstorSanPlugin)
}

func (p *OceanstorSanPlugin) UpdateMetroRemotePlugin(remote Plugin) {
	p.metroRemotePlugin = remote.(*OceanstorSanPlugin)
}

func (p *OceanstorSanPlugin) NodeExpandVolume(name, volumePath string) error {
	cli := p.cli.DuplicateClient()
	err := cli.Login()
	if err != nil {
		return err
	}
	defer cli.Logout()

	lunName := utils.GetLunName(name)
	lun, err := cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return err
	}
	if lun == nil {
		msg := fmt.Sprintf("LUN %s to expand doesn't exist", lunName)
		log.Errorln(msg)
		return errors.New(msg)
	}

	lunUniqueId, err := utils.GetLunUniqueId(p.protocol, lun)
	if err != nil {
		return err
	}

	err = connector.ResizeBlock(lunUniqueId)
	if err != nil {
		log.Errorf("Lun %s resize error: %v", lunUniqueId, err)
		return err
	}

	err = connector.ResizeMountPath(volumePath)
	if err != nil {
		log.Errorf("MountPath %s resize error: %v", volumePath, err)
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) CreateSnapshot(lunName, snapshotName string) (map[string]interface{}, error) {
	san := p.getSanObj()

	snapshotName = utils.GetSnapshotName(snapshotName)
	snapshot, err := san.CreateSnapshot(lunName, snapshotName)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (p *OceanstorSanPlugin) DeleteSnapshot(snapshotParentId, snapshotName string) error {
	san := p.getSanObj()

	snapshotName = utils.GetSnapshotName(snapshotName)
	err := san.DeleteSnapshot(snapshotName)
	if err != nil {
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) getClient() (*client.Client, *client.Client) {
	cli, err := p.duplicateClient()
	p.locStorageOnline = err == nil

	var metroCli *client.Client
	if p.metroRemotePlugin != nil {
		metroCli, err = p.metroRemotePlugin.duplicateClient()
		p.rmtStorageOnline = err == nil
	}

	return cli, metroCli
}

func (p *OceanstorSanPlugin) getLunInfo(localCli, remoteCli *client.Client, lunName string) (map[string]interface{}, error) {
	var lun map[string]interface{}
	var err error
	if p.locStorageOnline {
		lun, err = localCli.GetLunByName(lunName)
	} else if p.rmtStorageOnline {
		lun, err = remoteCli.GetLunByName(lunName)
	} else {
		return nil, errors.New("both the local and remote storage are not online")
	}

	return lun, err
}

// UpdateBackendCapabilities to update the block storage capabilities
func (p *OceanstorSanPlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	capabilities, err := p.OceanstorPlugin.UpdateBackendCapabilities()
	if err != nil {
		p.locStorageOnline = false
		return nil, err
	}

	p.locStorageOnline = true
	p.updateHyperMetroCapability(capabilities)
	p.updateReplicaCapability(capabilities)
	return capabilities, nil
}

func (p *OceanstorSanPlugin) updateHyperMetroCapability(capabilities map[string]interface{}) {
	if metroSupport, exist := capabilities["SupportMetro"]; !exist || metroSupport == false {
		return
	}

	capabilities["SupportMetro"] = p.metroRemotePlugin != nil &&
		p.locStorageOnline && p.metroRemotePlugin.locStorageOnline
}

func (p *OceanstorSanPlugin) updateReplicaCapability(capabilities map[string]interface{}) {
	if metroReplica, exist := capabilities["SupportReplication"]; !exist || metroReplica == false {
		return
	}

	capabilities["SupportReplication"] = p.replicaRemotePlugin != nil
}
