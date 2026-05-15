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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

// global test constants var
var (
	validCreateASeriesDTreeParams = CreateASeriesDTreeVolumeParameter{
		ParentName:   "test-parent-name",
		AuthClient:   "*;other.auth.client",
		AllSquash:    constants.AllSquash,
		RootSquash:   constants.RootSquash,
		FsPermission: "755",
		Size:         1024 * 1024,
		VolumeType:   "dtree",
	}
	invalidVolumeTypeASeriesDTreeParams = CreateASeriesDTreeVolumeParameter{
		VolumeType: "filesystem",
	}
	// Fix: Add explicit size to avoid size validation error
	invalidAuthClientASeriesDTreeParams = CreateASeriesDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "",
		Size:       1024 * 1024, // Add valid size
	}
	invalidAllSquashASeriesDTreeParams = CreateASeriesDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "*",
		AllSquash:  "wrong",
		Size:       1024 * 1024, // Add valid size
	}
	invalidRootSquashASeriesDTreeParams = CreateASeriesDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "*",
		RootSquash: "wrongSquash",
		Size:       1024 * 1024, // Add valid size
	}

	aSeriesScParentname                     = "sc-parent-name"
	aSeriesBackendParentname                = "backend-parent-name"
	hasParentnameCreateASeriesDTreeParams   = CreateASeriesDTreeVolumeParameter{ParentName: aSeriesScParentname}
	emptyParentnameCreateASeriesDTreeParams = CreateASeriesDTreeVolumeParameter{}
)

func TestCreateASeriesDTreeVolumeParameter_validate_NFS(t *testing.T) {
	// arrange
	tests := []struct {
		name    string
		param   *CreateASeriesDTreeVolumeParameter
		wantErr bool
	}{
		{name: "valid", param: &validCreateASeriesDTreeParams, wantErr: false},
		{name: "invalid volume type", param: &invalidVolumeTypeASeriesDTreeParams, wantErr: true},
		{name: "invalid auth client", param: &invalidAuthClientASeriesDTreeParams, wantErr: true},
		{name: "invalid all squash", param: &invalidAllSquashASeriesDTreeParams, wantErr: true},
		{name: "invalid root squash", param: &invalidRootSquashASeriesDTreeParams, wantErr: true},
	}

	// action & assert
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.param.validate(constants.ProtocolNfs)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateASeriesDTreeVolumeParameter_setValidParentname(t *testing.T) {
	// arrange
	tests := []struct {
		name               string
		param              *CreateASeriesDTreeVolumeParameter
		backendParentname  string
		expectedParentname string
		wantErr            bool
	}{
		{name: "all empty", param: &emptyParentnameCreateASeriesDTreeParams, backendParentname: "", wantErr: true},
		{name: "all has value and different", param: &hasParentnameCreateASeriesDTreeParams,
			backendParentname: aSeriesBackendParentname, wantErr: true},
		{name: "sc has value and backend empty", param: &hasParentnameCreateASeriesDTreeParams, backendParentname: "",
			expectedParentname: aSeriesScParentname, wantErr: false},
		{name: "sc empty and backend has value", param: &emptyParentnameCreateASeriesDTreeParams,
			backendParentname: aSeriesBackendParentname, expectedParentname: aSeriesBackendParentname, wantErr: false},
		{name: "has value and equal", param: &hasParentnameCreateASeriesDTreeParams,
			backendParentname: aSeriesScParentname, expectedParentname: aSeriesScParentname, wantErr: false},
	}

	// action & assert
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parentName, err := getValidParentname(tt.param.ParentName, tt.backendParentname)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedParentname, parentName)
			}
		})
	}
}

// TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_Success tests successful DTree model generation
func TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_Success(t *testing.T) {
	// arrange
	param := &CreateASeriesDTreeVolumeParameter{
		ParentName:   "backend-parent", // Fix: Use same parentname as backend
		Size:         1073741824,       // 1GB
		AllocType:    "thin",
		VolumeType:   "dtree",
		AuthClient:   "*;other.auth.client",
		AllSquash:    constants.AllSquash,
		RootSquash:   constants.RootSquash,
		FsPermission: "755",
	}

	dtreeName := "test-dtree"

	// action
	model, err := param.genCreateDTreeModel(dtreeName, "backend-parent", "nfs") // Fix: Use same parentname

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, dtreeName, model.DTreeName)
	assert.Equal(t, "backend-parent", model.ParentName)
	assert.Equal(t, "nfs", model.Protocol)
	assert.Equal(t, int64(1073741824), model.Capacity)
}

// TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_InvalidSize tests model generation with invalid size
func TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_InvalidSize(t *testing.T) {
	// arrange
	param := &CreateASeriesDTreeVolumeParameter{
		ParentName: "test-parent",
		Size:       -1, // Invalid negative size
		VolumeType: "dtree",
		AuthClient: "*", // Required for NFS protocol
	}

	dtreeName := "test-dtree"

	// action
	model, err := param.genCreateDTreeModel(dtreeName, "test-parent", "nfs")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "volume size must be greater than 0")
	assert.Contains(t, err.Error(), "-1")
	assert.Nil(t, model)
}

// TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_ZeroSize tests model generation with zero size
func TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_ZeroSize(t *testing.T) {
	// arrange
	param := &CreateASeriesDTreeVolumeParameter{
		ParentName: "test-parent",
		Size:       0, // Invalid zero size
		VolumeType: "dtree",
		AuthClient: "*", // Required for NFS protocol
	}

	dtreeName := "test-dtree"

	// action
	model, err := param.genCreateDTreeModel(dtreeName, "test-parent", "nfs")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "volume size must be greater than 0")
	assert.Contains(t, err.Error(), "0")
	assert.Nil(t, model)
}

// TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_InvalidVolumeType tests invalid volume type
func TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_InvalidVolumeType(t *testing.T) {
	// arrange
	param := &CreateASeriesDTreeVolumeParameter{
		VolumeType: "filesystem",
		AuthClient: "*",         // Required for NFS protocol
		Size:       1024 * 1024, // Add valid size
	}

	dtreeName := "test-dtree"

	// action
	model, err := param.genCreateDTreeModel(dtreeName, "backend-parent", "nfs")

	// assert
	assert.Error(t, err)
	// Fix: Match the exact error message
	assert.Contains(t, err.Error(), "volumeType must be")
	assert.Contains(t, err.Error(), "\"dtree\"")
	assert.Nil(t, model)
}

// TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_InvalidAuthClientForNFS tests empty auth client for NFS
func TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_InvalidAuthClientForNFS(t *testing.T) {
	// arrange
	param := &CreateASeriesDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "",          // Empty auth client for NFS should fail
		Size:       1024 * 1024, // Add valid size
	}

	dtreeName := "test-dtree"

	// action
	model, err := param.genCreateDTreeModel(dtreeName, "backend-parent", "nfs")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authClient field in StorageClass cannot be empty")
	assert.Nil(t, model)
}

// TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_InvalidParentname tests invalid parentname
func TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_InvalidParentname(t *testing.T) {
	// Fix: Use valid size and different parentnames to trigger the correct error
	// arrange
	param := &CreateASeriesDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "*",         // Required for NFS protocol
		ParentName: "sc-parent", // SC parentname
		Size:       1024 * 1024, // Valid size to trigger parentname validation first
	}

	dtreeName := "test-dtree"
	backendParentName := "backend-parent" // Different backend parentname

	// action
	model, err := param.genCreateDTreeModel(dtreeName, backendParentName, "nfs")

	// assert
	assert.Error(t, err)
	// Fix: Match the actual error message format
	assert.Contains(t, err.Error(), "parentname")
	assert.Contains(t, err.Error(), "sc-parent")
	assert.Contains(t, err.Error(), "backend-parent")
	assert.Contains(t, err.Error(), "is not equal to")
	assert.Nil(t, model)
}

// TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_NoParentnameProvided tests no parentname provided
func TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_NoParentnameProvided(t *testing.T) {
	// arrange
	param := &CreateASeriesDTreeVolumeParameter{
		VolumeType: "dtree",
		AuthClient: "*",         // Required for NFS protocol
		Size:       1024 * 1024, // Valid size
	}

	dtreeName := "test-dtree"
	backendParentName := "" // No backend parentname

	// action
	model, err := param.genCreateDTreeModel(dtreeName, backendParentName, "nfs")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parentname")
	assert.Contains(t, err.Error(), "StorageClass")
	assert.Contains(t, err.Error(), "backend")
	assert.Nil(t, model)
}

// TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_EmptyFsPermission tests empty fs permission
func TestCreateASeriesDTreeVolumeParameter_genCreateDTreeModel_EmptyFsPermission(t *testing.T) {
	// arrange
	param := &CreateASeriesDTreeVolumeParameter{
		ParentName:   "backend-parent", // Fix: Use same parentname
		Size:         1024 * 1024,
		VolumeType:   "dtree",
		AuthClient:   "*",
		FsPermission: "", // Empty fs permission should be allowed
	}

	dtreeName := "test-dtree"

	// action
	model, err := param.genCreateDTreeModel(dtreeName, "backend-parent", "nfs") // Fix: Use same parentname

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, "", model.FsPermission)
}
