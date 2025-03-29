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
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type MockTransport struct {
	Response *http.Response
	Err      error
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

func getMockClient(statusCode int, body string) *RestClient {
	testClient.client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}
	return testClient
}

func TestRestClient_GetDTreeByName(t *testing.T) {
	// arrange
	successRespBody := `{"data":[{"id":"83@8193","name":"test-csi-dtree"},{"id":"83@4098","name":"test-dtree-name"},
{"id":"83@12291","name":"test-csi-dtree-csi"}],"result":{"code":0,"description":""}}`
	notFoundRespBody := `{"data":[],"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":[],"result":{"code":12345,"description":"test description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		parentName   string
		dtreeName    string
		wantErr      bool
		wantResp     *DTreeResponse
	}{
		{name: "success", responseBody: successRespBody, parentName: "test-parentname", dtreeName: "test-dtree-name",
			wantErr: false, wantResp: &DTreeResponse{Id: "83@4098", Name: "test-dtree-name"}},
		{name: "not found", responseBody: notFoundRespBody,
			parentName: "test-parent-name", dtreeName: "not-found-dtree", wantErr: false, wantResp: nil},
		{name: "failure with error code", responseBody: failureWithRespBody, parentName: "test-parent-name",
			dtreeName: "test-dtree-name", wantErr: true, wantResp: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			resp, err := mockClient.GetDTreeByName(ctx, tt.parentName, tt.dtreeName)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, resp, tt.wantResp)
		})
	}
}

func TestRestClient_CreateDTree(t *testing.T) {
	// arrange
	successRespBody := `{"data":{"id":"83@4098","name":"test-dtree-name"},"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":[],"result":{"code":12345,"description":"test description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		parentName   string
		dtreeName    string
		wantErr      bool
		wantResp     *DTreeResponse
	}{
		{name: "success", responseBody: successRespBody, parentName: "test-parentname", dtreeName: "test-dtree-name",
			wantErr: false, wantResp: &DTreeResponse{Id: "83@4098", Name: "test-dtree-name"}},
		{name: "failure with error code", responseBody: failureWithRespBody, parentName: "test-parent-name",
			dtreeName: "test-dtree-name", wantErr: true, wantResp: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			resp, err := mockClient.CreateDTree(ctx, tt.parentName, tt.dtreeName, "777")

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, resp, tt.wantResp)
		})
	}
}

func TestRestClient_DeleteDTree(t *testing.T) {
	// arrange
	successRespBody := `{"data":{},"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":{},"result":{"code":12345,"description":"error description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		dtreeId      string
		wantErr      bool
	}{
		{name: "success", responseBody: successRespBody, dtreeId: "test-dtree-id", wantErr: false},
		{name: "failure with error code", responseBody: failureWithRespBody, dtreeId: "test-dtree-name", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			err := mockClient.DeleteDTree(ctx, tt.dtreeId)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRestClient_GetDTreeNfsShareByPath(t *testing.T) {
	// arrange
	successRespBody := `{"data":[{"id":"104","share_path":"/parent/dtree"}],"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":[],"result":{"code":12345,"description":"error description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		sharePath    string
		wantErr      bool
		wantResp     *GetDTreeNfsShareResponse
	}{
		{name: "success", responseBody: successRespBody, sharePath: "/parent/dtree", wantErr: false,
			wantResp: &GetDTreeNfsShareResponse{Id: "104", SharePath: "/parent/dtree"}},
		{name: "failure with error code", responseBody: failureWithRespBody, sharePath: "/parent/dtree", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			resp, err := mockClient.GetDTreeNfsShareByPath(ctx, tt.sharePath)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, resp, tt.wantResp)
		})
	}
}

func TestRestClient_CreateDTreeNfsShare(t *testing.T) {
	// arrange
	successRespBody := `{"data":{"id":"104","share_path":"/parent/dtree"},"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":{},"result":{"code":12345,"description":"error description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		wantErr      bool
		wantResp     *CreateDTreeNfsShareResponse
	}{
		{name: "success",
			responseBody: successRespBody, wantErr: false, wantResp: &CreateDTreeNfsShareResponse{Id: "104"}},
		{name: "failure with error code",
			responseBody: failureWithRespBody, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			resp, err := mockClient.CreateDTreeNfsShare(ctx, &CreateDTreeNfsShareRequest{})

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, resp, tt.wantResp)
		})
	}
}

func TestRestClient_DeleteDTreeNfsShare(t *testing.T) {
	// arrange
	successRespBody := `{"data":{},"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":{},"result":{"code":12345,"description":"error description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		shareId      string
		wantErr      bool
	}{
		{name: "success", responseBody: successRespBody, shareId: "test-share-id", wantErr: false},
		{name: "failure with error code", responseBody: failureWithRespBody, shareId: "test-share-id", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			err := mockClient.DeleteDTreeNfsShare(ctx, tt.shareId)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRestClient_AddNfsShareAuthClient(t *testing.T) {
	// arrange
	successRespBody := `{"data":{},"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":{},"result":{"code":12345,"description":"error description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		wantErr      bool
	}{
		{name: "success", responseBody: successRespBody, wantErr: false},
		{name: "failure with error code", responseBody: failureWithRespBody, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			err := mockClient.AddNfsShareAuthClient(ctx, &AddNfsShareAuthClientRequest{})

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
