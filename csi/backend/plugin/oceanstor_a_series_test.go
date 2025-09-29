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
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestOceanstorASeriesPlugin_CreateVolume_ConvertParametersError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{}
	parameters := map[string]interface{}{"invalid": "param"}
	wantErr := "convert parameters to struct failed"

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(json.Unmarshal, errors.New(wantErr))

	// act
	_, gotErr := p.CreateVolume(ctx, "test", parameters)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestOceanstorASeriesPlugin_CreateVolume_GenModelError(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{protocol: constants.ProtocolNfs}
	parameters := map[string]interface{}{"authClient": "client-test", "allSquash": "err-str"}

	// act
	_, gotErr := p.CreateVolume(ctx, "test", parameters)

	// assert
	assert.ErrorContains(t, gotErr, "allSquash field")
}

func TestOceanstorASeriesPlugin_CreateVolume_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{protocol: constants.ProtocolNfs}
	parameters := map[string]interface{}{"authClient": "client-test"}
	mockVolume := utils.NewVolume("test")

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&volume.Creator{}, "Create", mockVolume, nil)

	// act
	gotVolume, gotErr := p.CreateVolume(ctx, "test", parameters)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, mockVolume, gotVolume)
}

func TestOceanstorASeriesPlugin_verifyPortals_DtfsSuccess(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{protocol: constants.ProtocolDtfs}
	params := map[string]interface{}{}

	// act
	gotErr := p.verifyPortals(params)

	// assert
	assert.Nil(t, gotErr)
}

func TestOceanstorASeriesPlugin_verifyPortals_NfsSuccess(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{protocol: constants.ProtocolNfs}
	params := map[string]interface{}{
		"portals": []interface{}{"test1"},
	}

	// act
	gotErr := p.verifyPortals(params)

	// assert
	assert.Nil(t, gotErr)
}

func TestOceanstorASeriesPlugin_verifyPortals_NfsPortalNotProvided(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{protocol: constants.ProtocolNfs}
	params := map[string]interface{}{
		"portals": []interface{}{},
	}
	wantErr := "portals must be provided"

	// act
	gotErr := p.verifyPortals(params)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestOceanstorASeriesPlugin_verifyPortals_NfsTooManyPortal(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{protocol: constants.ProtocolNfs}
	params := map[string]interface{}{
		"portals": []interface{}{"test1", "test2"},
	}
	wantErr := "just support one portal"

	// act
	gotErr := p.verifyPortals(params)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestOceanstorASeriesPlugin_verifyAndSetProtocol_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolDtfs,
	}

	// act
	gotErr := p.verifyAndSetProtocol(params)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, p.protocol, constants.ProtocolDtfs)
}

func TestOceanstorASeriesPlugin_verifyAndSetProtocol_ProtocolNotProvided(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{}
	params := map[string]interface{}{}
	wantErr := "protocol must be provided"

	// act
	gotErr := p.verifyAndSetProtocol(params)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestOceanstorASeriesPlugin_verifyAndSetProtocol_ProtocolNotSupport(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{}
	params := map[string]interface{}{
		"protocol": "invalid",
	}
	wantErr := fmt.Sprintf("protocol must be %s or %s", constants.ProtocolNfs, constants.ProtocolDtfs)

	// act
	gotErr := p.verifyAndSetProtocol(params)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestOceanstorASeriesPlugin_Init_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{}
	params := map[string]interface{}{
		"protocol": constants.ProtocolDtfs,
	}
	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanASeriesClient{}, nil).
		ApplyMethodReturn(&base.RestClient{}, "Login", nil).
		ApplyMethodReturn(&client.OceanASeriesClient{}, "SetSystemInfo", nil).
		ApplyMethodReturn(&base.RestClient{}, "Logout")

	// act
	gotErr := p.Init(ctx, config, params, false)

	// assert
	assert.Nil(t, gotErr)
}

func TestOceanstorASeriesPlugin_Validate_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesPlugin{}

	config := map[string]interface{}{
		"urls":            []interface{}{"test"},
		"user":            "test",
		"secretName":      "test",
		"secretNamespace": "default",
		"backendID":       "id",
		"storage":         "s3",
		"name":            "test",
		"parameters":      map[string]interface{}{"protocol": constants.ProtocolDtfs},
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(client.NewClient, &client.OceanASeriesClient{}, nil).
		ApplyMethodReturn(&base.RestClient{}, "ValidateLogin", nil).
		ApplyMethodReturn(&base.RestClient{}, "Logout")

	// act
	gotErr := p.Validate(ctx, config)

	// assert
	assert.Nil(t, gotErr)
}

