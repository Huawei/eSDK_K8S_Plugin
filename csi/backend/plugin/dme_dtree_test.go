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
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	dmeDtree "github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/volume/dtree"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestDMEASeriesDtreePlugin_Init_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolNfs,
		"parentname": "fakeParentName",
		"portals":    []interface{}{"10.0.0.1"},
	}
	config := map[string]interface{}{
		"urls":             []interface{}{"https://127.0.0.1:8088"},
		"user":             "test_user",
		"storageDeviceSN":  "test_sn",
		"secretName":       "test_secret",
		"secretNamespace":  "default",
		"backendID":        "test_id",
		"maxClientThreads": "30",
		"storage":          constants.OceanStorASeriesDtreeDme,
		"name":             "test_name",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn((*client.BaseClient)(nil), "Login", nil)
	mock.ApplyMethod((*client.BaseClient)(nil), "Logout",
		func(cli *client.BaseClient, ctx context.Context) {})
	mock.ApplyMethodReturn((*client.BaseClient)(nil), "SetSystemInfo", nil)

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, "fakeParentName", p.parentName)
	assert.Equal(t, constants.ProtocolNfs, p.protocol)
}

func TestDMEASeriesDtreePlugin_Init_NoParentName(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolNfs,
		"portals":  []interface{}{"10.0.0.1"},
	}
	config := map[string]interface{}{
		"urls":             []interface{}{"https://127.0.0.1:8088"},
		"user":             "test_user",
		"storageDeviceSN":  "test_sn",
		"secretName":       "test_secret",
		"secretNamespace":  "default",
		"backendID":        "test_id",
		"maxClientThreads": "30",
		"storage":          constants.OceanStorASeriesDtreeDme,
		"name":             "test_name",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn((*client.BaseClient)(nil), "Login", nil)
	mock.ApplyMethod((*client.BaseClient)(nil), "Logout",
		func(cli *client.BaseClient, ctx context.Context) {})
	mock.ApplyMethodReturn((*client.BaseClient)(nil), "SetSystemInfo", nil)

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, "", p.parentName)
}

func TestDMEASeriesDtreePlugin_Init_InvalidParentNameType(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolNfs,
		"parentname": 123,
		"portals":    []interface{}{"10.0.0.1"},
	}
	config := map[string]interface{}{
		"urls":             []interface{}{"https://127.0.0.1:8088"},
		"user":             "test_user",
		"storageDeviceSN":  "test_sn",
		"secretName":       "test_secret",
		"secretNamespace":  "default",
		"backendID":        "test_id",
		"maxClientThreads": "30",
		"storage":          constants.OceanStorASeriesDtreeDme,
		"name":             "test_name",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn((*client.BaseClient)(nil), "Login", nil)
	mock.ApplyMethod((*client.BaseClient)(nil), "Logout",
		func(cli *client.BaseClient, ctx context.Context) {})
	mock.ApplyMethodReturn((*client.BaseClient)(nil), "SetSystemInfo", nil)

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), "parentName must be a string type")
}

func TestDMEASeriesDtreePlugin_Init_LoginError(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolNfs,
		"parentname": "fakeParentName",
		"portals":    []interface{}{"10.0.0.1"},
	}
	config := map[string]interface{}{
		"urls":             []interface{}{"https://127.0.0.1:8088"},
		"user":             "test_user",
		"storageDeviceSN":  "test_sn",
		"secretName":       "test_secret",
		"secretNamespace":  "default",
		"backendID":        "test_id",
		"maxClientThreads": "30",
		"storage":          constants.OceanStorASeriesDtreeDme,
		"name":             "test_name",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn((*client.BaseClient)(nil), "Login", errors.New("login failed"))

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), "init DME DTree plugin failed")
	assert.Contains(t, gotErr.Error(), "login failed")
}

func TestDMEASeriesDtreePlugin_UpdateBackendCapabilities_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.protocol = constants.ProtocolNfs

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&p.DMEASeriesPlugin, "UpdateBackendCapabilities",
		map[string]interface{}{
			string(constants.SupportApplicationType): true,
			string(constants.SupportQoS):             true,
			string(constants.SupportThick):           true,
		}, map[string]interface{}{}, nil)

	// act
	capabilities, _, err := p.UpdateBackendCapabilities(context.Background())

	// assert
	assert.NoError(t, err)
	assert.Equal(t, false, capabilities[string(constants.SupportApplicationType)])
	assert.Equal(t, false, capabilities[string(constants.SupportQoS)])
	assert.Equal(t, false, capabilities[string(constants.SupportThick)])
	assert.Equal(t, true, capabilities[string(constants.SupportQuota)])
}

