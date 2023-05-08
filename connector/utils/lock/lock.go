/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

// Package lock provide Lock and Unlock when manage the disk
package lock

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// Lock is a special lock interface provided for disk management
type Lock interface {
	SyncLock(lockName string)

	SyncUnlock(lockName string)
}

var (
	connectorThreads = flag.Int("connector-threads",
		4,
		"The concurrency supported during disk operations.")
	curDriverName string
	lockMutex     sync.Mutex
	semaphoreMap  map[string]*utils.Semaphore
	lockDir       = fmt.Sprintf("%s/lock/", lockDirPrefix)
)

const (
	connectVolume    = "connect"
	disConnectVolume = "disConnect"
	extendVolume     = "extend"
	lockNamePrefix   = "hw-pvc-lock-"
	lockDirPrefix    = "/csi"

	filePermission     = 0644
	dirPermission      = 0755
	minThreads         = 1
	maxThreads         = 10
	getLockInternalSec = 5

	// GetLockTimeoutSec is the maximum number of seconds to acquire a lock
	GetLockTimeoutSec = 30
	// GetSemaphoreTimeout is used to determine whether the acquisition of semaphore is time out
	GetSemaphoreTimeout = "get semaphore timeout"
	// GetLockTimeout is used to determine whether the acquisition of semaphore is time out
	GetLockTimeout = "get lock timeout"
)

// InitLock provide three semaphores for device connect, disconnect and expand
func InitLock(driverName string) error {
	err := checkConnectorThreads(context.Background())
	if err != nil {
		return err
	}

	err = checkLockPath(lockDir)
	if err != nil {
		return err
	}

	semaphoreMap = map[string]*utils.Semaphore{
		connectVolume:    utils.NewSemaphore(*connectorThreads),
		disConnectVolume: utils.NewSemaphore(*connectorThreads),
		extendVolume:     utils.NewSemaphore(*connectorThreads),
	}
	curDriverName = driverName
	log.Infoln("Init lock success.")
	return nil
}

// SyncLock provide lock for device connect, disconnect and expand
func SyncLock(ctx context.Context, lockName, operationType string) error {
	startTime := time.Now()

	err := createLockDir(filepath.Dir(lockDir))
	if err != nil {
		return fmt.Errorf("create dir failed, reason: %s", err)
	}

	err = waitGetLock(ctx, lockDir, lockName)
	if err != nil {
		return err
	}

	err = waitGetSemaphore(ctx, operationType)
	if err != nil {
		newErr := deleteLockFile(ctx, lockDir, lockName)
		if newErr != nil {
			log.AddContext(ctx).Errorln("new error occurred when delete lock file")
			return newErr
		}
		return err
	}

	log.AddContext(ctx).Infof("It took %s to acquire %s lock for %s.", time.Since(startTime), operationType, lockName)
	return nil
}

// SyncUnlock provide unlock for device connect, disconnect and expand
func SyncUnlock(ctx context.Context, lockName, operationType string) error {
	startTime := time.Now()
	releaseSemaphore(ctx, operationType)

	err := deleteLockFile(ctx, lockDir, lockName)
	if err != nil {
		return err
	}

	log.AddContext(ctx).Infof("It took %s to release %s lock for %s.", time.Since(startTime), operationType, lockName)
	return nil
}
