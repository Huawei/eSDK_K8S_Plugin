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

// Package client provides oceanstor A-series storage client
package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
)

func createMockClientWithResponse(t *testing.T, statusCode int, body string) *OceanASeriesClient {
	testClient, err := NewClient(context.Background(), &storage.NewClientConfig{
		Urls:            []string{"https://127.0.0.1:8088"},
		User:            "dev-account",
		SecretName:      "mock-sec-name",
		SecretNamespace: "mock-sec-namespace",
		ParallelNum:     "",
		BackendID:       "mock-backend-id",
	})
	require.NoError(t, err, "Failed to create test client")

	testClient.Client = &http.Client{
		Transport: &mockTransport{
			responseStatusCode: statusCode,
			responseBody:       io.NopCloser(bytes.NewBufferString(body)),
		},
	}

	return testClient
}

type mockTransport struct {
	responseStatusCode int
	responseBody       io.ReadCloser
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.responseStatusCode,
		Header:     http.Header{},
		Body:       m.responseBody,
	}, nil
}

func TestOceanASeriesClient_GetDTreeByName_EmptyDataArray(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentName := "test-fs"
	dtreeName := "not-exist"
	vstoreId := "0"
	// mock
	emptyBody := `{
		"data": [],
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, emptyBody)

	// action
	result, err := mockClient.GetDTreeByName(ctx, parentName, dtreeName, vstoreId)

	// assert
	require.NoError(t, err, "should not return error for empty array")
	assert.Nil(t, result, "should return nil for empty array")
}

func TestOceanASeriesClient_GetDTreeByName_EmptyData(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentName := "test-fs"
	dtreeName := "empty"
	vstoreId := "0"
	// mock
	emptyBody := `{
		"data": null,
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, emptyBody)

	// action
	result, err := mockClient.GetDTreeByName(ctx, parentName, dtreeName, vstoreId)

	// assert
	require.NoError(t, err, "should not return error for null data")
	assert.Nil(t, result, "should return nil for null data")
}

func TestOceanASeriesClient_GetDTreeByName_NotExistCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentName := "test-fs"
	dtreeName := "not-exist"
	vstoreId := "0"
	// mock
	notExistBody := `{
		"data": [],
		"error": {
			"code": 1077955336,
			"description": "not exist"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, notExistBody)

	// action
	result, err := mockClient.GetDTreeByName(ctx, parentName, dtreeName, vstoreId)

	// assert
	assert.NoError(t, err, "currently implementation returns nil error for 200 status")
	assert.Nil(t, result, "should return nil when data array is empty even with error code")
}

func TestOceanASeriesClient_GetDTreeByName_NotFoundCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentName := "test-fs"
	dtreeName := "not-found"
	vstoreId := "0"

	// mock
	notFoundBody := `{
		"data": [],
		"error": {
			"code": 1077955080,
			"description": "not found"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, notFoundBody)

	// action
	result, err := mockClient.GetDTreeByName(ctx, parentName, dtreeName, vstoreId)

	// assert
	require.NoError(t, err, "should not return error for not found code")
	assert.Nil(t, result, "should return nil for not found code")
}

func TestOceanASeriesClient_GetDTreeByName_WithDataKeyObject(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentName := "test-fs"
	dtreeName := "test-dtree"
	vstoreId := "0"

	// mock
	wrappedBody := `{
		"data": {
				"ID": "1",
				"NAME": "test-dtree",
				"PARENTNAME": "test-fs"
		},
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, wrappedBody)

	// action
	result, err := mockClient.GetDTreeByName(ctx, parentName, dtreeName, vstoreId)

	// assert
	require.NoError(t, err, "should not return error for wrapped data")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "1", result["ID"], "ID should match")
	assert.Equal(t, "test-dtree", result["NAME"], "NAME should match")
}

func TestOceanASeriesClient_CreateDTree_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	req := &DTreeCreateRequest{
		Name:            "new-dtree",
		ParentName:      "test-fs",
		UnixPermissions: "",
		VStoreID:        "",
	}

	// mock
	successBody := `{
		"data": {
			"ID": "1",
			"NAME": "new-dtree"
		},
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	result, err := mockClient.CreateDTree(ctx, req)

	// assert
	require.NoError(t, err, "should not return error for successful creation")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "1", result["ID"], "ID should match")
	assert.Equal(t, "new-dtree", result["NAME"], "NAME should match")
}

