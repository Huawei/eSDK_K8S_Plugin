/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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
	"reflect"
	"testing"

	"github.com/prashantv/gostub"
)

func TestRegisterClient(t *testing.T) {
	tests := []struct {
		testName string
		name     string
		client   KubernetesClient
	}{
		{
			testName: "test_register_kubectl_client",
			name:     "kubectl",
			client:   &KubernetesCLI{cli: "kubectl"},
		},
		{
			testName: "test_register_openshift_client",
			name:     "oc",
			client:   &KubernetesCLI{cli: "oc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			RegisterClient(tt.name, tt.client)
			client, ok := clientSet[tt.name]
			if !ok {
				t.Errorf("RegisterClient() excepted to got a client, but couldn't found")
			}
			if !reflect.DeepEqual(tt.client, client) {
				t.Errorf("RegisterClient() want = %v, but got = %v", tt.client, client)
			}
		})
	}
}

func TestLoadSupportedClient(t *testing.T) {
	mackData := map[string]KubernetesClient{
		"kubectl": &KubernetesCLI{cli: "kubectl"},
	}
	mockClientSet := gostub.Stub(&clientSet, mackData)

	tests := []struct {
		testName   string
		clientName string
		want       KubernetesClient
		wantErr    bool
	}{
		{
			testName:   "test_load_kubectl_success",
			clientName: "kubectl",
			want:       mackData["kubectl"],
			wantErr:    false,
		},
		{
			testName:   "test_load_unsupported_cli",
			clientName: "test-mock-name",
			want:       nil,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			got, err := LoadSupportedClient(tt.clientName)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadSupportedClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadSupportedClient() got = %v, want %v", got, tt.want)
			}
		})
	}

	t.Cleanup(func() {
		mockClientSet.Reset()
	})
}
