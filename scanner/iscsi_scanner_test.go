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

package scanner

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/iscsi"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestISCSIScanner_Scan_InvalidLunInfoType(t *testing.T) {
	// arrange
	scanner := &ISCSIScanner{}

	// act
	err := scanner.Scan(context.Background(), "invalid")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid lunInfo type")
}

func TestISCSIScanner_Scan_Success(t *testing.T) {
	// arrange
	scanner := &ISCSIScanner{}
	lunInfos := []ISCSILunInfo{
		{Portal: "192.168.1.100:3260", IQN: "iqn.xxx.com:disk1", HostLUN: "0"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(iscsi.SingleConnectISCSIPortal, "1", false).
		ApplyFuncReturn(filepath.Glob, []string{
			"/sys/class/iscsi_host/host3/device/session1",
		}, nil).
		ApplyFuncReturn(utils.WaitUntil, nil)

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.NoError(t, err)
}

func TestISCSIScanner_Scan_MultiLinkSuccess(t *testing.T) {
	// arrange
	scanner := &ISCSIScanner{}
	lunInfos := []ISCSILunInfo{
		{Portal: "192.168.1.100:3260", IQN: "iqn.xxx.com:disk1", HostLUN: "0"},
		{Portal: "192.168.1.101:3260", IQN: "iqn.xxx.com:disk2", HostLUN: "1"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncSeq(iscsi.SingleConnectISCSIPortal, []gomonkey.OutputCell{
		{Values: gomonkey.Params{"1", false}},
		{Values: gomonkey.Params{"2", false}},
	}).
		ApplyFuncSeq(filepath.Glob, []gomonkey.OutputCell{
			{Values: gomonkey.Params{[]string{"/sys/class/iscsi_host/host3/device/session1"}, nil}},
			{Values: gomonkey.Params{[]string{"/sys/class/iscsi_host/host4/device/session2"}, nil}},
		}).
		ApplyFuncReturn(utils.WaitUntil, nil)

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.NoError(t, err)
}

func TestISCSIScanner_Scan_EmptySessionId(t *testing.T) {
	// arrange
	scanner := &ISCSIScanner{}
	lunInfos := []ISCSILunInfo{
		{Portal: "192.168.1.100:3260", IQN: "iqn.xxx.com:disk1", HostLUN: "0"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(iscsi.SingleConnectISCSIPortal, "", true)

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "build iscsi session failed")
}

func TestISCSIScanner_Scan_ScanScsiFailed(t *testing.T) {
	// arrange
	scanner := &ISCSIScanner{}
	lunInfos := []ISCSILunInfo{
		{Portal: "192.168.1.100:3260", IQN: "iqn.xxx.com:disk1", HostLUN: "0"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(iscsi.SingleConnectISCSIPortal, "1", false).
		ApplyFuncReturn(filepath.Glob, []string{
			"/sys/class/iscsi_host/host3/device/session1",
		}, nil).
		ApplyFuncReturn(utils.WaitUntil, fmt.Errorf("scan scsi failed"))

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scan scsi failed")
}

func TestGetIscsiHostNobySessionId_Success(t *testing.T) {
	// arrange
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(filepath.Glob, []string{
		"/sys/class/iscsi_host/host3/device/session1",
	}, nil)

	// act
	hostNo, err := getIscsiHostNobySessionId("1")

	// assert
	assert.NoError(t, err)
	assert.Equal(t, "3", hostNo)
}

func TestGetIscsiHostNobySessionId_GlobError(t *testing.T) {
	// arrange
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(filepath.Glob, []string{}, fmt.Errorf("glob error"))

	// act
	hostNo, err := getIscsiHostNobySessionId("1")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get iscsi path failed")
	assert.Equal(t, "", hostNo)
}

func TestGetIscsiHostNobySessionId_NotFound(t *testing.T) {
	// arrange
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(filepath.Glob, []string{}, nil)

	// act
	hostNo, err := getIscsiHostNobySessionId("99")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "can not find iscsi path by session id 99")
	assert.Equal(t, "", hostNo)
}
