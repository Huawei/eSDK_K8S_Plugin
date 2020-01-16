package plugin

import (
	"dev"
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
)

type OceanstorSanPlugin struct {
	OceanstorPlugin
	protocol string
	portals  []string

	metroRemotePlugin *OceanstorSanPlugin
}

func init() {
	RegPlugin("oceanstor-san", &OceanstorSanPlugin{})
}

func (p *OceanstorSanPlugin) NewPlugin() Plugin {
	return &OceanstorSanPlugin{}
}

func (p *OceanstorSanPlugin) Init(config, parameters map[string]interface{}) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "iscsi" && protocol != "fc") {
		return errors.New("protocol must be provided as 'iscsi' or 'fc' for oceanstor-san backend")
	}

	if protocol == "iscsi" {
		portals, exist := parameters["portals"].([]interface{})
		if !exist {
			return errors.New("portals are required to configure for ISCSI backend")
		}

		IPs, err := proto.VerifyIscsiPortals(portals)
		if err != nil {
			return err
		}

		p.portals = IPs
	}

	err := p.init(config)
	if err != nil {
		return err
	}

	p.protocol = protocol

	return nil
}

func (p *OceanstorSanPlugin) getMetroRemoteCli() *client.Client {
	var cli *client.Client
	if p.metroRemotePlugin != nil {
		cli = p.metroRemotePlugin.cli
	}

	return cli
}

func (p *OceanstorSanPlugin) CreateVolume(name string, parameters map[string]interface{}) (string, error) {
	params := p.getParams(name, parameters)

	metroRemoteCli := p.getMetroRemoteCli()
	san := volume.NewSAN(p.cli, metroRemoteCli)
	err := san.Create(params)
	if err != nil {
		return "", err
	}

	return params["name"].(string), nil
}

func (p *OceanstorSanPlugin) DeleteVolume(name string) error {
	metroRemoteCli := p.getMetroRemoteCli()
	san := volume.NewSAN(p.cli, metroRemoteCli)
	return san.Delete(name)
}

func (p *OceanstorSanPlugin) isHyperMetro(lun map[string]interface{}) bool {
	var rss map[string]string
	rssStr := lun["HASRSSOBJECT"].(string)
	json.Unmarshal([]byte(rssStr), &rss)

	return rss["HyperMetro"] == "TRUE"
}

func (p *OceanstorSanPlugin) attachDetachHandler(method string, lun, parameters map[string]interface{}) (out []reflect.Value) {
	lunName := lun["NAME"].(string)

	if p.isHyperMetro(lun) {
		if p.metroRemotePlugin == nil {
			log.Errorln("No metro remote cli exists for hypermetro")
			return
		}

		localLunID := lun["ID"].(string)
		pair, err := p.cli.GetHyperMetroPairByLocalObjID(localLunID)
		if err != nil {
			log.Errorf("Get hypermetro pair by local obj ID %s error: %v", localLunID, err)
			return
		}
		if pair == nil || pair["RUNNINGSTATUS"].(string) != HYPERMETROPAIR_RUNNING_STATUS_NORMAL {
			msg := "Hypermetro pair doesn't exist or status is not normal"
			log.Errorln(msg)
			return
		}

		localAttacher := attacher.NewAttacher(p.cli, p.protocol, "csi", p.portals)
		remoteAttcher := attacher.NewAttacher(
			p.metroRemotePlugin.cli,
			p.metroRemotePlugin.protocol,
			"csi",
			p.metroRemotePlugin.portals)

		metroAttacher := attacher.NewMetroAttacher(localAttacher, remoteAttcher)
		out = utils.ReflectCall(metroAttacher, method, lunName, parameters)
	} else {
		localAttacher := attacher.NewAttacher(p.cli, p.protocol, "csi", p.portals)
		out = utils.ReflectCall(localAttacher, method, lunName, parameters)
	}

	return out
}

func (p *OceanstorSanPlugin) AttachVolume(name string, parameters map[string]interface{}) error {
	return nil
}

func (p *OceanstorSanPlugin) DetachVolume(name string, parameters map[string]interface{}) error {
	lunName := utils.GetLunName(name)
	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return err
	}
	if lun == nil {
		log.Warningf("LUN %s to detach doesn't exist", lunName)
		return nil
	}

	out := p.attachDetachHandler("ControllerDetach", lun, parameters)
	if len(out) != 2 {
		msg := fmt.Sprintf("Detach volume %s error", lunName)
		log.Errorln(msg)
		return errors.New(msg)
	}

	result := out[1].Interface()
	if result != nil {
		err := result.(error)
		msg := fmt.Sprintf("Detach volume %s error: %v", lunName, err)
		log.Errorln(msg)
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	lunName := utils.GetLunName(name)
	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return err
	}
	if lun == nil {
		msg := fmt.Sprintf("LUN %s to stage doesn't exist", lunName)
		log.Errorln(msg)
		return errors.New(msg)
	}

	out := p.attachDetachHandler("NodeStage", lun, parameters)
	if len(out) != 2 {
		msg := fmt.Sprintf("Stage volume %s error", lunName)
		log.Errorln(msg)
		return errors.New(msg)
	}

	result := out[1].Interface()
	if result != nil {
		err := result.(error)
		log.Errorln("Stage volume %s error: %v", lunName, err)
		return err
	}

	devPath := out[0].Interface().(string)
	targetPath := parameters["targetPath"].(string)
	fsType := parameters["fsType"].(string)
	mountFlags := parameters["mountFlags"].(string)

	err = dev.MountLunDev(devPath, targetPath, fsType, mountFlags)
	if err != nil {
		log.Errorf("Mount device %s to %s error: %v", devPath, targetPath, err)
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	targetPath := parameters["targetPath"].(string)
	err := dev.Unmount(targetPath)
	if err != nil {
		log.Errorf("Cannot unmount %s error: %v", targetPath, err)
		return err
	}

	lunName := utils.GetLunName(name)
	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return err
	}
	if lun == nil {
		log.Warningf("LUN %s to detach doesn't exist", lunName)
		return nil
	}

	out := p.attachDetachHandler("NodeUnstage", lun, parameters)
	if len(out) != 1 {
		msg := fmt.Sprintf("Unstage volume %s error", lunName)
		log.Errorln(msg)
		return errors.New(msg)
	}

	result := out[0].Interface()
	if result != nil {
		err := result.(error)
		msg := fmt.Sprintf("Unstage volume %s error: %v", lunName, err)
		log.Errorln(msg)
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) UpdatePoolCapabilities(poolNames []string) (map[string]interface{}, error) {
	pools, err := p.cli.GetAllPools()
	if err != nil {
		log.Errorf("Get all pools error: %v", err)
		return nil, err
	}

	log.Debugf("Get pools: %v", pools)

	var validPools []map[string]interface{}
	for _, name := range poolNames {
		if pool, exist := pools[name].(map[string]interface{}); exist {
			if pool["USAGETYPE"].(string) != "1" {
				log.Warningf("Pool %s is not for SAN", name)
			} else {
				validPools = append(validPools, pool)
			}
		} else {
			log.Warningf("Pool %s does not exist", name)
		}
	}

	capabilities := p.analyzePoolsCapacity(validPools)
	return capabilities, nil
}

func (p *OceanstorSanPlugin) UpdateMetroRemotePlugin(remote Plugin) {
	p.metroRemotePlugin = remote.(*OceanstorSanPlugin)
}
