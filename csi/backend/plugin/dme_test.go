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
	"encoding/json"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	dmeVol "github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestDmeASeriesPlugin_Init_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	p := GetPlugin(constants.OceanStorASeriesNasDme)

	parameters := map[string]any{"protocol": constants.ProtocolDtfs}
	config := map[string]any{"urls": []any{"https://127.0.0.1:8088"}, "user": "test_user", "storageDeviceSN": "test_sn",
		"secretName": "test_secret_name", "secretNamespace": "test_secret_ns", "name": "test_name",
		"backendID": "test_bk_id", "maxClientThreads": "30", "storage": constants.OceanStorASeriesNasDme}

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyMethodReturn((*client.BaseClient)(nil), "Login", nil)
	patch.ApplyMethod((*client.BaseClient)(nil), "Logout",
		func(cli *client.BaseClient, ctx context.Context) {})
	patch.ApplyMethodReturn((*client.BaseClient)(nil), "SetSystemInfo", nil)

	// act
	err := p.Init(ctx, config, parameters, false)

	// assert
	assert.NoError(t, err)
}

func TestDMEASeriesPlugin_VerifyAndSetProtocol_NotExistError(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	params := map[string]any{}

	// act
	err := p.verifyAndSetProtocol(params)

	// assert
	assert.Error(t, err)
}

func TestDMEASeriesPlugin_VerifyAndSetProtocol_InvalidError(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	params := map[string]any{"protocol": "test"}

	// act
	err := p.verifyAndSetProtocol(params)

	// assert
	assert.Error(t, err)
}

func TestDMEASeriesPlugin_VerifyAndSetProtocol_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	params := map[string]any{"protocol": constants.ProtocolNfs}

	// act
	err := p.verifyAndSetProtocol(params)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, constants.ProtocolNfs, p.protocol)
}

func TestDMEASeriesPlugin_VerifyPortals_PortalsNotExist(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{protocol: ProtocolNfs}
	params := map[string]any{}

	// act
	err := p.verifyPortals(params)

	// assert
	assert.Error(t, err)
}

func TestDMEASeriesPlugin_VerifyPortals_PortalsInvalid(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{protocol: ProtocolNfs}
	params := map[string]any{"portals": []any{"test1", "test2"}}

	// act
	err := p.verifyPortals(params)

	// assert
	assert.Error(t, err)
}

func TestDmeASeriesPlugin_CreateVolume_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	name := "test"
	parameters := map[string]interface{}{}

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()

	patch.ApplyMethodReturn((*dmeVol.Creator)(nil), "Create", utils.NewVolume(name), nil)

	// act
	vol, err := p.CreateVolume(ctx, name, parameters)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, name, vol.GetVolumeName())
}

func TestDmeASeriesPlugin_CreateVolume_Error(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	name := "test"
	parameters := map[string]interface{}{}

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	errJson := errors.New("json Unmarshal error")
	patch.ApplyFuncReturn(json.Unmarshal, errJson)

	// act
	volume, err := p.CreateVolume(ctx, name, parameters)

	// assert
	assert.Error(t, err)
	assert.Nil(t, volume)
	assert.ErrorIs(t, err, errJson)
}

func TestDmeASeriesPlugin_QueryVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	name := "test"
	parameters := map[string]interface{}{}

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyMethodReturn((*dmeVol.Querier)(nil), "Query", utils.NewVolume(name), nil)

	// act
	vol, err := p.QueryVolume(ctx, name, parameters)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, name, vol.GetVolumeName())
}

func TestDmeASeriesPlugin_DeleteVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	name := "test"

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyMethodReturn((*dmeVol.Deleter)(nil), "Delete", nil)

	// act
	err := p.DeleteVolume(ctx, name)

	// assert
	assert.NoError(t, err)
}

func TestDmeASeriesPlugin_ExpandVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	name := "test"
	size := int64(32)

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyMethodReturn((*dmeVol.Expander)(nil), "Expand", nil)

	// act
	expandRet, err := p.ExpandVolume(ctx, name, size)

	// assert
	assert.NoError(t, err)
	assert.False(t, expandRet)
}

func TestDmeASeriesPlugin_ModifyVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	volumeName := "test"
	modifyType := pkgVolume.Local2HyperMetro
	param := map[string]string{}

	// act
	err := p.ModifyVolume(ctx, volumeName, modifyType, param)

	// assert
	assert.Error(t, err)
}

func TestDmeASeriesPlugin_UpdateBackendCapabilities(t *testing.T) {
	// arrange
	config := map[string]any{"urls": []any{"https://127.0.0.1:8088"}, "user": "test_user",
		"secretName": "test_secret_name", "secretNamespace": "test_secret_ns", "name": "test_name",
		"backendID": "test_bk_id", "maxClientThreads": "30", "storage": constants.OceanStorASeriesNasDme}
	clientConfig, err := formatBaseClientConfig(config)
	assert.NoError(t, err)
	ctx := context.Background()
	cli, err := client.NewClient(ctx, clientConfig)
	assert.NoError(t, err)
	p := &DMEASeriesPlugin{cli: cli}

	// act
	gotCapabilities, gotSpecifications, err := p.UpdateBackendCapabilities(ctx)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, gotCapabilities)
	assert.NotNil(t, gotSpecifications)
	assert.True(t, gotCapabilities["SupportThin"].(bool))
}

