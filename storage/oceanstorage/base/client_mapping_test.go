/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
)

func getMockMappingClient(statusCode int, body string) *MappingClient {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	cli.Client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}

	return &MappingClient{RestClientInterface: cli}
}

func TestMappingClient_CreateMapping_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	expectedMapping := map[string]interface{}{"ID": "1"}
	successResp := `{
		"data": {
			"ID": "1"
		},
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockMappingClient(200, successResp)

	// action
	mapping, err := mockClient.CreateMapping(ctx, "mapping1")

	// assert
	assert.NoError(t, err)
	assert.Equal(t, expectedMapping, mapping)
}

func TestMappingClient_GetMappingByName_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	expectedMapping := map[string]interface{}{"ID": "1"}
	successResp := `{
		"data": [
			{
				"ID": "1"
			}
		],
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockMappingClient(200, successResp)

	// action
	mapping, err := mockClient.GetMappingByName(ctx, "mapping1")

	// assert
	assert.NoError(t, err)
	assert.Equal(t, expectedMapping, mapping)
}
