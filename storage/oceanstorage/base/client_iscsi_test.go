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

func getMockIscsiClient(statusCode int, body string) *IscsiClient {
	cli, _ := NewRestClient(context.Background(), &storage.NewClientConfig{})
	cli.Client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}

	return &IscsiClient{RestClientInterface: cli}
}

func TestIscsiClient_GetIscsiInitiatorByID_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	initiator := "iqn.xxxx.com.xxxx:xxxx.xxxx.com"
	successRespBody := `{
		"data": {
			"ID": "iqn.xxxx.com.xxxx:xxxx.xxxx.com",
			"name": "test-initiator"
		},
		"error": {
			"code": 0,
			"description": "Success"
		}
	}`

	// mock
	mockClient := getMockIscsiClient(200, successRespBody)

	// action
	gotData, err := mockClient.GetIscsiInitiatorByID(ctx, initiator)

	// assert
	assert.Nil(t, err)
	assert.NotNil(t, gotData)
	assert.Equal(t, "iqn.xxxx.com.xxxx:xxxx.xxxx.com", gotData["ID"])
}
