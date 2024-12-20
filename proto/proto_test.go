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

package proto

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "protoTest.log"
)

func TestGetISCSIInitiator(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		err     error
		wantIQN string
		wantErr error
	}{
		{
			"Normal scenario",
			"iqn.1994-05.com.redhat:98d87323a952",
			nil,
			"iqn.1994-05.com.redhat:98d87323a952",
			nil,
		},
		{
			"If the initiatorname.iscsi file does not exist",
			"awk: cmd. line:1: fatal: cannot open file `/etc/iscsi/initiatorname.iscsi' for reading (No such file or directory)",
			errors.New("status 2"),
			"",
			errors.New("no ISCSI initiator exist"),
		},
		{
			"Execution Error",
			"fork/exec awk 'BEGIN{FS=\"=\";ORS=\"\"}/^InitiatorName=/{print $2}' /etc/iscsi/initiatorname.iscs: no such file or directory",
			errors.New("status 2"),
			"",
			errors.New("status 2"),
		},
	}

	temp := utils.ExecShellCmd
	defer func() { utils.ExecShellCmd = temp }()
	for _, c := range cases {
		utils.ExecShellCmd = func(_ context.Context, _ string, _ ...interface{}) (string, error) {
			return c.output, c.err
		}
		iqn, err := GetISCSIInitiator(context.TODO())
		assert.Equal(t, c.wantErr, err, c.name)
		assert.Equal(t, c.wantIQN, iqn, c.name)
	}
}

func TestGetFCInitiator(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		err     error
		wantIQN []string
		wantErr error
	}{
		{
			"Normal scenario",
			"21000024ff3bd2b4 21000024ff3bd2b5",
			nil,
			[]string{"21000024ff3bd2b4", "21000024ff3bd2b5"},
			nil,
		},
		{
			"If the port_name file does not exist",
			"cat: '/sys/class/fc_host/host*/port_name': No such file or directory",
			nil,
			nil,
			errors.New("no FC initiator exist"),
		},
	}

	temp := utils.ExecShellCmd
	defer func() { utils.ExecShellCmd = temp }()
	for _, c := range cases {
		utils.ExecShellCmd = func(_ context.Context, _ string, _ ...interface{}) (string, error) {
			return c.output, c.err
		}
		iqn, err := GetFCInitiator(context.TODO())
		assert.Equal(t, c.wantErr, err, c.name)
		assert.Equal(t, c.wantIQN, iqn, c.name)
	}
}

func TestGetRoCEInitiator(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		err     error
		wantNQN string
		wantErr error
	}{
		{
			"Normal scenario",
			"nqn.2014-08.org.nvmexpress:uuid:a08ce5a6-fd34-e511-8193-d3f8199697e0",
			nil,
			"nqn.2014-08.org.nvmexpress:uuid:a08ce5a6-fd34-e511-8193-d3f8199697e0",
			nil,
		},
		{
			"If the hostnq file does not exist",
			"cat: /etc/nvme/hostnq: No such file or directory",
			errors.New("exit status 1"),
			"",
			errors.New("no NVME initiator exists"),
		},
		{
			"The output is empty",
			"",
			errors.New("status 2"),
			"",
			errors.New("status 2"),
		},
	}

	temp := utils.ExecShellCmd
	defer func() { utils.ExecShellCmd = temp }()
	for _, c := range cases {
		utils.ExecShellCmd = func(_ context.Context, _ string, _ ...interface{}) (string, error) {
			return c.output, c.err
		}
		iqn, err := GetRoCEInitiator(context.TODO())
		assert.Equal(t, c.wantErr, err, c.name)
		assert.Equal(t, c.wantNQN, iqn, c.name)
	}
}

func TestVerifyIscsiPortals(t *testing.T) {
	cases := []struct {
		name    string
		portals []interface{}
		wantVal []string
		wantErr error
	}{
		{
			"Normal scenario",
			[]interface{}{"127.0.0.1", "127.0.0.2"},
			[]string{"127.0.0.1", "127.0.0.2"},
			nil,
		},
		{
			"The portals parameter is empty",
			nil,
			nil,
			errors.New("at least 1 portal must be provided for iscsi backend"),
		},
		{
			"The format of the portals parameter is incorrect",
			[]interface{}{"127..0.1:3260", "127.0.0.2:3260"},
			nil,
			errors.New("127..0.1:3260 of portals is invalid"),
		},
	}

	for _, c := range cases {
		portals, err := VerifyIscsiPortals(context.Background(), c.portals)
		assert.Equal(t, c.wantErr, err, c.name)
		assert.Equal(t, c.wantVal, portals, c.name)
	}
}

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}
