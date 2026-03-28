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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
)

func Test_generateGetRoCEPortalUrlByIP(t *testing.T) {
	// arrange
	portals := []struct {
		name            string
		tgtPortal       string
		wantUrl         string
		wantErrContains string
	}{
		{
			name:            "IPv4 test",
			tgtPortal:       "127.0.0.1",
			wantUrl:         "/lif?filter=IPV4ADDR::127.0.0.1",
			wantErrContains: "",
		},
		{
			name:            "IPv6 test",
			tgtPortal:       "127:0:0::1",
			wantUrl:         "/lif?filter=IPV6ADDR::127\\:0\\:0\\:\\:1",
			wantErrContains: "",
		},
		{
			name:            "invalid IPv4 test",
			tgtPortal:       "",
			wantUrl:         "",
			wantErrContains: "invalid",
		},
	}

	for _, tt := range portals {
		t.Run(tt.name, func(t *testing.T) {
			// action
			gotUrl, gotErr := generateGetPortalUrlByIP(tt.tgtPortal)
			// assert
			if tt.wantErrContains == "" {
				assert.NoError(t, gotErr)
				assert.Equal(t, tt.wantUrl, gotUrl)
			} else {
				assert.ErrorContains(t, gotErr, tt.wantErrContains)
			}
		})
	}
}

func getMockRoCEClient(statusCode int, body string) *RoCEClient {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	cli.Client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}

	return &RoCEClient{RestClientInterface: cli}
}

func TestRoCEClient_GetInitiatorByID_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	successRespBody := `{
		"data": {
			"ID": "testnqn:uuid:001",
			"name": "test-initiator"
		},
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockRoCEClient(200, successRespBody)

	// action
	gotData, err := mockClient.GetInitiatorByID(ctx, initiator)

	// assert
	assert.Nil(t, err)
	assert.NotNil(t, gotData)
	assert.Equal(t, "testnqn:uuid:001", gotData["ID"])
}

func TestRoCEClient_GetInitiatorByID_ApiError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	errorRespBody := `{
		"data": {},
		"error": {
			"code": 1073741824,
			"description": "Internal error"
		}
	}`
	wantErr := fmt.Errorf("get nvme initiator %s failed, error code: %d, error msg: %s", initiator,
		int64(1073741824), "Internal error")

	// mock
	mockClient := getMockRoCEClient(200, errorRespBody)

	// action
	gotData, gotErr := mockClient.GetInitiatorByID(ctx, initiator)

	// assert
	assert.Nil(t, gotData)
	assert.Equal(t, wantErr, gotErr)
}

func TestRoCEClient_GetInitiatorByID_DataConversionError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	invalidDataRespBody := `{
		"data": ["invalid", "data", "structure"],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`
	wantErr := fmt.Errorf("convert initiator data to map failed, data: %v",
		[]interface{}{"invalid", "data", "structure"})

	// mock
	mockClient := getMockRoCEClient(200, invalidDataRespBody)

	// action
	gotData, gotErr := mockClient.GetInitiatorByID(ctx, initiator)

	// assert
	assert.Nil(t, gotData)
	assert.Equal(t, wantErr, gotErr)
}

func TestRoCEClient_AddInitiator_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	successRespBody := `{
		"data": {
			"ID": "testnqn:uuid:001",
			"name": "test-initiator"
		},
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockRoCEClient(200, successRespBody)

	// action
	gotData, err := mockClient.AddInitiator(ctx, initiator)

	// assert
	assert.Nil(t, err)
	assert.NotNil(t, gotData)
	assert.Equal(t, "testnqn:uuid:001", gotData["ID"])
}

func TestRoCEClient_AddInitiator_ApiError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	errorRespBody := `{
		"data": {},
		"error": {
			"code": 1073741824,
			"description": "Internal error"
		}
	}`
	wantErr := fmt.Errorf("add nvme initiator %s failed, error code: %d, error msg: %s", initiator,
		int64(1073741824), "Internal error")

	// mock
	mockClient := getMockRoCEClient(200, errorRespBody)

	// action
	gotData, gotErr := mockClient.AddInitiator(ctx, initiator)

	// assert
	assert.Nil(t, gotData)
	assert.Equal(t, wantErr, gotErr)
}

