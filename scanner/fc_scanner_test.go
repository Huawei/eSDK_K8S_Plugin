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
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestFCScanner_Scan_InvalidLunInfoType(t *testing.T) {
	// arrange
	scanner := &FCScanner{}

	// act
	err := scanner.Scan(context.Background(), "invalid")

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid lunInfo type")
}

func TestFCScanner_Scan_Success(t *testing.T) {
	// arrange
	scanner := &FCScanner{}
	lunInfos := []FCLunInfo{
		{WWPN: "50000974c0004321", HostLUN: "0"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
		// getHctlByWWPN: grep fc_transport
		{Values: gomonkey.Params{"/sys/class/fc_transport/target2:0:1/port_name", nil}},
	}).
		ApplyFuncReturn(scanScsiUtilFindDevice, nil)

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.NoError(t, err)
}

func TestFCScanner_Scan_MultiLinkSuccess(t *testing.T) {
	// arrange
	scanner := &FCScanner{}
	lunInfos := []FCLunInfo{
		{WWPN: "50000974c0004321", HostLUN: "0"},
		{WWPN: "50000974c0004322", HostLUN: "1"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
		// first link: grep fc_transport for WWPN1
		{Values: gomonkey.Params{"/sys/class/fc_transport/target2:0:1/port_name", nil}},
		// second link: grep fc_transport for WWPN2
		{Values: gomonkey.Params{"/sys/class/fc_transport/target3:0:2/port_name", nil}},
	}).
		ApplyFuncReturn(scanScsiUtilFindDevice, nil)

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.NoError(t, err)
}

func TestFCScanner_Scan_GetHctlFailed(t *testing.T) {
	// arrange
	scanner := &FCScanner{}
	lunInfos := []FCLunInfo{
		{WWPN: "50000974c0004321", HostLUN: "0"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "", fmt.Errorf("grep failed"))

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grep fc_transport failed")
}

func TestFCScanner_Scan_EmptyWWPNOutput(t *testing.T) {
	// arrange
	scanner := &FCScanner{}
	lunInfos := []FCLunInfo{
		{WWPN: "50000974c0004321", HostLUN: "0"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "  \n", nil)

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot find host number for WWPN")
}

func TestFCScanner_Scan_ScanScsiFailed(t *testing.T) {
	// arrange
	scanner := &FCScanner{}
	lunInfos := []FCLunInfo{
		{WWPN: "50000974c0004321", HostLUN: "0"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
		{Values: gomonkey.Params{"/sys/class/fc_transport/target2:0:1/port_name", nil}},
	}).
		ApplyFuncReturn(scanScsiUtilFindDevice, fmt.Errorf("scan scsi failed"))

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scan scsi failed")
}

func TestFCScanner_getHctlByWWPN_InvalidHCTFormat(t *testing.T) {
	// arrange
	scanner := &FCScanner{}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "/sys/class/fc_transport/targetinvalid/port_name", nil)

	// act
	result, err := scanner.getHctlByWWPN(context.Background(), "50000974c0004321", "0")

	// assert
	assert.Error(t, err)
	assert.Equal(t, 0, len(result))
}

func TestFCScanner_Scan_MultiTargetSuccess(t *testing.T) {
	// arrange
	scanner := &FCScanner{}
	lunInfos := []FCLunInfo{
		{WWPN: "50000974c0004321", HostLUN: "0"},
	}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
		// multi-target output: one WWPN maps to multiple fc_transport targets
		{Values: gomonkey.Params{
			"/sys/class/fc_transport/target11:0:14/port_name\n/sys/class/fc_transport/target12:0:14/port_name", nil}},
	}).ApplyFuncReturn(scanScsiUtilFindDevice, nil)

	// act
	err := scanner.Scan(context.Background(), lunInfos)

	// assert
	assert.NoError(t, err)
}
