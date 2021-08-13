package plugin

import (
	"errors"
	"fmt"
	"net"
	"storage/fusionstorage/volume"
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
	protocol, exist := parameters["protocol"].(string)
	if !exist || protocol != "nfs" {
		return errors.New("protocol must be provided and be nfs for fusionstorage-nas backend")
	}

	portals, exist := parameters["portals"].([]interface{})
	if !exist || len(portals) != 1 {
		return errors.New("portals must be provided for fusionstorage-nas backend and just support one portal")
	}

	portal := portals[0].(string)
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
	return p.fsStageVolume(name, p.portal, parameters)
}

func (p *FusionStorageNasPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	return p.unstageVolume(name, parameters)
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
