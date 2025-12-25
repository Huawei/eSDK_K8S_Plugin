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

package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
)

func TestOceanstorClient_SafeDeleteFileSystem_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	params := map[string]interface{}{"ID": "fs-001"}
	successResp := `{
        "data": {},
        "error": {"code": 0}
    }`

	// Mock
	mockClient := getMockClient(200, successResp)

	// Action
	err := mockClient.SafeDeleteFileSystem(ctx, params)

	// Assert
	require.NoError(t, err)
}

func TestOceanstorClient_SafeDeleteFileSystem_NotExist(t *testing.T) {
	// Arrange
	ctx := context.Background()
	params := map[string]interface{}{"ID": "non-exist-fs"}
	notExistResp := `{
        "data": {},
        "error": {"code": 1073752065}
    }`

	// Mock
	mockClient := getMockClient(200, notExistResp)

	// Action
	err := mockClient.SafeDeleteFileSystem(ctx, params)

	// Assert
	require.NoError(t, err)
}

func TestOceanstorClient_SafeDeleteFileSystem_OtherError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	params := map[string]interface{}{"ID": "fs-error-002"}
	errorResp := `{
        "data": {},
        "error": {"code": 1077939726}
    }`

	// Mock
	mockClient := getMockClient(200, errorResp)

	// Action
	err := mockClient.SafeDeleteFileSystem(ctx, params)

	// Assert
	require.ErrorContains(t, err, "Delete filesystem")
	require.Contains(t, err.Error(), "1077939726")
}

func TestOceanstorClient_SafeDeleteNfsShare_SuccessWithVStore(t *testing.T) {
	// Arrange
	ctx := context.Background()
	id := "nfs-001"
	vStoreID := "vs-001"
	successResp := `{
        "data": {},
        "error": {"code": 0}
    }`

	// Mock
	mockClient := getMockClient(200, successResp)

	// Action
	err := mockClient.SafeDeleteNfsShare(ctx, id, vStoreID)

	// Assert
	require.NoError(t, err)
}

func TestOceanstorClient_SafeDeleteNfsShare_SuccessWithoutVStore(t *testing.T) {
	// Arrange
	ctx := context.Background()
	id := "nfs-002"
	vStoreID := ""
	successResp := `{
        "data": {},
        "error": {"code": 0}
    }`

	// Mock
	mockClient := getMockClient(200, successResp)

	// Action
	err := mockClient.SafeDeleteNfsShare(ctx, id, vStoreID)

	// Assert
	require.NoError(t, err)
}

func TestOceanstorClient_SafeDeleteNfsShare_NotExist(t *testing.T) {
	// Arrange
	ctx := context.Background()
	id := "nfs-003"
	vStoreID := "vs-001"
	notExistResp := `{
        "data": {},
        "error": {"code": 1077939717}
    }`

	// Mock
	mockClient := getMockClient(200, notExistResp)

	// Action
	err := mockClient.SafeDeleteNfsShare(ctx, id, vStoreID)

	// Assert
	require.NoError(t, err)
}

func TestOceanstorClient_SafeDeleteNfsShare_OtherError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	id := "nfs-004"
	vStoreID := "vs-002"
	errorResp := `{
        "data": {},
        "error": {"code": 1077939726}
    }`

	// Mock
	mockClient := getMockClient(200, errorResp)

	// Action
	err := mockClient.SafeDeleteNfsShare(ctx, id, vStoreID)

	// Assert
	require.ErrorContains(t, err, "delete nfs share nfs-004 error: 1077939726")
}

func TestOceanstorClient_GetFileSystemByName_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	name := "test-fs"
	successResp := `{
        "data": [
            {
                "ID": "fs-001",
                "NAME": "test-fs",
                "vstoreName": "test-vstore"
            }
        ],
        "error": {"code": 0}
    }`

	// Mock
	mockClient := getMockClient(200, successResp)
	mockClient.VStoreName = "test-vstore"

	// Action
	result, err := mockClient.GetFileSystemByName(ctx, name)

	// Assert
	require.NoError(t, err)
	require.Equal(t, "fs-001", result["ID"])
}

func TestOceanstorClient_GetFileSystemByName_MultipleMatches(t *testing.T) {
	// Arrange
	ctx := context.Background()
	name := "multi-fs"
	respBody := `{
        "data": [
            {"ID": "fs-1", "vstoreName": "wrong-store"},
            {"ID": "fs-2", "vstoreName": "target-store"}
        ],
        "error": {"code": 0}
    }`

	// Mock
	mockClient := getMockClient(200, respBody)
	mockClient.VStoreName = "target-store"

	// Action
	result, _ := mockClient.GetFileSystemByName(ctx, name)

	// Assert
	require.Equal(t, "fs-2", result["ID"])
}

