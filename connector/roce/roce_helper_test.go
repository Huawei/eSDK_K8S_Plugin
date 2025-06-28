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

package roce

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "roce.log"
)

func TestMain(m *testing.M) {
	stubs := gostub.StubFunc(&app.GetGlobalConfig, config.MockCompletedConfig())
	defer stubs.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func Test_parseRoCEInfo(t *testing.T) {
	// arrange
	connectionProperties := map[string]interface{}{
		"tgtPortals":         []string{"127.0.0.1", "127::1", "127.0.0..1"},
		"tgtLunGuid":         "lun_guid",
		"volumeUseMultiPath": true,
		"multiPathType":      "type",
	}
	wantConnInfo := &connectorInfo{
		tgtPortals:         []string{"127.0.0.1", "127::1"},
		tgtLunGUID:         "lun_guid",
		volumeUseMultiPath: true,
		multiPathType:      "type",
	}
	ctx := context.Background()

	// mock
	mock := gomonkey.ApplyFuncReturn(utils.ExecShellCmd, "", nil)

	// action
	gotConnInfo, gotErr := parseRoCEInfo(ctx, connectionProperties)

	// assert
	assert.NoError(t, gotErr, "Test_parseRoCEInfo() failed, error: %v", gotErr)
	assert.Equal(t, wantConnInfo, &gotConnInfo, "Test_parseRoCEInfo() failed, "+
		"want: %v, got: %v", wantConnInfo, &gotConnInfo)

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}
