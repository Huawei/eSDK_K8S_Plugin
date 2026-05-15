/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.com/licenses/LICENSE-2.0
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
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/volume/dtree"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
)

// TestOceanstorASeriesDtreePlugin_Init_Success tests successful plugin initialization
func TestOceanstorASeriesDtreePlugin_Init_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolDtfs,
		"parentname": "fakeParentName",
	}
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanASeriesClient{}, nil).
		ApplyMethodReturn(&base.RestClient{}, "Login", nil).
		ApplyMethodReturn(&client.OceanASeriesClient{}, "SetSystemInfo", nil).
		ApplyMethodReturn(&base.RestClient{}, "Logout")

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, "fakeParentName", p.parentName)
	assert.Equal(t, constants.ProtocolDtfs, p.protocol)
}

// TestOceanstorASeriesDtreePlugin_Init_NoParentName tests initialization with empty parentname
func TestOceanstorASeriesDtreePlugin_Init_NoParentName(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolDtfs,
		// parentname not provided
	}
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanASeriesClient{}, nil).
		ApplyMethodReturn(&base.RestClient{}, "Login", nil).
		ApplyMethodReturn(&client.OceanASeriesClient{}, "SetSystemInfo", nil).
		ApplyMethodReturn(&base.RestClient{}, "Logout")

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, "", p.parentName)
	assert.Equal(t, constants.ProtocolDtfs, p.protocol)
}

// TestOceanstorASeriesDtreePlugin_Init_InvalidParentNameType tests validation of parentname parameter type
func TestOceanstorASeriesDtreePlugin_Init_InvalidParentNameType(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolDtfs,
		"parentname": 123, // Invalid type: should be string
	}
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanASeriesClient{}, nil).
		ApplyMethodReturn(&base.RestClient{}, "Login", nil).
		ApplyMethodReturn(&client.OceanASeriesClient{}, "SetSystemInfo", nil).
		ApplyMethodReturn(&base.RestClient{}, "Logout")

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), "parentName must be a string type")
}

// TestOceanstorASeriesDtreePlugin_Init_ClientCreationError tests client creation failure
func TestOceanstorASeriesDtreePlugin_Init_ClientCreationError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolDtfs,
		"parentname": "fakeParentName",
	}
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, nil, errors.New("client creation failed"))

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), "init A-series dtree plugin failed")
	assert.Contains(t, gotErr.Error(), "client creation failed")
}

// TestOceanstorASeriesDtreePlugin_Init_LoginError tests login failure
func TestOceanstorASeriesDtreePlugin_Init_LoginError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolDtfs,
		"parentname": "fakeParentName",
	}
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanASeriesClient{}, nil).
		ApplyMethodReturn(&base.RestClient{}, "Login", errors.New("login failed"))

	// act
	gotErr := p.Init(context.Background(), config, params, false)

	// assert
	assert.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), "init A-series dtree plugin failed")
	assert.Contains(t, gotErr.Error(), "login failed")
}

// TestOceanstorASeriesDtreePlugin_UpdateBackendCapabilities_ParentError tests parent method error
func TestOceanstorASeriesDtreePlugin_UpdateBackendCapabilities_ParentError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	p.protocol = constants.ProtocolDtfs

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&p.OceanstorASeriesPlugin, "UpdateBackendCapabilities",
		nil, nil, errors.New("backend error"))

	// act
	_, _, err := p.UpdateBackendCapabilities(context.Background())

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backend error")
}

// TestOceanstorASeriesDtreePlugin_UpdatePoolCapabilities tests pool capabilities update
func TestOceanstorASeriesDtreePlugin_UpdatePoolCapabilities(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	poolNames := []string{"pool1", "pool2", "pool3"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(getZeroPoolsCapacities, map[string]interface{}{
		"pool1": map[string]interface{}{"FreeCapacity": int64(0), "UsedCapacity": int64(0), "TotalCapacity": int64(0)},
		"pool2": map[string]interface{}{"FreeCapacity": int64(0), "UsedCapacity": int64(0), "TotalCapacity": int64(0)},
		"pool3": map[string]interface{}{"FreeCapacity": int64(0), "UsedCapacity": int64(0), "TotalCapacity": int64(0)},
	}, nil)

	// act
	result, err := p.UpdatePoolCapabilities(context.Background(), poolNames)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result))
	assert.Equal(t, int64(0), result["pool1"].(map[string]interface{})["FreeCapacity"])
	assert.Equal(t, int64(0), result["pool1"].(map[string]interface{})["UsedCapacity"])
	assert.Equal(t, int64(0), result["pool1"].(map[string]interface{})["TotalCapacity"])
}

// TestOceanstorASeriesDtreePlugin_UpdatePoolCapabilities_EmptyPools tests empty pool names
func TestOceanstorASeriesDtreePlugin_UpdatePoolCapabilities_EmptyPools(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	poolNames := []string{}

	// act
	result, err := p.UpdatePoolCapabilities(context.Background(), poolNames)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result))
}

