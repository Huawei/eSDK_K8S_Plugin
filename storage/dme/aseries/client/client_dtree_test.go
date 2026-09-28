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
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
)

// newMockDTreeClient creates a DTreeClient with a BaseClient for gomonkey method patching
func newMockDTreeClient() *DTreeClient {
	return &DTreeClient{BaseClientInterface: &BaseClient{}}
}

// --- CreateDTree ---

func TestDTreeClient_CreateDTree_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	wantResp := &CreateDTreeResponse{
		DtreeID:             "dtree-123",
		DtreeRawID:          "raw-123",
		DtreeName:           "pvc-abc",
		NfsShareID:          "nfs-456",
		NfsShareRawID:       "nfsraw-456",
		NfsSharePath:        "/fs/pvc-abc",
		DataturboShareID:    "dpc-789",
		DataturboShareRawID: "dpcraw-789",
		DataturboSharePath:  "/fs/pvc-abc",
	}
	mockRespBody, err := json.Marshal(wantResp)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.CreateDTree(context.Background(), &CreateDTreeParams{})

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, wantResp, resp)
}

func TestDTreeClient_CreateDTree_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("network error"))

	// Action
	resp, err := mockCli.CreateDTree(context.Background(), &CreateDTreeParams{})

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "create DTree failed")
}

func TestDTreeClient_CreateDTree_BusinessError(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	bizErr := BusinessError{ErrorCode: "1001", ErrorMessage: "invalid param"}
	mockRespBody, err := json.Marshal(bizErr)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.CreateDTree(context.Background(), &CreateDTreeParams{})

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "create DTree failed")
}

// --- DeleteDTreeByID ---

func TestDTreeClient_DeleteDTreeByID_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", []byte{}, nil)

	// Action
	err := mockCli.DeleteDTreeByID(context.Background(), "dtree-123")

	// Assert
	assert.Nil(t, err)
}

func TestDTreeClient_DeleteDTreeByID_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, storage.ErrUnconnected)
	patches.ApplyMethodReturn(mockCli, "ReLogin", errors.New("relogin failed"))

	// Action
	err := mockCli.DeleteDTreeByID(context.Background(), "dtree-123")

	// Assert
	assert.ErrorContains(t, err, "delete DTree for dtreeID: dtree-123 failed")
}

// --- GetDTreeByName ---

func TestDTreeClient_GetDTreeByName_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	wantDTree := &DTreeInfo{ID: "dtree-1", RawID: "raw-1", Name: "mytree", FsID: "fs-1"}
	respBody := &QueryDTreeListResponse{
		Total:  1,
		Dtrees: []*DTreeInfo{wantDTree},
	}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetDTreeByName(context.Background(), "fs-1", "mytree")

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, wantDTree, resp)
}

func TestDTreeClient_GetDTreeByName_NotFound(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	respBody := &QueryDTreeListResponse{Total: 0, Dtrees: []*DTreeInfo{}}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetDTreeByName(context.Background(), "fs-1", "notexist")

	// Assert
	assert.Nil(t, err)
	assert.Nil(t, resp)
}

func TestDTreeClient_GetDTreeByName_NameMismatch(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	otherDTree := &DTreeInfo{ID: "dtree-2", RawID: "raw-2", Name: "other", FsID: "fs-1"}
	respBody := &QueryDTreeListResponse{
		Total:  1,
		Dtrees: []*DTreeInfo{otherDTree},
	}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetDTreeByName(context.Background(), "fs-1", "mytree")

	// Assert
	assert.Nil(t, err)
	assert.Nil(t, resp)
}

func TestDTreeClient_GetDTreeByName_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("network error"))

	// Action
	resp, err := mockCli.GetDTreeByName(context.Background(), "fs-1", "mytree")

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "get DTree by name: mytree failed")
}

// --- CreateDTreeQuota ---

func TestDTreeClient_CreateDTreeQuota_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	wantQuota := &QuotaInfo{ID: "quota-1", SpaceHardQuota: 1073741824, ParentRawID: "raw-1"}
	mockRespBody, err := json.Marshal(wantQuota)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.CreateDTreeQuota(context.Background(), &CreateQuotaParams{
		ParentID:       "dtree-1",
		ParentType:     quotaType,
		QuotaType:      quotaType,
		SpaceHardQuota: 1073741824,
	})

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, wantQuota, resp)
}

