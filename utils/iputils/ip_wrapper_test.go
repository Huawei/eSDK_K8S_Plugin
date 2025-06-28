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

package iputils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetFormatPortalIP(t *testing.T) {
	// arrange
	ipInfos := []struct {
		name         string
		srcIP        string
		wantFormatIP string
	}{
		{
			name:         "IPv4 test",
			srcIP:        "127.0.0.1",
			wantFormatIP: "127.0.0.1",
		},
		{
			name:         "IPv6 test",
			srcIP:        "127::1",
			wantFormatIP: "[127::1]",
		},
	}

	for _, tt := range ipInfos {
		t.Run(tt.name, func(t *testing.T) {
			// action
			ipWrapper := NewIPWrapper(tt.srcIP)
			assert.NotEqual(t, ipWrapper, nil)
			gotFormatIP := ipWrapper.GetFormatPortalIP()
			// assert
			assert.Equal(t, tt.wantFormatIP, gotFormatIP)
		})
	}
}

func Test_GetPingCommand(t *testing.T) {
	// arrange
	ipInfos := []struct {
		name        string
		srcIP       string
		wantPingCmd string
	}{
		{
			name:        "IPv4 test",
			srcIP:       "127.0.0.1",
			wantPingCmd: pingIPv4Command,
		},
		{
			name:        "IPv6 test",
			srcIP:       "127::1",
			wantPingCmd: pingIPv6Command,
		},
	}

	for _, tt := range ipInfos {
		t.Run(tt.name, func(t *testing.T) {
			// action
			ipWrapper := NewIPWrapper(tt.srcIP)
			assert.NotEqual(t, ipWrapper, nil)
			gotPingCmd := ipWrapper.GetPingCommand()
			// assert
			assert.Equal(t, tt.wantPingCmd, gotPingCmd)
		})
	}
}
