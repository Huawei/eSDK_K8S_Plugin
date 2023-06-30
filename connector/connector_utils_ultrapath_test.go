/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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
	"errors"
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"huawei-csi-driver/utils"
)

func TestRunUpCommand(t *testing.T) {
	const stubFormat = "show vlun | grep -w %s"

	var stubCtx = context.TODO()

	var stubArgs = []interface{}{"test-targetLunWWN", "test-devName"}

	type args struct {
		ctx    context.Context
		upType string
		format string
		args   []interface{}
	}
	var ultraPathCommandArgs = args{
		stubCtx,
		UltraPathCommand,
		stubFormat,
		stubArgs[:1],
	}
	var UltraPathNVMeCommandArgs = args{
		stubCtx,
		UltraPathNVMeCommand,
		stubFormat,
		stubArgs[1:],
	}
	var noneUpTypeArgs = args{
		stubCtx,
		"",
		stubFormat,
		stubArgs,
	}

	type outputs struct {
		output string
		err    error
	}
	var cmdOutputs = outputs{"test output", nil}

	tests := []struct {
		name    string
		args    args
		outputs outputs
		want    string
		wantErr bool
	}{
		{"UltraPathCommand", ultraPathCommandArgs, cmdOutputs, "test output", false},
		{"UltraPathNVMeCommand", UltraPathNVMeCommandArgs, cmdOutputs, "test output", false},
		{"NoneUpType", noneUpTypeArgs, cmdOutputs, "", true},
	}

	stub := utils.ExecShellCmd
	defer func() {
		utils.ExecShellCmd = stub
	}()
	for _, tt := range tests {
		utils.ExecShellCmd = func(_ context.Context, format string, args ...interface{}) (string, error) {
			return tt.outputs.output, tt.outputs.err
		}

		t.Run(tt.name, func(t *testing.T) {
			got, err := runUpCommand(tt.args.ctx, tt.args.upType, tt.args.format, tt.args.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("runUpCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("runUpCommand() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckNVMeVersion(t *testing.T) {
	type stubVal struct {
		output string
		err    error
	}

	tests := []struct {
		name string
		stubVal
		wantErr bool
	}{
		{"Supported NVMe Version", stubVal{"nvme version 1.9", nil}, false},
		{"Supported NVMe Version", stubVal{"nvme version 2.1", nil}, false},
		{"Unsupported NVMe Version", stubVal{"nvme version 1.8", nil}, true},
		{"NVMe is not installed", stubVal{"-bash: nvme: command not found", errors.New("exit status 127")}, true},
	}

	stub := gostub.New()
	defer func() { stub.Reset() }()

	for _, test := range tests {
		stub.StubFunc(&utils.ExecShellCmd, test.output, test.err)

		err := checkNVMeVersion(context.TODO())
		assert.Equal(t, test.wantErr, err != nil, "%s, err:%v", test.name, err)
	}
}