func TestDTreeClient_CreateDTreeQuota_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("fail"))

	// Action
	resp, err := mockCli.CreateDTreeQuota(context.Background(), &CreateQuotaParams{})

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "create DTree quota failed")
}

// --- GetDTreeQuotaByRawID ---

func TestDTreeClient_GetDTreeQuotaByRawID_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	wantQuota := &QuotaInfo{ID: "quota-1", SpaceHardQuota: 1073741824, ParentRawID: "raw-1"}
	respBody := &QueryQuotaListResponse{
		Total: 1,
		Datas: []*QuotaInfo{wantQuota},
	}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetDTreeQuotaByRawID(context.Background(), "raw-1")

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, wantQuota, resp)
}

func TestDTreeClient_GetDTreeQuotaByRawID_NotFound(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	respBody := &QueryQuotaListResponse{Total: 0, Datas: []*QuotaInfo{}}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetDTreeQuotaByRawID(context.Background(), "raw-1")

	// Assert
	assert.Nil(t, err)
	assert.Nil(t, resp)
}

func TestDTreeClient_GetDTreeQuotaByRawID_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("fail"))

	// Action
	resp, err := mockCli.GetDTreeQuotaByRawID(context.Background(), "raw-1")

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "get DTree quota for parentRawID: raw-1 failed")
}

// --- UpdateDTreeQuota ---

func TestDTreeClient_UpdateDTreeQuota_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", []byte{}, nil)

	// Action
	err := mockCli.UpdateDTreeQuota(context.Background(), "quota-1", &UpdateQuotaParams{
		SpaceHardQuota: 5368709120,
	})

	// Assert
	assert.Nil(t, err)
}

func TestDTreeClient_UpdateDTreeQuota_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("fail"))

	// Action
	err := mockCli.UpdateDTreeQuota(context.Background(), "quota-1", &UpdateQuotaParams{
		SpaceHardQuota: 5368709120,
	})

	// Assert
	assert.ErrorContains(t, err, "update DTree quota for quotaID: quota-1 failed")
}

// --- DeleteDTreeQuota ---

func TestDTreeClient_DeleteDTreeQuota_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", []byte{}, nil)

	// Action
	err := mockCli.DeleteDTreeQuota(context.Background(), "quota-1")

	// Assert
	assert.Nil(t, err)
}

func TestDTreeClient_DeleteDTreeQuota_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("fail"))

	// Action
	err := mockCli.DeleteDTreeQuota(context.Background(), "quota-1")

	// Assert
	assert.ErrorContains(t, err, "delete DTree quota for quotaID: quota-1 failed")
}

// --- GetNfsShareByDTreePath ---

func TestDTreeClient_GetNfsShareByDTreePath_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	wantShare := &DTreeNfsShareInfo{ID: "nfs-1", SharePath: "/fs/pvc-abc"}
	respBody := &QueryDTreeNfsShareListResponse{
		Total: 1,
		Data:  []*DTreeNfsShareInfo{wantShare},
	}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetNfsShareByDTreePath(context.Background(), "/fs/pvc-abc")

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, wantShare, resp)
}

func TestDTreeClient_GetNfsShareByDTreePath_NotFound(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	respBody := &QueryDTreeNfsShareListResponse{Total: 0, Data: []*DTreeNfsShareInfo{}}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetNfsShareByDTreePath(context.Background(), "/fs/notexist")

	// Assert
	assert.Nil(t, err)
	assert.Nil(t, resp)
}

func TestDTreeClient_GetNfsShareByDTreePath_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("fail"))

	// Action
	resp, err := mockCli.GetNfsShareByDTreePath(context.Background(), "/fs/pvc-abc")

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "get NFS share by DTree path: /fs/pvc-abc failed")
}

// --- DeleteDTreeNfsShare ---

func TestDTreeClient_DeleteDTreeNfsShare_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", []byte{}, nil)

	// Action
	err := mockCli.DeleteDTreeNfsShare(context.Background(), "nfs-1")

	// Assert
	assert.Nil(t, err)
}

