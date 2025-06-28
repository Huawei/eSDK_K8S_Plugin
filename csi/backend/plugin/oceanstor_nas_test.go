/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
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

	var cli *client.RestClient
	p := gomonkey.ApplyMethod(reflect.TypeOf(cli), "Logout",
		func(*client.RestClient, context.Context) {}).
		ApplyMethod(reflect.TypeOf(cli), "Login",
			func(*client.RestClient, context.Context) error {
				return nil
			}).
		ApplyMethod(reflect.TypeOf(cli), "SetSystemInfo",
			func(*client.RestClient, context.Context) error {
				return nil
			})
	defer p.Reset()

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
	t.Run("Empty", func(t *testing.T) {
		err := mockOceanstorNasPlugin.Validate(ctx, map[string]interface{}{})
		require.Error(t, err)
	})

	t.Run("Normal", func(t *testing.T) {
		portals := []interface{}{"127.0.0.1"}
		parameters := map[string]interface{}{
			"protocol": "nfs",
			"portals":  portals,
		}
		urls := []interface{}{"127.0.0.1"}
		config := map[string]interface{}{
			"parameters":         parameters,
			"urls":               urls,
			"user":               "mock-user",
			"secretName":         "mock-secretName",
			"secretNamespace":    "secretNamespace",
			"backendID":          "mock-backendID",
			"authenticationMode": "ldap",
		}

		m := gomonkey.ApplyMethod(reflect.TypeOf(&client.OceanstorClient{}),
			"ValidateLogin",
			func(_ *client.OceanstorClient, _ context.Context) error { return nil },
		)
		defer m.Reset()

		err := mockOceanstorNasPlugin.Validate(ctx, config)
		require.NoError(t, err)
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
	p.metroRemotePlugin.cli = &client.OceanstorClient{}
	want := errors.New("mock GetFileSystemByName failed")

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetFileSystemByName",
		func(c *client.OceanstorClient, ctx context.Context, name string) (map[string]interface{}, error) {
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
	p.metroRemotePlugin.cli = &client.OceanstorClient{}
	want := errors.New("mock GetNfsShareByPath failed")

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetFileSystemByName",
		func(c *client.OceanstorClient, ctx context.Context, name string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"vstoreId": "1",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetNfsShareByPath",
		func(c *client.OceanstorClient, ctx context.Context, path, vStoreID string) (map[string]interface{}, error) {
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
	p.metroRemotePlugin.cli = &client.OceanstorClient{}
	want := errors.New("mock DeleteNfsShare failed")

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetFileSystemByName",
		func(c *client.OceanstorClient, ctx context.Context, name string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"vstoreId": "1",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetNfsShareByPath",
		func(c *client.OceanstorClient, ctx context.Context, path, vStoreID string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"ID": "test-id",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "SafeDeleteNfsShare",
		func(c *client.OceanstorClient, ctx context.Context, id, vStoreID string) error {
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
	p.metroRemotePlugin.cli = &client.OceanstorClient{}
	want := errors.New("use backend [] deleteFileSystem failed, error: mock DeleteFileSystem failed")

	// mock
	m := gomonkey.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetFileSystemByName",
		func(c *client.OceanstorClient, ctx context.Context, name string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"vstoreId": "1",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "GetNfsShareByPath",
		func(c *client.OceanstorClient, ctx context.Context, path, vStoreID string) (map[string]interface{}, error) {
			return map[string]interface{}{
				"ID": "test-id",
			}, nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "SafeDeleteNfsShare",
		func(c *client.OceanstorClient, ctx context.Context, id, vStoreID string) error {
			return nil
		})
	m.ApplyMethod(reflect.TypeOf(p.metroRemotePlugin.cli), "SafeDeleteFileSystem",
		func(c *client.OceanstorClient, ctx context.Context, params map[string]interface{}) error {
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
	p.cli = &client.OceanstorClient{}

	// mock
	var cli *client.OceanstorClient
	m := gomonkey.ApplyMethod(reflect.TypeOf(cli), "GetFileSystemByName",
		func(c *client.OceanstorClient, ctx context.Context, name string) (map[string]interface{}, error) {
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

func TestOceanstorNasPluginUpdateConsistentSnapshotCapability(t *testing.T) {
	// arrange
	cases := []struct {
		version   string
		supported bool
	}{
		{version: "6.0.1", supported: false},
		{version: "6.1.0", supported: false},
		{version: "6.1.2", supported: false},
		{version: "6.1.3", supported: false},
		{version: "6.1.5", supported: false},
		{version: "6.1.6", supported: true},
		{version: "6.1.7", supported: true},
		{version: "6.1.8", supported: true},
	}
	var genNas = func(version string) *OceanstorNasPlugin {
		return &OceanstorNasPlugin{
			OceanstorPlugin: OceanstorPlugin{
				cli: &client.OceanstorClient{
					RestClient: &client.RestClient{StorageVersion: version},
				},
				product: constants.OceanStorDoradoV6,
			},
		}
	}

	for _, c := range cases {
		capabilities := map[string]any{}
		specifications := map[string]any{}

		// act
		err := genNas(c.version).updateConsistentSnapshotCapability(capabilities, specifications)

		// assert
		require.NoError(t, err)
		require.Equal(t, c.supported, capabilities["SupportConsistentSnapshot"])
	}
}
