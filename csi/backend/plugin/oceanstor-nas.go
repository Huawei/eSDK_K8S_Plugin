package plugin

import (
	"dev"
	"errors"
	"fmt"
	"net"
	osVol "storage/oceanstor/volume"
	"utils"
	"utils/log"
)

type OceanstorNasPlugin struct {
	OceanstorPlugin
	portal string
}

func init() {
	RegPlugin("oceanstor-nas", &OceanstorNasPlugin{})
}

func (p *OceanstorNasPlugin) NewPlugin() Plugin {
	return &OceanstorNasPlugin{}
}

func (p *OceanstorNasPlugin) Init(config, parameters map[string]interface{}) error {
	portal, exist := parameters["portal"].(string)
	if !exist {
		return errors.New("portal must be provided for oceanstor-nas backend")
	}

	ip := net.ParseIP(portal)
	if ip == nil {
		return fmt.Errorf("portal %s is invalid", portal)
	}

	err := p.init(config)
	if err != nil {
		return err
	}

	p.portal = portal
	return nil
}

func (p *OceanstorNasPlugin) CreateVolume(name string, size int64, poolName string, parameters map[string]string) (string,error) {
	params, err := p.getParams(size, name ,poolName, parameters)
	if err != nil {
		return " ", err
	}
	volumeName := params["name"].(string)
	nas := osVol.NewNAS(p.cli)
	log.Infof("Start to create volume %s", volumeName)
	err = nas.Create(params)
	if err != nil {
		return volumeName, err
	}

	return volumeName, nil
}

func (p *OceanstorNasPlugin) DeleteVolume(name string) error {
	var err error
	nas := osVol.NewNAS(p.cli)
	if p.version =="9000" {
		err = nas.Delete9000(name)
	}else {
		err = nas.Delete(name)
	}
	return err
}

func (p *OceanstorNasPlugin) AttachVolume(name string, parameters map[string]interface{}) error {
	return nil
}

func (p *OceanstorNasPlugin) DetachVolume(name string, parameters map[string]interface{}) error {
	return nil
}

func (p *OceanstorNasPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	fsName := utils.GetFileSystemName(name)
	exportPath := p.portal + ":/" + fsName

	targetPath := parameters["targetPath"].(string)
	mountFlags := parameters["mountFlags"].(string)

	err := dev.MountFsDev(exportPath, targetPath, mountFlags)
	if err != nil {
		log.Errorf("Mount filesystem %s to %s error: %v", exportPath, targetPath, err)
		return err
	}

	return nil
}

func (p *OceanstorNasPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	targetPath := parameters["targetPath"].(string)
	err := dev.Unmount(targetPath)
	if err != nil {
		log.Errorf("Cannot unmount volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *OceanstorNasPlugin) checkPoolValid(pool map[string]interface{}) error {
	if pool["USAGETYPE"].(string) != "2" {
		msg := fmt.Sprintf("Pool %v is not for NAS", pool)
		log.Errorln(msg)
		return errors.New(msg)
	}

	return nil
}
