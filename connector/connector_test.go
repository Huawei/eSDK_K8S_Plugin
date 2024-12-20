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
	"reflect"
	"testing"

	"github.com/prashantv/gostub"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "connectorTest.log"
)

type stubConnector struct {
}

func (s *stubConnector) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	return "", nil
}

func (s *stubConnector) DisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	return nil
}

var testConnector VolumeConnector = &stubConnector{}

func TestRegisterConnector(t *testing.T) {
	defer func() {
		connectors = map[string]VolumeConnector{}
	}()

	connectors["fibreChannel"] = testConnector

	type args struct {
		cType string
		cnt   VolumeConnector
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"Unregistered", args{ISCSIDriver, testConnector}, false},
		{"Registered", args{FCDriver, testConnector}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RegisterConnector(tt.args.cType, tt.args.cnt); (err != nil) != tt.wantErr {
				t.Errorf("RegisterConnector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetConnector(t *testing.T) {
	connectors["iSCSI"] = testConnector

	type args struct {
		ctx   context.Context
		cType string
	}
	tests := []struct {
		name string
		args args
		want VolumeConnector
	}{
		{"NoExist", args{context.Background(), FCDriver}, nil},
		{"Existed", args{context.Background(), ISCSIDriver}, testConnector},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetConnector(tt.args.ctx, tt.args.cType); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetConnector() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	m.Run()
}
