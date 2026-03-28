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

package plugin

import (
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

func TestOceanstorSanPlugin_InitSuccess(t *testing.T) {
	// arrange
	p := &OceanstorSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolRoceNVMe,
		"portals":  []interface{}{"127.0.0.1"},
	}
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         constants.OceanStorSan,
		"name":            "test",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanstorClient{RestClient: &client.RestClient{
		Product: constants.OceanStorDoradoV6}}, nil).
		ApplyMethodReturn(&client.RestClient{}, "Login", nil).
		ApplyMethodReturn(&client.RestClient{}, "SetSystemInfo", nil).
		ApplyMethodReturn(&client.RestClient{}, "Logout")

	// act
	gotErr := p.Init(ctx, config, params, false)

	// assert
	assert.Nil(t, gotErr)
}

func TestOceanstorSanPlugin_InitProtocolError(t *testing.T) {
	// arrange
	p := &OceanstorSanPlugin{}
	params := map[string]interface{}{"protocol": "unknown"}
	config := map[string]interface{}{}

	// act
	gotErr := p.Init(ctx, config, params, false)

	// assert
	assert.ErrorContains(t, gotErr, "protocol must be provided as one of")
}

func TestOceanstorSanPlugin_InitPortalsError(t *testing.T) {
	// arrange
	p := &OceanstorSanPlugin{}
	params := map[string]interface{}{"protocol": constants.ProtocolRoceNVMe}
	config := map[string]interface{}{}

	// act
	gotErr := p.Init(ctx, config, params, false)

	// assert
	assert.ErrorContains(t, gotErr, "portals are required to configure for")
}

func TestOceanstorSanPlugin_ValidateSuccess(t *testing.T) {
	// arrange
	p := &OceanstorSanPlugin{}
	params := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol": constants.ProtocolRoceNVMe,
			"portals":  []interface{}{"127.0.0.1"},
		},
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         constants.OceanStorSan,
		"name":            "test",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanstorClient{}, nil).
		ApplyMethodReturn(&client.RestClient{}, "ValidateLogin", nil).
		ApplyMethodReturn(&client.RestClient{}, "Logout")

	// act
	gotErr := p.Validate(ctx, params)

	// assert
	assert.Nil(t, gotErr)
}

func TestOceanstorSanPlugin_ParamsNotExist(t *testing.T) {
	// arrange
	p := &OceanstorSanPlugin{}
	params := map[string]interface{}{}

	// act
	gotErr := p.Validate(ctx, params)

	// assert
	assert.ErrorContains(t, gotErr, "parameters must be provided")
}

func TestOceanstorSanPlugin_ProtocolNotExist(t *testing.T) {
	// arrange
	p := &OceanstorSanPlugin{}
	params := map[string]interface{}{
		"parameters": map[string]interface{}{
			"portals": []interface{}{"127.0.0.1"},
		},
	}

	// act
	gotErr := p.Validate(ctx, params)

	// assert
	assert.ErrorContains(t, gotErr, "protocol must be provided")
}

func TestOceanstorSanPlugin_PortalsNotExist(t *testing.T) {
	// arrange
	p := &OceanstorSanPlugin{}
	params := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol": constants.ProtocolRoceNVMe,
		},
	}

	// act
	gotErr := p.Validate(ctx, params)

	// assert
	assert.ErrorContains(t, gotErr, "portals are required to configure")
}

func TestUpdateBackendCapabilities_Succeed(t *testing.T) {
	// arrange
	oceanstorSanPlugin := &OceanstorSanPlugin{
		metroRemotePlugin: &OceanstorSanPlugin{storageOnline: true},
		OceanstorPlugin: OceanstorPlugin{
			cli: &client.OceanstorClient{},
		},
		metroDomain: "mock-domain",
	}

	// mock
	mock := gomonkey.NewPatches().ApplyMethodReturn(&oceanstorSanPlugin.OceanstorPlugin, "UpdateBackendCapabilities",
		map[string]interface{}{
			"SupportMetro":    true,
			"SupportMetroNAS": true,
		}, map[string]interface{}{}, nil).ApplyMethodReturn(oceanstorSanPlugin.cli,
		"GetHyperMetroDomainByName", map[string]interface{}{
			"RUNNINGSTATUS": hyperMetroDomainRunningStatusNormal,
		}, nil)
	defer mock.Reset()

	// action
	capabilities, _, err := oceanstorSanPlugin.UpdateBackendCapabilities(ctx)
	if err != nil {
		t.Fatalf("TestUpdateBackendCapabilities failed, err: %v", err)
	}

	// assert
	if _, ok := capabilities["SupportMetroNAS"]; ok {
		t.Fatalf("TestUpdateBackendCapabilities failed, capabilities: %v", capabilities)
	}
}

