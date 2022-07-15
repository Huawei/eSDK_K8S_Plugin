package plugin

import (
	"context"
	"errors"
	"fmt"
	"net"

	"huawei-csi-driver/storage/fusionstorage/volume"
	"huawei-csi-driver/utils"
)

type FusionStorageNasPlugin struct {
	FusionStoragePlugin
	portal   string
	protocol string
}

func init() {
	RegPlugin("fusionstorage-nas", &FusionStorageNasPlugin{})
}

func (p *FusionStorageNasPlugin) NewPlugin() Plugin {
	return &FusionStorageNasPlugin{}
}

func (p *FusionStorageNasPlugin) Init(config, parameters map[string]interface{}, keepLogin bool) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "nfs" && protocol != "dpc") {
		return errors.New("protocol must be provided and be \"nfs\" or \"dpc\" for fusionstorage-nas backend")
	}

	p.protocol = protocol

	var portal string
	if protocol == "nfs" {
		portals, exist := parameters["portals"].([]interface{})
		if !exist || len(portals) != 1 {
			return errors.New("portals must be provided for fusionstorage-nas nfs backend and just support one portal")
		}

		portal = portals[0].(string)
		ip := net.ParseIP(portal)
		if ip == nil {
			return fmt.Errorf("portal %s is invalid", portal)
		}
	}

	err := p.init(config, keepLogin)
	if err != nil {
		return err
	}
	p.portal = portal
	return nil
}

func (p *FusionStorageNasPlugin) updateNasCapacity(ctx context.Context, params, parameters map[string]interface{}) error {
	size, exist := parameters["size"].(int64)
	if !exist {
		return utils.Errorf(ctx, "the size does not exist in parameters %v", parameters)
	}
	params["capacity"] = utils.RoundUpSize(size, fileCapacityUnit)
	return nil
}

func (p *FusionStorageNasPlugin) CreateVolume(ctx context.Context,
	name string,
	parameters map[string]interface{}) (utils.Volume, error) {
	size, ok := parameters["size"].(int64)
	// for fusionStorage filesystem, the unit is KiB
	if !ok || !utils.IsCapacityAvailable(size, fileCapacityUnit) {
		return nil, utils.Errorf(ctx, "Create Volume: the capacity %d is not an integer multiple of %d.",
			size, fileCapacityUnit)
	}

	params, err := p.getParams(name, parameters)
	if err != nil {
		return nil, err
	}

	// last step get the capacity is MiB, but need trans to KiB
	err = p.updateNasCapacity(ctx, params, parameters)
	if err != nil {
		return nil, err
	}

	params["protocol"] = p.protocol

	nas := volume.NewNAS(p.cli)
	volObj, err := nas.Create(ctx, params)
	if err != nil {
		return nil, err
	}

	return volObj, nil
}

func (p *FusionStorageNasPlugin) DeleteVolume(ctx context.Context, name string) error {
	nas := volume.NewNAS(p.cli)
	return nas.Delete(ctx, name)
}

func (p *FusionStorageNasPlugin) StageVolume(ctx context.Context,
	name string,
	parameters map[string]interface{}) error {
	parameters["protocol"] = p.protocol
	return p.fsStageVolume(ctx, name, p.portal, parameters)
}

func (p *FusionStorageNasPlugin) UnstageVolume(ctx context.Context,
	name string,
	parameters map[string]interface{}) error {
	return p.unstageVolume(ctx, name, parameters)
}

// UpdateBackendCapabilities to update the backend capabilities, such as thin, thick, qos and etc.
func (p *FusionStorageNasPlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":  true,
		"SupportThick": false,
		"SupportQoS":   false,
		"SupportQuota": true,
		"SupportClone": false,
	}

	err := p.updateNFS4Capability(capabilities)
	if err != nil {
		return nil, err
	}

	return capabilities, nil
}

func (p *FusionStorageNasPlugin) NodeExpandVolume(context.Context, string, string, bool, int64) error {
	return fmt.Errorf("unimplemented")
}

func (p *FusionStorageNasPlugin) CreateSnapshot(ctx context.Context,
	lunName, snapshotName string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *FusionStorageNasPlugin) DeleteSnapshot(ctx context.Context,
	snapshotParentID, snapshotName string) error {
	return fmt.Errorf("unimplemented")
}

func (p *FusionStorageNasPlugin) ExpandVolume(ctx context.Context,
	name string,
	size int64) (bool, error) {
	return false, fmt.Errorf("unimplemented")
}

func (p *FusionStorageNasPlugin) UpdatePoolCapabilities(poolNames []string) (map[string]interface{}, error) {
	return p.updatePoolCapabilities(poolNames, FusionStorageNas)
}

func (p *FusionStorageNasPlugin) updateNFS4Capability(capabilities map[string]interface{}) error {
	nfsServiceSetting, err := p.cli.GetNFSServiceSetting(context.Background())
	if err != nil {
		return err
	}

	// NFS3 is enabled by default.
	capabilities["SupportNFS3"] = true
	capabilities["SupportNFS4"] = false
	capabilities["SupportNFS41"] = false

	if nfsServiceSetting["SupportNFS41"] {
		capabilities["SupportNFS41"] = true
	}

	return nil
}
