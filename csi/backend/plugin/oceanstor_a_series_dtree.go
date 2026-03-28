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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

// OceanstorASeriesDtreePlugin implements storage StoragePlugin interface
type OceanstorASeriesDtreePlugin struct {
	OceanstorASeriesPlugin

	parentName string
}

func init() {
	RegPlugin(constants.OceanStorASeriesDtree, &OceanstorASeriesDtreePlugin{})
}

// NewPlugin used to create new plugin
func (p *OceanstorASeriesDtreePlugin) NewPlugin() StoragePlugin {
	return &OceanstorASeriesDtreePlugin{}
}

// Init used to init the plugin
func (p *OceanstorASeriesDtreePlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	parentName, ok := parameters["parentname"]
	if ok {
		p.parentName, ok = parentName.(string)
		if !ok {
			return errors.New("parentname must be a string type")
		}
	}
	return p.OceanstorASeriesPlugin.Init(ctx, config, parameters, keepLogin)
}

// CreateVolume used to create volume
func (p *OceanstorASeriesDtreePlugin) CreateVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (utils.Volume, error) {
	return nil, fmt.Errorf("%s storage does not support volume management", constants.OceanStorASeriesDtree)
}

// QueryVolume used to query volume
func (p *OceanstorASeriesDtreePlugin) QueryVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	return nil, fmt.Errorf("%s storage does not support volume management", constants.OceanStorASeriesDtree)
}

// DeleteVolume used to delete volume
func (p *OceanstorASeriesDtreePlugin) DeleteVolume(ctx context.Context, name string,
	params map[string]interface{}) error {
	return fmt.Errorf("%s storage does not support volume management", constants.OceanStorASeriesDtree)
}

// ExpandVolume used to expand volume
func (p *OceanstorASeriesDtreePlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	return false, fmt.Errorf("%s storage does not support volume management", constants.OceanStorASeriesDtree)
}

// AttachVolume attach volume to node and return storage mapping info.
func (p *OceanstorASeriesDtreePlugin) AttachVolume(_ context.Context, _ string,
	parameters map[string]any) (map[string]any, error) {
	return attachDTreeVolume(parameters)
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *OceanstorASeriesDtreePlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	return getZeroPoolsCapacities(ctx, poolNames)
}

// CreateSnapshot used to create snapshot
func (p *OceanstorASeriesDtreePlugin) CreateSnapshot(ctx context.Context,
	fsName, snapshotName string, parameters map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesDtree)
}

// DeleteSnapshot used to delete snapshot
func (p *OceanstorASeriesDtreePlugin) DeleteSnapshot(ctx context.Context, snapshotParentId, snapshotName string) error {
	return fmt.Errorf("%s storage does not support snapshot feature", constants.OceanStorASeriesDtree)
}

// DeleteDTreeVolume used to delete DTree volume
func (p *OceanstorASeriesDtreePlugin) DeleteDTreeVolume(context.Context, string, string) error {
	return fmt.Errorf("%s storage does not support volume management", constants.OceanStorASeriesDtree)
}

// ExpandDTreeVolume used to expand DTree volume
func (p *OceanstorASeriesDtreePlugin) ExpandDTreeVolume(context.Context, string, string, int64) (bool, error) {
	return false, fmt.Errorf("%s storage does not support volume management", constants.OceanStorASeriesDtree)
}

// ModifyVolume used to modify volume hyperMetro status
func (p *OceanstorASeriesDtreePlugin) ModifyVolume(context.Context, string,
	pkgVolume.ModifyVolumeType, map[string]string) error {
	return fmt.Errorf("%s storage does not support volume management", constants.OceanStorASeriesDtree)
}

// GetDTreeParentName used to get dtree parent name
func (p *OceanstorASeriesDtreePlugin) GetDTreeParentName() string {
	return p.parentName
}
