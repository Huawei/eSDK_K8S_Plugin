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
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const secretTemplate = `
apiVersion: v1
kind: Secret
metadata:
  name: {SECRET_NAME}
  namespace: {NAMESPACE}
type: Opaque
stringData:
  password: "{BACKEND_PWD}"
  user: "{BACKEND_USER}"
`

const configMapTemplate = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: {CONFIGMAP_NAME}
  namespace: {NAMESPACE}
data:
  csi.json: {BACKEND_CONFIGMAP_DATA}
`

const storageBackendClaimTemplate = `
apiVersion: xuanwu.huawei.io/v1
kind: StorageBackendClaim
metadata:
  name: {STORAGE_BACKEND_NAME}
  namespace: {NAMESPACE}
spec:
  provider: csi.huawei.com
  configmapMeta: {BACKEND_CONFIGMAP_META}
  secretMeta: {BACKEND_SECRET_META}
  maxClientThreads: "{BACKEND_MAX_CLIENT}"
`

// GenerateSecretYaml generate secret yaml template
func GenerateSecretYaml(config SecretConfig) string {
	return GenerateYaml(secretTemplate, config)
}

// GenerateConfigMapYaml generate configmap yaml template
func GenerateConfigMapYaml(config ConfigMapConfig) string {
	jsonData := "{\"backends\":"
	jsonData += NewNormalizer(config.JsonData).NIndent(4)
	jsonData += "}"
	config.JsonData = strconv.Quote(jsonData)
	generateYaml := GenerateYaml(configMapTemplate, config)
	return generateYaml
}

// GenerateStorageBackendClaimYaml generate storagebackendclaim yaml template
func GenerateStorageBackendClaimYaml(config StorageBackendClaimConfig) string {
	return GenerateYaml(storageBackendClaimTemplate, config)
}

// GenerateYaml generate yaml by config.
// Note: yaml tag is required in config
func GenerateYaml(yaml string, config interface{}) string {
	mapping := GenerateTagMapping(config)
	yamlTemplate := yaml
	for key, value := range mapping {
		old := fmt.Sprintf("{%s}", key)
		yamlTemplate = strings.ReplaceAll(yamlTemplate, old, value)
	}
	return yamlTemplate
}

// GenerateTagMapping used to create a mapping between tag and value
// e.g. map[tag]=value
func GenerateTagMapping(config interface{}) map[string]string {
	mapping := map[string]string{}
	filedType := reflect.TypeOf(config)
	filedValue := reflect.ValueOf(config)
	for i := 0; i < filedType.NumField(); i++ {
		value, ok := filedValue.Field(i).Interface().(string)
		if !ok || value == "" {
			continue
		}
		mapping[filedType.Field(i).Tag.Get("yaml")] = value
	}
	return mapping
}
