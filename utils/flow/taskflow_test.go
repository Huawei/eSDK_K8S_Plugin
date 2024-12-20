/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package flow

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	errMsg = "an error occurred while run mock fun"

	mockFun1 = func(ctx context.Context, params map[string]interface{}) error {
		params["key_1"] = "value_1"
		return nil
	}
	mockFun2 = func(ctx context.Context, params map[string]interface{}) error {
		params["key_2"] = "value_2"
		return nil
	}
	mockFun3 = func(ctx context.Context, params map[string]interface{}) error {
		return errors.New(errMsg)
	}
)

const (
	logName = "taskFlowTest.log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestAllTaskReturnSuccess(t *testing.T) {
	testParams := map[string]interface{}{}
	err := NewTaskFlow(context.Background(), "test_all_task_return_success").
		AddTaskWithOutRevert(mockFun1).
		AddTaskWithOutRevert(mockFun2).
		RunWithOutRevert(testParams)
	if err != nil {
		t.Errorf("an error occurred while run TestTaskWithOutRevert(), err: %v", err)
	}

	result := map[string]interface{}{
		"key_1": "value_1",
		"key_2": "value_2",
	}
	if !reflect.DeepEqual(testParams, result) {
		t.Error("got an unexpected value while run TestTaskWithOutRevert()")
	}
}

func TestRunTaskFail(t *testing.T) {
	testParams := map[string]interface{}{}
	err := NewTaskFlow(context.Background(), "test_run_task_fail").
		AddTaskWithOutRevert(mockFun1).
		AddTaskWithOutRevert(mockFun2).
		AddTaskWithOutRevert(mockFun3).
		RunWithOutRevert(testParams)
	if err == nil {
		t.Error("an error should be returned while run TestRunTaskFail()")
	}

	if err.Error() != errMsg {
		t.Error("got an unexpected error while run TestRunTaskFail()")
	}
}