func TestRoCEClient_AddInitiator_DataConversionError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	invalidDataRespBody := `{
		"data": ["invalid", "data", "structure"],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`
	wantErr := fmt.Errorf("convert initiator data to map failed, data: %v",
		[]interface{}{"invalid", "data", "structure"})

	// mock
	mockClient := getMockRoCEClient(200, invalidDataRespBody)

	// action
	gotData, gotErr := mockClient.AddInitiator(ctx, initiator)

	// assert
	assert.Nil(t, gotData)
	assert.Equal(t, wantErr, gotErr)
}

func TestRoCEClient_AddInitiatorToHost_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	hostID := "host-123"
	successRespBody := `{
		"data": {},
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockRoCEClient(200, successRespBody)

	// action
	err := mockClient.AddInitiatorToHost(ctx, initiator, hostID)

	// assert
	assert.Nil(t, err)
}

func TestRoCEClient_AddInitiatorToHost_PutError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	hostID := "host-123"
	wantErr := fmt.Errorf("request failed")
	client := &RoCEClient{&RestClient{}}

	// mock
	patches := gomonkey.ApplyMethodReturn(&RestClient{}, "Put", nil, wantErr)
	defer patches.Reset()

	// action
	gotErr := client.AddInitiatorToHost(ctx, initiator, hostID)

	// assert
	assert.Equal(t, wantErr, gotErr)
}

func TestRoCEClient_AddInitiatorToHost_ApiError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	hostID := "host-123"
	errorRespBody := `{
		"data": {},
		"error": {
			"code": 1073741824,
			"description": "Internal error"
		}
	}`
	wantErr := fmt.Errorf("add roce-nvme initiator %s to host failed, error code: %d, error msg: %s", initiator,
		int64(1073741824), "Internal error")

	// mock
	mockClient := getMockRoCEClient(200, errorRespBody)

	// action
	gotErr := mockClient.AddInitiatorToHost(ctx, initiator, hostID)

	// assert
	assert.Equal(t, wantErr, gotErr)
}

func TestRoCEClient_GetPortalByIP_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	tgtPortal := "127.0.0.1"
	expectedPortal := map[string]interface{}{
		"ID":   "portal-001",
		"IPV4": "127.0.0.1",
	}
	respBody := `{
		"data": [
			{
				"ID": "portal-001",
				"IPV4": "127.0.0.1"
			}
		],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockRoCEClient(200, respBody)

	// action
	gotPortal, gotErr := mockClient.GetPortalByIP(ctx, tgtPortal)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, expectedPortal, gotPortal)
}

func TestRoCEClient_GetPortalByIP_GetError(t *testing.T) {
	// arrange
	ctx := context.Background()
	tgtPortal := "127.0.0.1"
	wantErr := fmt.Errorf("request failed")
	client := &RoCEClient{&RestClient{}}

	// mock
	patches := gomonkey.ApplyMethodReturn(&RestClient{}, "Get", nil, wantErr)
	defer patches.Reset()

	// action
	gotPortal, gotErr := client.GetPortalByIP(ctx, tgtPortal)

	// assert
	assert.Nil(t, gotPortal)
	assert.Equal(t, wantErr, gotErr)
}

func TestRoCEClient_GetPortalByIP_ApiError(t *testing.T) {
	// arrange
	ctx := context.Background()
	tgtPortal := "127.0.0.1"
	respBody := `{
		"data": [],
		"error": {
			"code": 1073741824,
			"description": "Internal error"
		}
	}`
	wantErr := fmt.Errorf("get logical ports failed, error code: %d, error msg: %s", int64(1073741824),
		"Internal error")

	// mock
	mockClient := getMockRoCEClient(200, respBody)

	// action
	gotPortal, gotErr := mockClient.GetPortalByIP(ctx, tgtPortal)

	// assert
	assert.Nil(t, gotPortal)
	assert.Equal(t, wantErr, gotErr)
}