func TestOceanASeriesClient_CreateDTree_WithUnixPermissions(t *testing.T) {
	// arrange
	ctx := context.Background()
	req := &DTreeCreateRequest{
		Name:            "new-dtree-perm",
		ParentName:      "test-fs",
		UnixPermissions: "755",
		VStoreID:        "123",
	}

	// mock
	successBody := `{
		"data": {
			"ID": "2",
			"NAME": "new-dtree-perm"
		},
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	result, err := mockClient.CreateDTree(ctx, req)

	// assert
	require.NoError(t, err, "should not return error for successful creation")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "2", result["ID"], "ID should match")
	assert.Equal(t, "new-dtree-perm", result["NAME"], "NAME should match")
}

func TestOceanASeriesClient_CreateDTree_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	req := &DTreeCreateRequest{
		Name:            "error-dtree",
		ParentName:      "test-fs",
		UnixPermissions: "",
		VStoreID:        "",
	}

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	_, err := mockClient.CreateDTree(ctx, req)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "create DTree failed", "should contain failure message")
}

func TestOceanASeriesClient_CreateDTree_InvalidDataFormat(t *testing.T) {
	// arrange
	ctx := context.Background()
	req := &DTreeCreateRequest{
		Name:            "invalid-data",
		ParentName:      "",
		UnixPermissions: "",
		VStoreID:        "",
	}

	// mock
	invalidBody := `{
		"data": "invalid_string",
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, invalidBody)

	// action
	_, err := mockClient.CreateDTree(ctx, req)

	// assert
	require.ErrorContains(t, err, "convert DTree response to map failed", "should return appropriate error")
}

func TestOceanASeriesClient_DeleteDTreeByID_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	vStoreID := "0"
	dtreeID := "1"

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.DeleteDTreeByID(ctx, vStoreID, dtreeID)

	// assert
	require.NoError(t, err, "should not return error for successful deletion")
}

func TestOceanASeriesClient_DeleteDTreeByID_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	vStoreID := "0"
	dtreeID := "1"

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	err := mockClient.DeleteDTreeByID(ctx, vStoreID, dtreeID)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "1", "should contain dtree ID")
}

func TestOceanASeriesClient_DeleteDTreeByID_EmptyVStoreID(t *testing.T) {
	// arrange
	ctx := context.Background()
	vStoreID := ""
	dtreeID := "1"

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.DeleteDTreeByID(ctx, vStoreID, dtreeID)

	// assert
	require.NoError(t, err, "should not return error for empty vStoreID")
}

func TestOceanASeriesClient_DeleteDTreeByName_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	vStoreID := "0"
	parentName := "test-fs"
	dtreeName := "test-dtree"

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.DeleteDTreeByName(ctx, vStoreID, parentName, dtreeName)

	// assert
	require.NoError(t, err, "should not return error for successful deletion")
}

func TestOceanASeriesClient_DeleteDTreeByName_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	vStoreID := "0"
	parentName := "test-fs"
	dtreeName := "error-dtree"

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	err := mockClient.DeleteDTreeByName(ctx, vStoreID, parentName, dtreeName)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "error-dtree", "should contain dtree name")
}

func TestOceanASeriesClient_DeleteDTreeByName_EmptyVStoreID(t *testing.T) {
	// arrange
	ctx := context.Background()
	vStoreID := ""
	parentName := "test-fs"
	dtreeName := "test-dtree"

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.DeleteDTreeByName(ctx, vStoreID, parentName, dtreeName)

	// assert
	require.NoError(t, err, "should not return error for empty vStoreID")
}

