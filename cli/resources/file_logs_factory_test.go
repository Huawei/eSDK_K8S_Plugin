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
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"huawei-csi-driver/cli/client"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/utils/log"
)

const (
	logName string = "test"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	config.Client = &client.KubernetesCLI{}

	m.Run()
}

func TestBaseFileLogsCollect_getContainerFileLogs_Fail(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	fileLogsPaths := "/var/log/path"
	execErr := fmt.Errorf("exec cmd error")
	wantErr := execErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, execErr
		})

	// act
	gotErr := mockCollect.getContainerFileLogs(namespace, podName, containerName, fileLogsPaths)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestBaseFileLogsCollect_getContainerFileLogs_Fail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_getContainerFileLogs_Success(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	fileLogsPaths := "/var/log/path"

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return []byte("log content"), nil
		})

	// act
	gotErr := mockCollect.getContainerFileLogs(namespace, podName, containerName, fileLogsPaths)

	// assert
	if gotErr != nil {
		t.Errorf("TestBaseFileLogsCollect_getContainerFileLogs_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_deleteFileLogsInContainer_Fail(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	fileLogsPaths := "/var/log/path"
	execErr := fmt.Errorf("exec cmd error")
	wantErr := execErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, execErr
		})

	// act
	gotErr := mockCollect.deleteFileLogsInContainer(namespace, podName, containerName, fileLogsPaths)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestBaseFileLogsCollect_deleteFileLogsInContainer_Fail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_deleteFileLogsInContainer_Success(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	fileLogsPaths := "/var/log/path"

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, nil
		})

	// act
	gotErr := mockCollect.deleteFileLogsInContainer(namespace, podName, containerName, fileLogsPaths)

	// assert
	if gotErr != nil {
		t.Errorf("TestBaseFileLogsCollect_deleteFileLogsInContainer_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_compressLogsInContainer_Fail(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	execErr := fmt.Errorf("exec cmd error")
	wantErr := execErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, execErr
		})

	// act
	gotErr := mockCollect.compressLogsInContainer(namespace, podName, containerName)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestBaseFileLogsCollect_compressLogsInContainer_Fail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_compressLogsInContainer_Success(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, nil
		})

	// act
	gotErr := mockCollect.compressLogsInContainer(namespace, podName, containerName)

	// assert
	if gotErr != nil {
		t.Errorf("TestBaseFileLogsCollect_compressLogsInContainer_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_CopyToLocal_Fail(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	namespace := "namespace1"
	nodeName := "nodeName1"
	podName := "pod1"
	containerName := "container1"
	copyErr := fmt.Errorf("copy to local error")
	wantErr := copyErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "CopyContainerFileToLocal",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, src, dst string,
			podName ...string) ([]byte, error) {
			return nil, copyErr
		})

	// act
	gotErr := mockCollect.CopyToLocal(namespace, nodeName, podName, containerName)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestBaseFileLogsCollect_CopyToLocal_Fail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_CopyToLocal_Success(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	namespace := "namespace1"
	nodeName := "nodeName1"
	podName := "pod1"
	containerName := "container1"

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "CopyContainerFileToLocal",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, src, dst string,
			podName ...string) ([]byte, error) {
			return nil, nil
		})

	// act
	gotErr := mockCollect.CopyToLocal(namespace, nodeName, podName, containerName)

	// assert
	if gotErr != nil {
		t.Errorf("TestBaseFileLogsCollect_CopyToLocal_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_GetHostInformation_ExecFail(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	nodeName := "node1"
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	execErr := fmt.Errorf("exec cmd error")
	wantErr := execErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, execErr
		})

	// act
	gotErr := mockCollect.GetHostInformation(namespace, containerName, nodeName, podName)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestBaseFileLogsCollect_GetHostInformation_ExecFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_GetHostInformation_CopyFail(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	nodeName := "node1"
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	copyErr := fmt.Errorf("copy to local error")
	wantErr := copyErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, nil
		}).ApplyMethod(reflect.TypeOf(config.Client), "CopyContainerFileToLocal",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, src, dst string,
			podName ...string) ([]byte, error) {
			return nil, copyErr
		})

	// act
	gotErr := mockCollect.GetHostInformation(namespace, containerName, nodeName, podName)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestBaseFileLogsCollect_GetHostInformation_CopyFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestBaseFileLogsCollect_GetHostInformation_Success(t *testing.T) {
	// arrange
	mockCollect := &BaseFileLogsCollect{}
	nodeName := "node1"
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, nil
		}).ApplyMethod(reflect.TypeOf(config.Client), "CopyContainerFileToLocal",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, src, dst string,
			podName ...string) ([]byte, error) {
			return nil, nil
		})

	// act
	gotErr := mockCollect.GetHostInformation(namespace, containerName, nodeName, podName)

	// assert
	if gotErr != nil {
		t.Errorf("TestBaseFileLogsCollect_GetHostInformation_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestFileLogsCollector_GetFileLogs_ExecContainerFail(t *testing.T) {
	// arrange
	mockCollect := &FileLogsCollector{}
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	fileLogPath := "/fileLogPath"
	execErr := fmt.Errorf("exec cmd error")
	wantErr := execErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, execErr
		})

	// act
	gotErr := mockCollect.GetFileLogs(namespace, podName, containerName, fileLogPath)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestFileLogsCollector_GetFileLogs_ExecContainerFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestFileLogsCollector_GetFileLogs_Success(t *testing.T) {
	// arrange
	mockCollect := &FileLogsCollector{}
	namespace := "namespace1"
	podName := "pod1"
	containerName := "container1"
	fileLogPath := "/fileLogPath"

	// mock
	p := gomonkey.NewPatches()
	p.ApplyMethod(reflect.TypeOf(config.Client), "ExecCmdInSpecifiedContainer",
		func(_ *client.KubernetesCLI, ctx context.Context, namespace, containerName, cmd string,
			podName ...string) ([]byte, error) {
			return nil, nil
		})

	// act
	gotErr := mockCollect.GetFileLogs(namespace, podName, containerName, fileLogPath)

	// assert
	if gotErr != nil {
		t.Errorf("TestFileLogsCollector_GetFileLogs_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}
