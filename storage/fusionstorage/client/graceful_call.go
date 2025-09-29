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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	noAuthCodeStr  = "10000003"
	offLineCodeStr = "1077949069"
)

// Retryable defines the retryable interface
type Retryable interface {
	NeedRetry() bool
}

// SanBaseResponse defines the base response fields of san restful call
type SanBaseResponse struct {
	Result      int64  `json:"result"`
	Suggestion  string `json:"suggestion"`
	ErrorCode   any    `json:"errorCode"`
	Description string `json:"description"`
}

// NeedRetry determines whether the request needs to be retried
func (resp *SanBaseResponse) NeedRetry() bool {
	if resp.ErrorCode == nil {
		return false
	}

	switch v := resp.ErrorCode.(type) {
	case string:
		return v == noAuthCodeStr || v == offLineCodeStr
	case float64:
		codeStr := strconv.FormatFloat(v, 'f', -1, 64)
		return codeStr == noAuthCodeStr || codeStr == offLineCodeStr
	default:
		return false
	}
}

// Error return the formated error msg of restful call response
func (resp *SanBaseResponse) Error() string {
	return fmt.Sprintf("error code: %v, description: %s, suggestion: %s",
		resp.ErrorCode, resp.Description, resp.Suggestion)
}

// IsErrorCodeSet return whether the ErrorCode has been set
func (resp *SanBaseResponse) IsErrorCodeSet() bool {
	if resp.ErrorCode == nil {
		return false
	}

	switch v := resp.ErrorCode.(type) {
	case string:
		return v != ""
	case float64:
		return v != 0
	default:
		return true
	}
}

// NasResult defines the response result from Restful API
type NasResult struct {
	Code        int64  `json:"code"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// NasResponse defines the response struct from Restful API
type NasResponse[T any] struct {
	Data       T         `json:"data"`
	Result     NasResult `json:"result"`
	ErrorCode  int64     `json:"errorCode"`
	Suggestion string    `json:"suggestion"`
}

// GetErrorCode returns the error code from the response
func (resp *NasResponse[T]) GetErrorCode() int64 {
	if resp.ErrorCode != 0 {
		return resp.ErrorCode
	}

	return resp.Result.Code
}

// NeedRetry determines whether the request needs to be retried
func (resp *NasResponse[T]) NeedRetry() bool {
	// when user is offline or is not authenticated, need retry request
	return resp.GetErrorCode() == offLineCodeInt || resp.GetErrorCode() == noAuthenticated
}

// san call
func gracefulSanPost[T Retryable](ctx context.Context, cli *RestClient, url string, reqData any) (T, error) {
	return gracefulCall[T](ctx, cli, "POST", url, reqData)
}

// nas call
func gracefulNasGet[T any](ctx context.Context, cli *RestClient, url string) (*NasResponse[T], error) {
	return gracefulCall[*NasResponse[T]](ctx, cli, "GET", url, nil)
}

func gracefulNasPost[T any](ctx context.Context, cli *RestClient, url string, reqData any) (*NasResponse[T], error) {
	return gracefulCall[*NasResponse[T]](ctx, cli, "POST", url, reqData)
}

func gracefulNasDelete[T any](ctx context.Context, cli *RestClient, url string, reqData any) (*NasResponse[T], error) {
	return gracefulCall[*NasResponse[T]](ctx, cli, "DELETE", url, reqData)
}

func gracefulNasPut[T any](ctx context.Context, cli *RestClient, url string, reqData any) (*NasResponse[T], error) {
	return gracefulCall[*NasResponse[T]](ctx, cli, "PUT", url, reqData)
}

func gracefulCall[T Retryable](ctx context.Context, cli *RestClient, method, url string, reqData any) (T, error) {
	resp, err := gracefulCallAndMarshal[T](ctx, cli, method, url, reqData)
	if err != nil {
		if err.Error() == unconnectedError {
			return gracefulRetryCall[T](ctx, cli, method, url, reqData)
		}

		return *new(T), err
	}

	if resp.NeedRetry() {
		log.AddContext(ctx).Warningf("User offline, try to relogin %s", cli.url)
		return gracefulRetryCall[T](ctx, cli, method, url, reqData)
	}

	return resp, nil
}

func gracefulRetryCall[T Retryable](ctx context.Context,
	cli *RestClient, method, url string, reqData any) (T, error) {
	log.AddContext(ctx).Debugf("retry call: method: %s, url: %s, data: %v.", method, url, reqData)

	err := cli.ReLogin(ctx)
	if err != nil {
		return *new(T), err
	}

	return gracefulCallAndMarshal[T](ctx, cli, method, url, reqData)
}

func gracefulCallAndMarshal[T Retryable](ctx context.Context,
	cli *RestClient, method string, url string, reqData any) (T, error) {
	_, respBody, err := cli.doCall(ctx, method, url, reqData)
	if err != nil {
		return *new(T), err
	}

	var resp T
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal response body %s error: %v", respBody, err)
		return *new(T), err
	}

	return resp, nil
}
