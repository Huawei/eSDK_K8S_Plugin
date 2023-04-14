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
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

const testFile string = `
storage: "oceanstor-san"
name: iscsi-127
namespace: huawei-csi
urls:
  - "https://127.0.0.1:0000"
pools:
  - "Pool001"
parameters:
  protocol: iscsi
  portals:
    - "127.0.0.1"
`

var mockYamlMap = map[string]interface{}{
	"storage":   "oceanstor-san",
	"name":      "iscsi-127",
	"namespace": "huawei-csi",
	"urls":      []interface{}{"https://127.0.0.1:0000"},
	"pools":     []interface{}{"Pool001"},
	"parameters": map[string]interface{}{
		"protocol": "iscsi",
		"portals":  []interface{}{"127.0.0.1"},
	},
}

var mockYamlValues YamlValues = mockYamlMap

func TestReadYamlFile(t *testing.T) {
	applyFunc := gomonkey.ApplyFunc(os.ReadFile, func(name string) ([]byte, error) {
		return []byte(name), nil
	})

	got, err := ReadYamlFile(testFile)
	if err != nil {
		t.Errorf("TestReadYamlFile failed, want : nil, got: %v", err)
	}
	if !reflect.DeepEqual(mockYamlValues, got) {
		t.Errorf("TestReadYamlFile failed, want: %+v, got: %+v", mockYamlValues, got)
	}

	t.Cleanup(func() {
		applyFunc.Reset()
	})
}

func TestYamlValuesAsMap(t *testing.T) {
	if !reflect.DeepEqual(mockYamlMap, mockYamlValues.AsMap()) {
		t.Errorf("TestYamlValuesAsMap failed, want: %+v, got: %+v", mockYamlValues, mockYamlValues.AsMap())
	}
}
