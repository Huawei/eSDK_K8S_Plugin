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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

func TestOceanstorClient_CreateLun_Success(t *testing.T) {
	// arrange
	params := map[string]any{
		"name":           "test-lun-name",
		"parentid":       "1",
		"capacity":       int64(1024 * 1024 * 1024),
		"description":    "test-desc",
		"alloctype":      1,
		"workloadTypeID": "1",
	}
	successRespBody := `{ "data": { "ID": "1", "WWN": "test-wwn" }, "error": { "code": 0, "description": "0" }}`

	// mock
	mockClient := getMockClient(200, successRespBody)

	// action
	lun, err := mockClient.CreateLun(context.Background(), params)

	// assert
	require.NoError(t, err)
	require.Contains(t, lun, "ID")
	require.Contains(t, lun, "WWN")
}

func TestOceanstorClient_CreateLun_UnmarshalAdvancedOptionsFailed(t *testing.T) {
	// arrange
	params := map[string]any{
		constants.AdvancedOptionsKey: "{",
	}

	// mock
	mockClient := getMockClient(200, "{}")

	// action
	_, err := mockClient.CreateLun(context.Background(), params)

	// assert
	require.ErrorContains(t, err, "failed to unmarshal advancedOptions")
}

func Test_generateCreateLunDataFromParams(t *testing.T) {
	// arrange
	tests := []struct {
		name            string
		params          map[string]any
		want            map[string]any
		wantErrContains string
	}{
		{name: "full parameters",
			params: map[string]any{"name": "test-lun-name", "parentid": "1", "capacity": int64(1024 * 1024 * 1024),
				"description": description, "alloctype": 1, "workloadTypeID": "1",
				constants.AdvancedOptionsKey: `{"key": "value"}`},
			want: map[string]any{"NAME": "test-lun-name", "PARENTID": "1", "CAPACITY": int64(1024 * 1024 * 1024),
				"DESCRIPTION": description, "ALLOCTYPE": 1, "WORKLOADTYPEID": "1", "key": "value"},
			wantErrContains: ""},
		{name: "description override",
			params: map[string]any{"name": "test-lun-name", "parentid": "1", "capacity": int64(1024 * 1024 * 1024),
				"description": "desc from sc", "alloctype": 1, "workloadTypeID": "1",
				constants.AdvancedOptionsKey: `{"DESCRIPTION": "desc from advanced options"}`},
			want: map[string]any{"NAME": "test-lun-name", "PARENTID": "1", "CAPACITY": int64(1024 * 1024 * 1024),
				"DESCRIPTION": "desc from advanced options", "ALLOCTYPE": 1, "WORKLOADTYPEID": "1"},
			wantErrContains: ""},
		{name: "unmarshal failed", params: map[string]any{constants.AdvancedOptionsKey: `{`}, want: nil,
			wantErrContains: "failed to unmarshal advancedOptions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			got, err := generateCreateLunDataFromParams(tt.params)

			// assert
			if tt.wantErrContains != "" {
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}
