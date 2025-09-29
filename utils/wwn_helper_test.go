/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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
	"reflect"
	"strings"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
)

const (
	testVolumeId  = "backend_name.pvc-xxx.wwn"
	testVolumeWwn = "600000000"
)

var testWwnFileName = fmt.Sprintf("%s/%s.wwn", defaultWwnFileDir, testVolumeId)

func TestCreateWwnDir(t *testing.T) {
	// mock
	p := gomonkey.ApplyFuncReturn(os.MkdirAll, nil)
	defer p.Reset()

	// act, assert
	if err := createWwnDir(context.Background()); err != nil {
		t.Errorf("TestCreateWwnDir() failed, error: %v", err)
	}
}

func TestCreateWwnDirWhenPathFileNil(t *testing.T) {
	// mock
	p := gomonkey.ApplyFuncReturn(os.Lstat, nil, nil)
	defer p.Reset()

	// act, assert
	if err := createWwnDir(context.Background()); err != nil {
		t.Errorf("TestCreateWwnDirWhenPathFileNil() failed, error: %v", err)
	}
}

func TestWriteWwnFile(t *testing.T) {
	// mock
	p := gomonkey.ApplyFuncReturn(os.MkdirAll, nil).
		ApplyFuncReturn(ioutil.WriteFile, nil)
	defer p.Reset()

	// act, assert
	if err := WriteWWNFile(context.Background(), testVolumeWwn, testVolumeId); err != nil {
		t.Errorf("TestWriteWwnFile() write wwn file error: %v", err)
	}
}

func TestWriteWwnFileButCreateDirFail(t *testing.T) {
	mockError := errors.New("create dir error")
	p := gomonkey.ApplyFunc(createWwnDir, func(ctx context.Context) error {
		return mockError
	})
	defer p.Reset()

	err := WriteWWNFile(context.Background(), testVolumeWwn, testVolumeId)
	if err == nil || err.Error() != "create dir error" {
		t.Errorf("TestWriteWwnFileButCreateDirFail() want: %v, got: %v", mockError, err)
	}
}

func TestReadWwnFileSuccess(t *testing.T) {
	// mock
	p := gomonkey.ApplyFuncReturn(os.Stat, nil, nil).
		ApplyFuncReturn(ioutil.ReadFile, []byte(testVolumeWwn), nil)
	defer p.Reset()

	// act
	wwn, err := ReadWwnFile(context.Background(), testVolumeId)

	// assert
	if err != nil {
		t.Errorf("TestReadWwnFileSuccess() read wwn file error: %v", err)
	}

	if wwn != testVolumeWwn {
		t.Errorf("TestReadWwnFileSuccess() failed, want: %s, got: %s", testVolumeWwn, wwn)
	}
}

func TestReadWwnFileFail(t *testing.T) {
	// mock
	wantErr := errors.New("read file error")
	p := gomonkey.ApplyFuncReturn(os.Stat, nil, nil).
		ApplyFuncReturn(ioutil.ReadFile, nil, wantErr)
	defer p.Reset()

	// act, assert
	_, err := ReadWwnFile(context.Background(), testVolumeId)
	if !reflect.DeepEqual(wantErr, err) {
		t.Errorf("TestReadWwnFileFail() want: %s, got: %v", wantErr, err)
	}
}

func TestReadWwnFileButFileNotExist(t *testing.T) {
	_, err := ReadWwnFile(context.Background(), testVolumeId)
	if err == nil || !strings.Contains(err.Error(), "not found wwn file") {
		t.Errorf("TestReadWwnFileButFileNotExist() want got an file not exist error "+
			"but got error: %v", err)
	}
}

func TestRemoveWwnFileWhenFileExist(t *testing.T) {
	// mock
	p := gomonkey.ApplyFuncReturn(os.Stat, nil, nil).
		ApplyFuncReturn(os.Remove, nil)
	defer p.Reset()

	// act, assert
	if err := RemoveWwnFile(context.Background(), testVolumeId); err != nil {
		t.Errorf("TestRemoveWwnFileWhenFileExist() remove wwn file error: %v", err)
	}
}

func TestRemoveWwnFileWhenFileNotExist(t *testing.T) {
	if err := RemoveWwnFile(context.Background(), testVolumeId); err != nil {
		t.Errorf("TestRemoveWwnFileWhenFileNotExist() remove wwn file error: %v", err)
	}
}
