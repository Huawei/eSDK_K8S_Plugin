/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

// Package host defines a set of useful methods, which can help Connector to operate host information
package host

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/proto"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "hostHelperTest.log"
)

var (
	mockInfo = &NodeHostInfo{
		HostName:       "test_hostname",
		IscsiInitiator: "test_iscsi_initiator",
		NVMeInitiator:  "test_nvme_initiator",
		FCInitiators:   []string{"test_fc_initiator_1", "test_fc_initiator_2"},
		HostIPs:        []string{"127.0.0.1"},
	}
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)
	m.Run()
}

func TestNewNodeHostInfo_Successful(t *testing.T) {
	// arrange
	want := &NodeHostInfo{
		HostName:       "test_hostname",
		IscsiInitiator: "test_iscsi_initiator",
		FCInitiators:   []string{"test_fc_initiator_1", "test_fc_initiator_2"},
		NVMeInitiator:  "test_nvme_initiator",
		HostIPs:        []string{"127.0.0.1"},
	}

	// mock
	patches := gomonkey.NewPatches()
	patches.Reset()
	mockHostInfo(patches)

	// action
	info, err := NewNodeHostInfo(context.Background(), true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, want, info)
}

func mockHostInfo(patches *gomonkey.Patches) {
	patches.ApplyFuncReturn(proto.GetISCSIInitiator, mockInfo.IscsiInitiator, nil)
	patches.ApplyFuncReturn(proto.GetFCInitiator, mockInfo.FCInitiators, nil)
	patches.ApplyFuncReturn(proto.GetNVMeInitiator, mockInfo.NVMeInitiator, nil)
	patches.ApplyFuncReturn(utils.GetHostIPs, mockInfo.HostIPs, nil)
	patches.ApplyFuncReturn(utils.GetHostName, mockInfo.HostName, nil)
}

func TestSaveNodeHostInfoToSecret_withDifferentSecretMode(t *testing.T) {
	tests := []struct {
		name          string
		enablePreNode bool
		reportIps     bool
	}{
		{name: "test_for_pre_node_secret_with_not_report_ips", enablePreNode: true, reportIps: false},
		{name: "test_for_unified_secret_with_not_report_ips", enablePreNode: false, reportIps: false},
		{name: "test_for_pre_node_secret_with_report_ips", enablePreNode: true, reportIps: true},
		{name: "test_for_unified_secret_with_report_ips", enablePreNode: false, reportIps: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			config := cfg.MockCompletedConfig()
			config.ReportNodeIP = tt.reportIps
			config.EnablePerNodeSecret = tt.enablePreNode
			stubs := gostub.StubFunc(&app.GetGlobalConfig, config)
			defer stubs.Reset()

			patches := gomonkey.NewPatches()
			patches.Reset()
			mockHostInfo(patches)

			// action
			err := SaveNodeHostInfoToSecret(context.Background())
			assert.NoError(t, err)

			// asset
			info, err := GetNodeHostInfosFromSecret(context.Background(), mockInfo.HostName)
			assert.NoError(t, err)

			assert.Equal(t, "", info.HostName)
			if tt.reportIps {
				assert.Equal(t, mockInfo.HostIPs, info.HostIPs)
			} else {
				assert.Equal(t, []string(nil), info.HostIPs)
			}
		})
	}
}

func TestMakeNodeHostInfoSecret(t *testing.T) {
	stubs := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer stubs.Reset()

	want := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      hostInfoSecretName,
			Namespace: app.GetGlobalConfig().Namespace,
			Labels:    map[string]string{hostInfoSecretKey: hostInfoSecretValue},
		},
		StringData: map[string]string{},
		Type:       corev1.SecretTypeOpaque,
	}

	hostInfoSecret := makeNodeHostInfoSecret(hostInfoSecretName, app.GetGlobalConfig().Namespace)
	assert.Equal(t, want, hostInfoSecret)
}
