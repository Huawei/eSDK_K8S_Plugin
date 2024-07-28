/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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
	"errors"
	"reflect"
	"testing"

	"bou.ke/monkey"
	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/require"

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
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "backendID": "mock-backendID",
				"user": "testUser", "secretName": "mock-secretname", "secretNamespace": "mock-namespace",
				"keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX", "storage": "oceanstor-nas", "name": "test"},
			map[string]interface{}{"protocol": "nfs", "portals": []interface{}{"*.*.*.*"}},
			false, false,
		},
		{"ProtocolErr",
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "backendID": "mock-backendID",
				"user": "testUser", "secretName": "mock-secretname", "secretNamespace": "mock-namespace",
				"keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX", "storage": "oceanstor-nas", "name": "test"},
			map[string]interface{}{"protocol": "wrong", "portals": []interface{}{"*.*.*.1"}},
			false, true,
		},
		{"PortNotUnique",
			map[string]interface{}{"urls": []interface{}{"*.*.*.*"}, "backendID": "mock-backendID",
				"user": "testUser", "secretName": "mock-secretname", "secretNamespace": "mock-namespace",
				"keyText": "0NuSPbY4r6rANmmAipqPTMRpSlz3OULX", "storage": "oceanstor-nas", "name": "test"},
			map[string]interface{}{"protocol": "wrong", "portals": []interface{}{"*.*.*.1", "*.*.*.2"}},
			false, true,
		},
	}

	var cli *client.BaseClient
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "Logout",
		func(*client.BaseClient, context.Context) {})
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "Login",
		func(*client.BaseClient, context.Context) error {
			return nil
		})
	monkey.PatchInstanceMethod(reflect.TypeOf(cli), "SetSystemInfo",
		func(*client.BaseClient, context.Context) error {
			return nil
		})
	defer monkey.UnpatchAll()

	for _, tt := range tests {
		var p = &OceanstorNasPlugin{}
		t.Run(tt.name, func(t *testing.T) {
			if err := p.Init(ctx, tt.config, tt.parameters, tt.keepLogin); (err != nil) != tt.wantErr {
				t.Errorf("Init error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	convey.Convey("Empty", t, func() {
		err := mockOceanstorNasPlugin.Validate(ctx, map[string]interface{}{})
		convey.So(err, convey.ShouldBeError)
	})

	convey.Convey("Normal", t, func() {
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
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestDeleteRemoteFilesystem_EmptyCli(t *testing.T) {
	// arrange
	p := &OceanstorNasPlugin{}
	want := errors.New("p.metroRemotePlugin or p.metroRemotePlugin.cli is nil")

	// mock
	p.metroRemotePlugin = nil

	// act
	err := p.deleteRemoteFilesystem(ctx, "pvc-test")

	// assert
	require.Contains(t, err.Error(), want.Error())
}

func TestDeleteRemoteFilesystem_GetFsFailed(t *testing.T) {
	// arrange
	p := &OceanstorNasPlugin{}
	p.metroRemotePlugin = &OceanstorNasPlugin{}
	p.metroRemotePlugin.cli = &client.BaseClient{}
	want := errors.New("mock GetFileSystemByName failed")

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetFileSystemByName",
		func(c *client.BaseClient, ctx context.Context, name string) (map[string]interface{}, error) {
			return nil, errors.New("mock GetFileSystemByName failed")
		})
	defer m.Reset()

	// act
	err := p.deleteRemoteFilesystem(ctx, "pvc-test")

	// assert
	require.Contains(t, err.Error(), want.Error())
}

func TestDeleteRemoteFilesystem_GetNfsShareByPathFailed(t *testing.T) {
	// arrange
	p := &OceanstorNasPlugin{}
	p.metroRemotePlugin = &OceanstorNasPlugin{}
	p.metroRemotePlugin.cli = &client.BaseClient{}
	want := errors.New("mock GetNfsShareByPath failed")

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetFileSystemByName",
		func(c *client.BaseClient, ctx context.Context, name string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"vstoreId": "1",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetNfsShareByPath",
		func(c *client.BaseClient, ctx context.Context, path, vStoreID string) (map[string]interface{}, error) {
			return nil, errors.New("mock GetNfsShareByPath failed")
		})
	defer m.Reset()

	// act
	err := p.deleteRemoteFilesystem(ctx, "pvc-test")

	// assert
	require.Contains(t, err.Error(), want.Error())
}

func TestDeleteRemoteFilesystem_DeleteNfsShareFailed(t *testing.T) {
	// arrange
	p := &OceanstorNasPlugin{}
	p.metroRemotePlugin = &OceanstorNasPlugin{}
	p.metroRemotePlugin.cli = &client.BaseClient{}
	want := errors.New("mock DeleteNfsShare failed")

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetFileSystemByName",
		func(c *client.BaseClient, ctx context.Context, name string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"vstoreId": "1",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetNfsShareByPath",
		func(c *client.BaseClient, ctx context.Context, path, vStoreID string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"ID": "test-id",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "SafeDeleteNfsShare",
		func(c *client.BaseClient, ctx context.Context, id, vStoreID string) error {
			return errors.New("mock DeleteNfsShare failed")
		})
	defer m.Reset()

	// act
	err := p.deleteRemoteFilesystem(ctx, "pvc-test")

	// assert
	require.Contains(t, err.Error(), want.Error())
}

func TestDeleteRemoteFilesystem_DeleteFsFailed(t *testing.T) {
	// arrange
	p := &OceanstorNasPlugin{}
	p.metroRemotePlugin = &OceanstorNasPlugin{}
	p.metroRemotePlugin.cli = &client.BaseClient{}
	want := errors.New("use backend [] deleteFileSystem failed, error: mock DeleteFileSystem failed")

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetFileSystemByName",
		func(c *client.BaseClient, ctx context.Context, name string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"vstoreId": "1",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetNfsShareByPath",
		func(c *client.BaseClient, ctx context.Context, path, vStoreID string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"ID": "test-id",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "SafeDeleteNfsShare",
		func(c *client.BaseClient, ctx context.Context, id, vStoreID string) error {
			return nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "SafeDeleteFileSystem",
		func(c *client.BaseClient, ctx context.Context, params map[string]interface{}) error {
			return errors.New("mock DeleteFileSystem failed")
		})
	defer m.Reset()

	// act
	err := p.deleteRemoteFilesystem(ctx, "pvc-test")

	// assert
	require.Contains(t, err.Error(), want.Error())
}

func TestGetLocal2HyperMetroParameters_EmptyParam(t *testing.T) {
	// arrange
	p := &OceanstorNasPlugin{}
	p.cli = &client.BaseClient{}

	// mock
	var cli *client.BaseClient
	m := gomonkey.ApplyMethod(reflect.TypeOf(cli), "GetFileSystemByName",
		func(c *client.BaseClient, ctx context.Context, name string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"CAPACITY":    "12345",
				"DESCRIPTION": "test-description",
				"PARENTNAME":  "test-parent-name",
			}, nil
		})
	defer m.Reset()

	// act
	_, err := p.GetLocal2HyperMetroParameters(ctx, "backend-test.pvc-test", nil)

	// assert
	require.Equal(t, nil, err)
}
