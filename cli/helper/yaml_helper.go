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
	"encoding/json"
	"os"

	"github.com/ghodss/yaml"
)

// YamlValues represents a collection of yaml values.
type YamlValues map[string]interface{}

// YAML encodes the Values into a YAML string.
func (v YamlValues) YAML() (string, error) {
	b, err := yaml.Marshal(v)
	return string(b), err
}

// AsMap is a utility function for converting YamlValues to a map[string]interface{}.
func (v YamlValues) AsMap() map[string]interface{} {
	if len(v) == 0 {
		return map[string]interface{}{}
	}
	return v
}

// ReadYamlValues will parse YAML byte data into a YamlValues.
func ReadYamlValues(data []byte) (val YamlValues, err error) {
	err = yaml.Unmarshal(data, &val)
	if len(val) == 0 {
		val = YamlValues{}
	}
	return val, err
}

// ReadYamlFile will parse a YAML file into a map of YamlValues.
func ReadYamlFile(filename string) (YamlValues, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return map[string]interface{}{}, err
	}
	return ReadYamlValues(data)
}

// ToPrettyJson encodes an item into a pretty (indented) JSON string
func (v YamlValues) ToPrettyJson() string {
	output, _ := json.MarshalIndent(v, "", "  ")
	return string(output)
}