func TestUpdateBackendCapabilities_WithErrorStatus(t *testing.T) {
	// arrange
	oceanstorSanPlugin := &OceanstorSanPlugin{
		metroRemotePlugin: &OceanstorSanPlugin{storageOnline: true},
		OceanstorPlugin: OceanstorPlugin{
			cli: &client.OceanstorClient{},
		},
		metroDomain: "mock-domain",
	}

	// mock
	mock := gomonkey.NewPatches().ApplyMethodReturn(&oceanstorSanPlugin.OceanstorPlugin, "UpdateBackendCapabilities",
		map[string]interface{}{
			"SupportMetro":    true,
			"SupportMetroNAS": true,
		}, map[string]interface{}{}, nil).ApplyMethodReturn(oceanstorSanPlugin.cli,
		"GetHyperMetroDomainByName", map[string]interface{}{
			"RUNNINGSTATUS": "2",
		}, nil)
	defer mock.Reset()

	// action
	capabilities, _, err := oceanstorSanPlugin.UpdateBackendCapabilities(ctx)
	assert.NoError(t, err)

	// assert
	supportMetro, ok := capabilities["SupportMetro"]
	if !ok {
		t.Fatalf("TestUpdateBackendCapabilities failed, capabilities: %v", capabilities)
	}
	if supportMetro.(bool) {
		t.Error("TestUpdateBackendCapabilities failed, supportMetro want false, got true")
	}
}

func TestUpdateBackendCapabilities_WithErrorDomain(t *testing.T) {
	// arrange
	oceanstorSanPlugin := &OceanstorSanPlugin{
		metroRemotePlugin: &OceanstorSanPlugin{storageOnline: true},
		OceanstorPlugin: OceanstorPlugin{
			cli: &client.OceanstorClient{},
		},
		metroDomain: "mock-domain",
	}

	// mock
	mock := gomonkey.NewPatches().ApplyMethodReturn(&oceanstorSanPlugin.OceanstorPlugin, "UpdateBackendCapabilities",
		map[string]interface{}{
			"SupportMetro":    true,
			"SupportMetroNAS": true,
		}, map[string]interface{}{}, nil).ApplyMethodReturn(oceanstorSanPlugin.cli,
		"GetHyperMetroDomainByName", nil, errors.New("mock-error"))
	defer mock.Reset()

	// action
	capabilities, _, err := oceanstorSanPlugin.UpdateBackendCapabilities(ctx)
	assert.NoError(t, err)

	// assert
	supportMetro, ok := capabilities["SupportMetro"]
	if !ok {
		t.Fatalf("TestUpdateBackendCapabilities failed, capabilities: %v", capabilities)
	}
	if supportMetro.(bool) {
		t.Error("TestUpdateBackendCapabilities failed, supportMetro want false, got true")
	}
}

func TestOceanstorSanPlugin_InitWithDomain(t *testing.T) {
	// arrange
	p := &OceanstorSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolRoceNVMe,
		"portals":  []interface{}{"127.0.0.1"},
	}
	config := map[string]interface{}{
		"urls":             []interface{}{"test"},
		"user":             "test",
		"secretName":       "test",
		"secretNamespace":  "default",
		"backendID":        "id",
		"storage":          constants.OceanStorSan,
		"name":             "test",
		"hyperMetroDomain": "mock-doamin",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanstorClient{RestClient: &client.RestClient{
		Product: constants.OceanStorDoradoV6}}, nil).
		ApplyMethodReturn(&client.RestClient{}, "Login", nil).
		ApplyMethodReturn(&client.RestClient{}, "SetSystemInfo", nil).
		ApplyMethodReturn(&client.RestClient{}, "Logout")

	// act
	gotErr := p.Init(ctx, config, params, false)

	// assert
	assert.Nil(t, gotErr)
}

func TestUpdateBackendCapabilities_WithEmptyDomain(t *testing.T) {
	// arrange
	oceanstorSanPlugin := &OceanstorSanPlugin{
		metroRemotePlugin: &OceanstorSanPlugin{storageOnline: true},
		OceanstorPlugin: OceanstorPlugin{
			cli: &client.OceanstorClient{},
		},
		metroDomain: "",
	}

	// mock
	mock := gomonkey.NewPatches().ApplyMethodReturn(&oceanstorSanPlugin.OceanstorPlugin, "UpdateBackendCapabilities",
		map[string]interface{}{
			"SupportMetro":    true,
			"SupportMetroNAS": true,
		}, map[string]interface{}{}, nil)
	defer mock.Reset()

	// action
	capabilities, _, err := oceanstorSanPlugin.UpdateBackendCapabilities(ctx)

	// assert
	assert.NoError(t, err)
	assert.Contains(t, capabilities, "SupportMetro")
	assert.Equal(t, false, capabilities["SupportMetro"])
}

func TestSetStorageOnline(t *testing.T) {
	// arrange
	plugin := &OceanstorSanPlugin{}
	plugin.SetStorageOnline(true)

	// assert
	assert.True(t, plugin.storageOnline)
}
