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

// Package scanner provides options for scan device
package scanner

import (
	"context"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestExecuteScan_Success(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "0", t: "0", l: "1"}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "", nil)

	// act
	err := executeScan(context.Background(), hctlInfo)

	// assert
	assert.NoError(t, err)
}

func TestExecuteScan_Failed(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "0", t: "0", l: "1"}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "", fmt.Errorf("write failed"))

	// act
	err := executeScan(context.Background(), hctlInfo)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to scan scsi device")
}

func TestExecuteScan_WildcardFields(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "", t: "", l: ""}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "", nil)

	// act
	err := executeScan(context.Background(), hctlInfo)

	// assert
	assert.NoError(t, err)
}

func TestCheckDeviceExisted_Success(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "0", t: "0", l: "1"}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "sda", nil)

	// act
	exist, err := checkDeviceExisted(context.Background(), hctlInfo)

	// assert
	assert.NoError(t, err)
	assert.True(t, exist)
}

func TestCheckDeviceExisted_NotFound(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "0", t: "0", l: "1"}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "", nil)

	// act
	exist, err := checkDeviceExisted(context.Background(), hctlInfo)

	// assert
	assert.NoError(t, err)
	assert.False(t, exist)
}

func TestCheckDeviceExisted_Error(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "0", t: "0", l: "1"}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "", fmt.Errorf("ls failed"))

	// act
	exist, err := checkDeviceExisted(context.Background(), hctlInfo)

	// assert
	assert.Error(t, err)
	assert.False(t, exist)
	assert.Contains(t, err.Error(), "failed to verify scsi device")
}

func TestCheckDeviceExisted_WildcardFields(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "", t: "", l: ""}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.ExecShellCmd, "sdb", nil)

	// act
	exist, err := checkDeviceExisted(context.Background(), hctlInfo)

	// assert
	assert.NoError(t, err)
	assert.True(t, exist)
}

func TestScanScsiUtilFindDevice_Success(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "0", t: "0", l: "1"}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.WaitUntil, nil).
		ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
			// executeScan: echo scan
			{Values: gomonkey.Params{"", nil}},
			// checkDeviceExisted: ls device
			{Values: gomonkey.Params{"sda", nil}},
		})

	// act
	err := scanScsiUtilFindDevice(context.Background(), hctlInfo)

	// assert
	assert.NoError(t, err)
}

func TestScanScsiUtilFindDevice_ScanFailed(t *testing.T) {
	// arrange
	hctlInfo := hctl{h: "3", c: "0", t: "0", l: "1"}

	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(utils.WaitUntil, fmt.Errorf("wait until failed"))

	// act
	err := scanScsiUtilFindDevice(context.Background(), hctlInfo)

	// assert
	assert.Error(t, err)
}
