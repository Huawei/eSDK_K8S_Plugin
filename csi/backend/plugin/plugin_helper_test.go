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

// Package plugin provide storage function
package plugin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/host"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func Test_formatBaseClientConfig_UrlsMissing(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}
	wantErr := "urls is not provided in config"

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.Nil(t, got)
	assert.ErrorContains(t, gotErr, wantErr)
}

func Test_formatBaseClientConfig_UrlInvalidType(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"urls":            []interface{}{123},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}
	wantErr := "convert to string failed"

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.Nil(t, got)
	assert.ErrorContains(t, gotErr, wantErr)
}

func Test_formatBaseClientConfig_UserMissing(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}
	wantErr := "user is not provided in config"

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.Nil(t, got)
	assert.ErrorContains(t, gotErr, wantErr)
}

func Test_formatBaseClientConfig_SecretNameMissing(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}
	wantErr := "secretName is not provided in config"

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.Nil(t, got)
	assert.ErrorContains(t, gotErr, wantErr)
}

func Test_formatBaseClientConfig_SecretNamespaceMissing(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"urls":       []interface{}{"test"},
		"user":       "test",
		"secretName": "test",
		"backendID":  "id",
		"storage":    "s3",
		"name":       "test",
	}
	wantErr := "secretNamespace is not provided in config"

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.Nil(t, got)
	assert.ErrorContains(t, gotErr, wantErr)
}

func Test_formatBaseClientConfig_BackendIDMissing(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"storage":         "s3",
		"name":            "test",
	}
	wantErr := "backendID is not provided in config"

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.Nil(t, got)
	assert.ErrorContains(t, gotErr, wantErr)
}

func Test_formatBaseClientConfig_StorageMissing(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"name":            "test",
	}
	wantErr := "storage is not provided in config"

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.Nil(t, got)
	assert.ErrorContains(t, gotErr, wantErr)
}

func Test_formatBaseClientConfig_NameMissing(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
	}
	wantErr := "name is not provided in config"

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.Nil(t, got)
	assert.ErrorContains(t, gotErr, wantErr)
}

func Test_formatBaseClientConfig_Success(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}

	// act
	got, gotErr := formatBaseClientConfig(config)

	// assert
	require.NoError(t, gotErr)
	require.NotNil(t, got)
}

