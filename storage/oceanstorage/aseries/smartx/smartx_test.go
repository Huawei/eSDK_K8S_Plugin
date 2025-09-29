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

// Package smartx provides operations for a-series storage qos
package smartx

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
)

var (
	testClient *client.OceanASeriesClient
)

func TestCreateQos_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	param := map[string]int{
		"LATENCY": 500,
	}
	res := map[string]any{
		"ID":           "test1",
		"ENABLESTATUS": "true",
	}

	// mock
	smartQoS := NewSmartX(testClient)
	m := gomonkey.ApplyMethodReturn(smartQoS.cli, "UpdateFileSystem", nil)
	m.ApplyMethodReturn(smartQoS.cli, "CreateQos", res, nil)
	defer m.Reset()

	// action
	_, err := smartQoS.CreateQos(ctx, "testId", "testVStore", param)

	// assert
	require.NoError(t, err)
}

func TestCreateQos_UpdateFSFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	param := map[string]int{
		"LATENCY": 500,
	}

	// mock
	smartQoS := NewSmartX(testClient)
	m := gomonkey.ApplyMethodReturn(smartQoS.cli, "UpdateFileSystem", errors.New("mock error"))
	defer m.Reset()

	// action
	_, err := smartQoS.CreateQos(ctx, "testId", "testVStore", param)

	// assert
	require.Error(t, err)
}

func TestCreateQos_CreateQoSFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	param := map[string]int{
		"LATENCY": 500,
	}

	// mock
	smartQoS := NewSmartX(testClient)
	m := gomonkey.ApplyMethodReturn(smartQoS.cli, "UpdateFileSystem", nil)
	m.ApplyMethodReturn(smartQoS.cli, "CreateQos", nil, errors.New("mock error"))
	defer m.Reset()

	// action
	_, err := smartQoS.CreateQos(ctx, "testId", "testVStore", param)

	// assert
	require.Error(t, err)
}

func TestDeleteQos_GetQoSFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockError := errors.New("get qos failed")

	// mock
	smartQoS := NewSmartX(testClient)
	m := gomonkey.ApplyMethodReturn(smartQoS.cli, "GetQosByID", nil, mockError)
	defer m.Reset()

	// action
	err := smartQoS.DeleteQos(ctx, "testId", "testObjId", "testVStore")

	// assert
	require.Error(t, err)
}

func TestDeleteQos_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	objID := "241"
	mockQoS := map[string]interface{}{
		"FSLIST": "[\"241\"]",
	}

	// mock
	smartQoS := NewSmartX(testClient)
	m := gomonkey.ApplyMethodReturn(smartQoS.cli, "GetQosByID", mockQoS, nil)
	m.ApplyMethodReturn(smartQoS.cli, "DeactivateQos", nil)
	m.ApplyMethodReturn(smartQoS.cli, "DeleteQos", nil)
	defer m.Reset()

	// action
	err := smartQoS.DeleteQos(ctx, "testId", objID, "testVStore")

	// assert
	require.NoError(t, err)
}

func TestDeleteQos_DeleteFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	objID := "241"
	mockQoS := map[string]interface{}{
		"FSLIST": "[\"241\"]",
	}
	mockError := errors.New("delete qos failed")

	// mock
	smartQoS := NewSmartX(testClient)
	m := gomonkey.ApplyMethodReturn(smartQoS.cli, "GetQosByID", mockQoS, nil)
	m.ApplyMethodReturn(smartQoS.cli, "DeactivateQos", nil)
	m.ApplyMethodReturn(smartQoS.cli, "DeleteQos", mockError)
	defer m.Reset()

	// action
	err := smartQoS.DeleteQos(ctx, "testId", objID, "testVStore")

	// assert
	require.Error(t, err)
}

func TestDeleteQos_GetFSLISTFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	objID := "241"
	mockQoS := map[string]interface{}{
		"FSLIST": 123,
	}

	// mock
	smartQoS := NewSmartX(testClient)
	m := gomonkey.ApplyMethodReturn(smartQoS.cli, "GetQosByID", mockQoS, nil)
	defer m.Reset()

	// action
	err := smartQoS.DeleteQos(ctx, "testId", objID, "testVStore")

	// assert
	require.Error(t, err)
}

func TestCheckQoSParameterSupport_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	qosStr := `{
"IOTYPE": 2,
"MAXBANDWIDTH": 1240,
"MINBANDWIDTH": 100,
"MAXIOPS": 1240,
"MINIOPS": 100,
"LATENCY": 0.5
}`

	// action
	err := CheckQoSParametersValueRange(ctx, qosStr)

	// assert
	require.NoError(t, err)
}

func TestCheckQoSParameterSupport_UnmarshalFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	qosStr := `{
"IOTYPE": abc,
"MAXBANDWIDTH": 1240,
"MINBANDWIDTH": 100,
"MAXIOPS": 1240,
"MINIOPS": 100,
"LATENCY": 0.5
}`

	// action
	err := CheckQoSParametersValueRange(ctx, qosStr)

	// assert
	require.Error(t, err)
}

func TestCheckQoSParameterSupport_InvalidKey(t *testing.T) {
	// arrange
	ctx := context.Background()
	qosStr := `{
"INVALID": 123,
"IOTYPE": 2,
"MAXBANDWIDTH": 1240,
"MINBANDWIDTH": 100,
"MAXIOPS": 1240,
"MINIOPS": 100,
"LATENCY": 0.5
}`

	// action
	err := CheckQoSParametersValueRange(ctx, qosStr)

	// assert
	require.Error(t, err)
}

func TestCheckQoSParameterSupport_InvalidValue(t *testing.T) {
	// arrange
	ctx := context.Background()
	qosStr := `{
"INVALID": 123,
"IOTYPE": 2,
"MAXBANDWIDTH": 1240,
"MINBANDWIDTH": 100,
"MAXIOPS": 1240,
"MINIOPS": 100,
"LATENCY": 123
}`

	// action
	err := CheckQoSParametersValueRange(ctx, qosStr)

	// assert
	require.Error(t, err)
}

func TestValidateQoSParameters_Success(t *testing.T) {
	// arrange
	qosParam := map[string]float64{
		"IOTYPE":       2,
		"MAXBANDWIDTH": 1240,
		"MINBANDWIDTH": 100,
		"MAXIOPS":      1240,
		"MINIOPS":      100,
		"LATENCY":      500,
	}

	// action
	_, err := ConvertQoSParametersValueToInt(qosParam)

	// assert
	require.NoError(t, err)
}

func TestValidateQoSParameters_ParamNotExist(t *testing.T) {
	// arrange
	qosParam := map[string]float64{
		"INVALID": 2,
	}

	// action
	_, err := ConvertQoSParametersValueToInt(qosParam)

	// assert
	require.Error(t, err)
}

func TestValidateQoSParameters_InvalidType(t *testing.T) {
	// arrange
	qosParam := map[string]float64{
		"IOTYPE":       2,
		"MAXBANDWIDTH": 1240,
		"MINBANDWIDTH": 100,
		"MAXIOPS":      1240,
		"MINIOPS":      100,
		"LATENCY":      0.5,
	}

	// action
	_, err := ConvertQoSParametersValueToInt(qosParam)

	// assert
	require.Error(t, err)
}
