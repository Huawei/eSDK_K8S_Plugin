/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/proto"
)

func TestFusionStorageSanPlugin_Init_SuccessWithPortals(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolIscsi,
		"portals":  []interface{}{"127.0.0.1"},
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(&FusionStoragePlugin{}, "init", func(_ *FusionStorageSanPlugin,
		_ context.Context, _ map[string]interface{}, _ bool) error {
		return nil
	})

	// act
	gotErr := p.Init(ctx, nil, params, false)

	// assert
	assert.Nil(t, gotErr)
}

func TestFusionStorageSanPlugin_Init_SuccessWithLinks(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolIscsi,
		"iscsiLinks": "5",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyPrivateMethod(&FusionStoragePlugin{}, "init", func(_ *FusionStorageSanPlugin,
		_ context.Context, _ map[string]interface{}, _ bool) error {
		return nil
	})

	// act
	gotErr := p.Init(ctx, nil, params, false)

	// assert
	assert.Nil(t, gotErr)
}

func TestFusionStorageSanPlugin_Init_InvalidProtocol(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol": "invalid_proto",
	}
	wantErr := fmt.Errorf("protocol %s configured is invalid", "invalid_proto")

	// act
	gotErr := p.Init(context.Background(), nil, params, false)

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
}

func TestFusionStorageSanPlugin_Init_IscsiMissingParams(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolIscsi,
	}
	wantErr := errors.New("one of portals or iscsiLinks must be provided")

	// act
	gotErr := p.Init(context.Background(), nil, params, false)

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
}

func TestFusionStorageSanPlugin_Init_IscsiLinksConvertError(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolIscsi,
		"iscsiLinks": "invalid_number",
	}
	wantErr := errors.New("iscsiLinks invalid_number can not convert to int")

	// act
	gotErr := p.Init(context.Background(), nil, params, false)

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
}

func TestFusionStorageSanPlugin_Init_ScsiMissingPortals(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolScsi,
	}
	wantErr := fmt.Errorf("portals must be configured")

	// act
	gotErr := p.Init(context.Background(), nil, params, false)

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
}

func TestFusionStorageSanPlugin_Init_ScsiInvalidPortalFormat(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolScsi,
		"portals":  []interface{}{"invalid_portal"},
	}
	wantErr := errors.New("scsi portals convert to map[string]interface{} failed")

	// act
	gotErr := p.Init(context.Background(), nil, params, false)

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
}

func TestFusionStorageSanPlugin_Init_ScsiInvalidIPAddress(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolScsi,
		"portals": []interface{}{
			map[string]interface{}{"host1": "invalid_ip"},
		},
	}
	wantErr := fmt.Errorf("manage IP invalid_ip of host host1 is invalid")

	// act
	gotErr := p.Init(context.Background(), nil, params, false)

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
}

func TestFusionStorageSanPlugin_Init_VerifyIscsiFailed(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolIscsi,
		"portals":  []interface{}{"invalid_portal"},
	}
	wantErr := errors.New("verify iscsi portals failed")

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(proto.VerifyIscsiPortals, nil, wantErr)

	// act
	gotErr := p.Init(context.Background(), nil, params, false)

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
}

func TestFusionStorageSanPlugin_Init_UnderlyingInitFailed(t *testing.T) {
	// arrange
	p := &FusionStorageSanPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolIscsi,
		"portals":  []interface{}{"127.0.0.1"},
	}
	wantErr := errors.New("underlying init failed")

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyPrivateMethod(&FusionStoragePlugin{}, "init", func(_ *FusionStorageSanPlugin,
		_ context.Context, _ map[string]interface{}, _ bool) error {
		return wantErr
	})

	// act
	gotErr := p.Init(context.Background(), nil, params, false)

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_MissingParameters(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"otherKey": "value",
	}
	wantErr := "parameters in config must be provided"

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_InvalidProtocol(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol": "invalid_proto",
		},
	}
	wantErr := "protocol must be provided and be one of \"scsi\" or \"iscsi\""

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_ScsiMissingPortals(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol": constants.ProtocolScsi,
		},
	}
	wantErr := "portals must be provided while protocol is scsi"

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_IscsiMissingBothParams(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol": constants.ProtocolIscsi,
		},
	}
	wantErr := "one of portals or iscsiLinks must be provided while protocol is iscsi"

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_InvalidIscsiLinksFormat(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol":   constants.ProtocolIscsi,
			"iscsiLinks": "invalid_number",
		},
	}
	wantErr := "can not convert to int"

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_InvalidIscsiLinksValue(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol":   constants.ProtocolIscsi,
			"iscsiLinks": "0",
		},
	}
	wantErr := "must be greater than zero"

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_ValidScsiConfig(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol": constants.ProtocolScsi,
			"portals":  []interface{}{"valid_portal"},
		},
	}

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.Nil(t, gotErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_ValidIscsiWithPortals(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol": constants.ProtocolIscsi,
			"portals":  []interface{}{"127.0.0.1"},
		},
	}

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.Nil(t, gotErr)
}

func TestFusionStorageSanPlugin_verifyFusionStorageSanParam_ValidIscsiWithLinks(t *testing.T) {
	// arrange
	config := map[string]interface{}{
		"parameters": map[string]interface{}{
			"protocol":   constants.ProtocolIscsi,
			"iscsiLinks": "2",
		},
	}

	// act
	gotErr := (&FusionStorageSanPlugin{}).verifyFusionStorageSanParam(config)

	// assert
	assert.Nil(t, gotErr)
}
