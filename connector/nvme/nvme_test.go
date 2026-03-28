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

package nvme

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils/lock"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestConnector_ConnectVolume_SuccessWithoutMultipath(t *testing.T) {
	// arrange
	mockConn := &Connector{}
	params := map[string]interface{}{
		"tgtLunGuid":         "guid1",
		"tgtPortals":         []string{"127.0.0.1"},
		"protocol":           constants.ProtocolTCPNVMe,
		"volumeUseMultiPath": false,
		"multiPathType":      "none",
	}

	// mock
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(lock.SyncLock, nil).
		ApplyFuncReturn(lock.SyncUnlock, nil).
		ApplyFuncReturn(utils.PathExist, true, nil).
		ApplyFuncReturn(ioutil.ReadFile, []byte("guid1"), nil).
		ApplyFuncReturn(connector.VerifySingleDevice, nil).
		ApplyFuncReturn(time.Sleep).
		ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
			// 1: mock ping command
			{Values: gomonkey.Params{"", nil}},
			// 2, 6: mock nvme version
			{Values: gomonkey.Params{"nvme version 2.3 (git 2.3)", nil}, Times: 2},
			// 8: mock nvme ns-rescan
			{Values: gomonkey.Params{"", nil}},
			// 9: mock ls <path> | grep nvme
			{Values: gomonkey.Params{"nvme0n1", nil}},
		}).
		ApplyFuncSeq(utils.ExecShellCmdFilterLog, []gomonkey.OutputCell{
			// 3: mock nvme list-subsys
			{Values: gomonkey.Params{"[]", nil}},
			// 4: mock nvme discovery
			{Values: gomonkey.Params{`
=====Discovery Log Entry 0======
trtype:  tcp
adrfam:  ipv4
subtype: nvme subsystem
treq:    not specified
portid:  32
trsvcid: 4420
subnqn:  domain:sign
traddr:  127.0.0.1
eflags:  none
sectype: none`, nil}},
			// 5: mock nvme connect
			{Values: gomonkey.Params{"", nil}},
			// 7: mock nvme list-subsys
			{Values: gomonkey.Params{`[
  {
    "HostNQN":"host-nqn",
    "HostID":"host-id",
    "Subsystems":[
      {
        "Name":"nvme-subsys0",
        "NQN":"domain:sign",
        "Paths":[
          {
            "Name":"nvme0",
            "Transport":"tcp",
            "Address":"traddr=127.0.0.1,trsvcid=4420,src_addr=192.168.0.0",
            "State":"live"
          }
        ]
      }
    ]
  }
]`, nil}},
		})

	// act
	disk, err := mockConn.ConnectVolume(context.Background(), params)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, "/dev/nvme0n1", disk)

}

func TestConnector_ConnectVolume_SuccessWithNVMeNative(t *testing.T) {
	// arrange
	mockConn := &Connector{}
	params := map[string]interface{}{
		"tgtLunGuid":         "guid1",
		"tgtPortals":         []string{"127.0.0.1"},
		"protocol":           constants.ProtocolTCPNVMe,
		"volumeUseMultiPath": true,
		"multiPathType":      connector.NVMeNative,
	}

	// mock
	defaultWait := app.GetGlobalConfig().AllPathOnline
	app.GetGlobalConfig().AllPathOnline = true
	defer func() {
		app.GetGlobalConfig().AllPathOnline = defaultWait
	}()
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(lock.SyncLock, nil).
		ApplyFuncReturn(lock.SyncUnlock, nil).
		ApplyFuncReturn(utils.PathExist, true, nil).
		ApplyFuncReturn(ioutil.ReadFile, []byte("guid1"), nil).
		ApplyFuncReturn(connector.VerifySingleDevice, nil).
		ApplyFuncReturn(os.ReadFile, []byte("Y"), nil).
		ApplyFuncReturn(os.Lstat, nil, nil).
		ApplyFuncReturn(os.Stat, nil, nil).
		ApplyFuncReturn(os.Readlink, "../../nvme0n1", nil).
		ApplyFuncReturn(time.Sleep).
		ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
			// 1: mock ping command
			{Values: gomonkey.Params{"", nil}},
			// 2, 6: mock nvme version
			{Values: gomonkey.Params{"nvme version 2.3 (git 2.3)", nil}, Times: 2},
			// 8: mock nvme ns-rescan
			{Values: gomonkey.Params{"", nil}},
			// 9: mock ls <path> | grep nvme
			{Values: gomonkey.Params{"nvme0c1n1", nil}},
			// 10: mock nvme version
			{Values: gomonkey.Params{"nvme version 2.3 (git 2.3)", nil}},
			// 12: mock nvme list
			{Values: gomonkey.Params{"nvme0n1    /dev/nvme0n1    XXXX", nil}},
		}).
		ApplyFuncSeq(utils.ExecShellCmdFilterLog, []gomonkey.OutputCell{
			// 3: mock nvme list-subsys
			{Values: gomonkey.Params{"[]", nil}},
			// 4: mock nvme discovery
			{Values: gomonkey.Params{`
=====Discovery Log Entry 0======
trtype:  tcp
adrfam:  ipv4
subtype: nvme subsystem
treq:    not specified
portid:  32
trsvcid: 4420
subnqn:  domain:sign
traddr:  127.0.0.1
eflags:  none
sectype: none`, nil}},
			// 5: mock nvme connect
			{Values: gomonkey.Params{"", nil}},
			// 7, 11: mock nvme list-subsys
			{Values: gomonkey.Params{`
[
  {
    "HostNQN":"host-nqn",
    "HostID":"host-id",
    "Subsystems":[
      {
        "Name":"nvme-subsys0",
        "NQN":"domain:sign",
        "Paths":[
          {
            "Name":"nvme0",
            "Transport":"tcp",
            "Address":"traddr=127.0.0.1,trsvcid=4420,src_addr=192.168.0.0",
            "State":"live"
          }
        ]
      }
    ]
  }
]`, nil}, Times: 2},
		})

	// act
	disk, err := mockConn.ConnectVolume(context.Background(), params)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, "/dev/nvme0n1", disk)
}

