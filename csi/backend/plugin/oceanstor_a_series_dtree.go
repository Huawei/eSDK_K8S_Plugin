/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/volume/dtree"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func init() {
	RegPlugin(constants.OceanStorASeriesDtree, &OceanstorASeriesDtreePlugin{})
}

// OceanstorASeriesDtreePlugin implements storage StoragePlugin interface for OceanStor A600/A800 DTree
type OceanstorASeriesDtreePlugin struct {
	OceanstorASeriesPlugin

	parentName string
}

// NewPlugin used to create new plugin
func (p *OceanstorASeriesDtreePlugin) NewPlugin() StoragePlugin {
	return &OceanstorASeriesDtreePlugin{}
}

// Init used to init the plugin
func (p *OceanstorASeriesDtreePlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	if err := p.validateParentname(parameters); err != nil {
		return err
	}
	err := p.OceanstorASeriesPlugin.Init(ctx, config, parameters, keepLogin)
	if err != nil {
		return fmt.Errorf("init A-series dtree plugin failed: %w", err)
	}
	return nil
}

// validateParentName validates the parentname parameter
func (p *OceanstorASeriesDtreePlugin) validateParentname(parameters map[string]interface{}) error {
	parentName, ok := parameters["parentname"]
	if !ok {
		return nil
	}

	strParentName, ok := parentName.(string)
	if !ok {
		return errors.New("parentName must be a string type")
	}

	p.parentName = strParentName
	return nil
}

// UpdateBackendCapabilities updates backend capabilities
func (p *OceanstorASeriesDtreePlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities, specifications, err := p.OceanstorASeriesPlugin.UpdateBackendCapabilities(ctx)
	if err != nil {
		return nil, nil, err
	}
	capabilities[string(constants.SupportApplicationType)] = false
	capabilities[string(constants.SupportQoS)] = false
	capabilities[string(constants.SupportThick)] = false
	capabilities[string(constants.SupportQuota)] = true

	return capabilities, specifications, nil
}

// UpdatePoolCapabilities updates pool capabilities (returns zero capacities for DTree)
func (p *OceanstorASeriesDtreePlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	return getZeroPoolsCapacities(ctx, poolNames)
}

// Validate validates A-series DTree plugin parameters
func (p *OceanstorASeriesDtreePlugin) Validate(ctx context.Context, param map[string]interface{}) error {
	if err := verifyDTreeParam(ctx, param, constants.OceanStorASeriesDtree); err != nil {
		return err
	}

	return p.OceanstorASeriesPlugin.Validate(ctx, param)
}

// CreateVolume creates a DTree volume on A-series storage
func (p *OceanstorASeriesDtreePlugin) CreateVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (utils.Volume, error) {
	name, err := getVolumeNameFromPVNameOrParameters(name, parameters)
	if err != nil {
		return nil, err
	}
	params, err := utils.ConvertMapToStruct[CreateASeriesDTreeVolumeParameter](parameters)
	if err != nil {
		return nil, fmt.Errorf("convert parameters to struct failed when creating %s volume: %w",
			constants.OceanStorASeriesDtree, err)
	}

	model, err := params.genCreateDTreeModel(name, p.parentName, p.protocol)
	if err != nil {
		return nil, err
	}

	volume, err := dtree.NewCreator(ctx, p.cli, model).Create()
	if err != nil {
		return nil, fmt.Errorf("create %s volume failed: %w", constants.OceanStorASeriesDtree, err)
	}

	return volume, nil
}

// QueryVolume queries a DTree volume
func (p *OceanstorASeriesDtreePlugin) QueryVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (utils.Volume, error) {
	backendParentName := p.parentName
	scParentName, ok := utils.GetValue[string](parameters, "parentname")

	var parentName = backendParentName
	if ok && scParentName != "" {
		var err error
		parentName, err = getValidParentname(scParentName, backendParentName)
		if err != nil {
			return nil, err
		}
	}
	return dtree.NewQuerier(ctx, p.cli, name, parentName).Query()
}

// DeleteVolume deletes a DTree volume (not implemented, use DeleteDTreeVolume instead)
func (p *OceanstorASeriesDtreePlugin) DeleteVolume(ctx context.Context, name string,
	params map[string]interface{}) error {
	return errors.New("not implement, use DeleteDTreeVolume instead")
}

// DeleteDTreeVolume deletes a DTree volume
func (p *OceanstorASeriesDtreePlugin) DeleteDTreeVolume(ctx context.Context, dTreeName, parentName string) error {
	return dtree.NewDeleter(ctx, p.cli, parentName, dTreeName, p.protocol).Delete()
}

// ExpandVolume expands a DTree volume (not implemented, use ExpandDTreeVolume instead)
func (p *OceanstorASeriesDtreePlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	return false, errors.New("not implement, use ExpandDTreeVolume instead")
}

// ExpandDTreeVolume expands a DTree volume capacity
func (p *OceanstorASeriesDtreePlugin) ExpandDTreeVolume(ctx context.Context,
	dTreeName, parentName string, spaceHardQuota int64) (bool, error) {

	param := &dtree.ExpandDTreeModel{
		ParentName: parentName,
		DTreeName:  dTreeName,
		Capacity:   spaceHardQuota,
	}

	err := dtree.NewExpander(ctx, p.cli, param).Expand()
	if err != nil {
		return false, fmt.Errorf("expand %s volume %s failed: %w",
			constants.OceanStorASeriesDtree, dTreeName, err)
	}

	return false, nil
}

// AttachVolume attach volume to node and return storage mapping info.
func (p *OceanstorASeriesDtreePlugin) AttachVolume(_ context.Context, _ string,
	parameters map[string]any) (map[string]any, error) {
	return attachDTreeVolume(parameters)
}

// CreateSnapshot creates snapshot (not supported for DTree)
func (p *OceanstorASeriesDtreePlugin) CreateSnapshot(ctx context.Context,
	fsName, snapshotName string, parameters map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesDtree)
}

// DeleteSnapshot deletes snapshot (not supported for DTree)
func (p *OceanstorASeriesDtreePlugin) DeleteSnapshot(ctx context.Context, snapshotParentId, snapshotName string) error {
	return fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesDtree)
}

// ModifyVolume modifies volume (not supported for DTree)
func (p *OceanstorASeriesDtreePlugin) ModifyVolume(context.Context, string,
	pkgVolume.ModifyVolumeType, map[string]string) error {
	return fmt.Errorf("%s storage does not support volume modification", constants.OceanStorASeriesDtree)
}

// GetDTreeParentName returns the DTree parent directory name
func (p *OceanstorASeriesDtreePlugin) GetDTreeParentName() string {
	return p.parentName
}

// GetSectorSize returns the sector size for DTree capacity unit
func (p *OceanstorASeriesDtreePlugin) GetSectorSize() int64 {
	return constants.ASeriesDTreeCapacityUnit
}
