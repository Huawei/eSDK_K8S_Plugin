package plugin

import (
	"dev"
	"errors"
	"fmt"
	"proto"
	osAttacher "storage/oceanstor/attacher"
	osVol "storage/oceanstor/volume"
	"utils"
	"utils/log"
)

type OceanstorSanPlugin struct {
	OceanstorPlugin
	protocol string
	portals  []string
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

func (p *OceanstorSanPlugin) CreateVolume(name string, size int64, poolName string, parameters map[string]string) (string,error) {
	params, err := p.getParams(size, name ,poolName, parameters)
	if err != nil {
		return " ", err
	}
	volumeName := params["name"].(string)
	san := osVol.NewSAN(p.cli,nil)
	log.Infof("Start to create volume %s", volumeName)
	err = san.Create(params)
	if err != nil {
		return volumeName, err
	}

	return volumeName, nil
}

func (p *OceanstorSanPlugin) DeleteVolume(name string) error {
	san := osVol.NewSAN(p.cli,nil)
	return san.Delete(name)
}

func (p *OceanstorSanPlugin) AttachVolume(name string, parameters map[string]interface{}) error {
	return nil
}

func (p *OceanstorSanPlugin) DetachVolume(name string, parameters map[string]interface{}) error {
	lunName := utils.GetLunName(name)

	attacher := osAttacher.NewAttacher(p.cli, p.protocol, "csi")
	_, err := attacher.ControllerDetach(lunName, parameters)
	if err != nil {
		log.Errorf("Detach volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	lunName := utils.GetLunName(name)
	parameters["portals"] = p.portals

	attacher := osAttacher.NewAttacher(p.cli, p.protocol, "csi")
	err := attacher.NodeStage(lunName, parameters)
	if err != nil {
		log.Errorf("Stage volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	targetPath := parameters["targetPath"].(string)
	err := dev.Unmount(targetPath)
	if err != nil {
		log.Errorf("Cannot unmount volume %s error: %v", name, err)
		return err
	}

	lunName := utils.GetLunName(name)

	attacher := osAttacher.NewAttacher(p.cli, p.protocol, "csi")
	err = attacher.NodeUnstage(lunName, parameters)
	if err != nil {
		log.Errorf("Unstage volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) checkPoolValid(pool map[string]interface{}) error {
	if pool["USAGETYPE"].(string) != "1" {
		msg := fmt.Sprintf("Pool %v is not for SAN", pool)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}
