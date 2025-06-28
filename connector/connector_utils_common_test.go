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

package connector

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func Test_CheckHostConnectivity_WithInvalidIP(t *testing.T) {
	// arrange
	portal := "127.0..0.1:9090"

	// action
	gotConnectionStatus := CheckHostConnectivity(context.Background(), portal)

	// assert
	if gotConnectionStatus {
		t.Errorf("Test_CheckHostConnectivity_WithInvalidIP() failed, "+
			"gotConnectionStatus = %v, want = %v", gotConnectionStatus, false)
	}
}

func Test_CheckHostConnectivity_WithValidIP(t *testing.T) {
	// arrange
	portal := "127.0.0.1:9090"

	// mock
	mock := gomonkey.ApplyFuncReturn(utils.ExecShellCmd, "", nil)

	// action
	gotConnectionStatus := CheckHostConnectivity(context.Background(), portal)

	// assert
	if !gotConnectionStatus {
		t.Errorf("Test_CheckHostConnectivity_WithValidIP() failed, "+
			"gotConnectionStatus = %v, want = %v", gotConnectionStatus, true)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}
