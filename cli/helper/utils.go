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
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/tabwriter"

	"k8s.io/apimachinery/pkg/util/uuid"

	"huawei-csi-driver/utils/log"
)

const BackendNameMaxLength = 63
const BackendNameUidMaxLength = 5

const dns1123LabelFmt = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
const dns1123SubdomainFmt = dns1123LabelFmt + "(" + dns1123LabelFmt + ")*"

var dns1123SubdomainRegexp = regexp.MustCompile("^" + dns1123SubdomainFmt + "$")

// LogErrorf write error log and return the error
func LogErrorf(format string, err error) error {
	log.Errorf(format, err)
	return err
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
func PrintOperateResult(resourceType, operate string, resourceNames ...string) {
	for _, name := range resourceNames {
		fmt.Printf("%s/%s %s\n", resourceType, name, operate)
	}
}

// AppendUid append uid after the name
func AppendUid(name string, uidLen int) string {
	uid := strings.ReplaceAll(string(uuid.NewUUID()), "-", "")
	if len(uid) > uidLen-1 {
		uid = uid[:uidLen-1]
	}
	return name + "-" + uid
}

// PrintWithYaml print yaml
func PrintWithYaml[T any](data []T) {
	if len(data) == 0 {
		return
	}

	marshal, err := StructToYAML(data)
	if err != nil {
		fmt.Printf("format to json failed: %v\n", err)
	}
	fmt.Println(string(marshal))
}

// PrintWithJson print json
func PrintWithJson[T any](data []T) {
	if len(data) == 0 {
		return
	}

	marshal, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("format to json failed: %v\n", err)
		return
	}
	fmt.Println(string(marshal))
}