func TestDMEASeriesDtreePlugin_UpdateBackendCapabilities_ParentError(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.protocol = constants.ProtocolNfs

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&p.DMEASeriesPlugin, "UpdateBackendCapabilities",
		nil, nil, errors.New("backend error"))

	// act
	_, _, err := p.UpdateBackendCapabilities(context.Background())

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backend error")
}

func TestDMEASeriesDtreePlugin_UpdatePoolCapabilities(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	poolNames := []string{"pool1", "pool2"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(getZeroPoolsCapacities, map[string]interface{}{
		"pool1": map[string]interface{}{"FreeCapacity": int64(0), "UsedCapacity": int64(0), "TotalCapacity": int64(0)},
		"pool2": map[string]interface{}{"FreeCapacity": int64(0), "UsedCapacity": int64(0), "TotalCapacity": int64(0)},
	}, nil)

	// act
	result, err := p.UpdatePoolCapabilities(context.Background(), poolNames)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result))
}

func TestDMEASeriesDtreePlugin_UpdatePoolCapabilities_EmptyPools(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	// act
	result, err := p.UpdatePoolCapabilities(context.Background(), []string{})

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result))
}

func TestDMEASeriesDtreePlugin_Validate_VerifyDTreeParamError(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.protocol = constants.ProtocolNfs

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(verifyDTreeParam, errors.New("dtree verify failed"))

	// act
	err := p.Validate(context.Background(), map[string]interface{}{
		"protocol":   constants.ProtocolNfs,
		"parentname": "testParent",
	})

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dtree verify failed")
}

func TestDMEASeriesDtreePlugin_Validate_ZoneSNConfigured(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.protocol = constants.ProtocolNfs

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(verifyDTreeParam, nil)

	// act
	err := p.Validate(context.Background(), map[string]interface{}{
		"protocol":          constants.ProtocolNfs,
		"parentname":        "testParent",
		constants.ZoneSNKey: "ZONE-SN-001",
	})

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zoneSN [ZONE-SN-001] is configured in backend")
	assert.Contains(t, err.Error(), "DTree does not support zone (local mode)")
}

func TestDMEASeriesDtreePlugin_CreateVolume_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.protocol = constants.ProtocolNfs
	p.parentName = "testParent"

	parameters := map[string]interface{}{
		"parentname":   "testParent",
		"authClient":   "10.0.0.1",
		"volumeType":   "dtree",
		"size":         int64(1073741824),
		"fsPermission": "755",
	}

	fakeVol := utils.NewVolume("testVolume")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(getVolumeNameFromPVNameOrParameters, "testVolume", nil)
	mock.ApplyFuncReturn(dmeDtree.NewCreator, &dmeDtree.Creator{})
	mock.ApplyMethodReturn((*dmeDtree.Creator)(nil), "Create", fakeVol, nil)

	// act
	vol, err := p.CreateVolume(context.Background(), "testVolume", parameters)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, "testVolume", vol.GetVolumeName())
}

func TestDMEASeriesDtreePlugin_CreateVolume_ParameterConversionError(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.protocol = constants.ProtocolNfs

	parameters := map[string]interface{}{
		"parentname": "testParent",
		"size":       "invalidSize",
		"volumeType": "dtree",
	}

	// act
	volume, err := p.CreateVolume(context.Background(), "testVolume", parameters)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "convert parameters to struct failed")
	assert.Nil(t, volume)
}

func TestDMEASeriesDtreePlugin_CreateVolume_GenModelError(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.protocol = constants.ProtocolNfs
	p.parentName = "testParent"

	parameters := map[string]interface{}{
		"parentname": "testParent",
		"volumeType": "invalid",
		"size":       int64(1073741824),
		"authClient": "10.0.0.1",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(getVolumeNameFromPVNameOrParameters, "testVolume", nil)

	// act
	volume, err := p.CreateVolume(context.Background(), "testVolume", parameters)

	// assert
	assert.Error(t, err)
	assert.Nil(t, volume)
}

func TestDMEASeriesDtreePlugin_CreateVolume_GetVolumeNameError(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.protocol = constants.ProtocolNfs

	parameters := map[string]interface{}{
		"parentname": "testParent",
		"volumeType": "dtree",
		"size":       int64(1073741824),
		"authClient": "10.0.0.1",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(getVolumeNameFromPVNameOrParameters, "", errors.New("name error"))

	// act
	volume, err := p.CreateVolume(context.Background(), "testVolume", parameters)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name error")
	assert.Nil(t, volume)
}

func TestDMEASeriesDtreePlugin_QueryVolume_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.parentName = "testParent"

	fakeVol := utils.NewVolume("testVolume")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(dmeDtree.NewQuerier, &dmeDtree.Querier{})
	mock.ApplyMethodReturn((*dmeDtree.Querier)(nil), "Query", fakeVol, nil)

	parameters := map[string]interface{}{}

	// act
	vol, err := p.QueryVolume(context.Background(), "testVolume", parameters)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, "testVolume", vol.GetVolumeName())
}

