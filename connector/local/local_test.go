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

package local

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/prashantv/gostub"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/connector/utils/lock"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/app/config"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	logName = "localTest.log"
)

func TestConnectVolume(t *testing.T) {
	var ctx = context.TODO()

	type args struct {
		ctx  context.Context
		conn map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "NoTgtLunWWN",
			args: args{
				ctx:  ctx,
				conn: map[string]interface{}{}},
			want:    "",
			wantErr: true,
		},
		{
			name: "devPathNoExist",
			args: args{
				ctx:  ctx,
				conn: map[string]interface{}{"tgtLunWWN": "test"},
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "Normal",
			args: args{
				ctx:  ctx,
				conn: map[string]interface{}{"tgtLunWWN": "tgtLunWWN"},
			},
			want:    "/dev/disk/by-id/wwn-0xtgtLunWWN",
			wantErr: false,
		},
	}

	stubs := gostub.Stub(&waitDevOnlineTimeInterval, time.Millisecond)
	defer stubs.Reset()

	stubs.Stub(&utils.ExecShellCmd, func(ctx context.Context, format string, args ...interface{}) (string, error) {
		if args[0] == "/dev/disk/by-id/wwn-0xtgtLunWWN" {
			return "/dev/disk/by-id/wwn-0xtgtLunWWN", nil
		}
		return "ls: cannot access '/dev/disk/by-id/wwn-0xtgtLunWWN': No such file or directory", nil
	})
	stubs.StubFunc(&connector.VerifySingleDevice, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := &Local{}
			got, err := loc.ConnectVolume(tt.args.ctx, tt.args.conn)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConnectVolume() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDisConnectVolume(t *testing.T) {
	var ctx = context.TODO()

	type args struct {
		ctx       context.Context
		tgtLunWWN string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "EmptyTgtLunWWN",
			args: args{
				ctx:       ctx,
				tgtLunWWN: "",
			},
			wantErr: false,
		},
		{
			name: "DeviceNotExist",
			args: args{
				ctx:       ctx,
				tgtLunWWN: "test",
			},
			wantErr: false,
		},
		{
			name: "Normal",
			args: args{
				ctx:       ctx,
				tgtLunWWN: "tgtLunWWN",
			},
			wantErr: true,
		},
	}

	stubs := gostub.Stub(&connector.DisconnectVolumeTimeOut, time.Millisecond)
	defer stubs.Reset()

	stubs.Stub(&connector.DisconnectVolumeTimeInterval, time.Millisecond)
	stubs.Stub(&utils.ExecShellCmd, func(ctx context.Context, format string, args ...interface{}) (string, error) {
		if args[0] == "tgtLunWWN" {
			return "./../../sd-tgtLunWWN\n", nil
		}
		return "No such file or directory", nil
	})
	stubs.StubFunc(&connector.GetPhysicalDevices, []string{"sd-tgtLunWWN"}, nil)
	stubs.StubFunc(&connector.RemoveAllDevice, "", nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := &Local{}
			err := loc.DisConnectVolume(tt.args.ctx, tt.args.tgtLunWWN)
			if (err != nil) != tt.wantErr {
				t.Errorf("DisConnectVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

const defaultDriverName = "csi.huawei.com"

var driverName = flag.String("driver-name", defaultDriverName, "CSI driver name")

func TestMain(m *testing.M) {
	stubs := gostub.StubFunc(&app.GetGlobalConfig, config.MockCompletedConfig())
	defer stubs.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	if err := lock.InitLock(*driverName); err != nil {
		log.Errorf("test lock init failed: %v", err)
		os.Exit(1)
	}

	m.Run()
}
