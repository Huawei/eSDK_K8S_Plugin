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
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	coreV1 "k8s.io/api/core/v1"
)

func TestLogs_initialize_Success(t *testing.T) {
	// arrange
	lg := &Logs{resource: &Resource{&ResourceBuilder{
		namespace:      "namespace1",
		nodeName:       "node1",
		maxNodeThreads: 50,
	}}}

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(initFun, func() {
		checkNamespaceExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		checkNodeExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		getPodListFun = func(ctx context.Context, ns string, node string,
			objectName ...string) (coreV1.PodList, error) {
			return coreV1.PodList{}, nil
		}
	})

	// act
	gotErr := lg.initialize()

	// assert
	if gotErr != nil {
		t.Errorf("Test_initialize_Success failed, gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestLogs_initialize_ConcurrencyCheckFail(t *testing.T) {
	// arrange
	lg := &Logs{resource: &Resource{&ResourceBuilder{
		namespace:      "namespace1",
		nodeName:       "node1",
		maxNodeThreads: 0,
	}}}
	checkErr := fmt.Errorf("threads-max must in range [1~1000]")
	wantErr := checkErr

	// act
	gotErr := lg.initialize()

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestLogs_initialize_ConcurrencyCheckFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}
}

func TestLogs_initialize_NamespaceCheckFail(t *testing.T) {
	// arrange
	lg := &Logs{resource: &Resource{&ResourceBuilder{
		namespace:      "namespace1",
		nodeName:       "node1",
		maxNodeThreads: 50,
	}}}
	checkErr := fmt.Errorf("check namespace error")
	wantErr := checkErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(initFun, func() {
		checkNamespaceExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return false, checkErr
		}
	})

	// act
	gotErr := lg.initialize()

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_initialize_NamespaceCheckFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestLogs_initialize_NodeCheckFail(t *testing.T) {
	// arrange
	lg := &Logs{resource: &Resource{&ResourceBuilder{
		namespace:      "namespace1",
		nodeName:       "node1",
		maxNodeThreads: 50,
	}}}
	checkErr := fmt.Errorf("check node error")
	wantErr := checkErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(initFun, func() {
		checkNamespaceExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		checkNodeExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return false, checkErr
		}
	})

	// act
	gotErr := lg.initialize()

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_initialize_NodeCheckFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestLogs_initialize_GetPodListFail(t *testing.T) {
	// arrange
	lg := &Logs{resource: &Resource{&ResourceBuilder{
		namespace:      "namespace1",
		nodeName:       "node1",
		maxNodeThreads: 50,
	}}}
	getErr := fmt.Errorf("get pod list error")
	wantErr := getErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(initFun, func() {
		checkNamespaceExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		checkNodeExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		getPodListFun = func(ctx context.Context, ns string, node string,
			objectName ...string) (coreV1.PodList, error) {
			return coreV1.PodList{}, getErr
		}
	})

	// act
	gotErr := lg.initialize()

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_initialize_GetPodListFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestLogs_Collect_Success(t *testing.T) {
	// arrange
	lg := &Logs{
		resource: &Resource{&ResourceBuilder{
			namespace:      "namespace1",
			nodeName:       "node1",
			maxNodeThreads: 50,
		}},
		nodePodList: map[string][]coreV1.Pod{"node1": {coreV1.Pod{}}},
	}

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(initFun, func() {
		checkNamespaceExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		checkNodeExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		getPodListFun = func(ctx context.Context, ns string, node string,
			objectName ...string) (coreV1.PodList, error) {
			return coreV1.PodList{}, nil
		}
	}).ApplyFunc(createNodeLogsPath, func(nodeList map[string][]coreV1.Pod) error {
		return nil
	}).ApplyFunc(compressLocalLogs, func(nodeList map[string][]coreV1.Pod, fileName string) error {
		return nil
	}).ApplyFunc(deleteLocalLogsFile, func() error {
		return nil
	})

	// act
	gotErr := lg.Collect()

	// assert
	if gotErr != nil {
		t.Errorf("Test_Collect_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestLogs_Collect_InitFail(t *testing.T) {
	// arrange
	lg := &Logs{
		resource: &Resource{&ResourceBuilder{
			namespace:      "namespace1",
			nodeName:       "node1",
			maxNodeThreads: 50,
		}},
		nodePodList: map[string][]coreV1.Pod{"node1": {coreV1.Pod{}}},
	}
	initErr := fmt.Errorf("init error")
	wantErr := initErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(initFun, func() {
		checkNamespaceExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return false, initErr
		}
	})

	// act
	gotErr := lg.Collect()

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_Collect_InitFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestLogs_Collect_createPathFail(t *testing.T) {
	// arrange
	lg := &Logs{
		resource: &Resource{&ResourceBuilder{
			namespace:      "namespace1",
			nodeName:       "node1",
			maxNodeThreads: 50,
		}},
		nodePodList: map[string][]coreV1.Pod{"node1": {coreV1.Pod{}}},
	}
	createPathErr := fmt.Errorf("create path error")
	wantErr := createPathErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(initFun, func() {
		checkNamespaceExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		checkNodeExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		getPodListFun = func(ctx context.Context, ns string, node string,
			objectName ...string) (coreV1.PodList, error) {
			return coreV1.PodList{}, nil
		}
	}).ApplyFunc(createNodeLogsPath, func(nodeList map[string][]coreV1.Pod) error {
		return createPathErr
	})

	// act
	gotErr := lg.Collect()

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_Collect_createPathFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func TestLogs_Collect_compressFail(t *testing.T) {
	// arrange
	lg := &Logs{
		resource: &Resource{&ResourceBuilder{
			namespace:      "namespace1",
			nodeName:       "node1",
			maxNodeThreads: 50,
		}},
		nodePodList: map[string][]coreV1.Pod{"node1": {coreV1.Pod{}}},
	}
	compressErr := fmt.Errorf("compress logs error")
	wantErr := compressErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(initFun, func() {
		checkNamespaceExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		checkNodeExistFun = func(ctx context.Context, ns string, node string, objectName string) (bool, error) {
			return true, nil
		}
		getPodListFun = func(ctx context.Context, ns string, node string,
			objectName ...string) (coreV1.PodList, error) {
			return coreV1.PodList{}, nil
		}
	}).ApplyFunc(createNodeLogsPath, func(nodeList map[string][]coreV1.Pod) error {
		return nil
	}).ApplyFunc(compressLocalLogs, func(nodeList map[string][]coreV1.Pod, fileName string) error {
		return compressErr
	}).ApplyFunc(deleteLocalLogsFile, func() error {
		return nil
	})

	// act
	gotErr := lg.Collect()

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_Collect_compressFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_createNodeLogsPath_Success(t *testing.T) {
	// arrange
	nodeList := map[string][]coreV1.Pod{"node1": {coreV1.Pod{}}}

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
		return nil
	})

	// act
	gotErr := createNodeLogsPath(nodeList)

	// assert
	if gotErr != nil {
		t.Errorf("Test_createNodeLogsPath_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_createNodeLogsPath_Fail(t *testing.T) {
	// arrange
	nodeList := map[string][]coreV1.Pod{"node1": {coreV1.Pod{}}}
	mkdirErr := fmt.Errorf("mkdir error")
	wantErr := mkdirErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
		return wantErr
	})

	// act
	gotErr := createNodeLogsPath(nodeList)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_createNodeLogsPath_Fail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_deleteLocalLogsFile_Success(t *testing.T) {
	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.RemoveAll, func(path string) error {
		return nil
	})

	// act
	gotErr := deleteLocalLogsFile()

	// assert
	if gotErr != nil {
		t.Errorf("Test_deleteLocalLogsFile_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_deleteLocalLogsFile_Fail(t *testing.T) {
	// arrange
	removeErr := fmt.Errorf("remove error")
	wantErr := removeErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.RemoveAll, func(path string) error {
		return removeErr
	})

	// act
	gotErr := deleteLocalLogsFile()

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_deleteLocalLogsFile_Fail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_zipMultiFiles_Success(t *testing.T) {
	// arrange
	zipPath := "/tmp"
	filePaths := []string{"file1", "file2"}
	mockFile := &os.File{}
	mockWriter := &zip.Writer{}

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
		return nil
	}).ApplyFunc(os.OpenFile, func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return mockFile, nil
	}).ApplyFunc(zip.NewWriter, func(w io.Writer) *zip.Writer {
		return mockWriter
	}).ApplyFunc(filepath.Walk, func(root string, fn filepath.WalkFunc) error {
		return nil
	}).ApplyMethod(reflect.TypeOf(mockFile), "Close", func(_ *os.File) error {
		return nil
	}).ApplyMethod(reflect.TypeOf(mockWriter), "Close", func(_ *zip.Writer) error {
		return nil
	})

	// act
	gotErr := zipMultiFiles(zipPath, filePaths...)

	// assert
	if gotErr != nil {
		t.Errorf("Test_zipMultiFiles_Success failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, nil)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_zipMultiFiles_MkdirFail(t *testing.T) {
	// arrange
	zipPath := "/tmp"
	filePaths := []string{"file1", "file2"}
	mkdirErr := fmt.Errorf("mkdir error")
	wantErr := mkdirErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
		return mkdirErr
	})

	// act
	gotErr := zipMultiFiles(zipPath, filePaths...)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_zipMultiFiles_MkdirFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_zipMultiFiles_OpenFileFail(t *testing.T) {
	// arrange
	zipPath := "/tmp"
	filePaths := []string{"file1", "file2"}
	openFileErr := fmt.Errorf("open file error")
	wantErr := openFileErr

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
		return nil
	}).ApplyFunc(os.OpenFile, func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return nil, openFileErr
	})

	// act
	gotErr := zipMultiFiles(zipPath, filePaths...)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_zipMultiFiles_OpenFileFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}

func Test_zipMultiFiles_walkFuncFail(t *testing.T) {
	// arrange
	zipPath := "/tmp"
	filePaths := []string{"file1", "file2"}
	walkFuncErr := fmt.Errorf("walk func error")
	wantErr := walkFuncErr
	mockFile := &os.File{}
	mockWriter := &zip.Writer{}

	// mock
	p := gomonkey.NewPatches()
	p.ApplyFunc(os.MkdirAll, func(path string, perm os.FileMode) error {
		return nil
	}).ApplyFunc(os.OpenFile, func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return mockFile, nil
	}).ApplyFunc(zip.NewWriter, func(w io.Writer) *zip.Writer {
		return mockWriter
	}).ApplyFunc(filepath.Walk, func(root string, fn filepath.WalkFunc) error {
		return walkFuncErr
	}).ApplyMethod(reflect.TypeOf(mockFile), "Close", func(_ *os.File) error {
		return nil
	}).ApplyMethod(reflect.TypeOf(mockWriter), "Close", func(_ *zip.Writer) error {
		return nil
	})

	// act
	gotErr := zipMultiFiles(zipPath, filePaths...)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("Test_zipMultiFiles_walkFuncFail failed, "+
			"gotErr [%v], wantErr [%v]", gotErr, wantErr)
	}

	// cleanup
	t.Cleanup(func() {
		p.Reset()
	})
}
