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

package attacher

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
)

func TestGetHostName_DefaultPrefix(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "k8s_"
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	am := &AttachmentManager{Invoker: "csi"}

	tests := []struct {
		name     string
		postfix  string
		expected string
	}{
		{
			name:     "short hostname with default prefix",
			postfix:  "node1",
			expected: "k8s_node1",
		},
		{
			name:     "hostname exceeds maxHostNameLengthForV5 is NOT truncated by getHostName",
			postfix:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",     // 30 chars, k8s_ (4) + 30 = 34 > 31
			expected: "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // not truncated, full name returned
		},
		{
			name:     "empty postfix",
			postfix:  "",
			expected: "k8s_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := am.getHostName(tt.postfix)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetHostName_NotConfiguredFallback(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	// Simulate --host-name-prefix not set: HostPrefix="" and HostPrefixSet=false
	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = ""
	mockCfg.HostNamePrefixSet = false
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	am := &AttachmentManager{Invoker: "csi"}

	got := am.getHostName("node1")
	assert.Equal(t, "k8s_node1", got) // fallback to k8s_
}

func TestGetHostName_ExplicitEmptyPrefix(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	// Simulate --host-name-prefix="" explicitly set: HostPrefix="" and HostPrefixSet=true
	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = ""
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	am := &AttachmentManager{Invoker: "csi"}

	got := am.getHostName("node1")
	assert.Equal(t, "node1", got) // no prefix when explicitly set to empty
}

func TestGetHostName_CustomPrefix(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "prod_"
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	am := &AttachmentManager{Invoker: "csi"}

	tests := []struct {
		name     string
		postfix  string
		expected string
	}{
		{
			name:     "short hostname with custom prefix",
			postfix:  "node1",
			expected: "prod_node1",
		},
		{
			name:     "no truncation with custom prefix (prod_ is 5 chars, getHostName returns full name)",
			postfix:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",      // 30 chars
			expected: "prod_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 5 + 30 = 35, not truncated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := am.getHostName(tt.postfix)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetHostGroupName_DefaultPrefix(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "k8s_"
	mockCfg.HostNamePrefixSet = false
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	am := &AttachmentManager{Invoker: "csi"}

	got := am.getHostGroupName("host123")
	assert.Equal(t, "k8s_csi_hostgroup_host123", got)
}

func TestGetMappingName_DefaultPrefix(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "k8s_"
	mockCfg.HostNamePrefixSet = false
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	am := &AttachmentManager{Invoker: "csi"}

	got := am.getMappingName("host123")
	assert.Equal(t, "k8s_csi_mapping_host123", got)
}

func TestHostNameTruncationCalculation(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	// getHostName no longer truncates — it returns the full name.
	// Truncation is handled in GetHost when needed.
	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = "ab_"
	mockCfg.HostNamePrefixSet = true
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	am := &AttachmentManager{Invoker: "csi"}

	// postfix of 28 chars — full name is 31 chars (ab_ + 28)
	longPostfix := "aaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 28 chars
	got := am.getHostName(longPostfix)
	assert.Equal(t, "ab_"+longPostfix, got)
	assert.Len(t, got, 31)

	// postfix of 29 chars — full name is 32 chars, NOT truncated by getHostName
	longerPostfix := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 29 chars
	got = am.getHostName(longerPostfix)
	assert.Equal(t, "ab_"+longerPostfix, got)
	assert.Len(t, got, 32) // no truncation
}

func TestGetHostName_EmptyPrefixFallback(t *testing.T) {
	origGetGlobalConfig := app.GetGlobalConfig
	defer func() { app.GetGlobalConfig = origGetGlobalConfig }()

	// When HostPrefix is not configured (HostPrefixSet=false), OceanStor should fallback to "k8s_"
	mockCfg := cfg.MockCompletedConfig()
	mockCfg.HostNamePrefix = ""
	mockCfg.HostNamePrefixSet = false
	app.GetGlobalConfig = func() *cfg.CompletedConfig {
		return mockCfg
	}

	am := &AttachmentManager{Invoker: "csi"}

	tests := []struct {
		name     string
		postfix  string
		expected string
	}{
		{
			name:     "empty prefix falls back to k8s_",
			postfix:  "node1",
			expected: "k8s_node1",
		},
		{
			name:     "empty prefix fallback with long hostname",
			postfix:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",     // 30 chars
			expected: "k8s_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // not truncated by getHostName
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := am.getHostName(tt.postfix)
			assert.Equal(t, tt.expected, got)
		})
	}
}
