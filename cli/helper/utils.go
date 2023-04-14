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
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/uuid"

	"huawei-csi-driver/utils/log"
)

// LogErrorf write error log and return the error
func LogErrorf(format string, err error) error {
	log.Errorf(format, err)
	return err
}

// PrintError used to print error to terminal
func PrintError(err error) error {
	fmt.Printf("%v", err)
	return nil
}

// PrintlnError used to print error to terminal
func PrintlnError(err error) error {
	fmt.Printf("%v\n", err)
	return nil
}

// PrintResult used to print result to terminal
func PrintResult(out string) {
	fmt.Printf("%s", out)
}

// PrintOperateResult used to print operate result to terminal
// e.g. backend/backend-name created
func PrintOperateResult(resourceNames []string, resourceType, operate string) {
	for _, name := range resourceNames {
		fmt.Printf("%s/%s %s\n", resourceType, name, operate)
	}
}

// ParseNumericString parse numeric string
func ParseNumericString(v interface{}) string {
	if strVal, ok := v.(string); !ok {
		return strVal
	}
	if intVal, ok := v.(int); ok {
		return strconv.Itoa(intVal)
	}
	return ""
}

// AppendUid append uid after the name
func AppendUid(name string, uidLen int) string {
	uid := strings.ReplaceAll(string(uuid.NewUUID()), "-", "")
	if len(uid) > uidLen-1 {
		uid = uid[:uidLen-1]
	}
	return name + "-" + uid
}
