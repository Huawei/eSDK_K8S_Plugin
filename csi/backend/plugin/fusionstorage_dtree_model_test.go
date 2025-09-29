/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

var (
	validCreateDTreeParams = CreateDTreeVolumeParameter{
		AccountName:  "test-account-name",
		ParentName:   "test-parent-name",
		AuthClient:   "*;other.auth.client",
		AllSquash:    constants.AllSquash,
		RootSquash:   constants.RootSquash,
		AllocType:    "thin",
		Description:  "test description",
		FsPermission: "755",
		Size:         1024 * 1024,
		VolumeType:   "dtree",
	}
	invalidVolumeTypeCreateDTreeParams = CreateDTreeVolumeParameter{
		VolumeType: "filesystem",
	}
	invalidAuthClientCreateDTreeParams = CreateDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "",
	}
	invalidAllSquashCreateDTreeParams = CreateDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "*",
		AllSquash:  "wrong",
	}
	invalidRootSquashCreateDTreeParams = CreateDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "*",
		RootSquash: "wrongSquash",
	}

	scParentname                     = "sc-parent-name"
	backendParentname                = "backend-parent-name"
	hasParentnameCreateDTreeParams   = CreateDTreeVolumeParameter{ParentName: scParentname}
	emptyParentnameCreateDTreeParams = CreateDTreeVolumeParameter{}
)

func TestCreateDTreeVolumeParameter_validate_NFS(t *testing.T) {
	// arrange
	tests := []struct {
		name    string
		param   *CreateDTreeVolumeParameter
		wantErr bool
	}{
		{name: "valid", param: &validCreateDTreeParams, wantErr: false},
		{name: "invalid volume type", param: &invalidVolumeTypeCreateDTreeParams, wantErr: true},
		{name: "invalid auth client", param: &invalidAuthClientCreateDTreeParams, wantErr: true},
		{name: "invalid all squash", param: &invalidAllSquashCreateDTreeParams, wantErr: true},
		{name: "invalid root squash", param: &invalidRootSquashCreateDTreeParams, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			err := tt.param.validate(constants.ProtocolNfs)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateDTreeVolumeParameter_validate_DPC(t *testing.T) {
	// arrange
	param := &invalidAuthClientCreateDTreeParams

	// act
	err := param.validate(constants.ProtocolDpc)

	// assert
	require.NoError(t, err)
}

func TestCreateDTreeVolumeParameter_setValidParentname(t *testing.T) {
	// arrange
	tests := []struct {
		name               string
		param              *CreateDTreeVolumeParameter
		backendParentname  string
		expectedParentname string
		wantErr            bool
	}{
		{name: "all empty", param: &emptyParentnameCreateDTreeParams, backendParentname: "", wantErr: true},
		{name: "all has value and different", param: &hasParentnameCreateDTreeParams,
			backendParentname: "backParentname", wantErr: true},
		{name: "sc has value and backend empty", param: &hasParentnameCreateDTreeParams, backendParentname: "",
			expectedParentname: scParentname, wantErr: false},
		{name: "sc empty and backend has value", param: &emptyParentnameCreateDTreeParams,
			backendParentname: backendParentname, expectedParentname: backendParentname, wantErr: false},
		{name: "has value and equal", param: &hasParentnameCreateDTreeParams,
			backendParentname: scParentname, expectedParentname: scParentname, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			parentName, err := tt.param.getValidParentname(tt.backendParentname)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedParentname, parentName)
			}
		})
	}
}