func TestOceanstorClient_GetFileSystemByName_NotFound(t *testing.T) {
	// Arrange
	ctx := context.Background()
	name := "non-exist-fs"
	emptyResp := `{
        "data": [],
        "error": {"code": 0}
    }`

	// Mock
	mockClient := getMockClient(200, emptyResp)

	// Action
	result, err := mockClient.GetFileSystemByName(ctx, name)

	// Assert
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestOceanstorClient_GetFileSystemByName_APIFailure(t *testing.T) {
	// Arrange
	ctx := context.Background()
	name := "error-fs"
	errorResp := `{
        "data": null,
        "error": {"code": 1077939726}
    }`

	// Mock
	mockClient := getMockClient(200, errorResp)

	// Action
	_, err := mockClient.GetFileSystemByName(ctx, name)

	// Assert
	require.ErrorContains(t, err, "error: 1077939726")
}

func TestOceanstorClient_GetFileSystemByName_DataConvertError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	name := "invalid-fs"
	invalidResp := `{
        "data": "invalid_type",
        "error": {"code": 0}
    }`

	// Mock
	mockClient := getMockClient(200, invalidResp)

	// Action
	_, err := mockClient.GetFileSystemByName(ctx, name)

	// Assert
	require.ErrorContains(t, err, "convert resp.Data to []interface{} failed")
}

func TestOceanstorClient_GetFileSystemByName_DefaultVStore(t *testing.T) {
	// Arrange
	ctx := context.Background()
	name := "default-fs"
	respBody := `{
        "data": [
            {"ID": "fs-003", "NAME": "default-fs"}
        ],
        "error": {"code": 0}
    }`

	// Mock
	mockClient := getMockClient(200, respBody)
	mockClient.VStoreName = storage.DefaultVStore

	// Action
	result, _ := mockClient.GetFileSystemByName(ctx, name)

	// Assert
	require.Equal(t, "fs-003", result["ID"])
}

func TestCreateFileSystem_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	params := map[string]interface{}{
		"name":     "test-fs",
		"capacity": 1024,
	}
	successResp := `{
        "data": {"ID":"fs-001"},
        "error": {"code":0}
    }`

	// Mock
	mockClient := getMockClient(200, successResp)

	// Action
	result, err := mockClient.CreateFileSystem(ctx, params)

	// Assert
	require.NoError(t, err)
	require.Equal(t, "fs-001", result["ID"])
}

