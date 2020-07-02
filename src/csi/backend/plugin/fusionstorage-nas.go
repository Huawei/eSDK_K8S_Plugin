package plugin

import (
	"dev"
	"errors"
	"fmt"
	"net"
	"storage/fusionstorage/volume"
	"utils"
	"utils/log"
)

type FusionStorageNasPlugin struct {
	FusionStoragePlugin
	portal   string
}

func init() {
	RegPlugin("fusionstorage-nas", &FusionStorageNasPlugin{})
}

func (p *FusionStorageNasPlugin) NewPlugin() Plugin {
	return &FusionStorageNasPlugin{}
}

func (p *FusionStorageNasPlugin) Init(config, parameters map[string]interface{}, keepLogin bool) error {
	portal, exist := parameters["portal"].(string)
	if !exist {
		return errors.New("portal must be provided for fusionstorage-nas backend")
	}

	ip := net.ParseIP(portal)
	if ip == nil {
		return fmt.Errorf("portal %s is invalid", portal)
	}

	err := p.init(config, keepLogin)
	if err != nil {
		return err
	}
	p.portal = portal
	return nil
}

func (p *FusionStorageNasPlugin) CreateVolume(name string, parameters map[string]interface{}) (string, error) {
	params, err := p.getParams(name, parameters)
	if err != nil {
		return "", err
	}

	nas := volume.NewNAS(p.cli)
	err = nas.Create(params)
	if err != nil {
		return "", err
	}

	return params["name"].(string), nil
}

func (p *FusionStorageNasPlugin) DeleteVolume(name string) error {
	nas := volume.NewNAS(p.cli)
	return nas.Delete(name)
}

func (p *FusionStorageNasPlugin) StageVolume(name string, parameters map[string]interface{}) error {
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

func (p *FusionStorageNasPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	targetPath := parameters["targetPath"].(string)
	err := dev.Unmount(targetPath)
	if err != nil {
		log.Errorf("Unmount volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *FusionStorageNasPlugin) NodeExpandVolume(string, string) error {
	return fmt.Errorf("unimplemented")
}

func (p *FusionStorageNasPlugin) CreateSnapshot(lunName, snapshotName string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *FusionStorageNasPlugin) DeleteSnapshot(snapshotParentId, snapshotName string) error {
	return fmt.Errorf("unimplemented")
}

func (p *FusionStorageNasPlugin) ExpandVolume(name string, size int64) (bool, error) {
	return false, fmt.Errorf("unimplemented")
}
