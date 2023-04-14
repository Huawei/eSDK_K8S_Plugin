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

package attacher

import (
	"context"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"huawei-csi-driver/connector/host"
	"huawei-csi-driver/utils/log"
)

const (
	logName = "attacher_utils.log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestGetSingleInitiator(t *testing.T) {
	type args struct {
		protocol   InitiatorType
		parameters map[string]interface{}
	}
	params := map[string]interface{}{
		"HostName": "test_name_1",
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "TestGetIscsiInitiator",
			args: args{
				protocol:   ISCSI,
				parameters: params,
			},
			want:    "iscsi_initiator_1",
			wantErr: false,
		},
		{name: "TestGetRoceInitiator",
			args: args{
				protocol:   ROCE,
				parameters: params,
			},
			want:    "roce_initiator_1",
			wantErr: false,
		},
		{name: "TestHostNameNotExist",
			args: args{
				protocol:   ROCE,
				parameters: map[string]interface{}{},
			},
			want:    "",
			wantErr: true,
		},
		{name: "TestConvertFail",
			args: args{
				protocol:   FC,
				parameters: params,
			},
			want:    "",
			wantErr: true,
		},
	}

	getNodeHostInfosFromSecret := gomonkey.ApplyFunc(host.GetNodeHostInfosFromSecret, func(ctx context.Context, hostName string) (*host.NodeHostInfo, error) {
		return &host.NodeHostInfo{
			HostName:       hostName,
			IscsiInitiator: "iscsi_initiator_1",
			FCInitiators:   []string{"fc_initiators_1", "fc_initiators_2"},
			RoCEInitiator:  "roce_initiator_1",
		}, nil
	})
	defer getNodeHostInfosFromSecret.Reset()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSingleInitiator(context.Background(), tt.args.protocol, tt.args.parameters)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSingleInitiator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetSingleInitiator() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMultipleInitiators(t *testing.T) {
	type args struct {
		protocol   InitiatorType
		parameters map[string]interface{}
	}
	params := map[string]interface{}{
		"HostName": "test_name_1",
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{name: "TestGetFCInitiator",
			args: args{
				protocol:   FC,
				parameters: params,
			},
			want:    []string{"fc_initiators_1", "fc_initiators_2"},
			wantErr: false,
		},
	}
	getNodeHostInfosFromSecret := gomonkey.ApplyFunc(host.GetNodeHostInfosFromSecret, func(ctx context.Context, hostName string) (*host.NodeHostInfo, error) {
		return &host.NodeHostInfo{
			HostName:     hostName,
			FCInitiators: []string{"fc_initiators_1", "fc_initiators_2"},
		}, nil
	})
	defer getNodeHostInfosFromSecret.Reset()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetMultipleInitiators(context.Background(), tt.args.protocol, tt.args.parameters)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMultipleInitiators() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMultipleInitiators() got = %v, want %v", got, tt.want)
			}
		})
	}
}
