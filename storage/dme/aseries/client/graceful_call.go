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

	// apiNotFoundCode is the error code returned when DME does not support the sync API
	apiNotFoundCode = "49401026001"

	// DME login credential exception IDs - indicate authentication/authorization failures
	// that should mark the backend as offline
	loginExceptionUserOrValueInvalid            = "user.login.user_or_value_invalid"
	loginExceptionUserOrValueInvalidLockDefault = "user.login.user_or_value_invalid_lock_default"
	loginExceptionPolicyViolationLock           = "user.user.policy_violation_lock"
	loginExceptionUserOrValueInvalidLock        = "user.login.user_or_value_invalid_lock"
	loginExceptionPwdExpired                    = "user.pwd.expired"
	loginExceptionPolicyViolationStop           = "user.user.policy_violation_stop"
	loginExceptionTouchOnlineLimit              = "user.login.touch_online_limit"
	loginExceptionPolicyViolationLockDefault    = "user.user.policy_violation_lock_default"

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
	ErrorMessage string `json:"error_msg"`
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

var loginCredentialExceptionIds = []string{
	loginExceptionUserOrValueInvalid,
	loginExceptionUserOrValueInvalidLockDefault,
	loginExceptionPolicyViolationLock,
	loginExceptionUserOrValueInvalidLock,
	loginExceptionPwdExpired,
	loginExceptionPolicyViolationStop,
	loginExceptionTouchOnlineLimit,
	loginExceptionPolicyViolationLockDefault,
}

// isCredentialError checks whether the login error is caused by invalid credentials
func isCredentialError(err error) bool {
	// Context cancellation and deadline errors are transient
	if err == nil || errors.Is(err, storage.ErrUnconnected) || errors.Is(err, context.Canceled) || errors.Is(err,
		context.DeadlineExceeded) {
		return false
	}
	// AuthError with session-related codes (4012/4011) is retriable, not credential
	var authErr AuthError
	if errors.As(err, &authErr) {
		return !authErr.NeedRetry()
	}
	// LoginError with credential exception IDs is a credential error
	var loginErr LoginError
	if errors.As(err, &loginErr) {
		for _, id := range loginCredentialExceptionIds {
			if loginErr.ExceptionId == id {
				return true
			}
		}
		return false
	}
	return false
}

// LegacyError defines the error response from old DME versions (camelCase fields)
type LegacyError struct {
	ErrorCode     string `json:"errorCode"`
	ExceptionInfo string `json:"exceptionInfo"`
}

func (e LegacyError) Error() string {
	return fmt.Sprintf("code: %s, err message: %s", e.ErrorCode, e.ExceptionInfo)
}

// IsApiNotFound returns true if the error indicates the sync API is not supported by DME
func (e LegacyError) IsApiNotFound() bool {
	return e.ErrorCode == apiNotFoundCode
}

// SyncFallbackUrls defines sync and async URL pair for gracefulCallWithSyncFallback
type SyncFallbackUrls struct {
	SyncUrl  string
	AsyncUrl string
}

// gracefulCallWithSyncFallback tries the sync API first; if DME does not support it
// (errorCode 49401026001), falls back to the async API for backward compatibility.
func gracefulCallWithSyncFallback(ctx context.Context, cli BaseClientInterface, method string,
	urls *SyncFallbackUrls, reqData any) error {
	_, err := gracefulCall[struct{}](ctx, cli, method, urls.SyncUrl, reqData)
	if err == nil {
		return nil
	}

	var legacyErr LegacyError
	if errors.As(err, &legacyErr) && legacyErr.IsApiNotFound() {
		return gracefulCallWithTaskWait(ctx, cli, method, urls.AsyncUrl, reqData)
	}

	return err
}

func gracefulCallWithTaskWait(ctx context.Context, cli BaseClientInterface, method, url string, reqData any) error {
	task, err := gracefulCall[TaskResponse](ctx, cli, method, url, reqData)
	if err != nil {
		return err
	}

	if task == nil || task.TaskID == "" {
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
		if errors.Is(err, storage.ErrUnconnected) {
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
	if len(respBody) == 0 {
		var resp T
		return &resp, nil
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

	var legacyErr LegacyError
	if json.Unmarshal(respBody, &legacyErr) == nil && legacyErr.ErrorCode != "" {
		return nil, legacyErr
	}

	var resp T
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		return nil, fmt.Errorf("unmarshal response body to %T failed, err: %w", resp, err)
	}
	return &resp, nil
}
