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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/volume/dtree"
)

const (
	fsPermissionLength = 3
	allocType          = "thin"
)

// CreateASeriesDTreeVolumeParameter is the parameter for creating A-series DTree volume
type CreateASeriesDTreeVolumeParameter struct {
	ParentName   string `json:"parentname"`
	AuthClient   string `json:"authClient"`
	AuthUser     string `json:"authUser"`
	AllSquash    string `json:"allSquash"`
	RootSquash   string `json:"rootSquash"`
	AllocType    string `json:"allocType"`
	FsPermission string `json:"fsPermission"`
	Size         int64  `json:"size"`
	VolumeType   string `json:"volumeType"`
}

func (param *CreateASeriesDTreeVolumeParameter) genCreateDTreeModel(dtreeName, backendParentName string,
	protocol string) (*dtree.CreateDTreeModel, error) {
	if err := param.validate(protocol); err != nil {
		return nil, err
	}

	parentName, err := getValidParentname(param.ParentName, backendParentName)
	if err != nil {
		return nil, err
	}

	model := &dtree.CreateDTreeModel{
		Protocol:     protocol,
		DTreeName:    dtreeName,
		ParentName:   parentName,
		AllSquash:    constants.NoAllSquashValue,
		RootSquash:   constants.NoRootSquashValue,
		FsPermission: param.FsPermission,
		Capacity:     param.Size,
		AuthClients:  strings.Split(param.AuthClient, ";"),
		AuthUsers:    strings.Split(param.AuthUser, ";"),
	}

	if param.AllSquash == constants.AllSquash {
		model.AllSquash = constants.AllSquashValue
	}

	if param.RootSquash == constants.RootSquash {
		model.RootSquash = constants.RootSquashValue
	}

	return model, nil
}

func (param *CreateASeriesDTreeVolumeParameter) validate(protocol string) error {
	if param.VolumeType != dtreeVolumeType {
		return fmt.Errorf("volumeType must be %q when create %s type volume",
			dtreeVolumeType, constants.OceanStorASeriesDtree)
	}

	if protocol == constants.ProtocolNfs && param.AuthClient == "" {
		return fmt.Errorf("authClient field in StorageClass cannot be empty "+
			"when create %s type volume with %s protocol",
			constants.OceanStorASeriesDtree, protocol)
	}

	if protocol == constants.ProtocolDtfs && param.AuthUser == "" {
		return fmt.Errorf("authUser field in StorageClass cannot be empty "+
			"when create %s type volume with %s protocol",
			constants.OceanStorASeriesDtree, protocol)
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

	if param.AllocType != "" && param.AllocType != allocType {
		return fmt.Errorf("allocType must be thin, got: %s", param.AllocType)
	}

	if param.Size <= 0 {
		return fmt.Errorf("volume size must be greater than 0, got: %d", param.Size)
	}

	return nil
}
