package plugin

import (
	"dev"
	"errors"
	"fmt"
	"net"
	"storage/oceanstor/volume"
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

func (p *OceanstorNasPlugin) CreateVolume(name string, parameters map[string]interface{}) (string, error) {
	params := p.getParams(name, parameters)

	nas := volume.NewNAS(p.cli)
	err := nas.Create(params)
	if err != nil {
		return "", err
	}

	return params["name"].(string), nil
}

func (p *OceanstorNasPlugin) DeleteVolume(name string) error {
	nas := volume.NewNAS(p.cli)
	return nas.Delete(name)
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
	pools, err := p.cli.GetAllPools()
	if err != nil {
		log.Errorf("Get all pools error: %v", err)
		return nil, err
	}

	log.Debugf("Get pools: %v", pools)

	var validPools []map[string]interface{}
	for _, name := range poolNames {
		if pool, exist := pools[name].(map[string]interface{}); exist {
			if pool["USAGETYPE"].(string) != "2" {
				log.Warningf("Pool %s is not for NAS", name)
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

func (p *OceanstorNasPlugin) UpdateMetroRemotePlugin(remote Plugin) {
}
