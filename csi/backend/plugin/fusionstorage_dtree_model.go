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

package plugin

import (
	"fmt"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/volume/dtree"
)

const (
	dtreeVolumeType = "dtree"
)

// CreateDTreeVolumeParameter is the parameter for creating volume
type CreateDTreeVolumeParameter struct {
	AccountName  string `json:"accountName"`
	ParentName   string `json:"parentname"`
	AuthClient   string `json:"authClient"`
	AllSquash    string `json:"allSquash"`
	RootSquash   string `json:"rootSquash"`
	AllocType    string `json:"allocType"`
	Description  string `json:"description"`
	FsPermission string `json:"fsPermission"`
	Size         int64  `json:"size"`
	VolumeType   string `json:"volumeType"`
}

func (param *CreateDTreeVolumeParameter) genCreateDTreeModel(dtreeName, backendParentName string,
	protocol string) (*dtree.CreateDTreeModel, error) {
	if err := param.validate(); err != nil {
		return nil, err
	}

	parentname, err := param.getValidParentname(backendParentName)
	if err != nil {
		return nil, err
	}

	model := &dtree.CreateDTreeModel{
		Protocol:     protocol,
		DTreeName:    dtreeName,
		ParentName:   parentname,
		AllSquash:    constants.AllSquashValue,
		RootSquash:   constants.RootSquashValue,
		Description:  param.Description,
		FsPermission: param.FsPermission,
		Capacity:     param.Size,
		AuthClients:  strings.Split(param.AuthClient, ";"),
	}

	if param.AllSquash == constants.NoAllSquash {
		model.AllSquash = constants.NoAllSquashValue
	}

	if param.RootSquash == constants.NoRootSquash {
		model.RootSquash = constants.NoRootSquashValue
	}

	return model, nil
}

func (param *CreateDTreeVolumeParameter) validate() error {
	if param.VolumeType != dtreeVolumeType {
		return fmt.Errorf("volumeType must be %q when create %s type volume", dtreeVolumeType, constants.FusionDTree)
	}

	if param.AuthClient == "" {
		return fmt.Errorf("authClient field in StorageClass cannot be empty when create %s type volume",
			constants.FusionDTree)
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

	return nil
}

func (param *CreateDTreeVolumeParameter) getValidParentname(bkParentname string) (string, error) {
	return getValidParentname(param.ParentName, bkParentname)
}
