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
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
)

type mockRespData struct {
	Name string `json:"name"`
}

func Test_gracefulCallAndMarshal_NasSuccess(t *testing.T) {
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
	resp, err := gracefulCallAndMarshal[*NasResponse[mockRespData]](ctx, testClient, "GET", "testUrl", nil)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, resp.Data.Name, "testName")
}

func Test_gracefulCall_NasSuccess(t *testing.T) {
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
	resp, err := gracefulCall[*NasResponse[mockRespData]](ctx, testClient, "GET", "testUrl", nil)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, resp.Data.Name, "testName")
}

func Test_gracefulCall_NasFailed(t *testing.T) {
	// arrange
	ctx := context.Background()

	// mock
	p := gomonkey.NewPatches().ApplyPrivateMethod(testClient, "doCall",
		func(_ *RestClient, _ context.Context, _, _ string, _ any) (http.Header, []byte, error) {
			return nil, nil, &url.Error{}
		})
	defer p.Reset()

	// action
	_, err := gracefulCall[*NasResponse[mockRespData]](ctx, testClient, "GET", "testUrl", nil)

	// assert
	assert.Error(t, err)
}

func TestResponse_NeedRetry(t *testing.T) {
	// arrange
	type testCase[T any] struct {
		name string
		resp NasResponse[T]
		want bool
	}
	tests := []testCase[mockRespData]{
		{name: "off line", resp: NasResponse[mockRespData]{Result: NasResult{Code: offLineCodeInt}}, want: true},
		{name: "not auth", resp: NasResponse[mockRespData]{Result: NasResult{Code: noAuthenticated}}, want: true},
		{name: "success", resp: NasResponse[mockRespData]{Result: NasResult{Code: 0}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			got := tt.resp.NeedRetry()

			// assert
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestResponse_GetErrorCode(t *testing.T) {
	type testCase[T any] struct {
		name string
		resp NasResponse[T]
		want int64
	}
	tests := []testCase[mockRespData]{
		{name: "response errorCode", resp: NasResponse[mockRespData]{ErrorCode: 1}, want: 1},
		{name: "response result code", resp: NasResponse[mockRespData]{Result: NasResult{Code: 2}}, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			got := tt.resp.GetErrorCode()

			// assert
			assert.Equal(t, got, tt.want)
		})
	}
}

type mockSanRespData struct {
	*SanBaseResponse
	Name string `json:"name"`
}

func Test_gracefulCall_SanSuccess(t *testing.T) {
	// arrange
	ctx := context.Background()
	testRespBody := []byte(`{"result":0,"name":"test"}`)

	// mock
	p := gomonkey.NewPatches().ApplyPrivateMethod(testClient, "doCall",
		func(_ *RestClient, _ context.Context, _, _ string, _ any) (http.Header, []byte, error) {
			return nil, testRespBody, nil
		})
	defer p.Reset()

	// action
	resp, err := gracefulCall[mockSanRespData](ctx, testClient, "GET", "testUrl", nil)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, resp.Name, "test")
}

func Test_gracefulCall_SanOffLineRetry(t *testing.T) {
	// arrange
	ctx := context.Background()
	retryBody := []byte(`{"result":2,"suggestion":"Log in to the system again.","errorCode":"1077949069",
"description":"The user is offline.","i18n_description":"The user is offline."}`)
	successBody := []byte(`{"result":0,"name":"test"}`)

	// mock
	count := 0
	p := gomonkey.NewPatches().ApplyPrivateMethod(testClient, "doCall",
		func(_ *RestClient, _ context.Context, _, _ string, _ any) (http.Header, []byte, error) {
			if count == 0 {
				count++
				return nil, retryBody, nil
			}

			return nil, successBody, nil
		}).ApplyMethodReturn(testClient, "ReLogin", nil)
	defer p.Reset()

	// action
	resp, err := gracefulCall[mockSanRespData](ctx, testClient, "GET", "testUrl", nil)

	// assert
	assert.Equal(t, count, 1)
	assert.NoError(t, err)
	assert.Equal(t, resp.Name, "test")
}

func Test_gracefulCall_SanUnConnectRetry(t *testing.T) {
	// arrange
	ctx := context.Background()
	successBody := []byte(`{"result":0,"name":"test"}`)

	// mock
	count := 0
	p := gomonkey.NewPatches().ApplyPrivateMethod(testClient, "doCall",
		func(_ *RestClient, _ context.Context, _, _ string, _ any) (http.Header, []byte, error) {
			if count == 0 {
				count++
				return nil, nil, errors.New(unconnectedError)
			}

			return nil, successBody, nil
		}).ApplyMethodReturn(testClient, "ReLogin", nil)
	defer p.Reset()

	// action
	resp, err := gracefulCall[mockSanRespData](ctx, testClient, "GET", "testUrl", nil)

	// assert
	assert.Equal(t, count, 1)
	assert.NoError(t, err)
	assert.Equal(t, resp.Name, "test")
}

func TestSanBaseResponse_NeedRetry(t *testing.T) {
	testCases := []struct {
		name        string
		errorCode   any
		expectRetry bool
	}{
		{
			name:        "nil error code",
			errorCode:   nil,
			expectRetry: false,
		},
		{
			name:        "no auth code as string",
			errorCode:   noAuthCodeStr,
			expectRetry: true,
		},
		{
			name:        "offline code as string",
			errorCode:   offLineCodeStr,
			expectRetry: true,
		},
		{
			name:        "other string code",
			errorCode:   "12345678",
			expectRetry: false,
		},
		{
			name:        "no auth code as float64",
			errorCode:   10000003.0,
			expectRetry: true,
		},
		{
			name:        "offline code as float64",
			errorCode:   1077949069.0,
			expectRetry: true,
		},
		{
			name:        "other float64 code",
			errorCode:   1234567890.0,
			expectRetry: false,
		},
		{
			name:        "int as error code (should not match)",
			errorCode:   10000003,
			expectRetry: false,
		},
		{
			name:        "bool as error code (should not match)",
			errorCode:   true,
			expectRetry: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			resp := &SanBaseResponse{
				ErrorCode: tc.errorCode,
			}

			// act
			result := resp.NeedRetry()

			// assert
			if result != tc.expectRetry {
				t.Errorf("NeedRetry() = %v, want %v", result, tc.expectRetry)
			}
		})
	}
}

func TestSanBaseResponse_IsErrorCodeSet(t *testing.T) {
	testCases := []struct {
		name      string
		errorCode any
		expectSet bool
	}{
		{
			name:      "nil error code",
			errorCode: nil,
			expectSet: false,
		},
		{
			name:      "empty string error code",
			errorCode: "",
			expectSet: false,
		},
		{
			name:      "non-empty string error code",
			errorCode: "123",
			expectSet: true,
		},
		{
			name:      "zero float64 error code",
			errorCode: 0.0,
			expectSet: false,
		},
		{
			name:      "non-zero float64 error code",
			errorCode: 123.45,
			expectSet: true,
		},
		{
			name:      "int error code (should be treated as non-zero)",
			errorCode: 100,
			expectSet: true,
		},
		{
			name:      "bool error code (true)",
			errorCode: true,
			expectSet: true,
		},
		{
			name:      "bool error code (false)",
			errorCode: false,
			expectSet: true,
		},
		{
			name:      "slice error code",
			errorCode: []int{},
			expectSet: true,
		},
		{
			name:      "map error code",
			errorCode: map[string]string{},
			expectSet: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			resp := &SanBaseResponse{
				ErrorCode: tc.errorCode,
			}

			// act
			result := resp.IsErrorCodeSet()

			// assert
			if result != tc.expectSet {
				t.Errorf("IsErrorCodeSet() = %v, want %v", result, tc.expectSet)
			}
		})
	}
}
