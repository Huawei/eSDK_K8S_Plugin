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
	"path/filepath"
	"strings"

	"huawei-csi-driver/utils/log"
)

const (
	defaultWwnFileDir        = "/csi/disks"
	defaultWwnDirPermission  = 0700
	defaultWwnFilePermission = 0600
	defaultWwnFileLength     = 64
)

// WriteWWNFile write the wwn info for use in unstage call.
func WriteWWNFile(ctx context.Context, wwn, volumeId string) error {
	if err := createWwnDir(ctx); err != nil {
		return err
	}
	wwnName := buildWwnFileName(volumeId)
	wwnFileName := buildWwnFilePath(wwnName)
	err := ioutil.WriteFile(wwnFileName, []byte(wwn), defaultWwnFilePermission)
	if err != nil {
		log.AddContext(ctx).Errorf("write wwn file error, fileName: %s, error: %v", wwnFileName, err)
		return err
	}
	return nil
}

// WriteWWNFileIfNotExist write new's wwn to file if the file doesn't exist in disk,
// or wwn file exist but content is empty.
func WriteWWNFileIfNotExist(ctx context.Context, wwn, volumeId string) error {
	wwnFromDisk, err := ReadWwnFile(ctx, volumeId)
	if err != nil || wwnFromDisk == "" {
		return WriteWWNFile(ctx, wwn, volumeId)
	}
	return nil
}

// ReadWwnFile read the wwn info file.
func ReadWwnFile(ctx context.Context, volumeId string) (string, error) {
	log.AddContext(ctx).Infof("start to read wwn file, volumeId: %s", volumeId)
	wwnFile, err := statWwnFile(ctx, volumeId)
	if err != nil {
		return "", err
	}
	wwnBytes, err := ioutil.ReadFile(wwnFile)
	if err != nil {
		log.AddContext(ctx).Warningf("read wwn file failed, volumeId: %s, error: %v",
			volumeId, err)
		return "", err
	}
	return string(wwnBytes), nil
}

// RemoveWwnFile remove the wwn info file.
func RemoveWwnFile(ctx context.Context, volumeId string) error {
	wwnFilePath, err := statWwnFile(ctx, volumeId)
	if err != nil {
		if strings.Contains(err.Error(), "not found wwn file") {
			return nil
		}
		return err
	}

	err = os.Remove(wwnFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.AddContext(ctx).Errorf("remove wwn file error, volumeId: %s, error: %v",
			volumeId, err)
		return err
	}
	return nil
}

func createWwnDir(ctx context.Context) error {
	dir, err := os.Lstat(defaultWwnFileDir)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(defaultWwnFileDir, defaultWwnDirPermission); err != nil {
			log.AddContext(ctx).Errorf("create wwn directory failed, dirPath: %s, error: %v",
				defaultWwnFileDir, err)
			return err
		}
	}

	if dir != nil && !dir.IsDir() {
		return fmt.Errorf("path %v exists but it is not a directory, please remove it", defaultWwnFileDir)
	}
	return nil
}

func buildWwnFilePath(volumeId string) string {
	return fmt.Sprintf("%s/%s.wwn", defaultWwnFileDir, volumeId)
}

func buildWwnFileName(volumeId string) string {
	if len(volumeId) > defaultWwnFileLength {
		volumeId = volumeId[len(volumeId)-64:]
	}
	return volumeId
}

func statWwnFile(ctx context.Context, volumeId string) (string, error) {
	wwnName := buildWwnFileName(volumeId)
	wwnFilePath := buildWwnFilePath(wwnName)
	oldWwnFilePath := buildWwnFilePath(volumeId)

	for _, filePath := range []string{wwnFilePath, oldWwnFilePath} {
		_, err := os.Stat(filePath)
		if err == nil {
			return filePath, nil
		}

		if err != nil && os.IsNotExist(err) {
			continue
		}

		if err != nil && !os.IsNotExist(err) {
			log.AddContext(ctx).Warningf("stat wwn file failed, volumeId: %s, error: %v",
				filepath.Base(filePath), err)
			return "", err
		}
	}
	return "", errors.New("not found wwn file")
}
