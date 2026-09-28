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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

// --- genCreateVolumeModel integration tests ---

func TestCreateDmeVolumeParameter_genCreateVolumeModel_GlobalNfsSuccess(t *testing.T) {
	// arrange
	param := &CreateDmeVolumeParameter{AuthClient: "test1;test2",
		AllSquash: constants.AllSquash, RootSquash: constants.NoRootSquash}

	// act
	model, err := param.genCreateVolumeModel("test", constants.ProtocolNfs, SectorSize, false)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, constants.AllSquashValue, model.AllSquash)
	assert.Equal(t, constants.NoRootSquashValue, model.RootSquash)
}

func TestCreateDmeVolumeParameter_genCreateVolumeModel_GlobalDtfsError(t *testing.T) {
	// arrange
	param := &CreateDmeVolumeParameter{}

	// act
	model, err := param.genCreateVolumeModel("test", constants.ProtocolDtfs, SectorSize, false)

	// assert
	assert.Error(t, err)
	assert.Nil(t, model)
	assert.True(t, strings.Contains(err.Error(), constants.ProtocolDtfs))
}

func TestCreateDmeVolumeParameter_GenCreateVolumeModel_WithKVCache(t *testing.T) {
	param := &CreateDmeVolumeParameter{
		StoragePool:       "pool1",
		EnableKVCache:     "true",
		EnableTimeAwareGc: "true",
		GcTimeThreshold:   "1",
	}
	model, err := param.genCreateVolumeModel("test", constants.ProtocolNfs, 512, true)
	assert.NoError(t, err)
	assert.True(t, model.EnableKVCache)
	assert.True(t, model.EnableTimeAwareGC)
	assert.Equal(t, int64(1), model.GCTimeThreshold)
}

func TestCreateDmeVolumeParameter_GenCreateVolumeModel_KVCacheValidation(t *testing.T) {
	param := &CreateDmeVolumeParameter{
		StoragePool:       "pool1",
		EnableKVCache:     "true",
		EnableTimeAwareGc: "true",
		// GcTimeThreshold missing
	}
	_, err := param.genCreateVolumeModel("test", constants.ProtocolNfs, 512, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gcTimeThreshold")
}

func TestCreateDmeVolumeParameter_GenCreateVolumeModel_KVCacheNfsNoAuthClient(t *testing.T) {
	// KVCache mode: NFS without authClient should succeed (DME handles auth internally)
	param := &CreateDmeVolumeParameter{
		StoragePool:   "pool1",
		EnableKVCache: "true",
	}
	model, err := param.genCreateVolumeModel("test", constants.ProtocolNfs, 512, true)
	assert.NoError(t, err)
	assert.True(t, model.EnableKVCache)
}

// --- validateCommon tests ---

func TestCreateDmeVolumeParameter_validateCommon_AllSquashError(t *testing.T) {
	param := &CreateDmeVolumeParameter{AuthClient: "test", AllSquash: "test"}
	err := param.validateCommon()
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), constants.AllSquash))
}

func TestCreateDmeVolumeParameter_validateCommon_RootSquashError(t *testing.T) {
	param := &CreateDmeVolumeParameter{AuthClient: "test", RootSquash: "test"}
	err := param.validateCommon()
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), constants.RootSquash))
}

func TestCreateDmeVolumeParameter_validateCommon_SnapshotDirVisibilityError(t *testing.T) {
	param := &CreateDmeVolumeParameter{SnapshotDirectoryVisibility: "invalid"}
	err := param.validateCommon()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "snapshotDirectoryVisibility")
}

func TestCreateDmeVolumeParameter_validateCommon_Success(t *testing.T) {
	param := &CreateDmeVolumeParameter{AuthClient: "test"}
	err := param.validateCommon()
	assert.NoError(t, err)
}

// --- validateGlobal tests ---

func TestCreateDmeVolumeParameter_validateGlobal_NfsNoAuthClient(t *testing.T) {
	param := &CreateDmeVolumeParameter{}
	err := param.validateGlobal(constants.ProtocolNfs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), constants.ProtocolNfs)
}

func TestCreateDmeVolumeParameter_validateGlobal_NfsWithAuthClient(t *testing.T) {
	param := &CreateDmeVolumeParameter{AuthClient: "client1"}
	err := param.validateGlobal(constants.ProtocolNfs)
	assert.NoError(t, err)
}

func TestCreateDmeVolumeParameter_validateGlobal_DtfsNoAuthUser(t *testing.T) {
	param := &CreateDmeVolumeParameter{}
	err := param.validateGlobal(constants.ProtocolDtfs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), constants.ProtocolDtfs)
}

func TestCreateDmeVolumeParameter_validateGlobal_DtfsWithAuthUser(t *testing.T) {
	param := &CreateDmeVolumeParameter{AuthUser: "user1"}
	err := param.validateGlobal(constants.ProtocolDtfs)
	assert.NoError(t, err)
}

// --- validateLocal tests ---

func TestCreateDmeVolumeParameter_validateLocal_NfsNoAuthClient(t *testing.T) {
	// KVCache mode: NFS without authClient is OK
	param := &CreateDmeVolumeParameter{EnableKVCache: "true"}
	err := param.validateLocal(constants.ProtocolNfs)
	assert.NoError(t, err)
}

func TestCreateDmeVolumeParameter_validateLocal_DtfsNoAuthUser(t *testing.T) {
	param := &CreateDmeVolumeParameter{EnableKVCache: "true"}
	err := param.validateLocal(constants.ProtocolDtfs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), constants.ProtocolDtfs)
}

func TestCreateDmeVolumeParameter_validateLocal_GcTimeThresholdEmpty(t *testing.T) {
	param := &CreateDmeVolumeParameter{
		EnableKVCache:     "true",
		EnableTimeAwareGc: "true",
		// GcTimeThreshold is empty
	}
	err := param.validateLocal(constants.ProtocolNfs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gcTimeThreshold must be provided")
}

func TestCreateDmeVolumeParameter_validateLocal_GcTimeThresholdProvided(t *testing.T) {
	param := &CreateDmeVolumeParameter{
		EnableKVCache:     "true",
		EnableTimeAwareGc: "true",
		GcTimeThreshold:   "1",
	}
	err := param.validateLocal(constants.ProtocolNfs)
	assert.NoError(t, err)
}

func TestCreateDmeVolumeParameter_validateLocal_GcDisabled(t *testing.T) {
	param := &CreateDmeVolumeParameter{
		EnableKVCache:     "true",
		EnableTimeAwareGc: "false",
		// GcTimeThreshold not needed when GC is disabled
	}
	err := param.validateLocal(constants.ProtocolNfs)
	assert.NoError(t, err)
}
