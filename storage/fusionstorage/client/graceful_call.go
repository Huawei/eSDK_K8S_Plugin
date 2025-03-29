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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// ResponseResult defines the response result from Restful API
type ResponseResult struct {
	Code        int64  `json:"code"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// Response defines the response struct from Restful API
type Response[T any] struct {
	Data       T              `json:"data"`
	Result     ResponseResult `json:"result"`
	ErrorCode  int64          `json:"errorCode"`
	Suggestion string         `json:"suggestion"`
}

// GetErrorCode returns the error code from the response
func (resp *Response[T]) GetErrorCode() int64 {
	if resp.ErrorCode != 0 {
		return resp.ErrorCode
	}

	return resp.Result.Code
}

// NeedRetry determines whether the request needs to be retried
func (resp *Response[T]) NeedRetry() bool {
	// when user is offline or is not authenticated, need retry request
	return resp.GetErrorCode() == offLineCodeInt || resp.GetErrorCode() == noAuthenticated
}

func gracefulGet[T any](ctx context.Context, cli *RestClient, url string) (*Response[T], error) {
	return gracefulCall[T](ctx, cli, "GET", url, nil)
}

func gracefulPost[T any](ctx context.Context, cli *RestClient, url string, reqData any) (*Response[T], error) {
	return gracefulCall[T](ctx, cli, "POST", url, reqData)
}

func gracefulDelete[T any](ctx context.Context, cli *RestClient, url string, reqData any) (*Response[T], error) {
	return gracefulCall[T](ctx, cli, "DELETE", url, reqData)
}

func gracefulPut[T any](ctx context.Context, cli *RestClient, url string, reqData any) (*Response[T], error) {
	return gracefulCall[T](ctx, cli, "PUT", url, reqData)
}

func gracefulCall[T any](ctx context.Context, cli *RestClient, method, url string, reqData any) (*Response[T], error) {
	resp, err := gracefulCallAndMarshal[T](ctx, cli, method, url, reqData)
	if err != nil {
		if err.Error() == unconnectedError {
			return gracefulRetryCall[T](ctx, cli, method, url, reqData)
		}

		return nil, err
	}

	if resp != nil && resp.NeedRetry() {
		log.AddContext(ctx).Warningf("User offline, try to relogin %s", cli.url)
		return gracefulRetryCall[T](ctx, cli, method, url, reqData)
	}

	return resp, nil
}

func gracefulRetryCall[T any](ctx context.Context,
	cli *RestClient, method, url string, reqData any) (*Response[T], error) {
	log.AddContext(ctx).Debugf("retry call: method: %s, url: %s, data: %v.", method, url, reqData)

	err := cli.reLogin(ctx)
	if err != nil {
		return nil, err
	}

	return gracefulCallAndMarshal[T](ctx, cli, method, url, reqData)
}

func gracefulCallAndMarshal[T any](ctx context.Context,
	cli *RestClient, method string, url string, reqData any) (*Response[T], error) {
	_, respBody, err := cli.doCall(ctx, method, url, reqData)
	if err != nil {
		return nil, err
	}

	var resp Response[T]
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal response body %s error: %v", respBody, err)
		return nil, err
	}

	return &resp, nil
}
