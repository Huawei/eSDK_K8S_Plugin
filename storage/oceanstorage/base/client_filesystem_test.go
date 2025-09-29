/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.
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

	"github.com/stretchr/testify/require"
)

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

type MockTransport struct {
	Response *http.Response
	Err      error
}

func getMockClient(statusCode int, body string) *FilesystemClient {
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	cli.Client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}

	return &FilesystemClient{RestClientInterface: cli}
}

func TestAllowNfsShareAccess_BasicSuccess(t *testing.T) {
	// Arrange
	ctx := context.Background()
	req := &AllowNfsShareAccessRequest{
		Name:        "test-share",
		ParentID:    "parent-001",
		AccessVal:   1,
		Sync:        1,
		AllSquash:   1,
		RootSquash:  1,
		VStoreID:    "vs-001",
		AccessKrb5:  1,
		AccessKrb5i: 1,
		AccessKrb5p: 1,
	}
	successResp := `{"error": {"code": 0}}`

	// Mock
	mockClient := getMockClient(200, successResp)

	// Action
	err := mockClient.AllowNfsShareAccess(ctx, req)

	// Assert
	require.NoError(t, err)
}

func TestAllowNfsShareAccess_OptionalParams(t *testing.T) {
	// Arrange
	ctx := context.Background()
	req := &AllowNfsShareAccessRequest{
		Name:       "minimal-share",
		ParentID:   "parent-002",
		AccessVal:  0,
		Sync:       0,
		AllSquash:  1,
		RootSquash: 0,
	}
	successResp := `{"error": {"code": 0}}`

	// Mock
	mockClient := getMockClient(200, successResp)

	// Action
	err := mockClient.AllowNfsShareAccess(ctx, req)

	// Assert
	require.NoError(t, err)
}

func TestAllowNfsShareAccess_APIFailure(t *testing.T) {
	// Arrange
	ctx := context.Background()
	req := &AllowNfsShareAccessRequest{
		Name:      "valid-share",
		ParentID:  "parent-003",
		AccessVal: 0,
		Sync:      1,
		AllSquash: 1,
	}
	errorResp := `{"error": {"code": 1077939726}}`

	// Mock
	mockClient := getMockClient(200, errorResp)

	// Action
	err := mockClient.AllowNfsShareAccess(ctx, req)

	// Assert
	require.ErrorContains(t, err, "allow nfs share")
	require.Contains(t, err.Error(), "1077939726")
}
