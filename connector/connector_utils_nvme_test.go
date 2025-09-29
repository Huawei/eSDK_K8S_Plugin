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

// Package connector provide methods of interacting with the host
package connector

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestGetNVMeDevice_Version1(t *testing.T) {
	// arrange
	mockSubsysInfo := "{\"Subsystems\":[{\"Name\":\"nvme-subsys0\"," +
		"\"NQN\":\"nqn-test\",\"Paths\":[{\"Name\":\"nvme0\"," +
		"\"Transport\":\"rdma\",\"Address\":\"traddr=addr-test\",\"State\":\"live\"}]}]}"

	// mock
	p := gomonkey.ApplyFuncReturn(utils.ExecShellCmd, "nvme version 1.9", nil).
		ApplyFuncReturn(utils.ExecShellCmdFilterLog, mockSubsysInfo, nil)
	defer p.Reset()

	// action
	info, err := GetSubSysInfo(context.Background())
	_, ok := info["Subsystems"].([]interface{})

	// assert
	assert.Nil(t, err)
	assert.Equal(t, true, ok)
}

func TestGetNVMeDevice_Version2(t *testing.T) {
	// arrange
	mockSubsysInfo := "[{\"Subsystems\":[{\"Name\":\"nvme-subsys0\"," +
		"\"NQN\":\"nqn-test\",\"Paths\":[{\"Name\":\"nvme0\"," +
		"\"Transport\":\"rdma\",\"Address\":\"traddr=addr-test\",\"State\":\"live\"}]}]}]"

	// mock
	p := gomonkey.ApplyFuncReturn(utils.ExecShellCmd, "nvme version 2.0", nil).
		ApplyFuncReturn(utils.ExecShellCmdFilterLog, mockSubsysInfo, nil)
	defer p.Reset()

	// action
	info, err := GetSubSysInfo(context.Background())
	_, ok := info["Subsystems"].([]interface{})

	// assert
	assert.Nil(t, err)
	assert.Equal(t, true, ok)
}