func TestDTreeClient_DeleteDTreeNfsShare_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("fail"))

	// Action
	err := mockCli.DeleteDTreeNfsShare(context.Background(), "nfs-1")

	// Assert
	assert.ErrorContains(t, err, "delete NFS share for nfsShareID: nfs-1 failed")
}

// --- GetDataTurboShareByDTreePath ---

func TestDTreeClient_GetDataTurboShareByDTreePath_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	wantShare := &DTreeDpcShareInfo{ID: "dpc-1", SharePath: "/fs/pvc-abc"}
	respBody := &QueryDTreeDpcShareListResponse{
		Total: 1,
		Data:  []*DTreeDpcShareInfo{wantShare},
	}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetDataTurboShareByDTreePath(context.Background(), "/fs/pvc-abc")

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, wantShare, resp)
}

func TestDTreeClient_GetDataTurboShareByDTreePath_NotFound(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	respBody := &QueryDTreeDpcShareListResponse{Total: 0, Data: []*DTreeDpcShareInfo{}}
	mockRespBody, err := json.Marshal(respBody)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.GetDataTurboShareByDTreePath(context.Background(), "/fs/notexist")

	// Assert
	assert.Nil(t, err)
	assert.Nil(t, resp)
}

func TestDTreeClient_GetDataTurboShareByDTreePath_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "GetStorageID", "storage-1")
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("fail"))

	// Action
	resp, err := mockCli.GetDataTurboShareByDTreePath(context.Background(), "/fs/pvc-abc")

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "get DataTurbo share by DTree path: /fs/pvc-abc failed")
}

// --- DeleteDTreeDataTurboShare ---

func TestDTreeClient_DeleteDTreeDataTurboShare_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", []byte{}, nil)

	// Action
	err := mockCli.DeleteDTreeDataTurboShare(context.Background(), "dpc-1")

	// Assert
	assert.Nil(t, err)
}

func TestDTreeClient_DeleteDTreeDataTurboShare_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("fail"))

	// Action
	err := mockCli.DeleteDTreeDataTurboShare(context.Background(), "dpc-1")

	// Assert
	assert.ErrorContains(t, err, "delete DataTurbo share for dpcShareID: dpc-1 failed")
}

// --- CreateDTreeNfsShare ---

func TestDTreeClient_CreateDTreeNfsShare_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	wantShare := &DTreeNfsShareInfo{ID: "nfs-new", SharePath: "/fs/pvc-abc"}
	mockRespBody, err := json.Marshal(wantShare)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.CreateDTreeNfsShare(context.Background(), CreateNfsShareRequestBody{
		CreateNfsShareParam: DTreeCreateNfsShareParam{
			SharePath:   "/fs/pvc-abc",
			FsID:        "fs-1",
			Description: "test share",
		},
	})

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, wantShare, resp)
}

func TestDTreeClient_CreateDTreeNfsShare_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("network error"))

	// Action
	resp, err := mockCli.CreateDTreeNfsShare(context.Background(), CreateNfsShareRequestBody{
		CreateNfsShareParam: DTreeCreateNfsShareParam{
			SharePath: "/fs/pvc-abc",
		},
	})

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "create NFS share for path /fs/pvc-abc failed")
}

// --- CreateDTreeDpcShare ---

func TestDTreeClient_CreateDTreeDpcShare_Success(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()
	wantShare := &DTreeDpcShareInfo{ID: "dpc-new", SharePath: "/fs/pvc-abc"}
	mockRespBody, err := json.Marshal(wantShare)
	require.NoError(t, err)

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// Action
	resp, err := mockCli.CreateDTreeDpcShare(context.Background(), CreateDpcShareParams{
		DtreeID: "dtree-1",
		Charset: "UTF_8",
	})

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, wantShare, resp)
}

func TestDTreeClient_CreateDTreeDpcShare_Error(t *testing.T) {
	// Arrange
	mockCli := newMockDTreeClient()

	// Mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", nil, errors.New("network error"))

	// Action
	resp, err := mockCli.CreateDTreeDpcShare(context.Background(), CreateDpcShareParams{
		DtreeID: "dtree-1",
		Charset: "UTF_8",
	})

	// Assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "create DPC share for DTree dtree-1 failed")
}
