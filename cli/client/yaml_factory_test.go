/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

import "testing"

const csiSecret = "huawei-csi-secret"
const defaultNameSpace = "huawei-csi"

const normalCaseExpected = `
apiVersion: v1
kind: Secret
metadata:
  name: huawei-csi-secret
  namespace: huawei-csi
type: Opaque
stringData:
  secret.json: |
    {
      "secrets": {
        "stringDataKey1": stringDataVal1
      }
    }
`

const emptyStringDataExpected = `
apiVersion: v1
kind: Secret
metadata:
  name: huawei-csi-secret
  namespace: huawei-csi
type: Opaque
stringData:
  secret.json: |
`

func TestGetSecretYAML(t *testing.T) {
	stringData := map[string]string{"stringDataKey1": "stringDataVal1"}

	cases := []struct {
		CaseName              string
		secretName, namespace string
		stringData            map[string]string
		Expected              string
	}{
		{"Normal", csiSecret, defaultNameSpace, stringData, normalCaseExpected},
		{"EmptyStringData", csiSecret, defaultNameSpace, nil, emptyStringDataExpected},
	}

	for _, c := range cases {
		t.Run(c.CaseName, func(t *testing.T) {
			if ans := GetSecretYAML(c.secretName, c.namespace, c.stringData); ans != c.Expected {
				t.Errorf("Test GetSecretYAML failed.\nExpected:%s \nGet:%s", c.Expected, ans)
			}
		})
	}
}
