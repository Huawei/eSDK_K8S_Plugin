/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

package resources

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	coreV1 "k8s.io/api/core/v1"
)

func Test_saveConsoleLog_Success(t *testing.T) {
	// arrange
	logs := []byte("log contents")
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	nodeName := "node1"
	mockFile := &os.File{}

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.Create, func(name string) (*os.File, error) {
		return mockFile, nil
	}).ApplyMethod(reflect.TypeOf(mockFile), "Chmod", func(_ *os.File, mode os.FileMode) error {
		return nil
	}).ApplyMethod(reflect.TypeOf(mockFile), "Write", func(_ *os.File, b []byte) (n int, err error) {
		return 1, nil
	}).ApplyMethod(reflect.TypeOf(mockFile), "Close", func(_ *os.File) error {
		return nil
	})

	// act
	gotErr := saveConsoleLog(logs, namespace, podName, containerName, nodeName, true)

	// assert
	if gotErr != nil {
		t.Errorf("Test_saveConsoleLog_Success failed, gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_saveConsoleLog_CreateFileFail(t *testing.T) {
	// arrange
	logs := []byte("log contents")
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	nodeName := "node1"
	createErr := fmt.Errorf("create file err")
	wantErr := createErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.Create, func(name string) (*os.File, error) {
		return nil, createErr
	})

	// act
	gotErr := saveConsoleLog(logs, namespace, podName, containerName, nodeName, true)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_saveConsoleLog_CreateFileFail failed, gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_saveConsoleLog_ChmodFileFail(t *testing.T) {
	// arrange
	logs := []byte("log contents")
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	nodeName := "node1"
	mockFile := &os.File{}
	chmodErr := fmt.Errorf("chmod file err")
	wantErr := chmodErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.Create, func(name string) (*os.File, error) {
		return mockFile, nil
	}).ApplyMethod(reflect.TypeOf(mockFile), "Chmod", func(_ *os.File, mode os.FileMode) error {
		return chmodErr
	}).ApplyMethod(reflect.TypeOf(mockFile), "Close", func(_ *os.File) error {
		return nil
	})

	// act
	gotErr := saveConsoleLog(logs, namespace, podName, containerName, nodeName, true)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_saveConsoleLog_ChmodFileFail failed, gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_saveConsoleLog_WriteFileFail(t *testing.T) {
	// arrange
	logs := []byte("log contents")
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	nodeName := "node1"
	mockFile := &os.File{}
	writeErr := fmt.Errorf("write file err")
	wantErr := writeErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.Create, func(name string) (*os.File, error) {
		return mockFile, nil
	}).ApplyMethod(reflect.TypeOf(mockFile), "Chmod", func(_ *os.File, mode os.FileMode) error {
		return nil
	}).ApplyMethod(reflect.TypeOf(mockFile), "Write", func(_ *os.File, b []byte) (n int, err error) {
		return 0, writeErr
	}).ApplyMethod(reflect.TypeOf(mockFile), "Close", func(_ *os.File) error {
		return nil
	})

	// act
	gotErr := saveConsoleLog(logs, namespace, podName, containerName, nodeName, true)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_saveConsoleLog_WriteFileFail failed, gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_getContainerFileLogPaths_Success(t *testing.T) {
	// arrange
	logPaths := "/tmp"
	container := &coreV1.Container{Name: "container", Args: []string{"--log-file-dir=" + logPaths}}

	// act
	str, gotErr := getContainerFileLogPaths(container)

	// assert
	if str != logPaths {
		t.Errorf("Test_getContainerFileLogPaths_Success failed, gotStr [%v], wantStr [%v]", str, logPaths)
	}
	if gotErr != nil {
		t.Errorf("Test_getContainerFileLogPaths_Success failed, gotErr [%v], wantErr [%v]", gotErr, nil)
	}
}

func Test_getContainerFileLogPaths_ArgsNilFail(t *testing.T) {
	// arrange
	container := &coreV1.Container{Name: "container", Args: nil}
	wantErr := fmt.Errorf("args is nil")

	// act
	_, gotErr := getContainerFileLogPaths(container)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_getContainerFileLogPaths_ArgsNilFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}
}

func Test_getContainerFileLogPaths_LogPathNotSetFail(t *testing.T) {
	// arrange
	container := &coreV1.Container{Name: "container", Args: []string{"--timeout=15s"}}
	wantErr := fmt.Errorf("log-file-dir is not set")

	// act
	_, gotErr := getContainerFileLogPaths(container)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_getContainerFileLogPaths_LogPathNotSetFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}
}

func Test_getContainerFileLogPaths_ArgsFormatFail(t *testing.T) {
	// arrange
	container := &coreV1.Container{Name: "container", Args: []string{"--log-file-dir=/tmp=/usr"}}
	wantErr := fmt.Errorf("log-file-dir is not set correctly")

	// act
	_, gotErr := getContainerFileLogPaths(container)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_getContainerFileLogPaths_ArgsFormatFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}
}