func TestOceanstorASeriesPlugin_UpdatePoolCapabilities_GetAllPoolsError(t *testing.T) {
	// arrange
	plugin := GetPlugin(constants.OceanStorASeriesNas)
	p, ok := plugin.(*OceanstorASeriesPlugin)
	if !ok {
		return
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	p.cli = cli

	mockVStoreName := "test"
	poolNames := []string{"pool1"}
	vstore := map[string]interface{}{
		"nasCapacityQuota":     "100",
		"nasFreeCapacityQuota": "50",
	}
	wantErr := errors.New("get pool error")

	// mock
	cli.EXPECT().GetvStoreName().Return(mockVStoreName).AnyTimes()
	cli.EXPECT().GetvStoreByName(ctx, mockVStoreName).Return(vstore, nil)
	cli.EXPECT().GetAllPools(ctx).Return(nil, wantErr)

	// act
	gotCapabilities, gotErr := p.UpdatePoolCapabilities(ctx, poolNames)

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
	assert.Nil(t, gotCapabilities)
}

func TestOceanstorASeriesPlugin_UpdatePoolCapabilities_Success(t *testing.T) {
	// arrange
	plugin := GetPlugin(constants.OceanStorASeriesNas)
	p, ok := plugin.(*OceanstorASeriesPlugin)
	if !ok {
		return
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	p.cli = cli

	mockVStoreName := "test"
	poolNames := []string{"pool1"}
	vstore := map[string]interface{}{
		"nasCapacityQuota":     "100",
		"nasFreeCapacityQuota": "50",
	}
	mockPools := map[string]interface{}{
		"pool1": map[string]interface{}{
			"NAME":              "pool1",
			"USERFREECAPACITY":  "200",
			"USERTOTALCAPACITY": "300",
		},
	}

	// mock
	cli.EXPECT().GetvStoreName().Return(mockVStoreName).AnyTimes()
	cli.EXPECT().GetvStoreByName(ctx, mockVStoreName).Return(vstore, nil)
	cli.EXPECT().GetAllPools(ctx).Return(mockPools, nil)

	// act
	gotCapabilities, gotErr := p.UpdatePoolCapabilities(ctx, poolNames)

	// assert
	assert.NoError(t, gotErr)
	assert.Contains(t, gotCapabilities, "pool1")
}

func TestOceanstorASeriesPlugin_UpdateBackendCapabilities(t *testing.T) {
	// arrange
	plugin := GetPlugin(constants.OceanStorASeriesNas)
	p, ok := plugin.(*OceanstorASeriesPlugin)
	if !ok {
		return
	}

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	p.cli = cli

	wantCapabilities := map[string]interface{}{
		"SupportThin":               true,
		"SupportApplicationType":    true,
		"SupportQoS":                false,
		"SupportThick":              false,
		"SupportMetro":              false,
		"SupportReplication":        false,
		"SupportClone":              false,
		"SupportMetroNAS":           false,
		"SupportConsistentSnapshot": false,
		"SupportNFS3":               false,
		"SupportNFS4":               false,
		"SupportNFS41":              true,
		"SupportNFS42":              false,
	}
	wantSpecifications := map[string]interface{}{
		"LocalDeviceSN": "test-sn",
		"VStoreID":      "test-vstore-id",
		"VStoreName":    "test-vstore-name",
		"DeviceWWN":     "test-wwn",
	}

	// mock
	cli.EXPECT().GetLicenseFeature(ctx).Return(map[string]int{"SmartQos": 0}, nil)
	cli.EXPECT().GetNFSServiceSetting(ctx).Return(map[string]bool{"SupportNFS41": true}, nil)
	cli.EXPECT().GetDeviceSN().Return("test-sn")
	cli.EXPECT().GetvStoreID().Return("test-vstore-id")
	cli.EXPECT().GetvStoreName().Return("test-vstore-name")
	cli.EXPECT().GetDeviceWWN().Return("test-wwn")

	// act
	gotCapabilities, gotSpecifications, gotErr := p.UpdateBackendCapabilities(ctx)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, wantCapabilities, gotCapabilities)
	assert.Equal(t, wantSpecifications, gotSpecifications)
}