func TestRoCEClient_GetPortalByIP_EmptyDataArray(t *testing.T) {
	// arrange
	ctx := context.Background()
	tgtPortal := "127.0.0.1"
	respBody := `{
		"data": [],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockRoCEClient(200, respBody)

	// action
	gotPortal, gotErr := mockClient.GetPortalByIP(ctx, tgtPortal)

	// assert
	assert.Nil(t, gotErr)
	assert.Nil(t, gotPortal)
}

func getMockTcpClient(statusCode int, body string) *TcpClient {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	cli.Client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}

	return &TcpClient{RestClientInterface: cli}
}

func TestTcpClient_GetInitiatorByID_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	successRespBody := `{
		"data": {
			"ID": "testnqn:uuid:001",
			"name": "test-initiator"
		},
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockTcpClient(200, successRespBody)

	// action
	gotData, err := mockClient.GetInitiatorByID(ctx, initiator)

	// assert
	assert.Nil(t, err)
	assert.NotNil(t, gotData)
	assert.Equal(t, "testnqn:uuid:001", gotData["ID"])
}

func TestTcpClient_GetInitiatorByID_ApiError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	errorRespBody := `{
		"data": {},
		"error": {
			"code": 1073741824,
			"description": "Internal error"
		}
	}`
	wantErr := fmt.Errorf("get nvme initiator %s failed, error code: %d, error msg: %s", initiator,
		int64(1073741824), "Internal error")

	// mock
	mockClient := getMockTcpClient(200, errorRespBody)

	// action
	gotData, gotErr := mockClient.GetInitiatorByID(ctx, initiator)

	// assert
	assert.Nil(t, gotData)
	assert.Equal(t, wantErr, gotErr)
}

func TestTcpClient_GetInitiatorByID_DataConversionError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	invalidDataRespBody := `{
		"data": ["invalid", "data", "structure"],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`
	wantErr := fmt.Errorf("convert initiator data to map failed, data: %v",
		[]interface{}{"invalid", "data", "structure"})

	// mock
	mockClient := getMockTcpClient(200, invalidDataRespBody)

	// action
	gotData, gotErr := mockClient.GetInitiatorByID(ctx, initiator)

	// assert
	assert.Nil(t, gotData)
	assert.Equal(t, wantErr, gotErr)
}

func TestTcpClient_AddInitiator_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	successRespBody := `{
		"data": {
			"ID": "testnqn:uuid:001",
			"name": "test-initiator"
		},
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockTcpClient(200, successRespBody)

	// action
	gotData, err := mockClient.AddInitiator(ctx, initiator)

	// assert
	assert.Nil(t, err)
	assert.NotNil(t, gotData)
	assert.Equal(t, "testnqn:uuid:001", gotData["ID"])
}

func TestTcpClient_AddInitiator_ApiError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	errorRespBody := `{
		"data": {},
		"error": {
			"code": 1073741824,
			"description": "Internal error"
		}
	}`
	wantErr := fmt.Errorf("add nvme initiator %s failed, error code: %d, error msg: %s", initiator,
		int64(1073741824), "Internal error")

	// mock
	mockClient := getMockTcpClient(200, errorRespBody)

	// action
	gotData, gotErr := mockClient.AddInitiator(ctx, initiator)

	// assert
	assert.Nil(t, gotData)
	assert.Equal(t, wantErr, gotErr)
}

func TestTcpClient_AddInitiator_DataConversionError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	invalidDataRespBody := `{
		"data": ["invalid", "data", "structure"],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`
	wantErr := fmt.Errorf("convert initiator data to map failed, data: %v",
		[]interface{}{"invalid", "data", "structure"})

	// mock
	mockClient := getMockTcpClient(200, invalidDataRespBody)

	// action
	gotData, gotErr := mockClient.AddInitiator(ctx, initiator)

	// assert
	assert.Nil(t, gotData)
	assert.Equal(t, wantErr, gotErr)
}

func TestTcpClient_AddInitiatorToHost_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	hostID := "host-123"
	successRespBody := `{
		"data": {},
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockTcpClient(200, successRespBody)

	// action
	err := mockClient.AddInitiatorToHost(ctx, initiator, hostID)

	// assert
	assert.Nil(t, err)
}

func TestTcpClient_AddInitiatorToHost_PutError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	hostID := "host-123"
	wantErr := fmt.Errorf("request failed")
	client := &TcpClient{&RestClient{}}

	// mock
	patches := gomonkey.ApplyMethodReturn(&RestClient{}, "Put", nil, wantErr)
	defer patches.Reset()

	// action
	gotErr := client.AddInitiatorToHost(ctx, initiator, hostID)

	// assert
	assert.Equal(t, wantErr, gotErr)
}

