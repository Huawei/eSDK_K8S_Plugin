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

// Package client provides DME A-series storage client
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
)

func writeJSON(t *testing.T, w http.ResponseWriter, data string) {
	t.Helper()
	_, err := w.Write([]byte(data))
	assert.NoError(t, err)
}

var taskSuccessResp = `
		{
			"task_id": "bbca21d3-cdd3-4de1-af3e-1407a07c7e50"
		}
	`

var queryTaskResp = `
[
    {
        "id": "bbca21d3-cdd3-4de1-af3e-1407a07c7e50",
        "name_en": "Modify File System",
        "parent_id": "bbca21d3-cdd3-4de1-af3e-1407a07c7e50",
        "status": 3,
        "detail_en": "The device failed to process the request."
    },
    {
        "id": "b1239b91-ce1c-46df-82ba-19094e9a0f17",
        "name_en": "Modify File System Pre-check",
        "parent_id": "bbca21d3-cdd3-4de1-af3e-1407a07c7e50",
        "status": 3,
        "detail_en": ""
    },
    {
        "id": "dcad2334-80a0-41dc-a46f-5e257df98b41",
        "name_en": "Check file system names",
        "parent_id": "b1239b91-ce1c-46df-82ba-19094e9a0f17",
        "status": 3,
        "detail_en": ""
    }
]
`

func getMockClientWithServer(serverURL string) *BaseClient {
	cli, _ := NewBaseClient(context.Background(), &storage.NewClientConfig{})
	cli.url = serverURL
	cli.token = "test-token"
	cli.client = &http.Client{}
	return cli
}

func TestFilesystemClient_UpdateFileSystem_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, "/rest/fileservice/v1-sync/filesystems/")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	err := cli.UpdateFileSystem(context.Background(), "bbb", &UpdateFileSystemParams{111})
	assert.NoError(t, err)
}

func TestFilesystemClient_UpdateFileSystem_NullPointerError(t *testing.T) {
	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer("http://localhost")}
	err := cli.UpdateFileSystem(context.Background(), "bbb", nil)
	assert.Error(t, err)
}

func TestFilesystemClient_UpdateFileSystem_responseError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"code":"500","description":"internal error"}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	err := cli.UpdateFileSystem(context.Background(), "bbb", &UpdateFileSystemParams{111})
	assert.Error(t, err)
}

func TestFilesystemClient_UpdateFileSystem_FallbackToAsync(t *testing.T) {
	// Arrange - sync API returns apiNotFoundCode in legacy format, async API succeeds
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.Contains(r.URL.Path, "v1-sync") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(t, w, `{"errorCode":"49401026001","exceptionInfo":"can not find api"}`)
		} else {
			// Async API returns task
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(t, w, `{"task_id":"task-123"}`)
		}
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodFunc(cli, "ReLogin", func(_ context.Context) error { return nil })
	patches.ApplyMethodReturn(cli, "GetTaskInfos", []*Task{{ID: "task-123", Status: TaskStatusSuccess}}, nil)

	err := cli.UpdateFileSystem(context.Background(), "bbb", &UpdateFileSystemParams{111})
	assert.NoError(t, err)
}

func TestFilesystemClient_UpdateFileSystem_OtherBizError(t *testing.T) {
	// Arrange - sync API returns a different business error, should not fallback
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"error_code":"12345","error_msg":"some other error"}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	err := cli.UpdateFileSystem(context.Background(), "bbb", &UpdateFileSystemParams{111})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "12345")
}

func TestFilesystemClient_DeleteFileSystem_Success(t *testing.T) {
	// Arrange
	patch := gomonkey.ApplyMethod((*MockTransport)(nil), "RoundTrip",
		func(t *MockTransport, req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "filesystems") {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(taskSuccessResp)),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(queryTaskResp)),
			}, nil
		}).
		ApplyFuncReturn(time.Sleep)
	defer patch.Reset()
	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, "")}

	// Action
	err := cli.DeleteFileSystem(context.Background(), "bbb")

	// Assert
	assert.NoError(t, err)
}

