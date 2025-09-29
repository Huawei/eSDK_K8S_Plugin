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
	"fmt"
	"strconv"

	xuanwuV1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// OceanstorASeriesPlugin implements storage StoragePlugin interface
type OceanstorASeriesPlugin struct {
	basePlugin

	protocol string

	cli client.OceanASeriesClientInterface
}

func init() {
	RegPlugin(constants.OceanStorASeriesNas, &OceanstorASeriesPlugin{})
}

// NewPlugin used to create new plugin
func (p *OceanstorASeriesPlugin) NewPlugin() StoragePlugin {
	return &OceanstorASeriesPlugin{}
}

// Init used to init the plugin
func (p *OceanstorASeriesPlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	var err error
	if err = p.verifyAndSetProtocol(parameters); err != nil {
		return fmt.Errorf("check protocol failed, err: %w", err)
	}

	if err = p.verifyPortals(parameters); err != nil {
		return fmt.Errorf("check portals failed, err: %w", err)
	}

	clientConfig, err := formatBaseClientConfig(config)
	if err != nil {
		return err
	}

	cli, err := client.NewClient(ctx, clientConfig)
	if err != nil {
		return err
	}

	if err = cli.Login(ctx); err != nil {
		return err
	}

	if err = cli.SetSystemInfo(ctx); err != nil {
		cli.Logout(ctx)
		return err
	}

	if !keepLogin {
		cli.Logout(ctx)
	}

	p.name = clientConfig.Name
	p.cli = cli
	return nil
}

func (p *OceanstorASeriesPlugin) verifyAndSetProtocol(params map[string]interface{}) error {
	protocol, ok := utils.GetValue[string](params, "protocol")
	if !ok {
		return fmt.Errorf("protocol must be provided for %s backend", constants.OceanStorASeriesNas)
	}

	if protocol != constants.ProtocolNfs && protocol != constants.ProtocolDtfs {
		return fmt.Errorf("protocol must be %s or %s for %s backend", constants.ProtocolNfs,
			constants.ProtocolDtfs, constants.OceanStorASeriesNas)
	}

	p.protocol = protocol
	return nil
}

func (p *OceanstorASeriesPlugin) verifyPortals(params map[string]interface{}) error {
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
func (p *OceanstorASeriesPlugin) CreateVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (utils.Volume, error) {
	name, err := getVolumeNameFromPVNameOrParameters(name, parameters)
	if err != nil {
		return nil, err
	}

	params, err := utils.ConvertMapToStruct[CreateASeriesVolumeParameter](parameters)
	if err != nil {
		return nil, fmt.Errorf("convert parameters to struct failed when create volume: %w", err)
	}

	model, err := params.genCreateVolumeModel(name, p.protocol)
	if err != nil {
		return nil, err
	}

	return volume.NewCreator(ctx, p.cli, model).Create()
}

// QueryVolume used to query volume
func (p *OceanstorASeriesPlugin) QueryVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	workLoadType, _ := utils.GetValue[string](parameters, "applicationType")
	model := &volume.QueryFilesystemModel{
		Name:         name,
		WorkloadType: workLoadType,
	}
	return volume.NewQuerier(ctx, p.cli, model).Query()
}

// DeleteVolume used to delete volume
func (p *OceanstorASeriesPlugin) DeleteVolume(ctx context.Context, name string) error {
	model := &volume.DeleteFilesystemModel{
		Protocol: p.protocol,
		Name:     name,
	}
	return volume.NewDeleter(ctx, p.cli, model).Delete()
}

// ExpandVolume used to expand volume
func (p *OceanstorASeriesPlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	model := &volume.ExpandFilesystemModel{
		Name:     name,
		Capacity: size,
	}
	return false, volume.NewExpander(ctx, p.cli, model).Expand()
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *OceanstorASeriesPlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	vStoreQuotaMap, err := p.getVstoreCapacity(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("get vstore capacity failed, err: %v", err)
		vStoreQuotaMap = map[string]interface{}{}
	}

	pools, err := p.cli.GetAllPools(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all pools failed, err: %w", err)
	}

	var selectedPools []map[string]interface{}
	for _, name := range poolNames {
		if pool, ok := utils.GetValue[map[string]interface{}](pools, name); ok {
			selectedPools = append(selectedPools, pool)
		} else {
			log.AddContext(ctx).Warningf("Pool %s does not exist", name)
		}
	}

	capabilities := analyzePoolsCapacity(ctx, selectedPools, vStoreQuotaMap)
	return capabilities, nil
}

