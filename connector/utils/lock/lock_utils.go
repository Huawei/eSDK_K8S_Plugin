/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package lock provide Lock and Unlock when manage the disk
package lock

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

func checkConnectorThreads(ctx context.Context) error {
	if *connectorThreads < minThreads || *connectorThreads > maxThreads {
		return utils.Errorf(ctx, "the connector-threads %d should be %d~%d",
			*connectorThreads, minThreads, maxThreads)
	}
	return nil
}

func clearLockFile(fileDir string) error {
	files, err := ioutil.ReadDir(fileDir)
	if err != nil {
		return fmt.Errorf("can't read lock file directory: %s", err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		fileName := f.Name()
		if !strings.HasPrefix(fileName, lockNamePrefix) {
			continue
		}

		err := os.Remove(filepath.Join(fileDir, fileName))
		if err != nil {
			return fmt.Errorf("failed to remove current lock file [%s]. %s", fileName, err)
		}
	}
	return nil
}

func createLockDir(lockPathRootDir string) error {
	dir, err := os.Lstat(lockPathRootDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(lockPathRootDir, dirPermission); err != nil {
			return fmt.Errorf("could not create lock directory %v. %v", lockPathRootDir, err)
		}
	}

	if dir != nil && !dir.IsDir() {
		return fmt.Errorf("lock path %v exists and is not a directory, please remove it", lockPathRootDir)
	}
	return nil
}

func checkLockPath(lockDir string) error {
	lockPathRootDir := filepath.Dir(lockDir)
	err := createLockDir(lockPathRootDir)
	if err != nil {
		return fmt.Errorf("create dir failed, reason: %s", err)
	}

	err = clearLockFile(lockPathRootDir)
	if err != nil {
		return fmt.Errorf("clear file failed, reason: %s", err)
	}
	return nil
}

func isFileExist(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

func createLockFile(ctx context.Context, filePath, lockName string) error {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, filePermission)
	if err != nil {
		return err
	}
	defer func() {
		err = file.Close()
		if err != nil {
			log.AddContext(ctx).Errorln("close file handle failed")
		}
	}()

	_, err = file.WriteString(lockName)
	return err
}

func deleteLockFile(ctx context.Context, lockDir, lockName string) error {
	log.AddContext(ctx).Infoln("DeleteLockFile start to get lock")
	lockMutex.Lock()
	defer lockMutex.Unlock()
	log.AddContext(ctx).Infoln("DeleteLockFile finish to get lock")
	filePath := fmt.Sprintf("%s%s%s", lockDir, lockNamePrefix, lockName)
	exist := isFileExist(filePath)
	if !exist {
		return nil
	}

	return os.Remove(filePath)
}

func waitGetLock(ctx context.Context, lockDir, lockName string) error {
	filePath := fmt.Sprintf("%s%s%s", lockDir, lockNamePrefix, lockName)
	log.AddContext(ctx).Infoln("WaitGetLock start to get lock")
	err := utils.WaitUntil(func() (bool, error) {
		lockMutex.Lock()
		defer lockMutex.Unlock()
		exist := isFileExist(filePath)

		if !exist {
			err := createLockFile(ctx, filePath, lockName)
			if err == nil {
				return true, nil
			}
		}
		return false, nil
	}, time.Second*GetLockTimeoutSec, time.Second*getLockInternalSec)
	log.AddContext(ctx).Infoln("WaitGetLock finish to get lock")
	if err != nil {
		newErr := deleteLockFile(ctx, lockDir, lockName)
		if newErr != nil {
			log.AddContext(ctx).Errorln("new error occurred when delete lock file")
			return newErr
		}
		if strings.Contains(err.Error(), "timeout") {
			return fmt.Errorf("%s, lock file path: [%s]", GetLockTimeout, filePath)
		}
		return err
	}
	return nil
}

func acquireSemaphore(ctx context.Context, operationType string) bool {
	semaphore, exist := semaphoreMap[operationType]
	if !exist {
		log.AddContext(ctx).Errorf("Acquire semaphore type: %s not exist in %v.", operationType, semaphoreMap)
		return false
	}

	log.AddContext(ctx).Infof("Before acquire, available permits is %d", semaphore.AvailablePermits())
	semaphore.Acquire()
	log.AddContext(ctx).Infof("After acquire, available permits is %d", semaphore.AvailablePermits())
	return true
}

func releaseSemaphore(ctx context.Context, operationType string) {
	semaphore, exist := semaphoreMap[operationType]
	if !exist {
		log.AddContext(ctx).Warningf("unsupport operation type %s", operationType)
	}
	log.AddContext(ctx).Infof("Before release, available permits is %d", semaphore.AvailablePermits())
	semaphore.Release()
	log.AddContext(ctx).Infof("After release, available permits is %d", semaphore.AvailablePermits())
}

func waitGetSemaphore(ctx context.Context, cancel context.CancelFunc, operationType string) error {
	go func(ctx context.Context, cancel context.CancelFunc, operationType string) {
		defer cancel()

		acquireSemaphore(ctx, operationType)
		select {
		case <-ctx.Done():
			log.AddContext(ctx).Warningf("acquire [%s] semaphore timeout, release this semaphore.", operationType)
			releaseSemaphore(ctx, operationType)
		default:
			log.AddContext(ctx).Debugf("acquire [%s] semaphore finish.", operationType)
		}
	}(ctx, cancel, operationType)

	select {
	case <-ctx.Done():
		if strings.Contains(ctx.Err().Error(), "context deadline exceeded") {
			// If the exit is due to timeout, "context deadline exceeded" is contains in ctx.Err().Error()
			return utils.Errorf(ctx, "%s, operation type: [%s] .", GetSemaphoreTimeout, operationType)
		}

		log.AddContext(ctx).Debugf("get [%s] semaphore finish.", operationType)
		return nil
	}
}
