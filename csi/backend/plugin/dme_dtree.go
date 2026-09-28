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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/volume/dtree"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func init() {
	RegPlugin(constants.OceanStorASeriesDtreeDme, &DMEASeriesDtreePlugin{})
}

// DMEASeriesDtreePlugin implements StoragePlugin for DME DTree
type DMEASeriesDtreePlugin struct {
	DMEASeriesPlugin
	parentName string
}

// NewPlugin creates a new DMEASeriesDtreePlugin instance
func (p *DMEASeriesDtreePlugin) NewPlugin() StoragePlugin {
	return &DMEASeriesDtreePlugin{}
}

// Init initializes the DME DTree plugin
func (p *DMEASeriesDtreePlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	if err := p.validateParentname(parameters); err != nil {
		return err
	}
	err := p.DMEASeriesPlugin.Init(ctx, config, parameters, keepLogin)
	if err != nil {
		return fmt.Errorf("init DME DTree plugin failed: %w", err)
	}
	return nil
}

func (p *DMEASeriesDtreePlugin) validateParentname(parameters map[string]interface{}) error {
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

// UpdateBackendCapabilities returns DME DTree backend capabilities
func (p *DMEASeriesDtreePlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities, specifications, err := p.DMEASeriesPlugin.UpdateBackendCapabilities(ctx)
	if err != nil {
		return nil, nil, err
	}
	capabilities[string(constants.SupportApplicationType)] = false
	capabilities[string(constants.SupportQoS)] = false
	capabilities[string(constants.SupportThick)] = false
	capabilities[string(constants.SupportQuota)] = true
	return capabilities, specifications, nil
}

// UpdatePoolCapabilities returns zero capacities for DTree
func (p *DMEASeriesDtreePlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	return getZeroPoolsCapacities(ctx, poolNames)
}

// Validate validates DME DTree plugin parameters
func (p *DMEASeriesDtreePlugin) Validate(ctx context.Context, param map[string]interface{}) error {
	if err := verifyDTreeParam(ctx, param, constants.OceanStorASeriesDtree); err != nil {
		return err
	}

	// DTree does not support zone (local mode), reject when zoneSN is configured
	if zoneSN, exists := param[constants.ZoneSNKey]; exists {
		if zoneSNStr, ok := zoneSN.(string); ok && zoneSNStr != "" {
			return fmt.Errorf("zoneSN [%s] is configured in backend, "+
				"but DTree does not support zone (local mode)", zoneSNStr)
		}
	}

	return p.DMEASeriesPlugin.Validate(ctx, param)
}

// CreateVolume creates a DME DTree volume (one-step creation)
func (p *DMEASeriesDtreePlugin) CreateVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (utils.Volume, error) {
	name, err := getVolumeNameFromPVNameOrParameters(name, parameters)
	if err != nil {
		return nil, err
	}
	params, err := utils.ConvertMapToStruct[CreateDmeDTreeVolumeParameter](parameters)
	if err != nil {
		return nil, fmt.Errorf("convert parameters to struct failed when creating %s volume: %w",
			constants.OceanStorASeriesDtreeDme, err)
	}

	model, err := params.genCreateDTreeVolumeModel(name, p.parentName, p.protocol, p.GetSectorSize())
	if err != nil {
		return nil, err
	}

	vol, err := dtree.NewCreator(ctx, p.cli, model).Create()
	if err != nil {
		return nil, fmt.Errorf("create %s volume failed: %w", constants.OceanStorASeriesDtreeDme, err)
	}
	return vol, nil
}

// QueryVolume queries a DME DTree volume
func (p *DMEASeriesDtreePlugin) QueryVolume(ctx context.Context, name string,
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

// DeleteVolume is not implemented for DTree, use DeleteDTreeVolume instead
func (p *DMEASeriesDtreePlugin) DeleteVolume(ctx context.Context, name string,
	params map[string]interface{}) error {
	return errors.New("not implemented, use DeleteDTreeVolume instead")
}

// DeleteDTreeVolume deletes a DME DTree volume
func (p *DMEASeriesDtreePlugin) DeleteDTreeVolume(ctx context.Context, dTreeName, parentName string) error {
	return dtree.NewDeleter(ctx, p.cli, parentName, dTreeName).Delete()
}

// ExpandVolume is not implemented for DTree, use ExpandDTreeVolume instead
func (p *DMEASeriesDtreePlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	return false, errors.New("not implemented, use ExpandDTreeVolume instead")
}

// ExpandDTreeVolume expands a DME DTree volume capacity
func (p *DMEASeriesDtreePlugin) ExpandDTreeVolume(ctx context.Context,
	dTreeName, parentName string, spaceHardQuota int64) (bool, error) {
	param := &dtree.ExpandDTreeModel{
		ParentName: parentName,
		DTreeName:  dTreeName,
		Capacity:   spaceHardQuota,
	}
	err := dtree.NewExpander(ctx, p.cli, param).Expand()
	if err != nil {
		return false, fmt.Errorf("expand %s volume %s failed: %w",
			constants.OceanStorASeriesDtreeDme, dTreeName, err)
	}
	return false, nil
}

// AttachVolume attaches DTree volume (returns DTree publish info)
func (p *DMEASeriesDtreePlugin) AttachVolume(_ context.Context, _ string,
	parameters map[string]any) (map[string]any, error) {
	return attachDTreeVolume(parameters)
}

// CreateSnapshot is not supported for DTree
func (p *DMEASeriesDtreePlugin) CreateSnapshot(ctx context.Context,
	fsName, snapshotName string, parameters map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesDtreeDme)
}

// DeleteSnapshot is not supported for DTree
func (p *DMEASeriesDtreePlugin) DeleteSnapshot(ctx context.Context, snapshotParentId, snapshotName string) error {
	return fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesDtreeDme)
}

// ModifyVolume is not supported for DTree
func (p *DMEASeriesDtreePlugin) ModifyVolume(context.Context, string,
	pkgVolume.ModifyVolumeType, map[string]string) error {
	return fmt.Errorf("%s storage does not support volume modification", constants.OceanStorASeriesDtreeDme)
}

// GetDTreeParentName returns the DTree parent directory name
func (p *DMEASeriesDtreePlugin) GetDTreeParentName() string {
	return p.parentName
}

// GetSectorSize returns the sector size for DTree capacity unit
func (p *DMEASeriesDtreePlugin) GetSectorSize() int64 {
	return constants.ASeriesDTreeCapacityUnit
}
