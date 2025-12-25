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
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
)

type mockRespData struct {
	Name string `json:"name"`
}

func Test_gracefulCall_UnconnectedError(t *testing.T) {
	// arrange
	mockCli := &BaseClient{}
	unconnectedErr := errors.New(storage.Unconnected)

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