func TestConnector_ConnectVolume_NVMeVersionNotSupport(t *testing.T) {
	// arrange
	mockConn := &Connector{}
	params := map[string]interface{}{
		"tgtLunGuid":         "guid1",
		"tgtPortals":         []string{"127.0.0.1"},
		"protocol":           constants.ProtocolTCPNVMe,
		"volumeUseMultiPath": false,
		"multiPathType":      "none",
	}

	// mock
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(lock.SyncLock, nil).
		ApplyFuncReturn(lock.SyncUnlock, nil).
		ApplyFuncReturn(utils.ExecShellCmd, "nvme version 1.5", nil)

	// act
	disk, err := mockConn.ConnectVolume(context.Background(), params)

	// assert
	assert.ErrorContains(t, err, "the current NVMe CLI version is not supported")
	assert.Empty(t, disk)

}

func TestConnector_DisConnectVolume_NVMeNative(t *testing.T) {
	// arrange
	mockConn := &Connector{}

	// mock
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(lock.SyncLock, nil).
		ApplyFuncReturn(lock.SyncUnlock, nil).
		ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
			// 1: mock ls -l /dev/disk/by-id/
			{Values: gomonkey.Params{"xxxxx nvme-eui.wwn -> ../../nvme0n1", nil}},
			// 2: mock get subpath number
			{Values: gomonkey.Params{"\n0", nil}},
			// 3: mock nvme disconnect
			{Values: gomonkey.Params{"", nil}},
		}).ApplyFuncReturn(filepath.Glob, []string{
		"/sys/class/nvme-fabrics/ctl/nvme0/nvme0c0n1",
		"/sys/class/nvme-fabrics/ctl/nvme1/nvme0c1n1",
	}, nil)

	// act
	err := mockConn.DisConnectVolume(context.Background(), "wwn")

	// assert
	assert.NoError(t, err)

}

func TestConnector_DisConnectVolume_NonMultipath(t *testing.T) {
	// arrange
	mockConn := &Connector{}

	// mock
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(lock.SyncLock, nil).
		ApplyFuncReturn(lock.SyncUnlock, nil).
		ApplyFuncSeq(utils.ExecShellCmd, []gomonkey.OutputCell{
			// 1: mock ls -l /dev/disk/by-id/
			{Values: gomonkey.Params{"xxxxx nvme-eui.wwn -> ../../nvme0n1", nil}},
			// 2: mock get subpath number
			{Values: gomonkey.Params{"\n0", nil}},
			// 3: mock nvme disconnect
			{Values: gomonkey.Params{"", nil}},
		}).ApplyFuncReturn(filepath.Glob, []string{
		"/sys/class/nvme-fabrics/ctl/nvme0/nvme0n1",
	}, nil)

	// act
	err := mockConn.DisConnectVolume(context.Background(), "wwn")

	// assert
	assert.NoError(t, err)

}