func TestCreateFileSystem_RetrySuccess(t *testing.T) {
	// Arrange
	ctx := context.Background()
	params := map[string]interface{}{
		"name":     "retry-fs",
		"capacity": 2048,
	}

	// Mock
	failedResp := `{
        "data": null,
        "error": {"code": 1077949001}
    }`
	mockClient := getMockClient(200, failedResp)
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(testClient, "GetFileSystemByName", map[string]interface{}{"ID": "fs-retry-01"}, nil)
	mock.ApplyFuncReturn(time.Sleep)

	// Action
	result, err := mockClient.CreateFileSystem(ctx, params)

	// Assert
	require.NoError(t, err)
	require.Equal(t, "fs-retry-01", result["ID"])

	// Clean up
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestCreateFileSystem_RetryFailed(t *testing.T) {
	// Arrange
	ctx := context.Background()
	params := map[string]interface{}{
		"name":     "retry-fail-fs",
		"capacity": 4096,
	}

	// Mock
	failedResp := `{
        "data": null,
        "error": {"code": 1077949001}
    }`
	mockClient := getMockClient(200, failedResp)
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(testClient, "GetFileSystemByName", nil, fmt.Errorf("unknow err"))
	mock.ApplyFuncReturn(time.Sleep)

	// Action
	_, err := mockClient.CreateFileSystem(ctx, params)

	// Assert
	require.ErrorContains(t, err, "Create filesystem error")

	// Clean up
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestCreateFileSystem_ExceedCapacity(t *testing.T) {
	// Arrange
	ctx := context.Background()
	params := map[string]interface{}{"name": "oversize-fs"}
	errorResp := `{
        "error": {"code": 1073844377}
    }`

	// Mock
	mockClient := getMockClient(200, errorResp)

	// Action
	_, err := mockClient.CreateFileSystem(ctx, params)

	// Assert
	require.ErrorContains(t, err, "greater than the maximum capacity")
	require.ErrorContains(t, err, "Suggestion: Delete current PVC")
}

func TestCreateFileSystem_MinimumCapacityNotReached(t *testing.T) {
	// Arrange
	ctx := context.Background()
	params := map[string]interface{}{"name": "too-small-fs"}
	errorResp := `{
        "error": {"code": 1073844376}
    }`

	// Mock
	mockClient := getMockClient(200, errorResp)

	// Action
	_, err := mockClient.CreateFileSystem(ctx, params)

	// Assert
	require.ErrorContains(t, err, "less than the minimum capacity")
	require.ErrorContains(t, err, "Suggestion: Delete current PVC")
}

func TestModifyNfsShareAccess_Success(t *testing.T) {
	// Arrange
	ctx := context.Background()
	accessID := "1"
	vStoreID := "0"
	accessVal := constants.AuthClientReadOnly

	successResp := `{"error":{"code":0}}`
	mockClient := getMockClient(http.StatusOK, successResp)

	// Action
	err := mockClient.ModifyNfsShareAccess(ctx, accessID, vStoreID, accessVal)

	// Assert
	assert.NoError(t, err)
}

func TestModifyNfsShareAccess_APIError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	accessID := "1"
	vStoreID := "0"
	accessVal := constants.AuthClientReadOnly

	errorResp := `{"error":{"code":1077939726}}`
	mockClient := getMockClient(200, errorResp)

	// Action
	gotErr := mockClient.ModifyNfsShareAccess(ctx, accessID, vStoreID, accessVal)

	// Assert
	require.ErrorContains(t, gotErr, "1077939726")
}

func TestCheckNfsShareAccessStatus_Success_AccessGranted(t *testing.T) {
	// Arrange
	ctx := context.Background()
	sharePath := "/mnt/nfs"
	client := "192.168.1.1"
	vStoreID := "vstore-789"
	accessVal := constants.AuthClientReadWrite

	successResp := `{ "data": { "status": "0" }, "error": { "code": 0, "description": "0" } }`
	mockClient := getMockClient(200, successResp)

	// Action
	granted, err := mockClient.CheckNfsShareAccessStatus(ctx, sharePath, client, vStoreID, accessVal)

	// Assert
	require.NoError(t, err)
	require.True(t, granted)
}

func TestCheckNfsShareAccessStatus_Success_AccessDenied(t *testing.T) {
	// Arrange
	ctx := context.Background()
	sharePath := "/mnt/nfs"
	client := "192.168.1.1"
	vStoreID := "vstore-789"
	accessVal := constants.AuthClientNoAccess

	successResp := `{ "data": { "status": "1" }, "error": { "code": 0, "description": "0" } }`
	mockClient := getMockClient(200, successResp)

	// Action
	granted, err := mockClient.CheckNfsShareAccessStatus(ctx, sharePath, client, vStoreID, accessVal)

	// Assert
	require.NoError(t, err)
	require.False(t, granted)
}

func TestCheckNfsShareAccessStatus_APIError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	sharePath := "/mnt/nfs"
	client := "192.168.1.1"
	vStoreID := "vstore-789"
	accessVal := constants.AuthClientReadOnly

	errorResp := `{"error":{"code":1077939726}}`
	mockClient := getMockClient(200, errorResp)

	// Action
	granted, err := mockClient.CheckNfsShareAccessStatus(ctx, sharePath, client, vStoreID, accessVal)

	// Assert
	require.Error(t, err)
	require.False(t, granted)
	require.Contains(t, err.Error(), "1077939726")
}

func TestCheckNfsShareAccessStatus_DataTypeError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	sharePath := "/mnt/nfs"
	client := "192.168.1.1"
	vStoreID := "vstore-789"
	accessVal := constants.AuthClientReadOnly

	invalidDataResp := `{
		"error": {"code": 0},
		"data": "invalid-type"
	}`
	mockClient := getMockClient(200, invalidDataResp)

	// Action
	granted, err := mockClient.CheckNfsShareAccessStatus(ctx, sharePath, client, vStoreID, accessVal)

	// Assert
	require.Error(t, err)
	require.False(t, granted)
	require.Contains(t, err.Error(), "failed to convert response data")
}

func TestCheckNfsShareAccessStatus_StatusMissing(t *testing.T) {
	// Arrange
	ctx := context.Background()
	sharePath := "/mnt/nfs"
	client := "192.168.1.1"
	vStoreID := "vstore-789"
	accessVal := constants.AuthClientReadOnly

	missingStatusResp := `{
		"error": {"code": 0},
		"data": {}
	}`
	mockClient := getMockClient(200, missingStatusResp)

	// Action
	granted, err := mockClient.CheckNfsShareAccessStatus(ctx, sharePath, client, vStoreID, accessVal)

	// Assert
	require.ErrorContains(t, err, "failed to get status from response data")
	require.False(t, granted)
}