func TestOceanASeriesClient_GetDTreeByID_EmptyData(t *testing.T) {
	// arrange
	ctx := context.Background()
	dtreeID := "empty"

	// mock
	emptyBody := `{
		"data": null,
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, emptyBody)

	// action
	result, err := mockClient.GetDTreeByID(ctx, dtreeID)

	// assert
	require.NoError(t, err, "should not return error for null data")
	assert.Nil(t, result, "should return nil for null data")
}

func TestOceanASeriesClient_GetDTreeByID_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	dtreeID := "1"

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	_, err := mockClient.GetDTreeByID(ctx, dtreeID)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "1", "should contain dtree ID")
}

func TestOceanASeriesClient_GetDTreeByID_WithDataKeyObject(t *testing.T) {
	// arrange
	ctx := context.Background()
	dtreeID := "1"

	// mock
	wrappedBody := `{
		"data": {
				"ID": "1",
				"NAME": "test-dtree"
		},
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, wrappedBody)

	// action
	result, err := mockClient.GetDTreeByID(ctx, dtreeID)

	// assert
	require.NoError(t, err, "should not return error for wrapped data")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "1", result["ID"], "ID should match")
	assert.Equal(t, "test-dtree", result["NAME"], "NAME should match")
}

func TestOceanASeriesClient_UpdateDTree_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	dtreeID := "1"
	req := &DTreeUpdateRequest{
		VStoreID: "123",
	}

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.UpdateDTree(ctx, dtreeID, req)

	// assert
	require.NoError(t, err, "should not return error for successful update")
}

func TestOceanASeriesClient_UpdateDTree_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	dtreeID := "1"
	req := &DTreeUpdateRequest{}

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	err := mockClient.UpdateDTree(ctx, dtreeID, req)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "1", "should contain dtree ID")
}

func TestOceanASeriesClient_CreateDTreeQuota_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	req := &DTreeQuotaRequest{
		PARENTTYPE:     "16445",
		PARENTID:       "1",
		QUOTATYPE:      "1",
		SPACEHARDQUOTA: "1024000000",
		SPACEUNITTYPE:  "0",
		VStoreID:       "0",
	}

	// mock
	successBody := `{
		"data": {
			"ID": "quota-1",
			"PARENTID": "1"
		},
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	result, err := mockClient.CreateDTreeQuota(ctx, req)

	// assert
	require.NoError(t, err, "should not return error for successful quota creation")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "quota-1", result["ID"], "ID should match")
	assert.Equal(t, "1", result["PARENTID"], "PARENTID should match")
}

func TestOceanASeriesClient_CreateDTreeQuota_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	req := &DTreeQuotaRequest{
		PARENTTYPE:     "16445",
		PARENTID:       "1",
		QUOTATYPE:      "0",
		SPACEHARDQUOTA: "0",
		SPACEUNITTYPE:  "0",
		VStoreID:       "0",
	}

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	_, err := mockClient.CreateDTreeQuota(ctx, req)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "create DTree quota failed", "should contain failure message")
}

func TestOceanASeriesClient_CreateDTreeQuota_InvalidDataFormat(t *testing.T) {
	// arrange
	ctx := context.Background()
	req := &DTreeQuotaRequest{
		PARENTID:       "1",
		PARENTTYPE:     "",
		QUOTATYPE:      "0",
		SPACEHARDQUOTA: "0",
		SPACEUNITTYPE:  "0",
		VStoreID:       "",
	}

	// mock
	invalidBody := `{
		"data": "invalid_string",
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, invalidBody)

	// action
	_, err := mockClient.CreateDTreeQuota(ctx, req)

	// assert
	require.ErrorContains(t, err, "convert DTree quota response to map failed", "should return appropriate error")
}

func TestOceanASeriesClient_GetDTreeQuota_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "1"
	vStoreID := "0"

	// mock
	successBody := `{
		"data": [
			{
				"ID": "quota-1",
				"PARENTID": "1",
				"PARENTTYPE": "16445",
				"SPACEHARDQUOTA": "1024000000"
			}
		],
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	result, err := mockClient.GetDTreeQuota(ctx, parentID, vStoreID)

	// assert
	require.NoError(t, err, "should not return error for successful get")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, "quota-1", result["ID"], "ID should match")
	assert.Equal(t, "1", result["PARENTID"], "PARENTID should match")
}

func TestOceanASeriesClient_GetDTreeQuota_EmptyData(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "1"
	vStoreID := "0"

	// mock
	emptyBody := `{
		"data": [],
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, emptyBody)

	// action
	result, err := mockClient.GetDTreeQuota(ctx, parentID, vStoreID)

	// assert
	require.NoError(t, err, "should not return error for empty array")
	assert.Nil(t, result, "should return nil for empty array")
}

