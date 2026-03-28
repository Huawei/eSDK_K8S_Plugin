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
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
)

func TestOceanstorASeriesDtreePlugin_Init_Success(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	params := map[string]interface{}{
		"protocol":   constants.ProtocolDtfs,
		"parentname": "fakeParentName",
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

func TestOceanstorASeriesDtreePlugin_NotSupportFunction(t *testing.T) {
	// arrange
	p := &OceanstorASeriesDtreePlugin{}
	wantErr := "oceanstor-a-series-dtree storage does not support volume management"

	// act
	_, gotErr := p.CreateVolume(nil, "", nil)
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

	// act
	_, gotErr = p.QueryVolume(nil, "", nil)
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

	// act
	gotErr = p.DeleteVolume(nil, "", nil)
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

	// act
	_, gotErr = p.ExpandVolume(nil, "", 0)
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

	// act
	gotErr = p.DeleteDTreeVolume(nil, "", "")
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

	// act
	_, gotErr = p.ExpandDTreeVolume(nil, "", "", 0)
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

	// act
	gotErr = p.ModifyVolume(nil, "", volume.HyperMetro2Local, nil)
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

	wantErr = "oceanstor-a-series-dtree storage does not support snapshot feature"
	// act
	_, gotErr = p.CreateSnapshot(nil, "", "", nil)
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

	// act
	gotErr = p.DeleteSnapshot(nil, "", "")
	// assert
	assert.ErrorContains(t, gotErr, wantErr)

}

func TestOceanstorASeriesDtreePlugin_AttachVolume(t *testing.T) {
	// arrange
	mockPlugin := &OceanstorASeriesDtreePlugin{}
	parameters := map[string]interface{}{"volumeContext": map[string]string{constants.DTreeParentKey: "fakeParent"}}

	// act
	result, err := mockPlugin.AttachVolume(context.Background(), "node", parameters)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, "fakeParent", result[constants.DTreeParentKey])
}

func TestOceanstorASeriesDtreePlugin_UpdatePoolCapabilities(t *testing.T) {
	// arrange
	mockPlugin := &OceanstorASeriesDtreePlugin{}
	poolName := "fakePoolName"

	// act
	result, err := mockPlugin.UpdatePoolCapabilities(context.Background(), []string{poolName})

	// assert
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestOceanstorASeriesDtreePlugin_NewPlugin(t *testing.T) {
	// arrange
	mockPlugin := &OceanstorASeriesDtreePlugin{}

	// act
	newMockPlugin := mockPlugin.NewPlugin()

	// assert
	assert.NotNil(t, newMockPlugin)
}
