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
	"testing"

	"github.com/stretchr/testify/assert"
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
			gotUrl, gotErr := generateGetRoCEPortalUrlByIP(tt.tgtPortal)
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
