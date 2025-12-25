/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

// Package plugin provide storage function
package plugin

import (
	"context"
	"errors"
	"fmt"

	xuanwuV1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// DMEASeriesPlugin implements storage StoragePlugin interface
type DMEASeriesPlugin struct {
	basePlugin
	protocol string
	cli      client.DMEASeriesClientInterface
}

func init() {
	RegPlugin(constants.OceanStorASeriesNasDme, &DMEASeriesPlugin{})
}

// NewPlugin used to create new plugin
func (p *DMEASeriesPlugin) NewPlugin() StoragePlugin {
	return &DMEASeriesPlugin{}
}

// Init used to init the plugin
func (p *DMEASeriesPlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	clientConfig, cli, err := p.verifyAndGetClient(ctx, config, parameters)
	if err != nil {
		return err
	}

	storageDeviceSN, ok := utils.GetValue[string](config, "storageDeviceSN")
	if !ok {
		return errors.New("storageDeviceSN failed loss")
	}

	if err = cli.Login(ctx); err != nil {
		return err
	}

	if err = cli.SetSystemInfo(ctx, storageDeviceSN); err != nil {
		cli.Logout(ctx)
		log.AddContext(ctx).Errorf("set client info failed, err: %v", err)
		return err
	}

	if !keepLogin {
		cli.Logout(ctx)
	}

	p.name = clientConfig.Name
	p.cli = cli
	return nil
}

func (p *DMEASeriesPlugin) verifyAndGetClient(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}) (*storage.NewClientConfig, *client.DMEASeriesClient, error) {
	var err error
	if err = p.verifyAndSetProtocol(parameters); err != nil {
		return nil, nil, fmt.Errorf("check protocol failed, err: %w", err)
	}

	if err = p.verifyPortals(parameters); err != nil {
		return nil, nil, fmt.Errorf("check portals failed, err: %w", err)
	}

	clientConfig, err := formatBaseClientConfig(config)
	if err != nil {
		return nil, nil, err
	}

	cli, err := client.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, nil, err
	}
	return clientConfig, cli, nil
}

func (p *DMEASeriesPlugin) verifyAndSetProtocol(params map[string]interface{}) error {
	protocol, ok := utils.GetValue[string](params, "protocol")
	if !ok {
		return fmt.Errorf("protocol must be provided for %s backend", constants.OceanStorASeriesNasDme)
	}

	if protocol != constants.ProtocolNfs && protocol != constants.ProtocolDtfs {
		return fmt.Errorf("protocol must be %s or %s for %s backend", constants.ProtocolNfs,
			constants.ProtocolDtfs, constants.OceanStorASeriesNasDme)
	}

	p.protocol = protocol
	return nil
}

func (p *DMEASeriesPlugin) verifyPortals(params map[string]interface{}) error {
	if p.protocol == constants.ProtocolDtfs {
		return nil
	}

	portals, ok := utils.GetValue[[]interface{}](params, "portals")
	if !ok || len(portals) == 0 {
		return fmt.Errorf("portals must be provided for %s protocol", p.protocol)
	}

	portalsStrs := pkgUtils.ConvertToStringSlice(portals)
	if p.protocol == ProtocolNfs && len(portalsStrs) != 1 {
		return fmt.Errorf("portals just support one portal for %s protocol", p.protocol)
	}

	return nil
}

// CreateVolume used to create volume
func (p *DMEASeriesPlugin) CreateVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (utils.Volume, error) {
	name, err := getVolumeNameFromPVNameOrParameters(name, parameters)
	if err != nil {
		return nil, err
	}

	params, err := utils.ConvertMapToStruct[CreateDmeVolumeParameter](parameters)
	if err != nil {
		return nil, fmt.Errorf("convert parameters to struct failed when create volume: %w", err)
	}

	model, err := params.genCreateVolumeModel(name, p.protocol, p.GetSectorSize())
	if err != nil {
		return nil, err
	}

	return volume.NewCreator(ctx, p.cli, model).Create()
}

// QueryVolume used to query volume
func (p *DMEASeriesPlugin) QueryVolume(ctx context.Context, name string,
	_ map[string]interface{}) (utils.Volume, error) {
	model := &volume.QueryVolumeModel{
		Name: name,
	}
	return volume.NewQuerier(ctx, p.cli, model).Query()
}

// DeleteVolume used to delete volume
func (p *DMEASeriesPlugin) DeleteVolume(ctx context.Context, name string) error {
	model := &volume.DeleteVolumeModel{
		Protocol: p.protocol,
		Name:     name,
	}
	return volume.NewDeleter(ctx, p.cli, model).Delete()
}

