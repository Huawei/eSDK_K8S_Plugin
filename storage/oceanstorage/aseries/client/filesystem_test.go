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

// Package client provides oceanstor A-series storage client
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName           = "aSeriesClientTest.log"
	mockErrCode int64 = 123456
	mockErrStr        = "123456"
)

var (
	testClient *OceanASeriesClient
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	m.Run()
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

type MockTransport struct {
	Response *http.Response
	Err      error
}

func getMockClientWithResponse(statusCode int, body string) *OceanASeriesClient {
	testClient, _ = NewClient(context.Background(), &storage.NewClientConfig{
		Urls:            []string{"https://127.0.0.1:8088"},
		User:            "dev-account",
		SecretName:      "mock-sec-name",
		SecretNamespace: "mock-sec-namespace",
		ParallelNum:     "",
		BackendID:       "mock-backend-id",
	})

	testClient.Client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}

	return testClient
}

func TestOceanASeriesClient_GetFileSystemByName_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	name := "test-fs"
	vstoreId := "0"
	successBody := `{
        "data": [
            {"ID": "1", "NAME": "test-fs"}
        ],
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, successBody)

	// action
	result, err := mockClient.GetFileSystemByName(ctx, name, vstoreId)

	// assert
	require.NoError(t, err)
	require.Equal(t, "test-fs", result["NAME"])
}

func TestOceanASeriesClient_GetFileSystemByName_NotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	name := "not-exist-fs"
	vstoreId := "0"
	emptyBody := `{
        "data": [],
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, emptyBody)

	// action
	result, err := mockClient.GetFileSystemByName(ctx, name, vstoreId)

	// assert
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestOceanASeriesClient_GetFileSystemByName_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	name := "error-fs"
	vstoreId := "0"
	errorBody := fmt.Sprintf(`{
        "data": null,
        "error": {
            "code": %d,
            "description": "permission denied"
        }
    }`, mockErrCode)

	// mock
	mockClient := getMockClientWithResponse(200, errorBody)

	// action
	_, err := mockClient.GetFileSystemByName(ctx, name, vstoreId)

	// assert
	require.ErrorContains(t, err, mockErrStr)
}

