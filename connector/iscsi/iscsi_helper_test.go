/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

package iscsi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils/lock"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName           = "iscsiTest.log"
	defaultDriverName = "csi.huawei.com"
)

func TestMain(m *testing.M) {
	stubs := gostub.StubFunc(&app.GetGlobalConfig, config.MockCompletedConfig())
	defer stubs.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	if err := lock.InitLock(defaultDriverName); err != nil {
		log.Errorf("test lock init failed: %v", err)
		return
	}

	m.Run()
}

func Test_findDiskOfUltraPath_Timeout(t *testing.T) {
	// arrange
	mockShareData := &shareData{}
	mockShareData.stoppedThreads.Store(1)

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(connector.GetDiskNameByWWN, "", errors.New("mock error")).
		ApplyFuncReturn(time.Sleep).ApplyFuncReturn(time.After, time.NewTimer(time.Millisecond).C)

	// act
	gotDiskName := findDiskOfUltraPath(context.Background(), 0, mockShareData, "test_type", "test_wwn")

	// assert
	assert.Equal(t, "", gotDiskName)
}

func Test_findDiskOfUltraPath_StoppedThreadsConditionMet(t *testing.T) {
	// arrange
	mockShareData := &shareData{}

	// act
	gotDiskName := findDiskOfUltraPath(context.Background(), 0, mockShareData, "test_type", "test_wwn")

	// assert
	assert.Equal(t, "", gotDiskName)
}

func Test_findDiskOfUltraPath_Success(t *testing.T) {
	// arrange
	mockShareData := &shareData{}
	mockShareData.stoppedThreads.Store(1)
	wantDiskName := "disk"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(connector.GetDiskNameByWWN, wantDiskName, nil)

	// act
	gotDiskName := findDiskOfUltraPath(context.Background(), 0, mockShareData, "test_type", "test_wwn")

	// assert
	assert.Equal(t, wantDiskName, gotDiskName)
}