func Test_getVolumeNameFromPVNameOrParameters(t *testing.T) {
	// arrange
	uid := "c2fd3f46-bf17-4a7d-b88e-2e3232bae434"

	type args struct {
		volumePrefix string
		pvName       string
		parameters   map[string]any
	}

	tests := []struct {
		name       string
		args       args
		want       string
		wantErrMsg string
	}{
		{name: "not configure volumeName",
			args: args{volumePrefix: "pvc", pvName: "pvc-" + uid,
				parameters: nil}, want: "pvc-" + uid, wantErrMsg: ""},
		{name: "validate volume name failed",
			args: args{volumePrefix: "pvc", pvName: "pvc-" + uid,
				parameters: map[string]any{"volumeName": "{{.PVCNamespace}}"}}, want: "" + uid,
			wantErrMsg: "{{.PVCNamespace}} or {{." +
				"PVCName}} must be configured in the volumeName parameter at the same time"},
		{name: "metadata key not found",
			args: args{volumePrefix: "pvc", pvName: "pvc-" + uid,
				parameters: map[string]any{"volumeName": "{{.PVCNamespace}}{{.PVCName}}"}}, want: "" + uid,
			wantErrMsg: "not found"},
		{name: "success", args: args{volumePrefix: "pvc", pvName: "pvc-" + uid,
			parameters: map[string]any{"volumeName": "{{.PVCNamespace}}-{{.PVCName}}", constants.PVCNameKey: "test-pvc",
				constants.PVCNamespaceKey: "test-namespace", constants.PVNameKey: "pvc-" + uid}},
			want: "test-namespace-test-pvc-" + strings.Replace(uid, "-", "", -1), wantErrMsg: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			app.GetGlobalConfig().VolumeNamePrefix = tt.args.volumePrefix

			// action
			got, err := getVolumeNameFromPVNameOrParameters(tt.args.pvName, tt.args.parameters)

			// assert
			if tt.wantErrMsg != "" {
				require.ErrorContains(t, err, tt.wantErrMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

}

func Test_ValidateAndNewNfsAutoAuthClient_ProtocolNotNfs(t *testing.T) {
	// arrange
	params := map[string]any{
		"protocol": "dpc",
	}

	// action
	got, gotErr := validateAndNewNfsAutoAuthClient(params)

	// assert
	assert.NoError(t, gotErr)
	assert.Empty(t, got)
}

func Test_validateAndNewNfsAutoAuthClient_nfsAutoAuthClientDisabled(t *testing.T) {
	// arrange
	params := map[string]any{
		"protocol":          constants.ProtocolNfs,
		"nfsAutoAuthClient": false,
	}
	want := &NfsAutoAuthClient{
		Enabled: false,
		CIDRs:   nil,
	}

	// action
	got, gotErr := validateAndNewNfsAutoAuthClient(params)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, want, got)
}

func Test_validateAndNewNfsAutoAuthClient_validCIDRs(t *testing.T) {
	// arrange
	params := map[string]any{
		"protocol":               constants.ProtocolNfs,
		"nfsAutoAuthClient":      true,
		"nfsAutoAuthClientCIDRs": []any{"192.168.1.0/24", "10.0.0.0/8"},
	}
	want := &NfsAutoAuthClient{
		Enabled: true,
		CIDRs:   []string{"192.168.1.0/24", "10.0.0.0/8"},
	}

	// action
	got, gotErr := validateAndNewNfsAutoAuthClient(params)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, want, got)
}

func Test_validateAndNewNfsAutoAuthClient_invalidCIDRFormat(t *testing.T) {
	// arrange
	params := map[string]any{
		"protocol":               constants.ProtocolNfs,
		"nfsAutoAuthClient":      true,
		"nfsAutoAuthClientCIDRs": []any{"192.168.1.0/24", "invalid-cidr"},
	}
	var want *NfsAutoAuthClient = nil

	// action
	got, gotErr := validateAndNewNfsAutoAuthClient(params)

	// assert
	assert.Error(t, gotErr)
	assert.Equal(t, want, got)
}

func Test_getFilteredIPs_success(t *testing.T) {
	// arrange
	ctx := context.Background()
	cidrs := []string{"192.168.130.0/24"}
	params := map[string]any{"HostName": "test-host"}
	expectedIPs := []string{"192.168.1.10"}

	// mock
	patchHost := gomonkey.ApplyFuncReturn(host.GetNodeHostInfosFromSecret,
		&host.NodeHostInfo{HostIPs: []string{"192.168.130.25", "10.0.0.5", "127.0.0.1"}}, nil).
		ApplyFuncReturn(utils.FilterIPsByCIDRs, expectedIPs, nil)
	defer patchHost.Reset()

	// action
	gotIPs, gotErr := getFilteredIPs(ctx, cidrs, params)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, expectedIPs, gotIPs)
}

func Test_getFilteredIPs_hostname_missing(t *testing.T) {
	// arrange
	ctx := context.Background()
	cidrs := []string{"192.168.1.0/24"}
	params := map[string]any{"InvalidKey": "test-host"}

	// action
	gotIPs, gotErr := getFilteredIPs(ctx, cidrs, params)

	// assert
	assert.ErrorContains(t, gotErr, "failed to get hostname from parameters")
	assert.Nil(t, gotIPs)
}

func Test_getFilteredIPs_get_host_info_failure(t *testing.T) {
	// arrange
	ctx := context.Background()
	cidrs := []string{"192.168.1.0/24"}
	params := map[string]any{"HostName": "test-host"}

	// mock
	patchHost := gomonkey.ApplyFuncReturn(host.GetNodeHostInfosFromSecret, nil, fmt.Errorf("connection error"))
	defer patchHost.Reset()

	// action
	gotIPs, gotErr := getFilteredIPs(ctx, cidrs, params)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotIPs)
}

func Test_getFilteredIPs_filter_ips_failure(t *testing.T) {
	// arrange
	ctx := context.Background()
	cidrs := []string{"192.168.1.0/24"}
	params := map[string]any{"HostName": "test-host"}

	// mock
	patchHost := gomonkey.ApplyFuncReturn(host.GetNodeHostInfosFromSecret,
		&host.NodeHostInfo{HostIPs: []string{"192.168.1.10", "10.0.0.5"}}, nil).
		ApplyFuncReturn(utils.FilterIPsByCIDRs, nil, fmt.Errorf("invalid cidr format"))
	defer patchHost.Reset()

	// action
	gotIPs, gotErr := getFilteredIPs(ctx, cidrs, params)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotIPs)
}
