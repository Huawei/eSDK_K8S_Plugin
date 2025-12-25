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
	"errors"
	"fmt"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	offLineCode     = "4012"
	noAuthenticated = "4011"

	// maxRetryTime define the max retry duration of dme async task
	maxRetryTime = 30 * time.Minute

	// initialRetryInterval defines the initial retry interval of querying dme async task status
	initialRetryInterval = 5 * time.Second

	// maxRetryInterval defines the max retry interval of querying dme async task status
	maxRetryInterval = 5 * time.Minute

	// TaskStatusInit defines the init status of task
	TaskStatusInit = 1

	// TaskStatusRunning defines the running status of task
	TaskStatusRunning = 2

	// TaskStatusSuccess defines the success status of task
	TaskStatusSuccess = 3

	// TaskStatusPartFailed defines the part failed status of task
	TaskStatusPartFailed = 4

	// TaskStatusFailed defines the failed status of task
	TaskStatusFailed = 5

	// TaskStatusTimeout defines the timeout status of task
	TaskStatusTimeout = 6
)

// TaskResponse defines the response body of task request
type TaskResponse struct {
	TaskID string `json:"task_id"`
}

// BusinessError defines the error response of business type
type BusinessError struct {
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// Error implements the error interface, and return the formated error info of BusinessError
func (e BusinessError) Error() string {
	return fmt.Sprintf("code: %s, err message: %s", e.ErrorCode, e.ErrorMessage)
}

// AuthError defines the error response of auth type
type AuthError struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// LoginError defines the error response of login exception type
type LoginError struct {
	ExceptionId   string `json:"exceptionId"`
	ExceptionType string `json:"exceptionType"`
}

// Error implements the error interface, and return the formated error info of LoginError
func (e LoginError) Error() string {
	return fmt.Sprintf("exceptionId: %s, exceptionType: %s", e.ExceptionId, e.ExceptionType)
}

// Error implements the error interface, and return the formated error info of AuthError
func (e AuthError) Error() string {
	return fmt.Sprintf("code: %s, description: %s", e.Code, e.Description)
}

// NeedRetry determines whether the current AuthError need to be retried
func (e AuthError) NeedRetry() bool {
	return e.Code == offLineCode || e.Code == noAuthenticated
}

type AuthBusinessError struct {
	*AuthError     `json:",inline"`
	*BusinessError `json:",inline"`
	*LoginError    `json:",inline"`
}

func gracefulCallWithTaskWait(ctx context.Context, cli BaseClientInterface, method, url string, reqData any) error {
	task, err := gracefulCall[TaskResponse](ctx, cli, method, url, reqData)
	if err != nil {
		return err
	}

	if task.TaskID == "" {
		return errors.New("run task failed with empty return")
	}

	retryInterval := initialRetryInterval
	for i := 0 * time.Second; i < maxRetryTime; {
		taskInfos, err := cli.GetTaskInfos(ctx, task.TaskID)
		if err != nil {
			return err
		}

		for _, taskInfo := range taskInfos {
			if taskInfo.ID != task.TaskID {
				continue
			}

			switch taskInfo.Status {
			case TaskStatusInit, TaskStatusRunning:
				continue
			case TaskStatusSuccess:
				return nil
			case TaskStatusPartFailed, TaskStatusFailed, TaskStatusTimeout:
				return fmt.Errorf("task id %s run failed, status: %d, err msg: %s",
					task.TaskID, taskInfo.Status, taskInfo.Detail)
			default:
				return fmt.Errorf("got task %s with unknown status: %d, err msg: %s",
					task.TaskID, taskInfo.Status, taskInfo.Detail)
			}
		}

		time.Sleep(retryInterval)
		i += retryInterval
		retryInterval = calculateNextSleepTime(retryInterval, maxRetryInterval)
	}

	return fmt.Errorf("run task %s time out", task.TaskID)
}

func calculateNextSleepTime(currentInterval, maxInterval time.Duration) time.Duration {
	nextInterval := currentInterval * 2
	if nextInterval > maxInterval {
		return maxInterval
	}

	return nextInterval
}

func gracefulCall[T any](ctx context.Context, cli BaseClientInterface, method, url string, reqData any) (*T, error) {
	resp, err := gracefulCallAndMarshal[T](ctx, cli, method, url, reqData)
	if err != nil {
		if err.Error() == storage.Unconnected {
			return gracefulRetryCall[T](ctx, cli, method, url, reqData)
		}

		var errResp AuthError
		if errors.As(err, &errResp) && errResp.NeedRetry() {
			log.AddContext(ctx).Warningln("User offline, try to relogin")
			return gracefulRetryCall[T](ctx, cli, method, url, reqData)
		}

		return nil, err
	}

	return resp, nil
}

func gracefulRetryCall[T any](ctx context.Context,
	cli BaseClientInterface, method, url string, reqData any) (*T, error) {
	log.AddContext(ctx).Debugf("retry call: method: %s, url: %s, data: %v.", method, url, reqData)

	err := cli.ReLogin(ctx)
	if err != nil {
		return nil, err
	}

	return gracefulCallAndMarshal[T](ctx, cli, method, url, reqData)
}

func IsArr(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte{'['})
}

func gracefulCallAndMarshal[T any](ctx context.Context,
	cli BaseClientInterface, method string, url string, reqData any) (*T, error) {
	respBody, err := cli.Call(ctx, method, url, reqData)
	if err != nil {
		return nil, err
	}

	if !IsArr(respBody) {
		var resp AuthBusinessError
		err = json.Unmarshal(respBody, &resp)
		if err != nil {
			return nil, fmt.Errorf("unmarshal response body to Response failed, err: %w", err)
		}

		if resp.AuthError != nil && resp.AuthError.Code != "" {
			return nil, *resp.AuthError
		}

		if resp.BusinessError != nil && resp.BusinessError.ErrorCode != "" {
			return nil, *resp.BusinessError
		}

		if resp.LoginError != nil && resp.LoginError.ExceptionId != "" {
			return nil, fmt.Errorf("login failed: %w", *resp.LoginError)
		}
	}

	var resp T
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		return nil, fmt.Errorf("unmarshal response body to %T failed, err: %w", resp, err)
	}
	return &resp, nil
}
