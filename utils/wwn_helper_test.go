/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package utils

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"huawei-csi-driver/utils/log"
)

const (
	testVolumeId  = "backend_name.pvc-xxx.wwn"
	testVolumeWwn = "600000000"
)

var testWwnFileName = fmt.Sprintf("%s/%s.wwn", defaultWwnFileDir, testVolumeId)

func TestCreateWwnDir(t *testing.T) {
	defer cleanMockFile()

	if err := createWwnDir(context.Background()); err != nil {
		t.Errorf("TestCreateWwnDir() failed, error: %v", err)
	}
}

func TestCreateWwnDirWhenPathExistFile(t *testing.T) {
	defer cleanMockFile()

	if err := os.MkdirAll(path.Dir(defaultWwnFileDir), defaultWwnFilePermission); err != nil {
		t.Errorf("create dir failed, err: %v", err)
	}

	if err := ioutil.WriteFile(defaultWwnFileDir, []byte{}, defaultWwnFilePermission); err != nil {
		t.Errorf("write file failed, err: %v", err)
	}

	if err := createWwnDir(context.Background()); err == nil {
		t.Errorf("TestCreateWwnDirWhenPathExistFile() want an path exist error but got error: %v", err)
	}
}

func TestWriteWwnFile(t *testing.T) {
	defer cleanMockFile()

	if err := WriteWWNFile(context.Background(), testVolumeWwn, testVolumeId); err != nil {
		t.Errorf("TestWriteWwnFile() write wwn file error: %v", err)
	}

	wwn, err := ioutil.ReadFile(testWwnFileName)
	if err != nil {
		t.Errorf("TestWriteWwnFile() read wwn file error: %v", err)
	}

	if string(wwn) != testVolumeWwn {
		t.Errorf("TestWriteWwnFile() failed, want: %s, got: %s", testVolumeWwn, string(wwn))
	}
}

func TestWriteWwnFileButCreateDirFail(t *testing.T) {
	mockError := errors.New("create dir error")
	createWwnDir := gomonkey.ApplyFunc(createWwnDir, func(ctx context.Context) error {
		return mockError
	})
	defer createWwnDir.Reset()

	err := WriteWWNFile(context.Background(), testVolumeWwn, testVolumeId)
	if err == nil || err.Error() != "create dir error" {
		t.Errorf("TestWriteWwnFileButCreateDirFail() want: %v, got: %v", mockError, err)
	}
}

func TestReadWwnFileSuccess(t *testing.T) {
	defer cleanMockFile()

	if err := mockWriteWwnFile(); err != nil {
		t.Errorf("TestReadWwnFileSuccess() mock write wwn file error: %v", err)
	}

	wwn, err := ReadWwnFile(context.Background(), testVolumeId)
	if err != nil {
		t.Errorf("TestReadWwnFileSuccess() read wwn file error: %v", err)
	}

	if wwn != testVolumeWwn {
		t.Errorf("TestReadWwnFileSuccess() failed, want: %s, got: %s", testVolumeWwn, wwn)
	}
}

func TestReadWwnFileFail(t *testing.T) {
	defer cleanMockFile()

	if err := mockWriteWwnFile(); err != nil {
		t.Errorf("TestReadWwnFileFail() mock write wwn file error: %v", err)
	}

	mockError := errors.New("read file error")
	readFile := gomonkey.ApplyFunc(os.ReadFile, func(string) ([]byte, error) {
		return nil, mockError
	})
	defer readFile.Reset()

	_, err := ReadWwnFile(context.Background(), testVolumeId)
	if err == nil {
		t.Errorf("TestReadWwnFileFail() want: %s, got: %v", mockError, err)
	}
}

func TestReadWwnFileButFileNotExist(t *testing.T) {
	_, err := ReadWwnFile(context.Background(), testVolumeId)
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("TestReadWwnFileButFileNotExist() want got an file not exist error "+
			"but got error: %v", err)
	}
}

func TestRemoveWwnFileWhenFileExist(t *testing.T) {
	defer cleanMockFile()

	if err := mockWriteWwnFile(); err != nil {
		t.Errorf("TestRemoveWwnFileWhenFileExist() mock write wwn file error: %v", err)
	}

	if err := RemoveWwnFile(context.Background(), testVolumeId); err != nil {
		t.Errorf("TestRemoveWwnFileWhenFileExist() remove wwn file error: %v", err)
	}

	_, err := ioutil.ReadFile(testWwnFileName)
	if err == nil {
		t.Errorf("TestRemoveWwnFileWhenFileExist() want got a file is not exist error but got error: %v", err)
	}
}

func TestRemoveWwnFileWhenFileNotExist(t *testing.T) {
	if err := RemoveWwnFile(context.Background(), testVolumeId); err != nil {
		t.Errorf("TestRemoveWwnFileWhenFileNotExist() remove wwn file error: %v", err)
	}
}

func mockWriteWwnFile() error {
	if err := os.MkdirAll(defaultWwnFileDir, defaultWwnDirPermission); err != nil {
		log.Errorf("mock write wwn file failed, error: %v", err)
		return err
	}

	return ioutil.WriteFile(testWwnFileName, []byte(testVolumeWwn), defaultWwnFilePermission)
}

func cleanMockFile() {
	err := os.RemoveAll(defaultWwnFileDir)
	if err != nil {
		log.Errorf("clean mock file failed, error: %v", err)
		return
	}
}
