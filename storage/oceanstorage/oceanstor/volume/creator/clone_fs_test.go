/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

// Package creator provides creator of volume
package creator

import (
	"context"
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const logName = "volume_creator_test.log"

func TestMain(m *testing.M) {
	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestCloneFsCreator_waitFsSplit_NoNeedWait(t *testing.T) {
	// arrange
	creator := &CloneFsCreator{waitForSplit: false}

	// act
	err := creator.waitFsSplit(context.Background(), "fs_id")

	// assert
	assert.Nil(t, err)
}
