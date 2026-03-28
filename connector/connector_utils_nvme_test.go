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
	"errors"
	"os"
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

func TestGetNVMeDevice_Version_Success(t *testing.T) {
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

func TestIsNVMeMultipathEnabled_ReadFileError(t *testing.T) {
	// arrange
	ctx := context.Background()

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.ReadFile, []byte(""), errors.New("read error"))

	// action
	gotResult := IsNVMeMultipathEnabled(ctx)

	// assert
	assert.Equal(t, false, gotResult)
}

func TestIsNVMeMultipathEnabled_MultipathDisabled(t *testing.T) {
	// arrange
	ctx := context.Background()
	fileData := "N"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.ReadFile, []byte(fileData), nil)

	// action
	gotResult := IsNVMeMultipathEnabled(ctx)

	// assert
	assert.Equal(t, false, gotResult)
}

func TestIsNVMeMultipathEnabled_MultipathEnabled(t *testing.T) {
	// arrange
	ctx := context.Background()
	fileData := "Y"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.ReadFile, []byte(fileData), nil)

	// action
	gotResult := IsNVMeMultipathEnabled(ctx)

	// assert
	assert.Equal(t, true, gotResult)
}

func TestIsNVMeMultipathEnabled_MultipathEnabledWithWhitespace(t *testing.T) {
	// arrange
	ctx := context.Background()
	fileData := "  Y  \n"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.ReadFile, []byte(fileData), nil)

	// action
	gotResult := IsNVMeMultipathEnabled(ctx)

	// assert
	assert.Equal(t, true, gotResult)
}

func TestGetNVMeDiskByGuid_SymlinkNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	guid := "test-guid"
	wantResult := ""

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.Lstat, nil, os.ErrNotExist)

	// action
	gotResult, gotErr := GetNVMeDiskByGuid(ctx, guid)

	// assert
	assert.Equal(t, wantResult, gotResult)
	assert.ErrorContains(t, gotErr, "symbolic link does not exist")
}

func TestGetNVMeDiskByGuid_ReadlinkError(t *testing.T) {
	// arrange
	ctx := context.Background()
	guid := "test-guid"
	wantResult := ""

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.Lstat, nil, nil).
		ApplyFuncReturn(os.Readlink, "", errors.New("readlink error"))

	// action
	gotResult, gotErr := GetNVMeDiskByGuid(ctx, guid)

	// assert
	assert.Equal(t, wantResult, gotResult)
	assert.ErrorContains(t, gotErr, "failed to read symbolic link")
}

func TestGetNVMeDiskByGuid_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	guid := "test-guid"
	devicePath := "/dev/nvme0n1"
	wantResult := "nvme0n1"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.Lstat, nil, nil).
		ApplyFuncReturn(os.Readlink, devicePath, nil)

	// action
	gotResult, gotErr := GetNVMeDiskByGuid(ctx, guid)

	// assert
	assert.Equal(t, wantResult, gotResult)
	assert.NoError(t, gotErr)
}

func TestIsNVMeSubPathExist_SubpathNotExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	subsystem := "nvme-subsys1"
	nvmePath := "nvme1"
	device := "nvme1n1"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.Stat, nil, os.ErrNotExist)

	// action
	gotResult := IsNVMeSubPathExist(ctx, subsystem, nvmePath, device)

	// assert
	assert.Equal(t, false, gotResult)
}

func TestIsNVMeSubPathExist_StatError(t *testing.T) {
	// arrange
	ctx := context.Background()
	subsystem := "nvme-subsys1"
	nvmePath := "nvme1"
	device := "nvme1n1"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.Stat, nil, errors.New("stat error"))

	// action
	gotResult := IsNVMeSubPathExist(ctx, subsystem, nvmePath, device)

	// assert
	assert.Equal(t, true, gotResult)
}

func TestIsNVMeSubPathExist_SubpathExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	subsystem := "nvme-subsys1"
	nvmePath := "nvme1"
	device := "nvme1n1"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(os.Stat, nil, nil)

	// action
	gotResult := IsNVMeSubPathExist(ctx, subsystem, nvmePath, device)

	// assert
	assert.Equal(t, true, gotResult)
}

func TestCheckIsTakeOverByNVMeNative_ExecShellCmdError(t *testing.T) {
	// arrange
	ctx := context.Background()
	device := "nvme0n1"
	wantErr := errors.New("exec error")

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.ExecShellCmd, "", wantErr)

	// action
	gotErr := CheckIsTakeOverByNVMeNative(ctx, device)

	// assert
	assert.Equal(t, wantErr, gotErr)
}

func TestCheckIsTakeOverByNVMeNative_DeviceNotTakeOver(t *testing.T) {
	// arrange
	ctx := context.Background()
	device := "nvme0n1"
	output := ""

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.ExecShellCmd, output, nil)

	// action
	gotErr := CheckIsTakeOverByNVMeNative(ctx, device)

	// assert
	assert.ErrorContains(t, gotErr, "is not takeover by NVMe-Native multipath")
}

func TestCheckIsTakeOverByNVMeNative_DeviceTakeOver(t *testing.T) {
	// arrange
	ctx := context.Background()
	device := "nvme0n1"
	output := "nvme0n1    /dev/nvme0n1    XXXX"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.ExecShellCmd, output, nil)

	// action
	gotErr := CheckIsTakeOverByNVMeNative(ctx, device)

	// assert
	assert.NoError(t, gotErr)
}

func TestCheckIsTakeOverByNVMeNative_EmptyLineInOutput(t *testing.T) {
	// arrange
	ctx := context.Background()
	device := "nvme0n1"
	output := "   \n\nnvme0n1    /dev/nvme0n1   XXXX"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(utils.ExecShellCmd, output, nil)

	// action
	gotErr := CheckIsTakeOverByNVMeNative(ctx, device)

	// assert
	assert.NoError(t, gotErr)
}

func TestGetNVMeDiskNumber(t *testing.T) {
	// arrange
	tests := []struct {
		name     string
		diskName string
		want1    string
		want2    string
		wantErr  bool
	}{
		{
			name:     "valid_nvme_disk_name",
			diskName: "nvme0n1",
			want1:    "0",
			want2:    "1",
			wantErr:  false,
		},
		{
			name:     "valid_nvme_disk_name_with_multiple_digits",
			diskName: "nvme12n34",
			want1:    "12",
			want2:    "34",
			wantErr:  false,
		},
		{
			name:     "invalid_nvme_disk_name",
			diskName: "sda",
			want1:    "",
			want2:    "",
			wantErr:  true,
		},
		{
			name:     "empty_disk_name",
			diskName: "",
			want1:    "",
			want2:    "",
			wantErr:  true,
		},
		{
			name:     "nvme_disk_missing_n",
			diskName: "nvme01",
			want1:    "",
			want2:    "",
			wantErr:  true,
		},
		{
			name:     "nvme_disk_incomplete_format",
			diskName: "nvme",
			want1:    "",
			want2:    "",
			wantErr:  true,
		},
		{
			name:     "nvme_disk_with_extra_characters",
			diskName: "nvme0n1p1",
			want1:    "",
			want2:    "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// action
			got1, got2, err := GetNVMeDiskNumber(tt.diskName)

			// assert
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, "", got1)
				assert.Equal(t, "", got2)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want1, got1)
				assert.Equal(t, tt.want2, got2)
			}
		})
	}
}