func TestTcpClient_AddInitiatorToHost_ApiError(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "testnqn:uuid:001"
	hostID := "host-123"
	errorRespBody := `{
		"data": {},
		"error": {
			"code": 1073741824,
			"description": "Internal error"
		}
	}`
	wantErr := fmt.Errorf("add tcp-nvme initiator %s to host failed, error code: %d, error msg: %s", initiator,
		int64(1073741824), "Internal error")

	// mock
	mockClient := getMockTcpClient(200, errorRespBody)

	// action
	gotErr := mockClient.AddInitiatorToHost(ctx, initiator, hostID)

	// assert
	assert.Equal(t, wantErr, gotErr)
}

func TestTcpClient_GetPortalByIP_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	tgtPortal := "127.0.0.1"
	expectedPortal := map[string]interface{}{
		"ID":   "portal-001",
		"IPV4": "127.0.0.1",
	}
	respBody := `{
		"data": [
			{
				"ID": "portal-001",
				"IPV4": "127.0.0.1"
			}
		],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockTcpClient(200, respBody)

	// action
	gotPortal, gotErr := mockClient.GetPortalByIP(ctx, tgtPortal)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, expectedPortal, gotPortal)
}

func TestTcpClient_GetPortalByIP_GetError(t *testing.T) {
	// arrange
	ctx := context.Background()
	tgtPortal := "127.0.0.1"
	wantErr := fmt.Errorf("request failed")
	client := &TcpClient{&RestClient{}}

	// mock
	patches := gomonkey.ApplyMethodReturn(&RestClient{}, "Get", nil, wantErr)
	defer patches.Reset()

	// action
	gotPortal, gotErr := client.GetPortalByIP(ctx, tgtPortal)

	// assert
	assert.Nil(t, gotPortal)
	assert.Equal(t, wantErr, gotErr)
}

func TestTcpClient_GetPortalByIP_ApiError(t *testing.T) {
	// arrange
	ctx := context.Background()
	tgtPortal := "127.0.0.1"
	respBody := `{
		"data": [],
		"error": {
			"code": 1073741824,
			"description": "Internal error"
		}
	}`
	wantErr := fmt.Errorf("get logical ports failed, error code: %d, error msg: %s", int64(1073741824),
		"Internal error")

	// mock
	mockClient := getMockTcpClient(200, respBody)

	// action
	gotPortal, gotErr := mockClient.GetPortalByIP(ctx, tgtPortal)

	// assert
	assert.Nil(t, gotPortal)
	assert.Equal(t, wantErr, gotErr)
}

func TestTcpClient_GetPortalByIP_EmptyDataArray(t *testing.T) {
	// arrange
	ctx := context.Background()
	tgtPortal := "127.0.0.1"
	respBody := `{
		"data": [],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockTcpClient(200, respBody)

	// action
	gotPortal, gotErr := mockClient.GetPortalByIP(ctx, tgtPortal)

	// assert
	assert.Nil(t, gotErr)
	assert.Nil(t, gotPortal)
}