// PrintWithTable print table
func PrintWithTable[T any](data []T) {
	if len(data) == 0 {
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	if _, err := fmt.Fprintln(w, strings.Join(ReadHeader(data[0]), "\t")); err != nil {
		fmt.Printf("format to table failed: %v\n", err)
	}
	for _, row := range data {
		if _, err := fmt.Fprintln(w, strings.Join(ReadRow(row), "\t")); err != nil {
			fmt.Printf("format to table failed: %v\n", err)
		}
	}
	if err := w.Flush(); err != nil {
		fmt.Printf("format to table failed: %v\n", err)
	}
}

// PrintBackend print backend
func PrintBackend[T any](t []T, notFound []string, printFunc func(t []T)) {
	printFunc(t)
	PrintNotFoundBackend(notFound...)
}

// PrintSecret print secret
func PrintSecret[T any](t []T, notFound []string, printFunc func(t []T)) {
	printFunc(t)
	PrintNotFoundSecret(notFound...)
}

// PrintNotFoundBackend print not found backend
func PrintNotFoundBackend(names ...string) {
	for _, name := range names {
		fmt.Printf("Error from server (NotFound): backend \"%s\" not found\n", name)
	}
}

// PrintNotFoundSecret print not found secret
func PrintNotFoundSecret(names ...string) {
	for _, name := range names {
		fmt.Printf("Error from server (NotFound): cert \"%s\" not found\n", name)
	}
}

// PrintNoResourceBackend print not found backend
func PrintNoResourceBackend(namespace string) {
	fmt.Printf("No backends found in %s namespace\n", namespace)
}

// PrintNoResourceCert print no cert found
func PrintNoResourceCert(backend, namespace string) {
	fmt.Printf("Error from server (NotFound): no cert found on backend %s in %s namespace\n", backend, namespace)
}

// PrintBackendAlreadyExists print backend already exists error
func PrintBackendAlreadyExists(backend string) {
	fmt.Printf("Error from server (AlreadyExists): backend \"%s\" already exists\n", backend)
}

// BackendAlreadyExistsError return a backend already exists error
func BackendAlreadyExistsError(backend string, filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	msgFormat := "Error from server (DuplicateName): the backend \"%s\" already exists. Please check %s file"
	return fmt.Errorf(msgFormat, backend, absPath)
}

// GetPrintFunc get print function by format type
func GetPrintFunc[T any](format string) func(t []T) {
	switch format {
	case "json":
		return PrintWithJson[T]
	case "yaml":
		return PrintWithYaml[T]
	default:
		return PrintWithTable[T]
	}
}

// ReadHeader read struct tag name
// T is a struct
func ReadHeader[T any](t T) []string {
	return ReadStruct(t, func(field reflect.StructField, value reflect.Value) (string, bool) {
		showName, ok := field.Tag.Lookup("show")
		return showName, ok
	})
}

// ReadRow read struct filed value
// T is a struct
func ReadRow[T any](t T) []string {
	return ReadStruct(t, func(field reflect.StructField, value reflect.Value) (string, bool) {
		if showName, ok := field.Tag.Lookup("show"); !ok {
			return showName, ok
		}

		result, ok := value.Interface().(string)
		return result, ok
	})
}

// ReadStruct read struct
// T is a struct
// readFunc is a read struct func, e.g. read struct filed value
func ReadStruct[T any, O any](t T, readFunc func(field reflect.StructField, value reflect.Value) (O, bool)) []O {
	var result []O
	filedType := reflect.TypeOf(t)
	filedValue := reflect.ValueOf(t)
	for i := 0; i < filedType.NumField(); i++ {
		if item, ok := readFunc(filedType.Field(i), filedValue.Field(i)); ok {
			result = append(result, item)
		}
	}
	return result
}

// MapTo  Returns an array consisting of the results of applying the given function to the elements of this list.
func MapTo[I any, O any](list []I, consumer func(I) O) []O {
	var result []O
	for _, item := range list {
		result = append(result, consumer(item))
	}
	return result
}

// IsDNSFormat Determine if the DNS format is met
func IsDNSFormat(source string) bool {
	if len(source) > BackendNameMaxLength {
		return false
	}
	return dns1123SubdomainRegexp.MatchString(source)
}

// GenerateHashCode generate hash code
func GenerateHashCode(txt string, max int) string {
	hashInstance := sha256.New()
	hashInstance.Write([]byte(txt))
	sum := hashInstance.Sum(nil)
	result := fmt.Sprintf("%x", sum)
	if len(result) < max {
		return result
	}
	return result[:max]
}

// BuildBackendName build backend name
func BuildBackendName(name string) string {
	nameLen := BackendNameMaxLength - BackendNameUidMaxLength - 1
	if len(name) > nameLen {
		name = name[:nameLen]
	}
	hashCode := GenerateHashCode(name, BackendNameUidMaxLength)
	mappingName := BackendNameMapping(name)
	return fmt.Sprintf("%s-%s", mappingName, hashCode)
}

// BackendNameMapping mapping backend name
func BackendNameMapping(name string) string {
	removeUnderline := strings.ReplaceAll(name, "_", "-")
	removePoint := strings.ReplaceAll(removeUnderline, ".", "-")
	return strings.ToLower(removePoint)
}

func GetBackendName(name string) string {
	if IsDNSFormat(name) {
		return name
	}
	return BuildBackendName(name)
}

// LogInfof write message
func LogInfof(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	tag := ctx.Value("tag")
	if tag == nil {
		log.Infof("%s", msg)
		return
	}
	log.Infof("<<%s>> %s", tag, msg)
}

// LogWarningf write error log and return the error
func LogWarningf(ctx context.Context, format string, err error) error {
	errMessage := fmt.Sprintf(format, err)
	tag := ctx.Value("tag")
	if tag == nil {
		log.Warningf("%s", errMessage)
		return err
	}
	log.Warningf("<<%s>> %s", tag, errMessage)
	return err
}

// ConvertInterface convert interface
func ConvertInterface(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			if key, ok := k.(string); ok {
				m2[key] = ConvertInterface(v)
			}
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = ConvertInterface(v)
		}
	default:
	}
	return i
}
