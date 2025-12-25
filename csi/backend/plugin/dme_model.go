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
	"fmt"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

const snapshotDirVisible = "visible"
const snapshotDirInvisible = "invisible"

// CreateDmeVolumeParameter is the parameter for creating dme volume
type CreateDmeVolumeParameter struct {
	SnapshotDirectoryVisibility string `json:"snapshotDirectoryVisibility"`
	StoragePool                 string `json:"storagepool"`
	AuthClient                  string `json:"authClient"`
	AuthUser                    string `json:"authUser"`
	AllSquash                   string `json:"allSquash"`
	RootSquash                  string `json:"rootSquash"`
	AllocType                   string `json:"allocType"`
	Description                 string `json:"description"`
	Size                        int64  `json:"size"`
}

func (p *CreateDmeVolumeParameter) genCreateVolumeModel(name, protocol string,
	sectorSize int64) (*volume.CreateVolumeModel, error) {
	if err := p.validate(protocol); err != nil {
		return nil, err
	}

	model := &volume.CreateVolumeModel{
		SnapshotDirVisible: p.SnapshotDirectoryVisibility != snapshotDirInvisible,
		Protocol:           protocol,
		Name:               name,
		PoolName:           p.StoragePool,
		Capacity:           utils.TransVolumeCapacity(p.Size, sectorSize) * sectorSize,
		Description:        p.Description,
		AllSquash:          constants.NoAllSquashValue,
		RootSquash:         constants.NoRootSquashValue,
		AllocationType:     p.AllocType,
	}

	if p.AuthClient != "" {
		model.AuthClients = strings.Split(p.AuthClient, ";")
	}

	if p.AuthUser != "" {
		model.AuthUsers = strings.Split(p.AuthUser, ";")
	}

	if p.AllSquash == constants.AllSquash {
		model.AllSquash = constants.AllSquashValue
	}

	if p.RootSquash == constants.RootSquash {
		model.RootSquash = constants.RootSquashValue
	}

	return model, nil
}

func (p *CreateDmeVolumeParameter) validate(protocol string) error {
	if protocol == constants.ProtocolNfs && p.AuthClient == "" {
		return fmt.Errorf("authClient field in StorageClass cannot be empty when create volume with %s protocol",
			constants.ProtocolNfs)
	}

	if protocol == constants.ProtocolDtfs && p.AuthUser == "" {
		return fmt.Errorf("authUser field in StorageClass cannot be empty when create volume with %s protocol",
			constants.ProtocolDtfs)
	}

	if p.AllSquash != "" &&
		p.AllSquash != constants.AllSquash &&
		p.AllSquash != constants.NoAllSquash {
		return fmt.Errorf("if the allSquash field in StorageClass is set, it must be set to %q or %q",
			constants.AllSquash, constants.NoAllSquash)
	}

	if p.RootSquash != "" &&
		p.RootSquash != constants.RootSquash &&
		p.RootSquash != constants.NoRootSquash {
		return fmt.Errorf("if the rootSquash field in StorageClass is set, it must be set to %q or %q",
			constants.RootSquash, constants.NoRootSquash)
	}

	if p.SnapshotDirectoryVisibility != "" &&
		p.SnapshotDirectoryVisibility != snapshotDirVisible &&
		p.SnapshotDirectoryVisibility != snapshotDirInvisible {
		return fmt.Errorf("if the snapshotDirectoryVisibility field in StorageClass is set, "+
			"it must be set to %q or %q", snapshotDirVisible, snapshotDirInvisible)
	}

	return nil
}