func TestOceanASeriesClient_GetFileSystemByName_InvalidDataFormat(t *testing.T) {
	// arrange
	ctx := context.Background()
	name := "invalid-fs"
	vstoreId := "0"
	invalidBody := `{
        "data": "invalid_string",
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, invalidBody)

	// action
	_, err := mockClient.GetFileSystemByName(ctx, name, vstoreId)

	// assert
	require.ErrorContains(t, err, "convert respData to array failed")
}

func TestOceanASeriesClient_GetFileSystemByName_EmptyData(t *testing.T) {
	// arrange
	ctx := context.Background()
	name := "empty-fs"
	vstoreId := "0"
	emptyBody := `{
        "data": null,
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, emptyBody)

	// action
	result, err := mockClient.GetFileSystemByName(ctx, name, vstoreId)

	// assert
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestOceanASeriesClient_GetFileSystemByName_InvalidElementType(t *testing.T) {
	// arrange
	ctx := context.Background()
	name := "invalid-element-fs"
	vstoreId := "0"
	invalidBody := `{
        "data": [123],
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, invalidBody)

	// action
	_, err := mockClient.GetFileSystemByName(ctx, name, vstoreId)

	// assert
	require.ErrorContains(t, err, "convert filesystem to map failed")
}

func TestOceanASeriesClient_CreateFileSystem_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &CreateFilesystemParams{
		Name:        "test-fs",
		ParentId:    "parent-123",
		Capacity:    1024,
		Description: "test filesystem",
	}
	mockRespBody := `{
        "data": {
            "ID": "fs-001",
            "NAME": "test-fs"
        },
        "error": {
            "code": 0
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, mockRespBody)

	// action
	result, err := mockClient.CreateFileSystem(ctx, params, nil)

	// assert
	require.NoError(t, err)
	assert.Equal(t, "fs-001", result["ID"])
	assert.Equal(t, "test-fs", result["NAME"])
}

func TestOceanASeriesClient_CreateFileSystem_ErrorCodeHandling(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &CreateFilesystemParams{
		Name:     "fs1",
		ParentId: "p1",
		Capacity: 100,
	}
	errorBody := fmt.Sprintf(`{
        "data": null,
        "error": {
            "code": %d,
            "description": "permission denied"
        }
    }`, mockErrCode)

	// mock
	mockClient := getMockClientWithResponse(200, errorBody)

	// action
	_, err := mockClient.CreateFileSystem(ctx, params, nil)

	// assert
	require.ErrorContains(t, err, mockErrStr)
}

func TestOceanASeriesClient_CreateFileSystem_AdvancedOptions(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &CreateFilesystemParams{
		Name:     "adv-fs",
		ParentId: "p1",
		Capacity: 1024,
		VstoreId: "vstore-1",
	}
	advOptions := map[string]interface{}{
		"QOS": map[string]int{"iops": 1000},
	}
	mockRespBody := `{
        "data": {"ID": "fs-adv"},
        "error": {"code": 0}
    }`

	// mock
	mockClient := getMockClientWithResponse(200, mockRespBody)

	// action
	result, err := mockClient.CreateFileSystem(ctx, params, advOptions)

	// assert
	require.NoError(t, err)
	assert.Equal(t, "fs-adv", result["ID"])
}

func TestOceanASeriesClient_CreateFileSystem_InvalidResponse(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &CreateFilesystemParams{
		Name:     "fs1",
		ParentId: "p1",
		Capacity: 100,
	}
	invalidRespBody := `{
        "data": "invalid_string",
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, invalidRespBody)

	// action
	_, err := mockClient.CreateFileSystem(ctx, params, nil)

	// assert
	require.ErrorContains(t, err, "convert filesystem to map failed")
}

func TestOceanASeriesClient_CreateFileSystem_OptionalParams(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &CreateFilesystemParams{
		Name:            "full-fs",
		ParentId:        "p1",
		Capacity:        1024,
		WorkLoadTypeId:  "workload-1",
		UnixPermissions: "755",
		VstoreId:        "vstore-1",
	}
	mockRespBody := `{
        "data": {"ID": "full-001"},
        "error": {"code": 0}
    }`

	// mock
	mockClient := getMockClientWithResponse(200, mockRespBody)

	// action
	_, err := mockClient.CreateFileSystem(ctx, params, nil)

	// assert
	require.NoError(t, err)
}

func TestOceanASeriesClient_CreateDataTurboShare_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &CreateDataTurboShareParams{
		SharePath:   "/data/share1",
		FsId:        "fs-001",
		Description: "test share",
	}
	mockRespBody := `{
        "data": {
            "ID": "share-001",
            "sharePath": "/data/share1"
        },
        "error": {
            "code": 0
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, mockRespBody)

	// action
	result, err := mockClient.CreateDataTurboShare(ctx, params)

	// assert
	require.NoError(t, err)
	assert.Equal(t, "share-001", result["ID"])
	assert.Equal(t, params.SharePath, result["sharePath"])
}

func TestOceanASeriesClient_CreateDataTurboShare_ErrorHandling(t *testing.T) {
	// arrange
	ctx := context.Background()
	validParams := &CreateDataTurboShareParams{
		SharePath: "/valid/path",
		FsId:      "fs-valid",
	}
	errorBody := fmt.Sprintf(`{
        "data": null,
        "error": {
            "code": %d,
            "description": "permission denied"
        }
    }`, mockErrCode)

	// mock
	mockClient := getMockClientWithResponse(200, errorBody)

	// action
	_, err := mockClient.CreateDataTurboShare(ctx, validParams)

	// assert
	require.ErrorContains(t, err, mockErrStr)
}

func TestOceanASeriesClient_CreateDataTurboShare_WithVstoreId(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &CreateDataTurboShareParams{
		SharePath: "/vstore/share",
		FsId:      "fs-vstore",
		VstoreId:  "vstore-001",
	}
	mockRespBody := `{
        "data": {"vstoreId": "vstore-001"},
        "error": {"code":0}
    }`

	// mock
	mockClient := getMockClientWithResponse(200, mockRespBody)

	// action
	result, err := mockClient.CreateDataTurboShare(ctx, params)

	// assert
	require.NoError(t, err)
	assert.Equal(t, params.VstoreId, result["vstoreId"])
}

func TestOceanASeriesClient_CreateDataTurboShare_InvalidResponse(t *testing.T) {
	// arrange
	ctx := context.Background()
	validParams := &CreateDataTurboShareParams{
		SharePath: "/valid",
		FsId:      "fs-001",
	}
	invalidRespBody := `{
        "data": "invalid_string",
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, invalidRespBody)

	// action
	_, err := mockClient.CreateDataTurboShare(ctx, validParams)

	// assert
	require.ErrorContains(t, err, "convert DataTurbo share to map failed")
}

func TestOceanASeriesClient_GetDataTurboShareByPath_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	path := "test-ds"
	vstoreId := "0"
	successBody := `{
        "data": [
            {"ID": "1", "NAME": "test-ds", "sharePath": "test-ds"}
        ],
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, successBody)

	// action
	result, err := mockClient.GetDataTurboShareByPath(ctx, path, vstoreId)

	// assert
	require.NoError(t, err)
	require.Equal(t, "test-ds", result["NAME"])
}

func TestOceanASeriesClient_GetDataTurboShareByPath_NotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	path := "not-exist-ds"
	vstoreId := "0"
	emptyBody := `{
        "data": [],
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, emptyBody)

	// action
	result, err := mockClient.GetDataTurboShareByPath(ctx, path, vstoreId)

	// assert
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestOceanASeriesClient_GetDataTurboShareByPath_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	path := "error-ds"
	vstoreId := "0"
	errorBody := fmt.Sprintf(`{
        "data": null,
        "error": {
            "code": %d,
            "description": "permission denied"
        }
    }`, mockErrCode)

	// mock
	mockClient := getMockClientWithResponse(200, errorBody)

	// action
	_, err := mockClient.GetDataTurboShareByPath(ctx, path, vstoreId)

	// assert
	require.ErrorContains(t, err, mockErrStr)
}

func TestOceanASeriesClient_GetDataTurboShareByPath_InvalidDataFormat(t *testing.T) {
	// arrange
	ctx := context.Background()
	path := "invalid-ds"
	vstoreId := "0"
	invalidBody := `{
        "data": "invalid_string",
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, invalidBody)

	// action
	_, err := mockClient.GetDataTurboShareByPath(ctx, path, vstoreId)

	// assert
	require.ErrorContains(t, err, "convert respData to array failed")
}

func TestOceanASeriesClient_GetDataTurboShareByPath_EmptyData(t *testing.T) {
	// arrange
	ctx := context.Background()
	path := "empty-ds"
	vstoreId := "0"
	emptyBody := `{
        "data": null,
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, emptyBody)

	// action
	result, err := mockClient.GetDataTurboShareByPath(ctx, path, vstoreId)

	// assert
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestOceanASeriesClient_GetDataTurboShareByPath_InvalidElementType(t *testing.T) {
	// arrange
	ctx := context.Background()
	path := "invalid-element-ds"
	vstoreId := "0"
	invalidBody := `{
        "data": [123],
        "error": {
            "code": 0,
            "description": "success"
        }
    }`

	// mock
	mockClient := getMockClientWithResponse(200, invalidBody)

	// action
	_, err := mockClient.GetDataTurboShareByPath(ctx, path, vstoreId)

	// assert
	require.NoError(t, err)
}

func TestOceanASeriesClient_DeleteDataTurboShare_WithVstoreId(t *testing.T) {
	// arrange
	ctx := context.Background()
	shareID := "share-vstore"
	vstoreID := "vstore-001"
	successRespBody := `{
        "error": {"code":0}
    }`

	// mock
	mockClient := getMockClientWithResponse(200, successRespBody)

	// action
	err := mockClient.DeleteDataTurboShare(ctx, shareID, vstoreID)

	// assert
	require.NoError(t, err)
}

func TestOceanASeriesClient_DeleteDataTurboShare_NotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	shareID := "non-exist-share"
	notExistRespBody := fmt.Sprintf(`{
        "error": {
            "code": %d
        }
    }`, storage.ShareNotExist)

	// mock
	mockClient := getMockClientWithResponse(200, notExistRespBody)

	// action
	err := mockClient.DeleteDataTurboShare(ctx, shareID, "")

	// assert
	require.NoError(t, err)
}

func TestOceanASeriesClient_DeleteDataTurboShare_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	shareID := "error-share"
	errorBody := fmt.Sprintf(`{
        "data": null,
        "error": {
            "code": %d,
            "description": "permission denied"
        }
    }`, mockErrCode)

	// mock
	mockClient := getMockClientWithResponse(200, errorBody)

	// action
	err := mockClient.DeleteDataTurboShare(ctx, shareID, "")

	// assert
	require.ErrorContains(t, err, mockErrStr)
}

func TestOceanASeriesClient_RemoveDataTurboShareUser_WithVstoreId(t *testing.T) {
	// arrange
	ctx := context.Background()
	authID := "auth-vstore"
	vstoreID := "vstore-001"
	successRespBody := `{
        "error": {"code":0}
    }`

	// mock
	mockClient := getMockClientWithResponse(200, successRespBody)

	// action
	err := mockClient.RemoveDataTurboShareUser(ctx, authID, vstoreID)

	// assert
	require.NoError(t, err)
}

func TestOceanASeriesClient_RemoveDataTurboShareUser_NotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	authID := "non-exist-auth"
	notExistRespBody := fmt.Sprintf(`{
        "error": {
            "code": %d
        }
    }`, storage.AuthUserNotExist)

	// mock
	mockClient := getMockClientWithResponse(200, notExistRespBody)

	// action
	err := mockClient.RemoveDataTurboShareUser(ctx, authID, "")

	// assert
	require.NoError(t, err)
}

func TestOceanASeriesClient_RemoveDataTurboShareUser_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	authID := "error-auth"
	errorBody := fmt.Sprintf(`{
        "data": null,
        "error": {
            "code": %d,
            "description": "permission denied"
        }
    }`, mockErrCode)

	// mock
	mockClient := getMockClientWithResponse(200, errorBody)

	// action
	err := mockClient.RemoveDataTurboShareUser(ctx, authID, "")

	// assert
	require.ErrorContains(t, err, mockErrStr)
}

func TestOceanASeriesClient_AddDataTurboShareUser_WithVstoreId(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &AddDataTurboShareUserParams{
		UserName:   "auth-user-success",
		ShareId:    "1",
		Permission: 755,
		VstoreId:   "0",
	}
	successRespBody := `{
        "error": {"code":0}
    }`

	// mock
	mockClient := getMockClientWithResponse(200, successRespBody)

	// action
	err := mockClient.AddDataTurboShareUser(ctx, params)

	// assert
	require.NoError(t, err)
}

func TestOceanASeriesClient_AddDataTurboShareUser_ErrorCode(t *testing.T) {
	// arrange
	ctx := context.Background()
	params := &AddDataTurboShareUserParams{
		UserName:   "auth-user-error",
		ShareId:    "1",
		Permission: 755,
		VstoreId:   "0",
	}
	errorBody := fmt.Sprintf(`{
        "data": null,
        "error": {
            "code": %d,
            "description": "permission denied"
        }
    }`, mockErrCode)

	// mock
	mockClient := getMockClientWithResponse(200, errorBody)

	// action
	err := mockClient.AddDataTurboShareUser(ctx, params)

	// assert
	require.ErrorContains(t, err, mockErrStr)
}
