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
	"net/http"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
)

type mockRespData struct {
	Name string `json:"name"`
}

func Test_gracefulCall_UnconnectedError(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	unconnectedErr := storage.ErrUnconnected

	wantResp := &mockRespData{Name: "testName"}
	successBody, err := json.Marshal(wantResp)
	if err != nil {
		return
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	callTimes := 0
	patches.ApplyMethodFunc(mockCli, "Call", func(_ context.Context, _, _ string, _ any) ([]byte, error) {
		if callTimes == 0 {
			callTimes++
			return nil, unconnectedErr
		}
		return successBody, nil
	}).ApplyMethodReturn(mockCli, "ReLogin", nil)

	// act
	gotResp, gotErr := gracefulCall[mockRespData](context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.Equal(t, 1, callTimes, "should retry call")
	assert.Nil(t, gotErr)
	assert.Equal(t, wantResp, gotResp)
}

func Test_gracefulCall_AuthErrorNeedRetry(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	authErr := AuthError{Code: offLineCode}
	authErrBody, err := json.Marshal(authErr)
	if err != nil {
		return
	}

	wantResp := &mockRespData{Name: "testName"}
	successBody, err := json.Marshal(wantResp)
	if err != nil {
		return
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	callTimes := 0
	patches.ApplyMethodFunc(mockCli, "Call", func(_ context.Context, _, _ string, _ any) ([]byte, error) {
		if callTimes == 0 {
			callTimes++
			return authErrBody, nil
		}
		return successBody, nil
	}).ApplyMethodReturn(mockCli, "ReLogin", nil)

	// act
	gotResp, gotErr := gracefulCall[mockRespData](context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.Equal(t, 1, callTimes, "should retry call")
	assert.Nil(t, gotErr)
	assert.Equal(t, wantResp, gotResp)
}

func Test_gracefulCall_OtherAuthError(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	wantErr := AuthError{Code: "5001", Description: "unknown error"}
	mockRespBody, err := json.Marshal(wantErr)
	if err != nil {
		return
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// act
	gotResp, gotErr := gracefulCall[mockRespData](context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.Nil(t, gotResp)
	assert.Equal(t, wantErr, gotErr)
}

func Test_gracefulCall_BusinessError(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	wantErr := BusinessError{ErrorCode: "1001", ErrorMessage: "invalid params"}
	mockRespBody, err := json.Marshal(wantErr)
	if err != nil {
		return
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// act
	gotResp, gotErr := gracefulCall[mockRespData](context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.Nil(t, gotResp)
	assert.Equal(t, wantErr, gotErr)
}

func Test_gracefulCall_Success(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	wantResp := &mockRespData{Name: "testName"}
	mockRespBody, err := json.Marshal(wantResp)
	if err != nil {
		return
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// act
	gotResp, gotErr := gracefulCall[mockRespData](context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, wantResp, gotResp)
}

func Test_gracefulCallWithSync_Success(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	taskID := "taskID"
	taskResp := &TaskResponse{TaskID: taskID}
	mockRespBody, err := json.Marshal(taskResp)
	if err != nil {
		return
	}
	taskInfo := &Task{ID: taskID, Status: TaskStatusSuccess}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil).
		ApplyMethodReturn(mockCli, "GetTaskInfos", []*Task{taskInfo}, nil).
		ApplyFuncReturn(time.Sleep)

	// act
	gotErr := gracefulCallWithTaskWait(context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.Nil(t, gotErr)
}

func Test_gracefulCallWithSync_Timeout(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	taskID := "taskID"
	taskResp := &TaskResponse{TaskID: taskID}
	mockRespBody, err := json.Marshal(taskResp)
	if err != nil {
		return
	}
	taskInfo := &Task{ID: taskID, Status: TaskStatusRunning}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil).
		ApplyMethodReturn(mockCli, "GetTaskInfos", []*Task{taskInfo}, nil).
		ApplyFuncReturn(time.Sleep)

	// act
	gotErr := gracefulCallWithTaskWait(context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.ErrorContains(t, gotErr, "time out")
}

func Test_gracefulCallWithSync_TaskError(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	taskID := "taskID"
	taskResp := &TaskResponse{TaskID: taskID}
	mockRespBody, err := json.Marshal(taskResp)
	if err != nil {
		return
	}
	taskInfo := &Task{ID: taskID, Status: TaskStatusFailed, Detail: "unknown err"}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil).
		ApplyMethodReturn(mockCli, "GetTaskInfos", []*Task{taskInfo}, nil).
		ApplyFuncReturn(time.Sleep)

	// act
	gotErr := gracefulCallWithTaskWait(context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.ErrorContains(t, gotErr, taskInfo.Detail)
}

func Test_gracefulCallWithSync_EmptyTask(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	taskResp := &TaskResponse{TaskID: ""}
	mockRespBody, err := json.Marshal(taskResp)
	if err != nil {
		return
	}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", mockRespBody, nil)

	// act
	gotErr := gracefulCallWithTaskWait(context.Background(), mockCli, "GET", "testUrl", nil)

	// assert
	assert.ErrorContains(t, gotErr, "run task failed with empty return")
}

func Test_gracefulCallWithSyncFallback_SyncSuccess(t *testing.T) {
	// Arrange - sync API succeeds, no fallback needed
	mockCli := &BaseClient{}
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", []byte("{}"), nil)

	// Action
	gotErr := gracefulCallWithSyncFallback(context.Background(), mockCli, http.MethodPut,
		&SyncFallbackUrls{SyncUrl: "syncUrl", AsyncUrl: "asyncUrl"}, nil)

	// Assert
	assert.NoError(t, gotErr)
}

func Test_gracefulCallWithSyncFallback_LegacyErrorFallbackSuccess(t *testing.T) {
	// Arrange - sync API returns LegacyError(apiNotFound), async API succeeds
	mockCli := &BaseClient{}
	legacyErrBody, err := json.Marshal(LegacyError{ErrorCode: apiNotFoundCode, ExceptionInfo: "can not find api"})
	require.NoError(t, err)
	taskRespBody, err := json.Marshal(&TaskResponse{TaskID: "task-1"})
	require.NoError(t, err)
	taskInfo := &Task{ID: "task-1", Status: TaskStatusSuccess}

	callCount := 0
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodFunc(mockCli, "Call", func(_ context.Context, _, url string, _ any) ([]byte, error) {
		callCount++
		if url == "syncUrl" {
			return legacyErrBody, nil
		}
		return taskRespBody, nil
	}).ApplyMethodReturn(mockCli, "GetTaskInfos", []*Task{taskInfo}, nil).
		ApplyFuncReturn(time.Sleep)

	// Action
	gotErr := gracefulCallWithSyncFallback(context.Background(), mockCli, http.MethodPut,
		&SyncFallbackUrls{SyncUrl: "syncUrl", AsyncUrl: "asyncUrl"}, nil)

	// Assert
	assert.NoError(t, gotErr)
	assert.Equal(t, 2, callCount)
}

func Test_gracefulCallWithSyncFallback_LegacyErrorFallbackFail(t *testing.T) {
	// Arrange - sync API returns LegacyError(apiNotFound), async API also fails
	mockCli := &BaseClient{}
	legacyErrBody, err := json.Marshal(LegacyError{ErrorCode: apiNotFoundCode, ExceptionInfo: "can not find api"})
	require.NoError(t, err)
	asyncErrBody, err := json.Marshal(BusinessError{ErrorCode: "500", ErrorMessage: "internal error"})
	require.NoError(t, err)

	callCount := 0
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodFunc(mockCli, "Call", func(_ context.Context, _, url string, _ any) ([]byte, error) {
		callCount++
		if url == "syncUrl" {
			return legacyErrBody, nil
		}
		return asyncErrBody, nil
	})

	// Action
	gotErr := gracefulCallWithSyncFallback(context.Background(), mockCli, http.MethodPut,
		&SyncFallbackUrls{SyncUrl: "syncUrl", AsyncUrl: "asyncUrl"}, nil)

	// Assert
	assert.Error(t, gotErr)
	assert.Equal(t, 2, callCount)
}

func Test_gracefulCallWithSyncFallback_OtherError_NoFallback(t *testing.T) {
	// Arrange - sync API returns non-ApiNotFound error, should not fallback
	mockCli := &BaseClient{}
	wantErr := BusinessError{ErrorCode: "12345", ErrorMessage: "some error"}
	errBody, err := json.Marshal(wantErr)
	require.NoError(t, err)

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", errBody, nil)

	// Action
	gotErr := gracefulCallWithSyncFallback(context.Background(), mockCli, http.MethodPut,
		&SyncFallbackUrls{SyncUrl: "syncUrl", AsyncUrl: "asyncUrl"}, nil)

	// Assert
	assert.Equal(t, wantErr, gotErr)
}

func Test_gracefulCallWithSyncFallback_AuthError_NoFallback(t *testing.T) {
	// Arrange - sync API returns AuthError, should not fallback (gracefulCall retries once then returns)
	mockCli := &BaseClient{}
	authErrBody, err := json.Marshal(AuthError{Code: "5001", Description: "auth error"})
	require.NoError(t, err)

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", authErrBody, nil)

	// Action
	gotErr := gracefulCallWithSyncFallback(context.Background(), mockCli, http.MethodPut,
		&SyncFallbackUrls{SyncUrl: "syncUrl", AsyncUrl: "asyncUrl"}, nil)

	// Assert
	assert.Error(t, gotErr)
}

func Test_gracefulCallWithSyncFallback_LegacyErrorOtherCode_NoFallback(t *testing.T) {
	// Arrange - sync API returns LegacyError with non-ApiNotFound code
	mockCli := &BaseClient{}
	wantErr := LegacyError{ErrorCode: "99999", ExceptionInfo: "other legacy error"}
	errBody, err := json.Marshal(wantErr)
	require.NoError(t, err)

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", errBody, nil)

	// Action
	gotErr := gracefulCallWithSyncFallback(context.Background(), mockCli, http.MethodPut,
		&SyncFallbackUrls{SyncUrl: "syncUrl", AsyncUrl: "asyncUrl"}, nil)

	// Assert
	assert.Equal(t, wantErr, gotErr)
}

func Test_gracefulCallAndMarshal_EmptyBody_DeleteMethod(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	// mock Call to return empty body (e.g. HTTP 204 No Content)
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", []byte{}, nil)

	// act - DELETE with empty body should succeed
	gotResp, gotErr := gracefulCallAndMarshal[mockRespData](
		context.Background(), mockCli, http.MethodDelete, "testUrl", nil)

	// assert
	assert.NoError(t, gotErr)
	assert.NotNil(t, gotResp)
}

func Test_gracefulCallAndMarshal_EmptyBody_ReturnsZeroValue(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(mockCli, "Call", []byte{}, nil)

	// act - empty body returns zero value without error
	gotResp, gotErr := gracefulCallAndMarshal[mockRespData](context.Background(), mockCli, http.MethodGet, "testUrl", nil)

	// assert
	assert.NoError(t, gotErr)
	assert.NotNil(t, gotResp)
	assert.Equal(t, "", gotResp.Name)
}
