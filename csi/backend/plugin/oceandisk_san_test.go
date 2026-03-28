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

// Package plugin provide storage function
package plugin

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
)

func TestOceandiskSanPlugin_InitSuccess(t *testing.T) {
	// arrange
	p := &OceandiskSanPlugin{}
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
		"storage":         oceandiskSanBackend,
		"name":            "test",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceandiskClient{}, nil).
		ApplyMethodReturn(&base.RestClient{}, "Login", nil).
		ApplyMethodReturn(&client.OceandiskClient{}, "SetSystemInfo", nil).
		ApplyMethodReturn(&base.RestClient{}, "Logout")

	// act
	gotErr := p.Init(ctx, config, params, false)

	// assert
	assert.Nil(t, gotErr)
}

func TestOceandiskSanPlugin_InitError(t *testing.T) {
	// arrange
	p := &OceandiskSanPlugin{}
	params := map[string]interface{}{"protocol": "unknown"}
	config := map[string]interface{}{}

	// act
	gotErr := p.Init(ctx, config, params, false)

	// assert
	assert.ErrorContains(t, gotErr, "protocol must be provided as one of")
}

func TestOceandiskSanPlugin_ValidateSuccess(t *testing.T) {
	// arrange
	p := &OceandiskSanPlugin{}
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
		"storage":         oceandiskSanBackend,
		"name":            "test",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceandiskClient{}, nil).
		ApplyMethodReturn(&base.RestClient{}, "ValidateLogin", nil).
		ApplyMethodReturn(&base.RestClient{}, "Logout")

	// act
	gotErr := p.Validate(ctx, params)

	// assert
	assert.Nil(t, gotErr)
}

func TestOceandiskSanPlugin_ProtocolNotExist(t *testing.T) {
	// arrange
	p := &OceandiskSanPlugin{}
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

func TestOceandiskSanPlugin_PortalsNotExist(t *testing.T) {
	// arrange
	p := &OceandiskSanPlugin{}
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