func TestOceanASeriesClient_GetDTreeQuota_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "1"
	vStoreID := "0"

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	_, err := mockClient.GetDTreeQuota(ctx, parentID, vStoreID)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "1", "should contain parent ID")
}

func TestOceanASeriesClient_GetDTreeQuota_InvalidDataFormat(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "1"
	vStoreID := "0"

	// mock
	invalidBody := `{
		"data": "invalid_string",
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, invalidBody)

	// action
	_, err := mockClient.GetDTreeQuota(ctx, parentID, vStoreID)

	// assert
	require.ErrorContains(t, err, "convert respData to array failed", "should return appropriate error")
}

func TestOceanASeriesClient_GetDTreeQuota_InvalidElementType(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "1"
	vStoreID := "0"

	// mock
	invalidBody := `{
		"data": [123],
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, invalidBody)

	// action
	_, err := mockClient.GetDTreeQuota(ctx, parentID, vStoreID)

	// assert
	require.ErrorContains(t, err, "convert quota to map failed", "should return appropriate error")
}

func TestOceanASeriesClient_UpdateDTreeQuota_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	quotaID := "quota-1"
	req := &DTreeQuotaUpdateRequest{
		SPACEHARDQUOTA: "2048000000",
		SPACEUNITTYPE:  "0",
		VStoreID:       "123",
	}

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.UpdateDTreeQuota(ctx, quotaID, req)

	// assert
	require.NoError(t, err, "should not return error for successful update")
}

func TestOceanASeriesClient_UpdateDTreeQuota_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	quotaID := "quota-1"
	req := &DTreeQuotaUpdateRequest{
		SPACEHARDQUOTA: "2048000000",
		SPACEUNITTYPE:  "0",
		VStoreID:       "123",
	}

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	err := mockClient.UpdateDTreeQuota(ctx, quotaID, req)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "quota-1", "should contain quota ID")
}

func TestOceanASeriesClient_DeleteDTreeQuota_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	quotaID := "quota-1"
	vStoreID := "0"

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.DeleteDTreeQuota(ctx, quotaID, vStoreID)

	// assert
	require.NoError(t, err, "should not return error for successful deletion")
}

func TestOceanASeriesClient_DeleteDTreeQuota_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	quotaID := "quota-1"
	vStoreID := "0"

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	err := mockClient.DeleteDTreeQuota(ctx, quotaID, vStoreID)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "quota-1", "should contain quota ID")
}

func TestOceanASeriesClient_DeleteDTreeQuota_EmptyVStoreID(t *testing.T) {
	// arrange
	ctx := context.Background()
	quotaID := "quota-1"
	vStoreID := ""

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.DeleteDTreeQuota(ctx, quotaID, vStoreID)

	// assert
	require.NoError(t, err, "should not return error for empty vStoreID")
}

func TestOceanASeriesClient_DeleteDTreeQuotaByParentID_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "1"
	vStoreID := "0"

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.DeleteDTreeQuotaByParentID(ctx, parentID, vStoreID)

	// assert
	require.NoError(t, err, "should not return error for successful deletion")
}

func TestOceanASeriesClient_DeleteDTreeQuotaByParentID_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "1"
	vStoreID := "0"

	// mock
	errorBody := `{
		"data": null,
		"error": {
			"code": 123456,
			"description": "permission denied"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, errorBody)

	// action
	err := mockClient.DeleteDTreeQuotaByParentID(ctx, parentID, vStoreID)

	// assert
	require.ErrorContains(t, err, "123456", "should contain error code")
	assert.Contains(t, err.Error(), "1", "should contain parent ID")
}

func TestOceanASeriesClient_DeleteDTreeQuotaByParentID_EmptyVStoreID(t *testing.T) {
	// arrange
	ctx := context.Background()
	parentID := "1"
	vStoreID := ""

	// mock
	successBody := `{
		"error": {
			"code": 0,
			"description": "success"
		}
	}`
	mockClient := createMockClientWithResponse(t, 200, successBody)

	// action
	err := mockClient.DeleteDTreeQuotaByParentID(ctx, parentID, vStoreID)

	// assert
	require.NoError(t, err, "should not return error for empty vStoreID")
}
