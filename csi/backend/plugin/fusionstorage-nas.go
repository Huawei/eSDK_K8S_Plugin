/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package plugin

import (
	"context"
	"errors"
	"fmt"

	"huawei-csi-driver/storage/fusionstorage/client"
	"huawei-csi-driver/storage/fusionstorage/volume"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// FusionStorageNasPlugin implements storage StoragePlugin interface
type FusionStorageNasPlugin struct {
	FusionStoragePlugin
	portal   string
	protocol string
}

func init() {
	RegPlugin("fusionstorage-nas", &FusionStorageNasPlugin{})
}

// NewPlugin used to create new plugin
func (p *FusionStorageNasPlugin) NewPlugin() StoragePlugin {
	return &FusionStorageNasPlugin{}
}

// Init used to init the plugin
func (p *FusionStorageNasPlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "nfs" && protocol != "dpc") {
		return errors.New("protocol must be provided and be \"nfs\" or \"dpc\" for fusionstorage-nas backend")
	}

	p.protocol = protocol
	if protocol == "nfs" {
		portals, exist := parameters["portals"].([]interface{})
		if !exist || len(portals) != 1 {
			return errors.New("portals must be provided for fusionstorage-nas nfs backend and just support one portal")
		}
		p.portal, exist = portals[0].(string)
		if !exist {
			return errors.New(fmt.Sprintf("portals: %v must be string", portals[0]))
		}
	}

	err := p.init(ctx, config, keepLogin)
	if err != nil {
		return err
	}
	return nil
}

func (p *FusionStorageNasPlugin) updateNasCapacity(ctx context.Context,
	params, parameters map[string]interface{}) error {
	size, exist := parameters["size"].(int64)
	if !exist {
		return utils.Errorf(ctx, "the size does not exist in parameters %v", parameters)
	}
	params["capacity"] = utils.RoundUpSize(size, fileCapacityUnit)
	return nil
}

// CreateVolume used to create volume
func (p *FusionStorageNasPlugin) CreateVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {

	size, ok := parameters["size"].(int64)
	// for fusionStorage filesystem, the unit is KiB
	if !ok || !utils.IsCapacityAvailable(size, fileCapacityUnit) {
		return nil, utils.Errorf(ctx, "Create Volume: the capacity %d is not an integer or not multiple of %d.",
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

// QueryVolume used to query volume
func (p *FusionStorageNasPlugin) QueryVolume(ctx context.Context, name string, params map[string]interface{}) (
	utils.Volume, error) {
	nas := volume.NewNAS(p.cli)
	return nas.Query(ctx, name)
}

// DeleteVolume used to delete volume
func (p *FusionStorageNasPlugin) DeleteVolume(ctx context.Context, name string) error {
	nas := volume.NewNAS(p.cli)
	return nas.Delete(ctx, name)
}

// UpdateBackendCapabilities to update the backend capabilities, such as thin, thick, qos and etc.
func (p *FusionStorageNasPlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":  true,
		"SupportThick": false,
		"SupportQoS":   true,
		"SupportQuota": true,
		"SupportClone": false,
	}

	err := p.updateNFS4Capability(ctx, capabilities)
	if err != nil {
		return nil, nil, err
	}
	return capabilities, nil, nil
}

// CreateSnapshot used to create snapshot
func (p *FusionStorageNasPlugin) CreateSnapshot(ctx context.Context,
	lunName, snapshotName string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("unimplemented")
}

// DeleteSnapshot used to delete snapshot
func (p *FusionStorageNasPlugin) DeleteSnapshot(ctx context.Context,
	snapshotParentID, snapshotName string) error {
	return fmt.Errorf("unimplemented")
}

// ExpandVolume used to expand volume
func (p *FusionStorageNasPlugin) ExpandVolume(ctx context.Context,
	name string,
	size int64) (bool, error) {
	nas := volume.NewNAS(p.cli)
	return false, nas.Expand(ctx, name, size)
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *FusionStorageNasPlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	return p.updatePoolCapabilities(ctx, poolNames, FusionStorageNas)
}

func (p *FusionStorageNasPlugin) updateNFS4Capability(ctx context.Context, capabilities map[string]interface{}) error {
	if capabilities == nil {
		capabilities = make(map[string]interface{})
	}

	nfsServiceSetting, err := p.cli.GetNFSServiceSetting(ctx)
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

// Validate used to validate FusionStorageNasPlugin parameters
func (p *FusionStorageNasPlugin) Validate(ctx context.Context, param map[string]interface{}) error {
	log.AddContext(ctx).Infoln("Start to validate FusionStorageNasPlugin parameters.")

	err := p.verifyFusionStorageNasParam(ctx, param)
	if err != nil {
		return err
	}

	clientConfig, err := p.getNewClientConfig(ctx, param)
	if err != nil {
		return err
	}

	// Login verification
	cli := client.NewClient(ctx, clientConfig)
	err = cli.ValidateLogin(ctx)
	if err != nil {
		return err
	}
	cli.Logout(ctx)

	return nil
}

func (p *FusionStorageNasPlugin) verifyFusionStorageNasParam(ctx context.Context, config map[string]interface{}) error {
	parameters, exist := config["parameters"].(map[string]interface{})
	if !exist {
		msg := fmt.Sprintf("Verify parameters: [%v] failed. \nparameters must be provided", config["parameters"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "nfs" && protocol != "dpc") {
		msg := fmt.Sprintf("Verify protocol: [%v] failed. \nprotocol must be provided and be \"nfs\" or \"dpc\" "+
			"for fusionstorage-nas backend\n", parameters["protocol"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	if protocol == "dpc" {
		return nil
	}

	portals, exist := parameters["portals"].([]interface{})
	if !exist || len(portals) != 1 {
		msg := fmt.Sprintf("Verify portals: [%v] failed. \nportals must be provided for fusionstorage-nas "+
			"backend of the nfs protocol and only one portal can be configured.\n", parameters["portals"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

// DeleteDTreeVolume used to delete DTree volume
func (p *FusionStorageNasPlugin) DeleteDTreeVolume(ctx context.Context, m map[string]interface{}) error {
	return errors.New("not implement")
}

// ExpandDTreeVolume used to expand DTree volume
func (p *FusionStorageNasPlugin) ExpandDTreeVolume(ctx context.Context, m map[string]interface{}) (bool, error) {
	return false, errors.New("not implement")
}