func TestFilesystemClient_DeleteFileSystem_Error(t *testing.T) {
	// Arrange
	errorResp := ""

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, errorResp)}

	// Action
	err := cli.DeleteFileSystem(context.Background(), "bbb")

	// Assert
	assert.Error(t, err)
}

func TestFilesystemClient_GetFileSystemByID_Success(t *testing.T) {
	// Arrange
	successResp := `
		{
			"id": "aaa",
			"name": "bbb",
			"description": "ccc",
			"health_status": "normal",
			"running_status": "online",
			"alloc_type": "thin",
			"type": "normal",
			"total_capacity_in_byte": 20,
			"available_capacity_in_byte": 10
		}
	`
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/rest/fileservice/v1/filesystems/")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, successResp)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}

	// Action
	filesystem, err := cli.GetFileSystemByID(context.Background(), "aaa")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "aaa", filesystem.ID)
	assert.Equal(t, int64(10), filesystem.AvailableCapacityInByte)
}

func TestFilesystemClient_GetFileSystemByID_Error(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}

	// Action
	fs, err := cli.GetFileSystemByID(context.Background(), "aaa")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, fs)
	assert.Contains(t, err.Error(), "get filesystem for fsId: aaa failed")
}

func TestFilesystemClient_GetFileSystemByName_Success(t *testing.T) {
	// Arrange
	successResp := `
		{
			"total": 2,
			"data": [
				{
					"id": "aaa",
					"name": "ccc",
					"description": "FileSystem in hyscale",
					"health_status": "normal",
					"running_status": "online",
					"alloc_type": "thin",
					"type": "normal",
					"capacity": 2,
					"available_capacity": 1.999
				},
				{
					"id": "bbb",
					"name": "cccsss",
					"description": "FileSystem in hyscale",
					"health_status": "normal",
					"running_status": "online",
					"alloc_type": "thin",
					"type": "normal",
					"capacity": 5.3,
					"available_capacity": 3.999
				}
			]
		}
	`
	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	info, err := cli.GetFileSystemByName(context.Background(), "ccc")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, info.Name, "ccc")
}

var createParam = `
		{
			"storage_id": "89ecf0ab-95aa-30ee-8480-7ca596c9bd64",
			"zone_id": "89ecf0ab-95aa-30ee-8480-7ca596c9bd64",
			"pool_raw_id": "1",
			"filesystem_specs": [
				{
					"name": "hyscale-filesystem11",
					"capacity": 2,
					"count": 1,
					"description": "FileSystem in hyscale"
				}
			],
			"create_nfs_share_param": {
				"storage_id": "89ecf0ab-95aa-30ee-8480-7ca596c9bd64",
				"description": "",
				"share_path": "/hyscale-filesystem11/",
				"nfs_share_client_addition": [
					{
						"name": "*",
						"permission": "read/write",
						"write_mode": "synchronization",
						"permission_constraint": "no_all_squash",
						"root_permission_constraint": "no_root_squash",
						"accesskrb5": "no_permission",
						"accesskrb5i": "no_permission",
						"accesskrb5p": "no_permission"
					}
				]
			},
			"create_dpc_share_param": {
				"description": "dpc share",
				"charset": "UTF_8",
				"dpc_share_auth": [
					{
						"dpc_user_id": "8F65541068B63E4E85F351979203823A",
						"permission": "read_and_write"
					}
				]
			},
			"snapshot_dir_visible": true,
			"tuning": {
				"allocation_type": "thin"
			}
		}
	`

func TestFilesystemClient_CreateFileSystem_Success(t *testing.T) {
	// Arrange
	patch := gomonkey.ApplyMethod((*MockTransport)(nil), "RoundTrip",
		func(t *MockTransport, req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "filesystems") {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(taskSuccessResp)),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(queryTaskResp)),
			}, nil
		}).
		ApplyFuncReturn(time.Sleep)
	defer patch.Reset()
	param := CreateFilesystemParams{}
	err := json.Unmarshal([]byte(createParam), &param)

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, "")}

	// Action
	err = cli.CreateFileSystem(context.Background(), &param)

	// Assert
	assert.NoError(t, err)
}

