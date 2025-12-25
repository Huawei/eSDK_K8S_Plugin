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

// Package utils
package utils

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
)

func Test_FsInfo_Success(t *testing.T) {
	// arrange
	path := "/test/path"
	expectedStatFs := &unix.Statfs_t{
		Blocks: 1000,
		Bavail: 500,
		Bfree:  600,
		Bsize:  4096,
		Files:  2000,
		Ffree:  1500,
	}
	expectedFsInfo := &unixFsInfo{
		inodes:     2000,
		inodesFree: 1500,
		inodesUsed: 500,
		available:  500 * 4096,
		capacity:   1000 * 4096,
		usage:      (1000 - 600) * 4096,
	}

	// mock
	patches := gomonkey.ApplyFunc(unix.Statfs, func(_ string, buf *unix.Statfs_t) error {
		*buf = *expectedStatFs
		return nil
	})
	defer patches.Reset()

	// action
	gotFsInfo, gotErr := fsInfo(path)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, expectedFsInfo, gotFsInfo)
}

func Test_ExecShellCmd_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	format := "echo %s"
	args := []any{"hello"}
	wantOutput := "hello\n"
	app.GetGlobalConfig().ExecCommandTimeout = 10

	// mock
	patches := gomonkey.ApplyMethodReturn(&exec.Cmd{}, "CombinedOutput", []byte(wantOutput), nil)
	defer patches.Reset()

	// action
	gotOutput, gotTimeout, gotErr := execShellCmd(ctx, format, false, args...)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, wantOutput, gotOutput)
	assert.False(t, gotTimeout)
}

func Test_ExecShellCmd_WithError(t *testing.T) {
	// arrange
	ctx := context.Background()
	format := "false"
	var args []any
	wantOutput := ""
	wantErr := &exec.ExitError{}
	app.GetGlobalConfig().ExecCommandTimeout = 10

	// mock
	patches := gomonkey.ApplyMethodReturn(&exec.Cmd{}, "CombinedOutput", []byte(wantOutput), wantErr)
	defer patches.Reset()

	// action
	gotOutput, gotTimeout, gotErr := execShellCmd(ctx, format, false, args...)

	// assert
	assert.Error(t, gotErr)
	assert.Empty(t, gotOutput)
	assert.False(t, gotTimeout)
}

func Test_ExecShellCmd_Timeout(t *testing.T) {
	// arrange
	ctx := context.Background()
	format := "sleep %d"
	args := []any{5}
	app.GetGlobalConfig().ExecCommandTimeout = 0
	ch := make(chan struct{})
	wantErr := errors.New("timeout kill process")

	// mock
	patches := gomonkey.
		ApplyFuncReturn(exec.Command, &exec.Cmd{Process: &os.Process{}}).
		ApplyMethod(&exec.Cmd{}, "CombinedOutput", func(_ *exec.Cmd) ([]byte, error) {
			<-ch
			return nil, wantErr
		}).
		ApplyMethod(&os.Process{}, "Kill", func(_ *os.Process) error {
			close(ch)
			return wantErr
		})
	defer patches.Reset()

	// action
	gotOutput, gotTimeout, gotErr := execShellCmd(ctx, format, false, args...)

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
	assert.Empty(t, gotOutput)
	assert.True(t, gotTimeout)
}
