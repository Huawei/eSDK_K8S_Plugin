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

package plugin

import (
	"fmt"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/volume/dtree"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

// CreateDmeDTreeVolumeParameter is the parameter for creating DME DTree volume
type CreateDmeDTreeVolumeParameter struct {
	ParentName   string `json:"parentname"`
	AuthClient   string `json:"authClient"`
	AuthUser     string `json:"authUser"`
	AllSquash    string `json:"allSquash"`
	RootSquash   string `json:"rootSquash"`
	AllocType    string `json:"allocType"`
	FsPermission string `json:"fsPermission"`
	Size         int64  `json:"size"`
	VolumeType   string `json:"volumeType"`
	Description  string `json:"description"`
}

func (param *CreateDmeDTreeVolumeParameter) genCreateDTreeVolumeModel(
	dtreeName, backendParentName, protocol string, sectorSize int64,
) (*dtree.CreateDTreeVolumeModel, error) {
	if err := param.validate(protocol); err != nil {
		return nil, err
	}

	parentName, err := getValidParentname(param.ParentName, backendParentName)
	if err != nil {
		return nil, err
	}

	desc := param.Description
	if desc == "" {
		desc = constants.DefaultDescription
	}

	model := &dtree.CreateDTreeVolumeModel{
		Protocol:     protocol,
		DTreeName:    dtreeName,
		ParentName:   parentName,
		AllSquash:    constants.NoAllSquash,
		RootSquash:   constants.NoRootSquash,
		FsPermission: param.FsPermission,
		Capacity:     utils.TransVolumeCapacity(param.Size, sectorSize) * sectorSize,
		Description:  desc,
	}

	if param.AuthClient != "" {
		model.AuthClients = strings.Split(param.AuthClient, ";")
	}

	if param.AuthUser != "" {
		model.AuthUsers = strings.Split(param.AuthUser, ";")
	}

	if param.AllSquash == constants.AllSquash {
		model.AllSquash = constants.AllSquash
	}

	if param.RootSquash == constants.RootSquash {
		model.RootSquash = constants.RootSquash
	}

	return model, nil
}

func (param *CreateDmeDTreeVolumeParameter) validate(protocol string) error {
	if param.VolumeType != dtreeVolumeType {
		return fmt.Errorf("volumeType must be %q when create %s type volume",
			dtreeVolumeType, constants.OceanStorASeriesDtreeDme)
	}

	if param.AuthClient == "" && param.AuthUser == "" {
		return fmt.Errorf("authClient or authUser field in StorageClass cannot be both empty "+
			"when create %s type volume", constants.OceanStorASeriesDtreeDme)
	}

	if protocol == constants.ProtocolNfs && param.AuthClient == "" {
		return fmt.Errorf("authClient field in StorageClass cannot be empty "+
			"when create %s type volume with %s protocol",
			constants.OceanStorASeriesDtreeDme, protocol)
	}

	if protocol == constants.ProtocolDtfs && param.AuthUser == "" {
		return fmt.Errorf("authUser field in StorageClass cannot be empty "+
			"when create %s type volume with %s protocol",
			constants.OceanStorASeriesDtreeDme, protocol)
	}

	if param.AllSquash != "" &&
		param.AllSquash != constants.AllSquash &&
		param.AllSquash != constants.NoAllSquash {
		return fmt.Errorf("allSquash field in StorageClass must be set to %q or %q",
			constants.AllSquash, constants.NoAllSquash)
	}

	if param.RootSquash != "" &&
		param.RootSquash != constants.RootSquash &&
		param.RootSquash != constants.NoRootSquash {
		return fmt.Errorf("rootSquash field in StorageClass must be set to %q or %q",
			constants.RootSquash, constants.NoRootSquash)
	}

	if param.FsPermission != "" {
		if len(param.FsPermission) != fsPermissionLength {
			return fmt.Errorf("fsPermission must be a 3-character string (e.g., '755'), got: %s",
				param.FsPermission)
		}
		for i, char := range param.FsPermission {
			if char < '0' || char > '7' {
				return fmt.Errorf("fsPermission must contain digits 0-7, invalid character '%c' at position %d",
					char, i)
			}
		}
	}

	if param.AllocType != "" && param.AllocType != allocTypeThin {
		return fmt.Errorf("allocType must be thin, got: %s", param.AllocType)
	}

	if param.Size <= 0 {
		return fmt.Errorf("volume size must be greater than 0, got: %d", param.Size)
	}

	return nil
}