func TestFilesystemClient_GetDataTurboShareByPath_Success(t *testing.T) {
	// Arrange
	successResp := `
		{
			"total": 1,
			"data": [
				{
					"id": "D7B61A59AEA63A70B50ACE49269E31CE"
				}
			]
		}
`

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	share, err := cli.GetDataTurboShareByPath(context.Background(), "")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, share)
	assert.Equal(t, "D7B61A59AEA63A70B50ACE49269E31CE", share.ID)
}

func TestFilesystemClient_GetDataTurboShareByPath_Empty(t *testing.T) {
	// Arrange - query returns total=0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"total":0,"data":[]}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}

	// Action
	share, err := cli.GetDataTurboShareByPath(context.Background(), "")

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, share)
}

func TestFilesystemClient_DeleteDataTurboShare_Success(t *testing.T) {
	// Arrange
	patch := gomonkey.ApplyMethod((*MockTransport)(nil), "RoundTrip",
		func(t *MockTransport, req *http.Request) (*http.Response, error) {
			if req.URL.String() == deleteDataTurboShareUrl {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(taskSuccessResp)),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(queryTaskResp)),
			}, nil
		}).
		ApplyFuncReturn(time.Sleep)
	defer patch.Reset()

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, "")}

	// Action
	err := cli.DeleteDataTurboShare(context.Background(), "aaa")

	// Assert
	assert.NoError(t, err)
}

func TestFilesystemClient_DeleteDataTurboShare_Error(t *testing.T) {
	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, "")}

	// Action
	err := cli.DeleteDataTurboShare(context.Background(), "aaa")

	// Assert
	assert.Error(t, err)
}

func TestFilesystemClient_GetDataTurboUserByName_Success(t *testing.T) {
	// Arrange
	successResp := `
		{
			"total": 1,
			"administrators": [
				{
					"id": "8F65541068B63E4E85F351979203823A",
					"name": "dpcmanager"
				}
			]
		}
`
	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	admin, err := cli.GetDataTurboUserByName(context.Background(), "aaa")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, admin)
	assert.Equal(t, "8F65541068B63E4E85F351979203823A", admin.ID)
}

func TestFilesystemClient_GetDataTurboUserByName_Empty(t *testing.T) {
	// Arrange - query returns total=0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"total":0,"administrators":[]}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}

	// Action
	admin, err := cli.GetDataTurboUserByName(context.Background(), "aaa")

	// Assert
	assert.NoError(t, err)
	assert.Empty(t, admin)
}

func TestFilesystemClient_GetNfsShareByPath_Success(t *testing.T) {
	// Arrange
	var successResp = `
		{
			"total": 1,
			"nfs_share_info_list": [
				{
					"id": "D670FE30F9AA3AC29E510568966B8A5E"
				}
			]
		}
`

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	resp, err := cli.GetNfsShareByPath(context.Background(), "/local-filesystem/")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "D670FE30F9AA3AC29E510568966B8A5E", resp.ID)
}

func TestFilesystemClient_DeleteNfsShare_Success(t *testing.T) {
	// Arrange
	patch := gomonkey.ApplyMethod((*MockTransport)(nil), "RoundTrip",
		func(t *MockTransport, req *http.Request) (*http.Response, error) {
			if req.URL.String() == deleteNfsShareUrl {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(taskSuccessResp)),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(queryTaskResp)),
			}, nil
		}).
		ApplyFuncReturn(time.Sleep)
	defer patch.Reset()

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, "")}

	// Action
	err := cli.DeleteNfsShare(context.Background(), "")

	// Assert
	assert.Nil(t, err)
}

func TestFilesystemClient_DeleteNfsPrivateShare_Success(t *testing.T) {
	// Arrange
	patch := gomonkey.ApplyMethod((*MockTransport)(nil), "RoundTrip",
		func(t *MockTransport, req *http.Request) (*http.Response, error) {
			if req.URL.String() == deleteNfsShareUrl {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(taskSuccessResp)),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(queryTaskResp)),
			}, nil
		}).
		ApplyFuncReturn(time.Sleep)
	defer patch.Reset()

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, "")}

	// Action
	err := cli.DeleteNfsPrivateShare(context.Background(), "")

	// Assert
	assert.Nil(t, err)
}

