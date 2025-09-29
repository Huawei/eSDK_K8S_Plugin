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
	"math"
	"testing"

	"github.com/stretchr/testify/require"

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