// TestOceanstorASeriesDtreePlugin_Validate_VerifyDTreeParamError tests verifyDTreeParam error
func TestOceanstorASeriesDtreePlugin_Validate_VerifyDTreeParamError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	p.protocol = constants.ProtocolDtfs

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(verifyDTreeParam, errors.New("dtree verify failed"))

	// act
	err := p.Validate(context.Background(), map[string]interface{}{
		"protocol":   constants.ProtocolNfs,
		"parentname": "testParent",
		"parameters": map[string]interface{}{
			"size":       int64(1024 * 1024),
			"volumeType": "dtree",
		},
	})

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dtree verify failed")
}

// TestOceanstorASeriesDtreePlugin_Validate_ParentError tests parent Validate error
func TestOceanstorASeriesDtreePlugin_Validate_ParentError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	p.protocol = constants.ProtocolDtfs

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(verifyDTreeParam, nil)
	mock.ApplyMethodReturn(&p.OceanstorASeriesPlugin, "Validate", errors.New("parent validation error"))

	// act
	err := p.Validate(context.Background(), map[string]interface{}{
		"protocol":   constants.ProtocolNfs,
		"parentname": "testParent",
		"parameters": map[string]interface{}{
			"size":       int64(1024 * 1024),
			"volumeType": "dtree",
		},
	})

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent validation error")
}

// TestOceanstorASeriesDtreePlugin_CreateVolume_ParameterConversionError tests parameter conversion failure
func TestOceanstorASeriesDtreePlugin_CreateVolume_ParameterConversionError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	p.protocol = constants.ProtocolDtfs

	parameters := map[string]interface{}{
		"parentname": "testParent",
		"size":       "invalidSize", // Should be int64
		"volumeType": "dtree",
	}

	// act
	volume, err := p.CreateVolume(context.Background(), "testVolume", parameters)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "convert parameters to struct failed")
	assert.Nil(t, volume)
}

// TestOceanstorASeriesDtreePlugin_QueryVolume_GetValidParentNameError tests parent name validation error
func TestOceanstorASeriesDtreePlugin_QueryVolume_GetValidParentNameError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	p.parentName = "backendParent"

	parameters := map[string]interface{}{
		"parentname": "differentParent", // Different from backend
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(getValidParentname, "", errors.New("parent name mismatch"))

	// act
	volume, err := p.QueryVolume(context.Background(), "testVolume", parameters)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent name mismatch")
	assert.Nil(t, volume)
}

// TestOceanstorASeriesDtreePlugin_DeleteVolume_NotImplemented tests DeleteVolume error
func TestOceanstorASeriesDtreePlugin_DeleteVolume(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	name := "testVolume"
	params := map[string]interface{}{}

	// act
	err := p.DeleteVolume(context.Background(), name, params)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implement, use DeleteDTreeVolume instead")
	assert.Equal(t, errors.New("not implement, use DeleteDTreeVolume instead"), err)
}

// TestOceanstorASeriesDtreePlugin_DeleteDTreeVolume_Success tests successful DTree deletion
func TestOceanstorASeriesDtreePlugin_DeleteDTreeVolume_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	dTreeName := "testDTree"
	parentName := "testParent"

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(dtree.NewDeleter, &dtree.Deleter{})
	mock.ApplyFuncReturn((*dtree.Deleter).Delete, nil)

	// act
	err := p.DeleteDTreeVolume(context.Background(), dTreeName, parentName)

	// assert
	assert.NoError(t, err)
}

// TestOceanstorASeriesDtreePlugin_DeleteDTreeVolume_DeletionError tests deletion error
func TestOceanstorASeriesDtreePlugin_DeleteDTreeVolume_DeletionError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	dTreeName := "testDTree"
	parentName := "testParent"

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(dtree.NewDeleter, &dtree.Deleter{})
	mock.ApplyFuncReturn((*dtree.Deleter).Delete, errors.New("deletion failed"))

	// act
	err := p.DeleteDTreeVolume(context.Background(), dTreeName, parentName)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deletion failed")
}

// TestOceanstorASeriesDtreePlugin_ExpandVolume_NotImplemented tests ExpandVolume error
func TestOceanstorASeriesDtreePlugin_ExpandVolume(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	name := "testVolume"
	size := int64(1024 * 1024)

	// act
	result, err := p.ExpandVolume(context.Background(), name, size)

	// assert
	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not implement, use ExpandDTreeVolume instead")
}

// TestOceanstorASeriesDtreePlugin_ExpandDTreeVolume_Success tests successful DTree expansion
func TestOceanstorASeriesDtreePlugin_ExpandDTreeVolume_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	dTreeName := "testDTree"
	parentName := "testParent"
	spaceHardQuota := int64(2048 * 1024)

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(dtree.NewExpander, &dtree.Expander{})
	mock.ApplyFuncReturn((*dtree.Expander).Expand, nil)

	// act
	result, err := p.ExpandDTreeVolume(context.Background(), dTreeName, parentName, spaceHardQuota)

	// assert
	assert.NoError(t, err)
	assert.False(t, result)
}

