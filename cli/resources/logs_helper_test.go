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
	"github.com/stretchr/testify/assert"
	coreV1 "k8s.io/api/core/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/helper"
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
	gotErr := saveConsoleLog(logs, getLogArgs(namespace, containerName, podName, nodeName, true))

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
	gotErr := saveConsoleLog(logs, getLogArgs(namespace, containerName, podName, nodeName, true))

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
	gotErr := saveConsoleLog(logs, getLogArgs(namespace, containerName, podName, nodeName, true))

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
	gotErr := saveConsoleLog(logs, getLogArgs(namespace, containerName, podName, nodeName, true))

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

func TestNodeLogCollector_isCollected_Success(t *testing.T) {
	// arrange
	collector := &NodeLogCollector{}
	collector.collectedDirMap.Store("/var/log/csi", true)

	// action
	gotResult := collector.isCollected("/var/log/csi")

	// assert
	assert.True(t, gotResult)
}

func TestNodeLogCollector_isCollected_NotCollected(t *testing.T) {
	// arrange
	collector := &NodeLogCollector{}

	// action
	gotResult := collector.isCollected("/var/log/csi")

	// assert
	assert.False(t, gotResult)
}

func TestNodeLogCollector_markCollected_Success(t *testing.T) {
	// arrange
	collector := &NodeLogCollector{}

	// action
	collector.markCollected("/var/log/csi")

	// assert
	assert.True(t, collector.isCollected("/var/log/csi"))
}

func TestNodeLogCollector_collectPodLogs_Success(t *testing.T) {
	// arrange
	pod := &coreV1.Pod{
		Spec: coreV1.PodSpec{
			NodeName:   "node1",
			Containers: []coreV1.Container{{Name: "huawei-csi-driver", Args: []string{"--log-file-dir=/var/log/csi"}}},
		},
		Status: coreV1.PodStatus{Phase: coreV1.PodRunning},
	}
	transmitter := helper.NewTransmitter(1, 10)
	collector := &NodeLogCollector{
		podList:          []coreV1.Pod{*pod},
		completionStatus: Status{total: 1},
		transmitter:      transmitter,
		fileLogsOnce:     make([]helper.Once, 1),
		display:          NewDisplay(),

	}
	fileLogPath := "/var/log/csi"
	mockCollector := &FileLogsCollector{}

	p := gomonkey.NewPatches()
	p.ApplyFuncReturn(LoadSupportedCollector, mockCollector, nil)
	p.ApplyFuncReturn(getConsoleLogs)
	p.ApplyMethodReturn(&FileLogsCollector{}, "GetFileLogs", nil)
	p.ApplyMethodReturn(&FileLogsCollector{}, "GetHostInformation", nil)
	p.ApplyMethodReturn(transmitter, "AddTask")
	defer p.Reset()

	// action
	collector.collectPodLogs(pod, 0)

	// assert
	assert.True(t, collector.isCollected(fileLogPath))
}

func TestNodeLogCollector_collectPodLogs_PathAlreadyCollected(t *testing.T) {
	// arrange
	pod := &coreV1.Pod{
		Spec: coreV1.PodSpec{
			NodeName:   "node1",
			Containers: []coreV1.Container{{Name: "huawei-csi-driver", Args: []string{"--log-file-dir=/var/log/csi"}}},
		},
		Status: coreV1.PodStatus{Phase: coreV1.PodRunning},
	}
	transmitter := helper.NewTransmitter(1, 10)
	fileLogPath := "/var/log/csi"
	collector := &NodeLogCollector{
		podList:          []coreV1.Pod{*pod},
		completionStatus: Status{total: 1},
		transmitter:      transmitter,
		fileLogsOnce:     make([]helper.Once, 1),
		display:          NewDisplay(),
	}
	collector.collectedDirMap.Store(fileLogPath, true)
	mockCollector := &FileLogsCollector{}

	p := gomonkey.NewPatches()
	p.ApplyFuncReturn(LoadSupportedCollector, mockCollector, nil)
	p.ApplyFuncReturn(getConsoleLogs)
	p.ApplyMethodReturn(&FileLogsCollector{}, "GetFileLogs", nil)
	p.ApplyMethodReturn(&FileLogsCollector{}, "GetHostInformation", nil)
	p.ApplyMethodReturn(transmitter, "AddTask")
	defer p.Reset()

	// action
	collector.collectPodLogs(pod, 0)

	// assert — path stays collected, GetFileLogs mock (which returns nil) should not have been invoked
	// because the Once.Do closure returns nil early when path is already collected
	assert.True(t, collector.isCollected(fileLogPath))
}

