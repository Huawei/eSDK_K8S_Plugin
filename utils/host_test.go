/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.
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

	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const FilePermission0777 = os.FileMode(0777)

func TestChmodFsPermission(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Errorf("Get current directory failed.")
	}

	targetPath := path.Join(currentDir, "fsPermissionTest.txt")
	targetFile, err := os.Create(targetPath)
	if err != nil {
		log.Errorf("Create file/directory [%s] failed.", targetPath)
	}
	defer func() {
		if err := targetFile.Close(); err != nil {
			t.Errorf("close file %s failed, error: %v", targetFile.Name(), err)
		}
	}()
	err = targetFile.Chmod(FilePermission0777)
	if err != nil {
		log.Errorf("file targetFile chmod to 0600 failed, error: %v", err)
	}

	defer func() {
		err := os.Remove(targetPath)
		if err != nil {
			log.Errorf("Remove file/directory [%s] failed.", targetPath)
		}
	}()

	t.Run("Change target directory to 777 permission", func(t *testing.T) {
		ChmodFsPermission(context.TODO(), targetPath, "777")
		fileInfo, err := os.Stat(targetPath)
		require.NoError(t, err)

		filePerm := fileInfo.Mode().Perm()
		require.Equal(t, os.FileMode(0777), filePerm)
	})

	t.Run("Change target directory to 555 permission", func(t *testing.T) {
		ChmodFsPermission(context.TODO(), targetPath, "555")
		fileInfo, err := os.Stat(targetPath)
		require.NoError(t, err)

		filePerm := fileInfo.Mode().Perm()
		require.Equal(t, os.FileMode(0555), filePerm)
	})

	t.Run("Change target directory to 000 permission", func(t *testing.T) {
		ChmodFsPermission(context.TODO(), targetPath, "000")
		fileInfo, err := os.Stat(targetPath)
		require.NoError(t, err)

		filePerm := fileInfo.Mode().Perm()
		require.Equal(t, os.FileMode(0000), filePerm)
	})

	t.Run("Change target directory to 456 permission", func(t *testing.T) {
		ChmodFsPermission(context.TODO(), targetPath, "456")
		fileInfo, err := os.Stat(targetPath)
		require.NoError(t, err)

		filePerm := fileInfo.Mode().Perm()
		require.Equal(t, os.FileMode(0456), filePerm)
	})
}