func (p *OceanstorASeriesPlugin) getVstoreCapacity(ctx context.Context) (map[string]interface{}, error) {
	if p.cli.GetvStoreName() == "" {
		return map[string]interface{}{}, nil
	}

	vStore, err := p.cli.GetvStoreByName(ctx, p.cli.GetvStoreName())
	if err != nil {
		return nil, err
	}
	if vStore == nil {
		return nil, fmt.Errorf("not find vstore by name, name: %s", p.cli.GetvStoreName())
	}

	var nasCapacityQuota, nasFreeCapacityQuota int64
	if totalStr, ok := utils.GetValue[string](vStore, "nasCapacityQuota"); ok {
		nasCapacityQuota, err = strconv.ParseInt(totalStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
		if err != nil {
			return nil, fmt.Errorf("parse vstore nasCapacityQuota failed, error: %w", err)
		}
	}

	if freeStr, ok := vStore["nasFreeCapacityQuota"].(string); ok {
		nasFreeCapacityQuota, err = strconv.ParseInt(freeStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
		if err != nil {
			return nil, fmt.Errorf("parse vstore nasFreeCapacityQuota failed, error: %w", err)
		}
	}

	// if not set quota, nasCapacityQuota is 0, nasFreeCapacityQuota is -1
	if nasCapacityQuota == 0 || nasFreeCapacityQuota == -1 {
		return map[string]interface{}{}, nil
	}

	return map[string]interface{}{
		string(xuanwuV1.FreeCapacity):  nasFreeCapacityQuota * constants.AllocationUnitBytes,
		string(xuanwuV1.TotalCapacity): nasCapacityQuota * constants.AllocationUnitBytes,
		string(xuanwuV1.UsedCapacity):  (nasCapacityQuota - nasFreeCapacityQuota) * constants.AllocationUnitBytes,
	}, nil
}

// UpdateBackendCapabilities used to update backend capabilities
func (p *OceanstorASeriesPlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities, err := p.getBackendCapabilities(ctx)
	if err != nil {
		return nil, nil, err
	}

	return capabilities, p.getBackendSpecifications(), nil
}

func (p *OceanstorASeriesPlugin) getBackendCapabilities(ctx context.Context) (map[string]interface{}, error) {
	features, err := p.cli.GetLicenseFeature(ctx)
	if err != nil {
		return nil, fmt.Errorf("get license feature failed, err: %w", err)
	}

	capabilities := map[string]interface{}{
		"SupportThin":               true,
		"SupportApplicationType":    true,
		"SupportQoS":                utils.IsSupportFeature(features, "SmartQoS"),
		"SupportThick":              false,
		"SupportMetro":              false,
		"SupportReplication":        false,
		"SupportClone":              false,
		"SupportMetroNAS":           false,
		"SupportConsistentSnapshot": false,
	}

	nfsServiceSetting, err := p.cli.GetNFSServiceSetting(ctx)
	if err != nil {
		return nil, fmt.Errorf("get nfs service setting failed, err: %w", err)
	}

	updateCapabilityByNfsServiceSetting(capabilities, nfsServiceSetting)
	return capabilities, nil
}

func (p *OceanstorASeriesPlugin) getBackendSpecifications() map[string]interface{} {
	specifications := map[string]interface{}{
		"LocalDeviceSN": p.cli.GetDeviceSN(),
		"VStoreID":      p.cli.GetvStoreID(),
		"VStoreName":    p.cli.GetvStoreName(),
		"DeviceWWN":     p.cli.GetDeviceWWN(),
	}
	return specifications
}

// Validate used to validate OceanstorASeriesPlugin parameters
func (p *OceanstorASeriesPlugin) Validate(ctx context.Context, config map[string]interface{}) error {
	parameters, ok := utils.GetValue[map[string]interface{}](config, "parameters")
	if !ok {
		return fmt.Errorf("verify config %v failed. parameters field must be provided", config)
	}

	err := p.verifyAndSetProtocol(parameters)
	if err != nil {
		return err
	}

	err = p.verifyPortals(parameters)
	if err != nil {
		return err
	}

	clientConfig, err := formatBaseClientConfig(config)
	if err != nil {
		return err
	}

	cli, err := client.NewClient(ctx, clientConfig)
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

// SupportQoSParameters checks requested QoS parameters support by A-series plugin
func (p *OceanstorASeriesPlugin) SupportQoSParameters(ctx context.Context, qosConfig string) error {
	return smartx.CheckQoSParametersValueRange(ctx, qosConfig)
}

// Logout is to logout the storage session
func (p *OceanstorASeriesPlugin) Logout(ctx context.Context) {
	if p.cli != nil {
		p.cli.Logout(ctx)
	}
}

// GetSectorSize gets the sector size of plugin
func (p *OceanstorASeriesPlugin) GetSectorSize() int64 {
	return SectorSize
}

// CreateSnapshot used to create snapshot
func (p *OceanstorASeriesPlugin) CreateSnapshot(ctx context.Context,
	fsName, snapshotName string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesNas)
}

// DeleteSnapshot used to delete snapshot
func (p *OceanstorASeriesPlugin) DeleteSnapshot(ctx context.Context, snapshotParentId, snapshotName string) error {
	return fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesNas)
}

// DeleteDTreeVolume used to delete DTree volume
func (p *OceanstorASeriesPlugin) DeleteDTreeVolume(context.Context, string, string) error {
	return fmt.Errorf("%s storage does not support DTree feature", constants.OceanStorASeriesNas)
}

// ExpandDTreeVolume used to expand DTree volume
func (p *OceanstorASeriesPlugin) ExpandDTreeVolume(context.Context, string, string, int64) (bool, error) {
	return false, fmt.Errorf("%s storage does not support DTree feature", constants.OceanStorASeriesNas)
}

// ModifyVolume used to modify volume hyperMetro status
func (p *OceanstorASeriesPlugin) ModifyVolume(context.Context, string,
	pkgVolume.ModifyVolumeType, map[string]string) error {
	return fmt.Errorf("%s storage does not support volume modify feature", constants.OceanStorASeriesNas)
}

// ReLogin will refresh the user session of storage
func (p *OceanstorASeriesPlugin) ReLogin(ctx context.Context) error {
	if p.cli == nil {
		return nil
	}

	return p.cli.ReLogin(ctx)
}
