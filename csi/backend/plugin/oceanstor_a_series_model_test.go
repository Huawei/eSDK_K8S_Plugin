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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/volume"
)

func TestCreateASeriesVolumeParameter_genCreateVolumeModel_NFSAuthClientEmpty(t *testing.T) {
	// arrange
	param := &CreateASeriesVolumeParameter{}
	protocol := constants.ProtocolNfs
	wantErr := "authClient field in StorageClass cannot be empty"

	// act
	_, gotErr := param.genCreateVolumeModel("vol1", protocol)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestCreateASeriesVolumeParameter_genCreateVolumeModel_DTFSAuthUserEmpty(t *testing.T) {
	// arrange
	param := &CreateASeriesVolumeParameter{}
	protocol := constants.ProtocolDtfs
	wantErr := "authUser field in StorageClass cannot be empty"

	// act
	_, gotErr := param.genCreateVolumeModel("vol1", protocol)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestCreateASeriesVolumeParameter_genCreateVolumeModel_InvalidAllSquash(t *testing.T) {
	// arrange
	param := &CreateASeriesVolumeParameter{
		AllSquash:  "invalid",
		AuthClient: "client-test",
	}
	protocol := constants.ProtocolNfs
	wantErr := fmt.Sprintf("allSquash field in StorageClass")

	// act
	_, gotErr := param.genCreateVolumeModel("vol1", protocol)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestCreateASeriesVolumeParameter_genCreateVolumeModel_InvalidRootSquash(t *testing.T) {
	// arrange
	param := &CreateASeriesVolumeParameter{
		RootSquash: "invalid",
		AuthClient: "client-test",
	}
	protocol := constants.ProtocolNfs
	wantErr := fmt.Sprintf("rootSquash field in StorageClass")

	// act
	_, gotErr := param.genCreateVolumeModel("vol1", protocol)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestCreateASeriesVolumeParameter_genCreateVolumeModel_AdvancedOptionsUnmarshalError(t *testing.T) {
	// arrange
	param := &CreateASeriesVolumeParameter{
		AuthClient:      "client-test",
		AdvancedOptions: "validJSON",
	}
	protocol := constants.ProtocolNfs
	wantErr := "fail to unmarshal advancedOptions"

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(json.Unmarshal, errors.New("mock error"))

	// act
	_, gotErr := param.genCreateVolumeModel("vol1", protocol)

	// assert
	assert.ErrorContains(t, gotErr, wantErr)
}

func TestCreateASeriesVolumeParameter_genCreateVolumeModel_Success(t *testing.T) {
	// arrange
	param := &CreateASeriesVolumeParameter{
		AuthClient:      "client-test",
		AdvancedOptions: `{"key": "value"}`,
	}
	name := "vol1"
	protocol := constants.ProtocolNfs
	wantModel := &volume.CreateFilesystemModel{
		Protocol:        protocol,
		Name:            name,
		AuthClients:     []string{"client-test"},
		AuthUsers:       []string{""},
		AdvancedOptions: map[string]interface{}{"key": "value"},
		AllSquash:       constants.NoAllSquashValue,
		RootSquash:      constants.NoRootSquashValue,
	}

	// act
	gotModel, gotErr := param.genCreateVolumeModel("vol1", protocol)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, wantModel, gotModel)
}
