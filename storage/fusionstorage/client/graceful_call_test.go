/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
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
	"net/http"
	"net/url"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
)

type mockRespData struct {
	Name string `json:"name"`
}

func Test_gracefulCallAndMarshal(t *testing.T) {
	// arrange
	ctx := context.Background()
	testRespBody := []byte(`{ "data": { "name": "testName" }, "result": { "code": 0, "description": "" } }`)

	// mock
	p := gomonkey.NewPatches().ApplyPrivateMethod(testClient, "doCall",
		func(_ *RestClient, _ context.Context, _, _ string, _ any) (http.Header, []byte, error) {
			return nil, testRespBody, nil
		})
	defer p.Reset()

	// action
	resp, err := gracefulCallAndMarshal[mockRespData](ctx, testClient, "GET", "testUrl", nil)

	// assert
	require.NoError(t, err)
	require.Equal(t, resp.Data.Name, "testName")
}

func Test_gracefulCall_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	testRespBody := []byte(`{ "data": { "name": "testName" }, "result": { "code": 0, "description": "" } }`)

	// mock
	p := gomonkey.NewPatches().ApplyPrivateMethod(testClient, "doCall",
		func(_ *RestClient, _ context.Context, _, _ string, _ any) (http.Header, []byte, error) {
			return nil, testRespBody, nil
		})
	defer p.Reset()

	// action
	resp, err := gracefulCall[mockRespData](ctx, testClient, "GET", "testUrl", nil)

	// assert
	require.NoError(t, err)
	require.Equal(t, resp.Data.Name, "testName")
}

func Test_gracefulCall_Failed(t *testing.T) {
	// arrange
	ctx := context.Background()

	// mock
	p := gomonkey.NewPatches().ApplyPrivateMethod(testClient, "doCall",
		func(_ *RestClient, _ context.Context, _, _ string, _ any) (http.Header, []byte, error) {
			return nil, nil, &url.Error{}
		})
	defer p.Reset()

	// action
	_, err := gracefulCall[mockRespData](ctx, testClient, "GET", "testUrl", nil)

	// assert
	require.Error(t, err)
}

func TestResponse_NeedRetry(t *testing.T) {
	// arrange
	type testCase[T any] struct {
		name string
		resp Response[T]
		want bool
	}
	tests := []testCase[mockRespData]{
		{name: "off line", resp: Response[mockRespData]{Result: ResponseResult{Code: offLineCodeInt}}, want: true},
		{name: "not auth", resp: Response[mockRespData]{Result: ResponseResult{Code: noAuthenticated}}, want: true},
		{name: "success", resp: Response[mockRespData]{Result: ResponseResult{Code: 0}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			got := tt.resp.NeedRetry()

			// assert
			require.Equal(t, got, tt.want)
		})
	}
}

func TestResponse_GetErrorCode(t *testing.T) {
	type testCase[T any] struct {
		name string
		resp Response[T]
		want int64
	}
	tests := []testCase[mockRespData]{
		{name: "response errorCode", resp: Response[mockRespData]{ErrorCode: 1}, want: 1},
		{name: "response result code", resp: Response[mockRespData]{Result: ResponseResult{Code: 2}}, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			got := tt.resp.GetErrorCode()

			// assert
			require.Equal(t, got, tt.want)
		})
	}
}
