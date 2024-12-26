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

package nvme

import (
	"context"
	"errors"
	"flag"
	"sync"
	"testing"
	"time"

	"github.com/prashantv/gostub"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils/lock"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "nvmeTest.log"
)

type args struct {
	ctx  context.Context
	conn map[string]any
}

func TestConnectVolume(t *testing.T) {
	var ctx = context.TODO()
	var mutex = sync.Mutex{}
	var GetSubSysInfoOutput = map[string]any{
		"Subsystems": []any{map[string]any{"Paths": []any{map[string]any{
			"Transport": "fc", "State": "live", "Name": "channelName", "Address": "address"}}}}}
	var normalConnMap = map[string]any{"tgtLunGuid": "LunGUID", "volumeUseMultiPath": true,
		"multiPathType": "UseUltraPath",
		"portWWNList":   []PortWWNPair{{"address", "address"}}}
	var noTgtLunGuidConnMap = map[string]any{}
	var noVolumeUseMultiPathConnMap = map[string]any{"tgtLunGuid": "LunGUID"}
	var noMultiPathTypeConnMap = map[string]any{"tgtLunGuid": "LunGUID", "volumeUseMultiPath": true}
	var noPortWWNListConnMap = map[string]any{"tgtLunGuid": "LunGUID", "volumeUseMultiPath": true,
		"multiPathType": "UseUltraPath"}

	tests := []struct {
		name    string
		mutex   sync.Mutex
		args    args
		want    string
		wantErr bool
	}{
		{"Normal", mutex, args{ctx, normalConnMap}, "/dev/NVMeVirtualDevice", false},
		{"NoTgtLunGuid", mutex, args{ctx, noTgtLunGuidConnMap}, "", true},
		{"NoVolumeUseMultiPath", mutex, args{ctx, noVolumeUseMultiPathConnMap}, "", true},
		{"NoMultiPathType", mutex, args{ctx, noMultiPathTypeConnMap}, "", true},
		{"NoPortWWNList", mutex, args{ctx, noPortWWNListConnMap}, "", true},
	}
	stubs := gostub.StubFunc(&connector.GetSubSysInfo, GetSubSysInfoOutput, nil)
	defer stubs.Reset()
	stubs.StubFunc(&connector.DoScanNVMeDevice, nil)
	stubs.StubFunc(&connector.GetDevNameByLunWWN, "NVMeVirtualDevice", nil)
	stubs.StubFunc(&connector.IsUpNVMeResidualPath, false, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &FCNVMe{mutex: tt.mutex}
			got, err := fc.ConnectVolume(tt.args.ctx, tt.args.conn)
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

	var mutex = sync.Mutex{}

	type args struct {
		ctx        context.Context
		tgtLunGuid string
	}
	tests := []struct {
		name    string
		mutex   sync.Mutex
		args    args
		wantErr bool
	}{
		{"Normal", mutex, args{ctx, "LunGUID"}, false},
		{"GetVirtualDeviceError", mutex, args{ctx, "errTgtLunGUID"}, true},
		{"emptyVirtualDevice", mutex, args{ctx, "emptyTgtLunGUID"}, false},
	}

	stubs := gostub.Stub(&connector.GetVirtualDevice, func(ctx context.Context, tgtLunGUID string) (string, int, error) {
		if tgtLunGUID == "errTgtLunGUID" {
			return "", 0, errors.New("test err")
		}
		if tgtLunGUID == "emptyTgtLunGUID" {
			return "", 0, nil
		}
		return "test", 1, nil
	})
	defer stubs.Reset()

	stubs.StubFunc(&connector.GetNVMePhysicalDevices, []string{}, nil)
	stubs.StubFunc(&connector.RemoveAllDevice, "test", nil)
	stubs.StubFunc(&connector.FlushDMDevice, nil)
	var interval = flushTimeInterval
	stubs.Stub(&interval, time.Microsecond)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &FCNVMe{
				mutex: tt.mutex,
			}
			if err := fc.DisConnectVolume(tt.args.ctx, tt.args.tgtLunGuid); (err != nil) != tt.wantErr {
				t.Errorf("DisConnectVolume() error = %v, wantErr %v", err, tt.wantErr)
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
		return
	}

	m.Run()
}