// ExpandVolume used to expand volume
func (p *DMEASeriesPlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	model := &volume.ExpandVolumeModel{
		Name:     name,
		Capacity: size * p.GetSectorSize(),
	}
	return false, volume.NewExpander(ctx, p.cli, model).Expand()
}

// ModifyVolume used to modify volume hyperMetro status
func (p *DMEASeriesPlugin) ModifyVolume(context.Context, string, pkgVolume.ModifyVolumeType, map[string]string) error {
	return fmt.Errorf("%s storage does not support volume modify feature", constants.OceanStorASeriesNasDme)
}

// UpdateBackendCapabilities used to update backend capabilities
func (p *DMEASeriesPlugin) UpdateBackendCapabilities(_ context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities, err := p.getBackendCapabilities()
	if err != nil {
		return nil, nil, err
	}

	return capabilities, p.getBackendSpecifications(), nil
}

func (p *DMEASeriesPlugin) getBackendCapabilities() (map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":               true,
		"SupportApplicationType":    false,
		"SupportQoS":                false,
		"SupportThick":              false,
		"SupportMetro":              false,
		"SupportReplication":        false,
		"SupportClone":              false,
		"SupportMetroNAS":           false,
		"SupportConsistentSnapshot": false,
	}
	return capabilities, nil
}

func (p *DMEASeriesPlugin) getBackendSpecifications() map[string]interface{} {
	specifications := map[string]interface{}{
		"LocalDeviceSN": p.cli.GetDeviceSN(),
	}
	return specifications
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *DMEASeriesPlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	pools, err := p.cli.GetHyperScalePools(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all pools failed, err: %w", err)
	}

	poolMap := make(map[string]*client.HyperScalePool, len(pools))
	for _, pool := range pools {
		poolMap[pool.Name] = pool
	}
	capabilities := make(map[string]interface{})
	for _, name := range poolNames {
		if pool, ok := poolMap[name]; ok {
			totalCap := int64(pool.TotalCapacity * float64(constants.DmeCapacityUnitMb))
			freeCap := int64(pool.FreeCapacity * float64(constants.DmeCapacityUnitMb))
			poolCapacityMap := map[string]interface{}{
				string(xuanwuV1.FreeCapacity):  freeCap,
				string(xuanwuV1.TotalCapacity): totalCap,
				string(xuanwuV1.UsedCapacity):  totalCap - freeCap,
			}
			capabilities[pool.Name] = poolCapacityMap
		}
	}
	return capabilities, nil
}

// CreateSnapshot used to create snapshot
func (p *DMEASeriesPlugin) CreateSnapshot(context.Context, string, string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesNasDme)
}

// DeleteSnapshot used to delete snapshot
func (p *DMEASeriesPlugin) DeleteSnapshot(context.Context, string, string) error {
	return fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesNasDme)
}

// SupportQoSParameters checks requested QoS parameters support by dme plugin
func (p *DMEASeriesPlugin) SupportQoSParameters(context.Context, string) error {
	return nil
}

// Logout is to logout the storage session
func (p *DMEASeriesPlugin) Logout(ctx context.Context) {
	if p.cli != nil {
		p.cli.Logout(ctx)
	}
}

// ReLogin will refresh the user session of storage
func (p *DMEASeriesPlugin) ReLogin(ctx context.Context) error {
	if p.cli == nil {
		return nil
	}

	return p.cli.ReLogin(ctx)
}

// Validate used to check parameters, include login verification
func (p *DMEASeriesPlugin) Validate(ctx context.Context, config map[string]interface{}) error {
	parameters, ok := utils.GetValue[map[string]interface{}](config, "parameters")
	if !ok {
		return fmt.Errorf("verify config %v failed. parameters field must be provided", config)
	}

	_, cli, err := p.verifyAndGetClient(ctx, config, parameters)
	if err != nil {
		return err
	}

	err = cli.ValidateLogin(ctx)
	if err != nil {
		return err
	}

	cli.Logout(ctx)
	return nil
}

// DeleteDTreeVolume used to delete DTree volume
func (p *DMEASeriesPlugin) DeleteDTreeVolume(context.Context, string, string) error {
	return fmt.Errorf("%s storage does not support DTree feature", constants.OceanStorASeriesNasDme)
}

// ExpandDTreeVolume used to expand DTree volume
func (p *DMEASeriesPlugin) ExpandDTreeVolume(context.Context, string, string, int64) (bool, error) {
	return false, fmt.Errorf("%s storage does not support DTree feature", constants.OceanStorASeriesNasDme)
}

// GetSectorSize gets the sector size of plugin
func (p *DMEASeriesPlugin) GetSectorSize() int64 {
	return SectorSize
}