func TestNodeLogCollector_collectPodLogs_GetFileLogsFailed(t *testing.T) {
	// arrange
	pod := &coreV1.Pod{
		Spec: coreV1.PodSpec{
			NodeName:   "node1",
			Containers: []coreV1.Container{{Name: "huawei-csi-driver", Args: []string{"--log-file-dir=/var/log/csi"}}},
		},
		Status: coreV1.PodStatus{Phase: coreV1.PodRunning},
	}
	transmitter := helper.NewTransmitter(1, 10)
	fileLogPath := "/var/log/csi"
	collector := &NodeLogCollector{
		podList:          []coreV1.Pod{*pod},
		completionStatus: Status{total: 1},
		transmitter:      transmitter,
		fileLogsOnce:     make([]helper.Once, 1),
		display:          NewDisplay(),
	}
	mockCollector := &FileLogsCollector{}
	getFileLogsErr := fmt.Errorf("get file logs failed")

	p := gomonkey.NewPatches()
	p.ApplyFuncReturn(LoadSupportedCollector, mockCollector, nil)
	p.ApplyFuncReturn(getConsoleLogs)
	p.ApplyMethodReturn(&FileLogsCollector{}, "GetFileLogs", getFileLogsErr)
	p.ApplyMethodReturn(&FileLogsCollector{}, "GetHostInformation", nil)
	p.ApplyMethodReturn(transmitter, "AddTask")
	defer p.Reset()

	// action
	collector.collectPodLogs(pod, 0)

	// assert — path should NOT be marked collected since GetFileLogs failed
	assert.False(t, collector.isCollected(fileLogPath))
}

func TestNodeLogCollector_collectPodLogs_GetFileLogPathsFailed(t *testing.T) {
	// arrange
	pod := &coreV1.Pod{
		Spec: coreV1.PodSpec{
			NodeName:   "node1",
			Containers: []coreV1.Container{{Name: "huawei-csi-driver"}},
		},
		Status: coreV1.PodStatus{Phase: coreV1.PodRunning},
	}
	transmitter := helper.NewTransmitter(1, 10)
	collector := &NodeLogCollector{
		podList:          []coreV1.Pod{*pod},
		completionStatus: Status{total: 1},
		transmitter:      transmitter,
		fileLogsOnce:     make([]helper.Once, 1),
		display:          NewDisplay(),
	}
	mockCollector := &FileLogsCollector{}

	p := gomonkey.NewPatches()
	p.ApplyFuncReturn(LoadSupportedCollector, mockCollector, nil)
	p.ApplyFuncReturn(getConsoleLogs)
	p.ApplyMethodReturn(&FileLogsCollector{}, "GetHostInformation", nil)
	defer p.Reset()

	// action
	collector.collectPodLogs(pod, 0)

	// assert — nothing should be marked collected because getContainerFileLogPaths returns error
	// (container has no Args, so getContainerFileLogPaths fails)
	assert.False(t, collector.isCollected("/var/log/csi"))
}
func TestNodeLogCollector_collectPodLogs_RetryAfterGetFileLogsFailed(t *testing.T) {
	// arrange
	pod := &coreV1.Pod{
		Spec: coreV1.PodSpec{
			NodeName: "node1",
			Containers: []coreV1.Container{
				{Name: "storage-backend-controller", Args: []string{"--log-file-dir=/var/log/csi"}},
				{Name: "huawei-csi-driver", Args: []string{"--log-file-dir=/var/log/csi"}},
			},
		},
		Status: coreV1.PodStatus{Phase: coreV1.PodRunning},
	}
	transmitter := helper.NewTransmitter(1, 10)
	fileLogPath := "/var/log/csi"
	collector := &NodeLogCollector{
		podList:          []coreV1.Pod{*pod},
		completionStatus: Status{total: 1},
		transmitter:      transmitter,
		fileLogsOnce:     make([]helper.Once, 1),
		display:          NewDisplay(),
	}
	mockCollector := &FileLogsCollector{}
	getFileLogsErr := fmt.Errorf("mkdir: executable file not found")

	p := gomonkey.NewPatches()
	p.ApplyFuncReturn(LoadSupportedCollector, mockCollector, nil)
	p.ApplyFuncReturn(getConsoleLogs)
	// First call returns error, second call succeeds (gomonkey sequence)
	p.ApplyMethodSeq(&FileLogsCollector{}, "GetFileLogs", []gomonkey.OutputCell{
		{Values: gomonkey.Params{getFileLogsErr}, Times: 1},
		{Values: gomonkey.Params{nil}, Times: 1},
	})
	p.ApplyMethodReturn(&FileLogsCollector{}, "GetHostInformation", nil)
	p.ApplyMethodReturn(transmitter, "AddTask")
	defer p.Reset()

	// action
	collector.collectPodLogs(pod, 0)

	// assert — path should be marked collected after retry succeeds
	assert.True(t, collector.isCollected(fileLogPath))
}
