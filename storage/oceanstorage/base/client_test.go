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

// Package base provide base operations for oceanstor base storage
package base

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func TestResponse_GetInt64Code(t *testing.T) {
	// arrange
	tests := []struct {
		name            string
		resp            *Response
		wantCode        int64
		wantErrContains string
	}{
		{name: "success", resp: &Response{Error: map[string]any{"code": float64(12345)}}, wantCode: int64(12345),
			wantErrContains: ""},
		{name: "code not exists", resp: &Response{Error: map[string]any{"code1": float64(12345)}},
			wantErrContains: "not exists"},
		{name: "code is not float64", resp: &Response{Error: map[string]any{"code": "12345"}},
			wantErrContains: "not float64"},
		{name: "code is not accuracy", resp: &Response{Error: map[string]any{"code": float64(math.MaxUint64)}},
			wantErrContains: "not accuracy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			code, err := tt.resp.getInt64Code()

			// assert
			if tt.wantErrContains == "" {
				require.NoError(t, err)
				require.Equal(t, tt.wantCode, code)
			} else {
				require.ErrorContains(t, err, tt.wantErrContains)
			}
		})
	}
}

func TestResponse_AssertErrorCode(t *testing.T) {
	// arrange
	tests := []struct {
		name            string
		resp            *Response
		wantErrContains string
	}{
		{name: "assert not error",
			resp: &Response{Error: map[string]any{"code": float64(12345),
				"description": "test description"}},
			wantErrContains: "test description"},
		{name: "assert has error", resp: &Response{Error: map[string]any{"code": float64(0)}}, wantErrContains: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			err := tt.resp.AssertErrorCode()

			// assert
			if tt.wantErrContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErrContains)
			}
		})
	}
}

func TestResponse_AssertErrorWithTolerantErrors(t *testing.T) {
	// arrange
	log.MockInitLogging("test")
	defer log.MockStopLogging("test")
	tests := []struct {
		name            string
		resp            *Response
		tolerantErrs    []ResponseToleration
		wantErrContains string
	}{
		{
			name:         "no error with no tolerant",
			resp:         &Response{Error: map[string]any{"code": float64(0), "description": ""}},
			tolerantErrs: nil, wantErrContains: "",
		},
		{
			name: "has error with no tolerant",
			resp: &Response{Error: map[string]any{"code": float64(12345),
				"description": "test description"}},
			tolerantErrs: nil, wantErrContains: "test description",
		},
		{
			name: "has error with tolerant",
			resp: &Response{Error: map[string]any{"code": float64(12345),
				"description": "test description"}},
			tolerantErrs: []ResponseToleration{{Code: 12345, Reason: "fake reason"}}, wantErrContains: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			err := tt.resp.AssertErrorWithTolerations(context.Background(), tt.tolerantErrs...)

			// assert
			if tt.wantErrContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErrContains)
			}
		})
	}
}

func TestHostClient_GetHostByID_Success(t *testing.T) {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	hostCli := &HostClient{RestClientInterface: cli}
	ctx := context.Background()
	hostID := "123"

	m := gomonkey.ApplyMethodReturn(cli, "Get",
		Response{
			Data:  map[string]interface{}{"ID": "123", "NAME": "test-host"},
			Error: map[string]interface{}{"code": float64(0), "description": ""},
		}, nil)
	defer m.Reset()

	host, err := hostCli.GetHostByID(ctx, hostID)
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{"ID": "123", "NAME": "test-host"}, host)
}

func TestHostClient_GetHostByID_NotFound(t *testing.T) {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	hostCli := &HostClient{RestClientInterface: cli}
	ctx := context.Background()
	hostID := "123"

	m := gomonkey.ApplyMethodReturn(cli, "Get",
		Response{
			Data:  nil,
			Error: map[string]interface{}{"code": float64(storage.ObjectNotExist), "description": "not found"},
		}, nil)
	defer m.Reset()

	host, err := hostCli.GetHostByID(ctx, hostID)
	require.NoError(t, err)
	require.Empty(t, host)
}

func TestHostClient_GetHostByID_GetError(t *testing.T) {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	hostCli := &HostClient{RestClientInterface: cli}
	ctx := context.Background()
	hostID := "123"

	m := gomonkey.ApplyMethodReturn(cli, "Get", Response{}, errors.New("network error"))
	defer m.Reset()

	_, err := hostCli.GetHostByID(ctx, hostID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "network error")
}

func TestHostClient_GetHostByID_ResponseError(t *testing.T) {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	hostCli := &HostClient{RestClientInterface: cli}
	ctx := context.Background()
	hostID := "123"

	m := gomonkey.ApplyMethodReturn(cli, "Get",
		Response{
			Data:  nil,
			Error: map[string]interface{}{"code": float64(1), "description": "internal error"},
		}, nil)
	defer m.Reset()

	_, err := hostCli.GetHostByID(ctx, hostID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "internal error")
}

func TestHostClient_GetHostByID_ConvertError(t *testing.T) {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	hostCli := &HostClient{RestClientInterface: cli}
	ctx := context.Background()
	hostID := "123"

	m := gomonkey.ApplyMethodReturn(cli, "Get",
		Response{
			Data:  "invalid data",
			Error: map[string]interface{}{"code": float64(0), "description": ""},
		}, nil)
	defer m.Reset()

	_, err := hostCli.GetHostByID(ctx, hostID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "convert host to map failed")
}

func TestSystemClient_GetAllPools_PoolNotMap(t *testing.T) {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	systemCli := &SystemClient{RestClientInterface: cli}
	ctx := context.Background()

	m := gomonkey.ApplyMethodReturn(cli, "Get",
		Response{
			Data:  []interface{}{"not a map", 123, true},
			Error: map[string]interface{}{"code": float64(0), "description": ""},
		}, nil)
	defer m.Reset()

	result, err := systemCli.GetAllPools(ctx)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestSystemClient_GetAllPools_NameNotString(t *testing.T) {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	systemCli := &SystemClient{RestClientInterface: cli}
	ctx := context.Background()

	m := gomonkey.ApplyMethodReturn(cli, "Get",
		Response{
			Data:  []interface{}{map[string]interface{}{"NAME": 123, "ID": "pool-1"}},
			Error: map[string]interface{}{"code": float64(0), "description": ""},
		}, nil)
	defer m.Reset()

	result, err := systemCli.GetAllPools(ctx)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestSystemClient_GetLicenseFeature_FeatureNotMap(t *testing.T) {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	systemCli := &SystemClient{RestClientInterface: cli}
	ctx := context.Background()

	m := gomonkey.ApplyMethodReturn(cli, "Get",
		Response{
			Data:  []interface{}{"not a map", 123, true},
			Error: map[string]interface{}{"code": float64(0), "description": ""},
		}, nil)
	defer m.Reset()

	result, err := systemCli.GetLicenseFeature(ctx)
	require.NoError(t, err)
	require.Empty(t, result)
}
