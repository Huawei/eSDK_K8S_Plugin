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

package helper

import (
	"testing"
)

const testSecretTemplate = `
apiVersion: v1
kind: Secret
metadata:
  name: test-name
  namespace: test-namespace
type: Opaque
stringData:
  password: "test-pwd"
  user: "test-user"
`

const testConfigMapTemplate = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-name
  namespace: test-namespace
data:
  csi.json: "{\"backends\":\n    {\"name\":\"test-name\",\"storage\": \"oceanstor-san\"}}"
`

const testStorageBackendClaimTemplate = `
apiVersion: xuanwu.huawei.io/v1
kind: StorageBackendClaim
metadata:
  name: test-name
  namespace: test-namespace
spec:
  provider: csi.huawei.com
  configmapMeta: test-namespace/test-name
  secretMeta: test-namespace/test-name
  maxClientThreads: "30"
`

func TestGenerateSecretYaml(t *testing.T) {
	config := SecretConfig{
		Name:      "test-name",
		Namespace: "test-namespace",
		User:      "test-user",
		Pwd:       "test-pwd",
	}
	template := GenerateSecretYaml(config)
	if template != testSecretTemplate {
		t.Errorf("GenerateSecretYaml failed, want: %s, got: %s", testSecretTemplate, template)
	}
}

func TestGenerateConfigMapYaml1(t *testing.T) {
	config := ConfigMapConfig{
		Name:      "test-name",
		Namespace: "test-namespace",
		JsonData:  "{\"name\":\"test-name\",\"storage\": \"oceanstor-san\"}",
	}
	template := GenerateConfigMapYaml(config)
	if template != testConfigMapTemplate {
		t.Errorf("GenerateConfigMapYaml failed, want: %s, got: %s", testConfigMapTemplate, template)
	}
}

func TestGenerateStorageBackendClaimYaml(t *testing.T) {
	config := StorageBackendClaimConfig{
		Name:             "test-name",
		Namespace:        "test-namespace",
		ConfigmapMeta:    "test-namespace/test-name",
		SecretMeta:       "test-namespace/test-name",
		MaxClientThreads: "30",
	}
	template := GenerateStorageBackendClaimYaml(config)
	if template != testStorageBackendClaimTemplate {
		t.Errorf("GenerateStorageBackendClaimYaml failed, want: %s, got: %s",
			testStorageBackendClaimTemplate, template)
	}
}
