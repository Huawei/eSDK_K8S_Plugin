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
	"strings"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
)

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

func TestFilesystemClient_UpdateFileSystem_Success(t *testing.T) {
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
	err := cli.UpdateFileSystem(context.Background(), "bbb", &UpdateFileSystemParams{111})

	// Assert
	assert.NoError(t, err)
}

func TestFilesystemClient_UpdateFileSystem_NullPointerError(t *testing.T) {
	// Arrange
	errorResp := ""

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, errorResp)}

	// Action
	err1 := cli.UpdateFileSystem(context.Background(), "bbb", nil)

	// Assert
	assert.Error(t, err1)
}

func TestFilesystemClient_UpdateFileSystem_responseError(t *testing.T) {
	// Arrange
	errorResp := ""

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, errorResp)}

	// Action
	err2 := cli.UpdateFileSystem(context.Background(), "bbb", &UpdateFileSystemParams{111})

	// Assert
	assert.Error(t, err2)
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
	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	filesystem, err := cli.GetFileSystemByID(context.Background(), "aaa")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "aaa", filesystem.ID)
	assert.Equal(t, int64(10), filesystem.AvailableCapacityInByte)
}

func TestFilesystemClient_GetFileSystemByID_Error(t *testing.T) {
	// Arrange
	errorResp := ""

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, errorResp)}

	// Action
	resp, err := cli.GetFileSystemByID(context.Background(), "bbb")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resp)
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

func TestFilesystemClient_GetDataTurboShareByPath_Error(t *testing.T) {
	// Arrange
	successResp := ""

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	share, err := cli.GetDataTurboShareByPath(context.Background(), "")

	// Assert
	assert.Error(t, err)
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

func TestFilesystemClient_GetDataTurboUserByName_Error(t *testing.T) {
	// Arrange
	successResp := ""

	// Mock
	cli := &FilesystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	admin, err := cli.GetDataTurboUserByName(context.Background(), "aaa")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, admin)
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
