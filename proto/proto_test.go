/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	logDir  = "/var/log/huawei/"
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
			errors.New("No ISCSI initiator exist"),
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

func TestVerifyIscsiPortals(t *testing.T) {
	cases := []struct {
		name    string
		portals []interface{}
		wantVal []string
		wantErr error
	}{
		{
			"Normal scenario",
			[]interface{}{"192.168.125.25", "192.168.125.26"},
			[]string{"192.168.125.25", "192.168.125.26"},
			nil,
		},
		{
			"The portals parameter is empty",
			nil,
			nil,
			errors.New("At least 1 portal must be provided for iscsi backend"),
		},
		{
			"The format of the portals parameter is incorrect",
			[]interface{}{"192..125.25:3260", "192.168.125.26:3260"},
			nil,
			errors.New("192..125.25:3260 of portals is invalid"),
		},
	}

	for _, c := range cases {
		portals, err := VerifyIscsiPortals(c.portals)
		assert.Equal(t, c.wantErr, err, c.name)
		assert.Equal(t, c.wantVal, portals, c.name)
	}
}

func TestMain(m *testing.M) {
	if err := log.InitLogging(logName); err != nil {
		log.Errorf("init logging: %s failed. error: %v", logName, err)
		os.Exit(1)
	}
	logFile := path.Join(logDir, logName)
	defer func() {
		if err := os.RemoveAll(logFile); err != nil {
			log.Errorf("Remove file: %s failed. error: %s", logFile, err)
		}
	}()

	m.Run()
}
