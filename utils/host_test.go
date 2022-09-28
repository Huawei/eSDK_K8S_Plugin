/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.
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
	"os"
	"path"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"huawei-csi-driver/utils/log"
)

func TestChmodFsPermission(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Errorf("Get current directory failed.")
	}

	targetPath := path.Join(currentDir, "fsPermissionTest.txt")
	_, err = os.Create(targetPath)
	if err != nil {
		log.Errorf("Create file/directory [%s] failed.", targetPath)
	}

	defer func() {
		err := os.Remove(targetPath)
		if err != nil {
			log.Errorf("Remove file/directory [%s] failed.", targetPath)
		}
	}()

	Convey("Change target directory to 777 permission", t, func() {
		ChmodFsPermission(context.TODO(), targetPath, "777")
		fileInfo, err := os.Stat(targetPath)
		if err != nil {
			log.Errorf("Get file/directory [%s] info failed.", targetPath)
			So(err, ShouldBeNil)
		}
		filePerm := fileInfo.Mode().Perm()
		So(filePerm, ShouldEqual, os.FileMode(0777))
	})

	Convey("Change target directory to 555 permission", t, func() {
		ChmodFsPermission(context.TODO(), targetPath, "555")
		fileInfo, err := os.Stat(targetPath)
		if err != nil {
			log.Errorf("Get file/directory [%s] info failed.", targetPath)
			So(err, ShouldBeNil)
		}
		filePerm := fileInfo.Mode().Perm()
		So(filePerm, ShouldEqual, os.FileMode(0555))
	})

	Convey("Change target directory to 000 permission", t, func() {
		ChmodFsPermission(context.TODO(), targetPath, "000")
		fileInfo, err := os.Stat(targetPath)
		if err != nil {
			log.Errorf("Get file/directory [%s] info failed.", targetPath)
			So(err, ShouldBeNil)
		}
		filePerm := fileInfo.Mode().Perm()
		So(filePerm, ShouldEqual, os.FileMode(0000))
	})

	Convey("Change target directory to 456 permission", t, func() {
		ChmodFsPermission(context.TODO(), targetPath, "456")
		fileInfo, err := os.Stat(targetPath)
		if err != nil {
			log.Errorf("Get file/directory [%s] info failed.", targetPath)
			So(err, ShouldBeNil)
		}
		filePerm := fileInfo.Mode().Perm()
		So(filePerm, ShouldEqual, os.FileMode(0456))
	})
}