func TestDmeASeriesPlugin_UpdatePoolCapabilities_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	config := map[string]any{"urls": []any{"https://127.0.0.1:8088"}, "user": "test_user",
		"secretName": "test_secret_name", "secretNamespace": "test_secret_ns", "name": "test_name",
		"backendID": "test_bk_id", "maxClientThreads": "30", "storage": constants.OceanStorASeriesNasDme}
	clientConfig, err := formatBaseClientConfig(config)
	assert.NoError(t, err)
	cli, err := client.NewClient(ctx, clientConfig)
	assert.NoError(t, err)
	p := &DMEASeriesPlugin{cli: cli}
	poolName := "pool1"
	pools := []*client.HyperScalePool{{ID: "1", Name: poolName, TotalCapacity: 100.0, FreeCapacity: 20.0}}

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyMethodReturn((*client.SystemClient)(nil), "GetHyperScalePools", pools, nil)

	// act
	capabilities, err := p.UpdatePoolCapabilities(ctx, []string{poolName, "pool2"})

	// assert
	assert.NoError(t, err)
	assert.True(t, len(capabilities) == 1)
	_, ok := capabilities[poolName]
	assert.True(t, ok)
}

func TestDmeASeriesPlugin_UpdatePoolCapabilities_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	config := map[string]any{"urls": []any{"https://127.0.0.1:8088"}, "user": "test_user",
		"secretName": "test_secret_name", "secretNamespace": "test_secret_ns", "name": "test_name",
		"backendID": "test_bk_id", "maxClientThreads": "30", "storage": constants.OceanStorASeriesNasDme}
	clientConfig, err := formatBaseClientConfig(config)
	assert.NoError(t, err)
	cli, err := client.NewClient(ctx, clientConfig)
	assert.NoError(t, err)
	p := &DMEASeriesPlugin{cli: cli}
	poolName := "pool1"
	errPool := errors.New("get pool err")

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyMethodReturn((*client.SystemClient)(nil), "GetHyperScalePools", nil, errPool)

	// act
	capabilities, err := p.UpdatePoolCapabilities(ctx, []string{poolName, "pool2"})

	// assert
	assert.Error(t, err)
	assert.Nil(t, capabilities)
	assert.ErrorIs(t, err, errPool)
}

func TestDmeASeriesPlugin_CreateSnapshot(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	fsName := "fs"
	snapshotName := "test"

	// act
	ret, err := p.CreateSnapshot(ctx, fsName, snapshotName)

	// assert
	assert.Error(t, err)
	assert.Nil(t, ret)
}

func TestDmeASeriesPlugin_DeleteSnapshot(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	snapshotParentId := "fsId"
	snapshotName := "test"

	// act
	err := p.DeleteSnapshot(ctx, snapshotParentId, snapshotName)

	// assert
	assert.Error(t, err)
}

func TestDmeASeriesPlugin_SupportQoSParameters(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	qosConfig := "test"

	// act
	err := p.SupportQoSParameters(ctx, qosConfig)

	// assert
	assert.NoError(t, err)
}

func TestDmeASeriesPlugin_ReLogin(t *testing.T) {
	// arrange
	ctx := context.Background()
	config := map[string]any{"urls": []any{"https://127.0.0.1:8088"}, "user": "test_user",
		"secretName": "test_secret_name", "secretNamespace": "test_secret_ns", "name": "test_name",
		"backendID": "test_bk_id", "maxClientThreads": "30", "storage": constants.OceanStorASeriesNasDme}
	clientConfig, err := formatBaseClientConfig(config)
	assert.NoError(t, err)
	cli, err := client.NewClient(ctx, clientConfig)
	assert.NoError(t, err)
	p := &DMEASeriesPlugin{cli: cli}

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyMethodReturn((*client.BaseClient)(nil), "ReLogin", nil)

	// act
	err = p.ReLogin(ctx)

	// assert
	assert.NoError(t, err)
}

func TestDmeASeriesPlugin_Validate_Success(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()
	config := map[string]any{"urls": []any{"https://127.0.0.1:8088"}, "user": "test_user",
		"secretName": "test_secret_name", "secretNamespace": "test_secret_ns", "name": "test_name",
		"backendID": "test_bk_id", "maxClientThreads": "30", "storage": constants.OceanStorASeriesNasDme,
		"parameters": map[string]any{"protocol": constants.ProtocolDtfs}}

	// mock
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyMethodReturn((*client.BaseClient)(nil), "ValidateLogin", nil)
	patch.ApplyMethod((*client.BaseClient)(nil), "Logout",
		func(cli *client.BaseClient, ctx context.Context) {})

	// act
	err := p.Validate(ctx, config)

	// assert
	assert.NoError(t, err)
}

func TestDmeASeriesPlugin_DeleteDTreeVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()

	// act
	err := p.DeleteDTreeVolume(ctx, "test", "test")

	// assert
	assert.Error(t, err)
}

func TestDmeASeriesPlugin_ExpandDTreeVolume(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}
	ctx := context.Background()

	// act
	ret, err := p.ExpandDTreeVolume(ctx, "test", "test", 0)

	// assert
	assert.Error(t, err)
	assert.False(t, ret)
}

func TestDmeASeriesPlugin_GetSectorSize(t *testing.T) {
	// arrange
	p := &DMEASeriesPlugin{}

	// act
	size := p.GetSectorSize()

	// assert
	assert.Equal(t, SectorSize, size)
}