func TestFilesystemClient_CreateKVCache(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/rest/fileservice/v1-sync/kv-cache-stores", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"id":"kv-id-1","raw_id":"1","name":"test-kv","capacity":20971520}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	params := &CreateKVCacheParams{
		StorageID:         "storage-1",
		ZoneID:            "zone-1",
		PoolRawID:         "1",
		VstoreID:          "vstore-1",
		DataCleanupSwitch: "off",
		KVCacheStores:     []KVCacheStoreBaseInfo{{Name: "test-kv", Capacity: 20971520}},
	}
	result, err := cli.CreateKVCache(context.Background(), params)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "1", result.RawID)
}

func TestFilesystemClient_DeleteKVCache(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/rest/fileservice/v1-sync/kv-cache-stores/raw-id-1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	err := cli.DeleteKVCache(context.Background(), "raw-id-1")
	assert.NoError(t, err)
}

func TestFilesystemClient_DeleteKVCache_EmptyBody(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/rest/fileservice/v1-sync/kv-cache-stores/raw-id-1", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	err := cli.DeleteKVCache(context.Background(), "raw-id-1")
	assert.NoError(t, err)
}

func TestFilesystemClient_QueryKVCache_ByName(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/rest/kvcachemgmt/v1/kv-cache-stores/query", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"total":1,"kv_cache_stores":[`+
			`{"id":"kv-id-1","raw_id":"1","name":"test-kv","vstore_id":"vstore-1"}]}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	params := &QueryKVCacheParams{Name: "test-kv", VstoreID: "vstore-1"}
	result, err := cli.QueryKVCache(context.Background(), params)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestFilesystemClient_QueryKVCache_NotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"total":0,"kv_cache_stores":[]}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	params := &QueryKVCacheParams{Name: "nonexistent"}
	result, err := cli.QueryKVCache(context.Background(), params)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestFilesystemClient_UpdateFileSystem_FallbackAsyncError(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/sessions") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(t, w, "{}")
			return
		}
		if strings.Contains(r.URL.Path, "v1-sync") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(t, w, `{"errorCode":"49401026001","exceptionInfo":"can not find api"}`)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
		}
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodFunc(cli, "ReLogin", func(_ context.Context) error { return nil })

	// Action
	err := cli.UpdateFileSystem(context.Background(), "fs-1", &UpdateFileSystemParams{100})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update filesystem for fsId: fs-1 failed")
}

func TestFilesystemClient_UpdateFileSystem_LegacyErrorCodeFallback(t *testing.T) {
	// Arrange - DME old env returns errorCode/exceptionInfo format
	// gracefulCall returns LegacyError directly (no retry for apiNotFound)
	// UpdateFileSystem detects LegacyError.IsApiNotFound() → fallback to async API → success
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.Contains(r.URL.Path, "v1-sync") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(t, w, `{"errorCode":"49401026001",`+
				`"exceptionInfo":"can not find api, please check if the request url is valid or the api has published!`+
				` url=/rest/fileservice/v1-sync/filesystems/123"}`)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(t, w, `{"task_id":"task-456"}`)
		}
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(cli, "GetTaskInfos", []*Task{{ID: "task-456", Status: TaskStatusSuccess}}, nil)

	// Action
	err := cli.UpdateFileSystem(context.Background(), "123", &UpdateFileSystemParams{200})

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 2, callCount) // sync + async(fallback)
}

func TestFilesystemClient_CreateKVCache_Error(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	params := &CreateKVCacheParams{StorageID: "storage-1", ZoneID: "zone-1"}

	// Action
	result, err := cli.CreateKVCache(context.Background(), params)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "create KVCache failed")
}

func TestFilesystemClient_DeleteKVCache_Error(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}

	// Action
	err := cli.DeleteKVCache(context.Background(), "raw-id-1")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete KVCache raw-id-1 failed")
}

func TestFilesystemClient_QueryKVCache_Error(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
	}))
	defer mockServer.Close()

	cli := &FilesystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}
	params := &QueryKVCacheParams{Name: "test-kv"}

	// Action
	result, err := cli.QueryKVCache(context.Background(), params)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query KVCache failed")
}