func TestDMEASeriesDtreePlugin_QueryVolume_UsesBackendParentName(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	p.parentName = "backendParent"

	fakeVol := utils.NewVolume("testVolume")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(dmeDtree.NewQuerier, &dmeDtree.Querier{})
	mock.ApplyMethodReturn((*dmeDtree.Querier)(nil), "Query", fakeVol, nil)

	// SC has no parentname — should use backend parentName
	parameters := map[string]interface{}{}

	// act
	vol, err := p.QueryVolume(context.Background(), "testVolume", parameters)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
}

func TestDMEASeriesDtreePlugin_DeleteVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	// act
	err := p.DeleteVolume(context.Background(), "testVolume", map[string]interface{}{})

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented, use DeleteDTreeVolume instead")
}

func TestDMEASeriesDtreePlugin_DeleteDTreeVolume_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(dmeDtree.NewDeleter, &dmeDtree.Deleter{})
	mock.ApplyFuncReturn((*dmeDtree.Deleter).Delete, nil)

	// act
	err := p.DeleteDTreeVolume(context.Background(), "testDTree", "testParent")

	// assert
	assert.NoError(t, err)
}

func TestDMEASeriesDtreePlugin_ExpandVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	// act
	result, err := p.ExpandVolume(context.Background(), "testVolume", 1024)

	// assert
	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented, use ExpandDTreeVolume instead")
}

func TestDMEASeriesDtreePlugin_ExpandDTreeVolume_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(dmeDtree.NewExpander, &dmeDtree.Expander{})
	mock.ApplyFuncReturn((*dmeDtree.Expander).Expand, nil)

	// act
	result, err := p.ExpandDTreeVolume(context.Background(), "testDTree", "testParent", 2048)

	// assert
	assert.NoError(t, err)
	assert.False(t, result)
}

func TestDMEASeriesDtreePlugin_AttachVolume_WithoutParentKey(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	parameters := map[string]interface{}{
		"volumeContext": map[string]string{},
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(attachDTreeVolume, map[string]interface{}{}, nil)

	// act
	result, err := p.AttachVolume(context.Background(), "node", parameters)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{}, result)
}

func TestDMEASeriesDtreePlugin_AttachVolume_NoVolumeContext(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}
	parameters := map[string]interface{}{}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(attachDTreeVolume, map[string]interface{}{}, nil)

	// act
	result, err := p.AttachVolume(context.Background(), "node", parameters)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{}, result)
}

func TestDMEASeriesDtreePlugin_CreateSnapshot(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	// act
	result, err := p.CreateSnapshot(context.Background(), "fs", "snap", nil)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support snapshot feature")
	assert.Nil(t, result)
}

func TestDMEASeriesDtreePlugin_DeleteSnapshot(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	// act
	err := p.DeleteSnapshot(context.Background(), "parentId", "snapName")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support snapshot feature")
}

func TestDMEASeriesDtreePlugin_ModifyVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	// act
	err := p.ModifyVolume(context.Background(), "vol", pkgVolume.Local2HyperMetro, nil)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not support volume modification")
}

func TestDMEASeriesDtreePlugin_NewPlugin(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	// act
	newPlugin := p.NewPlugin()

	// assert
	assert.NotNil(t, newPlugin)
	assert.IsType(t, &DMEASeriesDtreePlugin{}, newPlugin)
}

func TestDMEASeriesDtreePlugin_GetDTreeParentName(t *testing.T) {
	tests := []struct {
		name         string
		parentName   string
		expectedName string
	}{
		{"with parent name", "testParentName", "testParentName"},
		{"empty parent name", "", ""},
	}

	for _, tt := range tests {
		p := &DMEASeriesDtreePlugin{
			parentName: tt.parentName,
		}
		actualName := p.GetDTreeParentName()
		assert.Equal(t, tt.expectedName, actualName)
	}
}

func TestDMEASeriesDtreePlugin_GetSectorSize(t *testing.T) {
	// arrange
	p := &DMEASeriesDtreePlugin{}

	// act
	sectorSize := p.GetSectorSize()

	// assert
	assert.Equal(t, constants.ASeriesDTreeCapacityUnit, sectorSize)
	assert.Equal(t, int64(1), sectorSize)
}
