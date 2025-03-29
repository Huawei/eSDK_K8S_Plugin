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

// Package utils provides utils for test
package utils

import (
	"fmt"
	"reflect"
)

// StructToStringMap converts the struct to map[string]string by given field tag.
func StructToStringMap(input any, tagName string) map[string]string {
	result := make(map[string]string)
	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		tag := val.Type().Field(i).Tag.Get(tagName)

		if tag == "" {
			continue
		}

		if field.IsZero() {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			result[tag] = field.String()
		default:
			result[tag] = fmt.Sprintf("%v", field.Interface())
		}
	}

	return result
}
