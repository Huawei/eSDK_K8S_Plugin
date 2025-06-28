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

package plugin

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

func Test_validateVolumeName(t *testing.T) {
	// arrange
	tests := []struct {
		name          string
		volumeNameTpl string
		wantErr       bool
	}{
		{name: "correct", volumeNameTpl: "{{.PVCNamespace}}{{.PVCName}}", wantErr: false},
		{name: "correct with space and custom content", volumeNameTpl: "prefix-{{ .PVCNamespace   }}.{{ .PVCName}}",
			wantErr: false},
		{name: "without PVCName", volumeNameTpl: "{{.PVCNamespace}}", wantErr: true},
		{name: "without PVCNamespace", volumeNameTpl: "{{.PVCName}}", wantErr: true},
		{name: "empty", volumeNameTpl: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			err := validateVolumeName(tt.volumeNameTpl)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_newExtraCreateMetadataFromParameters(t *testing.T) {
	// arrange
	tests := []struct {
		name         string
		parameters   map[string]any
		volumePrefix string
		want         map[string]string
		wantErr      bool
	}{
		{name: "default volume prefix", volumePrefix: "pvc",
			parameters: map[string]any{constants.PVCNameKey: "test-pvc", constants.PVCNamespaceKey: "test-namespace",
				constants.PVNameKey: "pvc-c2fd3f46-bf17-4a7d-b88e-2e3232bae434"},
			want: map[string]string{"PVCNamespace": "test-namespace", "PVCName": "test-pvc",
				"PVName": "pvc-c2fd3f46-bf17-4a7d-b88e-2e3232bae434", "PVCUid": "c2fd3f46bf174a7db88e2e3232bae434"},
			wantErr: false},
		{name: "complex volume prefix", volumePrefix: "prefix-first.whatever",
			parameters: map[string]any{constants.PVCNameKey: "test-pvc", constants.PVCNamespaceKey: "test-namespace",
				constants.PVNameKey: "prefix-first.whatever-c2fd3f46-bf17-4a7d-b88e-2e3232bae434"},
			want: map[string]string{"PVCNamespace": "test-namespace", "PVCName": "test-pvc",
				"PVName": "prefix-first.whatever-c2fd3f46-bf17-4a7d-b88e-2e3232bae434",
				"PVCUid": "c2fd3f46bf174a7db88e2e3232bae434"}, wantErr: false},
		{name: "no metadata", parameters: map[string]any{}, want: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			originVolumeNamePrefix := app.GetGlobalConfig().VolumeNamePrefix
			defer func() { app.GetGlobalConfig().VolumeNamePrefix = originVolumeNamePrefix }()
			app.GetGlobalConfig().VolumeNamePrefix = tt.volumePrefix

			// action
			got, err := newExtraCreateMetadataFromParameters(tt.parameters)

			// assert
			if tt.wantErr {
				require.Error(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestOceanstorPlugin_getVolumeNameFromPVNameOrParameters(t *testing.T) {
	// arrange
	p := &OceanstorPlugin{}
	uid := "c2fd3f46-bf17-4a7d-b88e-2e3232bae434"

	type args struct {
		product      constants.OceanstorVersion
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
		{name: "not dorado v6 or v7",
			args: args{product: constants.OceanStorV3, volumePrefix: "pvc", pvName: "pvc-" + uid, parameters: nil},
			want: "pvc-" + uid, wantErrMsg: ""},
		{name: "not configure volumeName",
			args: args{product: constants.OceanStorDoradoV6, volumePrefix: "pvc", pvName: "pvc-" + uid,
				parameters: nil}, want: "pvc-" + uid, wantErrMsg: ""},
		{name: "validate volume name failed",
			args: args{product: constants.OceanStorDoradoV6, volumePrefix: "pvc", pvName: "pvc-" + uid,
				parameters: map[string]any{"volumeName": "{{.PVCNamespace}}"}}, want: "" + uid,
			wantErrMsg: "{{.PVCNamespace}} or {{." +
				"PVCName}} must be configured in the volumeName parameter at the same time"},
		{name: "metadata key not found",
			args: args{product: constants.OceanStorDoradoV6, volumePrefix: "pvc", pvName: "pvc-" + uid,
				parameters: map[string]any{"volumeName": "{{.PVCNamespace}}{{.PVCName}}"}}, want: "" + uid,
			wantErrMsg: "not found"},
		{name: "success", args: args{product: constants.OceanStorDoradoV6, volumePrefix: "pvc", pvName: "pvc-" + uid,
			parameters: map[string]any{"volumeName": "{{.PVCNamespace}}-{{.PVCName}}", constants.PVCNameKey: "test-pvc",
				constants.PVCNamespaceKey: "test-namespace", constants.PVNameKey: "pvc-" + uid}},
			want: "test-namespace-test-pvc-" + strings.Replace(uid, "-", "", -1), wantErrMsg: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock
			p.product = tt.args.product
			app.GetGlobalConfig().VolumeNamePrefix = tt.args.volumePrefix

			// action
			got, err := p.getVolumeNameFromPVNameOrParameters(tt.args.pvName, tt.args.parameters)

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