// TestOceanstorASeriesDtreePlugin_AttachVolume_Success tests successful volume attachment
func TestOceanstorASeriesDtreePlugin_AttachVolume_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	expectedParentName := "testKey"
	parameters := map[string]interface{}{
		"volumeContext": map[string]string{
			constants.DTreeParentKey: expectedParentName,
		},
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(attachDTreeVolume, map[string]interface{}{constants.DTreeParentKey: expectedParentName}, nil)

	// act
	result, err := p.AttachVolume(context.Background(), "node", parameters)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedParentName, result[constants.DTreeParentKey])
}

// TestOceanstorASeriesDtreePlugin_AttachVolume_AttachError tests attachment error
func TestOceanstorASeriesDtreePlugin_AttachVolume_AttachError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	parameters := map[string]interface{}{
		"volumeContext": map[string]string{
			constants.DTreeParentKey: "testParent",
		},
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(attachDTreeVolume, nil, errors.New("attach failed"))

	// act
	result, err := p.AttachVolume(context.Background(), "node", parameters)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attach failed")
	assert.Nil(t, result)
}

// TestOceanstorASeriesDtreePlugin_AttachVolume_WithComplexParameters tests attachment with complex parameters
func TestOceanstorASeriesDtreePlugin_AttachVolume_WithComplexParameters(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	expectedParentName := "complexParent"
	parameters := map[string]interface{}{
		"volumeContext": map[string]string{
			constants.DTreeParentKey: expectedParentName,
		},
		"extraParam": "extraValue",
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(attachDTreeVolume, map[string]interface{}{constants.DTreeParentKey: expectedParentName}, nil)

	// act
	result, err := p.AttachVolume(context.Background(), "node", parameters)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, expectedParentName, result[constants.DTreeParentKey])
}

// TestOceanstorASeriesDtreePlugin_AttachVolume_WithoutParentKey tests attachment without parent key
func TestOceanstorASeriesDtreePlugin_AttachVolume_WithoutParentKey(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	parameters := map[string]interface{}{
		"volumeContext": map[string]string{}, // No parent key
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

// TestOceanstorASeriesDtreePlugin_AttachVolume_NoVolumeContext tests attachment without volume context
func TestOceanstorASeriesDtreePlugin_AttachVolume_NoVolumeContext(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	parameters := map[string]interface{}{} // No volume context

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(attachDTreeVolume, map[string]interface{}{}, nil)

	// act
	result, err := p.AttachVolume(context.Background(), "node", parameters)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{}, result)
}

// TestOceanstorASeriesDtreePlugin_AttachVolume_MultipleNodes tests attachment with multiple nodes
func TestOceanstorASeriesDtreePlugin_AttachVolume_MultipleNodes(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	expectedParentName := "testParent"
	parameters := map[string]interface{}{
		"volumeContext": map[string]string{
			constants.DTreeParentKey: expectedParentName,
		},
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(attachDTreeVolume, map[string]interface{}{constants.DTreeParentKey: expectedParentName}, nil)

	// act
	nodeNames := []string{"node1", "node2", "node3"}
	for _, nodeName := range nodeNames {
		result, err := p.AttachVolume(context.Background(), nodeName, parameters)
		// assert
		assert.NoError(t, err)
		assert.Equal(t, expectedParentName, result[constants.DTreeParentKey])
	}
}

// TestOceanstorASeriesDtreePlugin_NewPlugin tests plugin creation
func TestOceanstorASeriesDtreePlugin_NewPlugin(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}

	// act
	newPlugin := p.NewPlugin()

	// assert
	assert.NotNil(t, newPlugin)
	assert.IsType(t, &OceanstorASeriesDtreePlugin{}, newPlugin)
}

// TestOceanstorASeriesDtreePlugin_GetDTreeParentName tests parent name retrieval
func TestOceanstorASeriesDtreePlugin_GetDTreeParentName(t *testing.T) {
	// arrange
	tests := []struct {
		name         string
		parentName   string
		expectedName string
	}{
		{" with parent name", "testParentName", "testParentName"},
		{" empty parent name", "", ""},
	}

	for _, tt := range tests {
		// act
		p := &OceanstorASeriesDtreePlugin{
			parentName: tt.parentName,
		}
		actualName := p.GetDTreeParentName()

		// assert
		assert.Equal(t, tt.expectedName, actualName)
	}
}

// TestOceanstorASeriesDtreePlugin_GetSectorSize tests sector size retrieval
func TestOceanstorASeriesDtreePlugin_GetSectorSize(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}

	// act
	sectorSize := p.GetSectorSize()

	// assert
	assert.Equal(t, constants.ASeriesDTreeCapacityUnit, sectorSize)
	assert.Equal(t, int64(1), sectorSize)
}
