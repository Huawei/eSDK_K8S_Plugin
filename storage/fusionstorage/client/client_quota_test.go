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
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
)

func TestRestClient_QueryQuotasByFsId_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	testRespBody := []byte(`{
    "data": [
        {
            "id": "270@5",
            "space_hard_quota": 1048576,
            "space_soft_quota": 18446744073709551615,
            "space_unit_type": 0
        }
    ],
    "result": {
        "code": 0,
        "description": ""
    }
}`)
	var (
		expectedId               = "270@5"
		expectedHardQuota uint64 = 1048576
		expectedSoftQuota uint64 = 18446744073709551615
	)

	// mock
	p := gomonkey.NewPatches().ApplyPrivateMethod(testClient, "doCall",
		func(_ *RestClient, _ context.Context, _, _ string, _ any) (http.Header, []byte, error) {
			return nil, testRespBody, nil
		})
	defer p.Reset()

	// action
	resp, err := testClient.QueryQuotaByFsId(ctx, "")

	// assert
	require.NoError(t, err)
	require.Equal(t, expectedId, resp.Id)
	require.Equal(t, expectedHardQuota, resp.SpaceHardQuota)
	require.Equal(t, expectedSoftQuota, resp.SpaceSoftQuota)
}

func TestRestClient_GetQuotaByDTreeId(t *testing.T) {
	// arrange
	successRespBody := `{"data":[{"id":"83@8193@3","parent_id":"83@8193","space_hard_quota":1024,
"space_soft_quota":18446744073709551615,"space_unit_type":0}],"result":{"code":0,"description":""}}`
	notFoundRespBody := `{"data":[],"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":[],"result":{"code":12345,"description":"test description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		dtreeId      string
		wantErr      bool
		wantResp     *DTreeQuotaResponse
	}{
		{name: "success", responseBody: successRespBody, dtreeId: "83@8193",
			wantErr: false, wantResp: &DTreeQuotaResponse{
				Id:             "83@8193@3",
				ParentId:       "83@8193",
				SpaceHardQuota: 1024,
				SpaceSoftQuota: 18446744073709551615,
				SpaceUnitType:  0,
			}},
		{name: "not found", responseBody: notFoundRespBody, dtreeId: "test-dtree-id", wantErr: false},
		{name: "failure with error code", responseBody: failureWithRespBody, dtreeId: "test-dtree-id", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			resp, err := mockClient.GetQuotaByDTreeId(ctx, tt.dtreeId)

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

func TestRestClient_CreateDTreeQuota(t *testing.T) {
	// arrange
	successRespBody := `{"data":{},"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":{},"result":{"code":12345,"description":"test description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		parentId     string
		capacity     int64
		wantErr      bool
	}{
		{name: "success", responseBody: successRespBody, parentId: "83@8193", wantErr: false},
		{name: "failure with error code", responseBody: failureWithRespBody, parentId: "test-dtree-id", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			resp, err := mockClient.CreateDTreeQuota(ctx, tt.parentId, tt.capacity)

			// assert
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}

func TestRestClient_DeleteDTreeQuota(t *testing.T) {
	// arrange
	successRespBody := `{"data":{},"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":{},"result":{"code":12345,"description":"test description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		quotaId      string
		wantErr      bool
	}{
		{name: "success", responseBody: successRespBody, quotaId: "83@8193@1", wantErr: false},
		{name: "failure with error code", responseBody: failureWithRespBody, quotaId: "83@8193@1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			err := mockClient.DeleteDTreeQuota(ctx, tt.quotaId)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRestClient_UpdateDTreeQuota(t *testing.T) {
	// arrange
	successRespBody := `{"data":{},"result":{"code":0,"description":""}}`
	failureWithRespBody := `{"data":{},"result":{"code":12345,"description":"test description"}}`
	ctx := context.Background()
	tests := []struct {
		name         string
		responseBody string
		quotaId      string
		capacity     int64
		wantErr      bool
	}{
		{name: "success", responseBody: successRespBody, quotaId: "83@8193@1", capacity: 1024, wantErr: false},
		{name: "failure with error code", responseBody: failureWithRespBody, quotaId: "83@8193@1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			mockClient := getMockClient(200, tt.responseBody)

			// action
			err := mockClient.UpdateDTreeQuota(ctx, tt.quotaId, tt.capacity)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
