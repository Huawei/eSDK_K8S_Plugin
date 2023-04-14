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

package plugin

import (
	"context"
	"reflect"
	"testing"

	"bou.ke/monkey"
	"github.com/agiledragon/gomonkey/v2"
	. "github.com/smartystreets/goconvey/convey"

	"huawei-csi-driver/storage/oceanstor/client"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name       string
		config     map[string]interface{}
		parameters map[string]interface{}
		keepLogin  bool
		wantErr    bool
	}{
		{"Normal",
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "backendID": "mock-backendID", "user": "testUser", "secretName": "mock-secretname", "secretNamespace": "mock-namespace", "keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX"},
			map[string]interface{}{"protocol": "nfs", "portals": []interface{}{"*.*.*.*"}},
			false, false,
		},
		{"ProtocolErr",
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "backendID": "mock-backendID", "user": "testUser", "secretName": "mock-secretname", "secretNamespace": "mock-namespace", "keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX"},
			map[string]interface{}{"protocol": "wrong", "portals": []interface{}{"*.*.*.1"}},
			false, true,
		},
		{"PortNotUnique",
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "backendID": "mock-backendID", "user": "testUser", "secretName": "mock-secretname", "secretNamespace": "mock-namespace", "keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX"},
			map[string]interface{}{"protocol": "wrong", "portals": []interface{}{"*.*.*.1", "*.*.*.2"}},
			false, true,
		},
	}

	var cli *client.BaseClient
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "Logout", func(*client.BaseClient, context.Context) {})
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "Login", func(*client.BaseClient, context.Context) error {
		return nil
	})
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "GetSystem", func(*client.BaseClient, context.Context) (map[string]interface{}, error) {
		return map[string]interface{}{"PRODUCTVERSION": "Test"}, nil
	})
	defer monkey.UnpatchAll()

	for _, tt := range tests {
		var p = &OceanstorNasPlugin{}
		t.Run(tt.name, func(t *testing.T) {
			if err := p.Init(tt.config, tt.parameters, tt.keepLogin); (err != nil) != tt.wantErr {
				t.Errorf("Init error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	Convey("Empty", t, func() {
		err := mockOceanstorNasPlugin.Validate(ctx, map[string]interface{}{})
		So(err, ShouldBeError)
	})

	Convey("Normal", t, func() {
		portals := []interface{}{"127.0.0.1"}
		parameters := map[string]interface{}{
			"protocol": "nfs",
			"portals":  portals,
		}
		urls := []interface{}{"127.0.0.1"}
		config := map[string]interface{}{
			"parameters":      parameters,
			"urls":            urls,
			"user":            "mock-user",
			"secretName":      "mock-secretName",
			"secretNamespace": "secretNamespace",
			"backendID":       "mock-backendID",
		}

		m := gomonkey.ApplyMethod(reflect.TypeOf(&client.BaseClient{}),
			"ValidateLogin",
			func(_ *client.BaseClient, _ context.Context) error { return nil },
		)
		defer m.Reset()

		err := mockOceanstorNasPlugin.Validate(ctx, config)
		So(err, ShouldBeNil)
	})
}
