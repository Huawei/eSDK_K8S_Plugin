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

	// init the nfs connector
	_ "huawei-csi-driver/connector/nfs"
	pkgVolume "huawei-csi-driver/pkg/volume"
	"huawei-csi-driver/utils"
)

// StoragePlugin defines storage plugin interfaces
type StoragePlugin interface {
	NewPlugin() StoragePlugin
	Init(context.Context, map[string]interface{}, map[string]interface{}, bool) error
	CreateVolume(context.Context, string, map[string]interface{}) (utils.Volume, error)
	QueryVolume(context.Context, string, map[string]interface{}) (utils.Volume, error)
	DeleteVolume(context.Context, string) error
	ExpandVolume(context.Context, string, int64) (bool, error)
	AttachVolume(context.Context, string, map[string]interface{}) (map[string]interface{}, error)
	DetachVolume(context.Context, string, map[string]interface{}) error
	ModifyVolume(context.Context, string, pkgVolume.ModifyVolumeType, map[string]string) error

	UpdateBackendCapabilities(context.Context) (map[string]interface{}, map[string]interface{}, error)
	UpdatePoolCapabilities(context.Context, []string) (map[string]interface{}, error)
	UpdateMetroRemotePlugin(context.Context, StoragePlugin)
	CreateSnapshot(context.Context, string, string) (map[string]interface{}, error)
	DeleteSnapshot(context.Context, string, string) error
	SmartXQoSQuery
	Logout(context.Context)
	// Validate used to check parameters, include login verification
	Validate(context.Context, map[string]interface{}) error

	DeleteDTreeVolume(context.Context, map[string]interface{}) error
	ExpandDTreeVolume(context.Context, map[string]interface{}) (bool, error)

	// SetOnline sets the online status of plugin
	SetOnline(bool)
	// GetOnline gets the online status of plugin
	GetOnline() bool
}

// SmartXQoSQuery provides Quality of Service(QoS) Query operations
type SmartXQoSQuery interface {
	// SupportQoSParameters checks requested QoS parameters support by StoragePlugin
	SupportQoSParameters(ctx context.Context, qos string) error
}

var (
	plugins = map[string]StoragePlugin{}
)

const (
	// SectorSize means Sector size
	SectorSize int64 = 512
)

// RegPlugin used to register plugin
func RegPlugin(storageType string, plugin StoragePlugin) {
	plugins[storageType] = plugin
}

// GetPlugin used to get plugin by storage type
func GetPlugin(storageType string) StoragePlugin {
	if plugin, exist := plugins[storageType]; exist {
		return plugin.NewPlugin()
	}

	return nil
}

type basePlugin struct {
	name   string
	online bool
}

func (p *basePlugin) AttachVolume(context.Context, string, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (p *basePlugin) DetachVolume(context.Context, string, map[string]interface{}) error {
	return nil
}

func (p *basePlugin) UpdateMetroRemotePlugin(context.Context, StoragePlugin) {
}

// SetOnline sets the online status of plugin
func (p *basePlugin) SetOnline(online bool) {
	p.online = online
}

// GetOnline gets the online status of plugin
func (p *basePlugin) GetOnline() bool {
	return p.online
}
