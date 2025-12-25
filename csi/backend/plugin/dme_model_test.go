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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

func TestCreateDmeVolumeParameter_genCreateVolumeModel_Success(t *testing.T) {

	// arrange
	param := &CreateDmeVolumeParameter{AuthClient: "test1;test2",
		AllSquash: constants.AllSquash, RootSquash: constants.NoRootSquash}

	// act
	model, err := param.genCreateVolumeModel("test", constants.ProtocolNfs, SectorSize)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, constants.AllSquashValue, model.AllSquash)
	assert.Equal(t, constants.NoRootSquashValue, model.RootSquash)
}

func TestCreateDmeVolumeParameter_genCreateVolumeModel_Error(t *testing.T) {

	// arrange
	param := &CreateDmeVolumeParameter{}

	// act
	model, err := param.genCreateVolumeModel("test", constants.ProtocolDtfs, SectorSize)

	// assert
	assert.Error(t, err)
	assert.Nil(t, model)
	assert.True(t, strings.Contains(err.Error(), constants.ProtocolDtfs))

}

func TestCreateDmeVolumeParameter_validate_NfsError(t *testing.T) {

	// arrange
	param := &CreateDmeVolumeParameter{}

	// act
	err := param.validate(constants.ProtocolNfs)

	// assert
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), constants.ProtocolNfs))
}

func TestCreateDmeVolumeParameter_validate_DtfsError(t *testing.T) {

	// arrange
	param := &CreateDmeVolumeParameter{}

	// act
	err := param.validate(constants.ProtocolDtfs)

	// assert
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), constants.ProtocolDtfs))
}

func TestCreateDmeVolumeParameter_validate_AllSquashError(t *testing.T) {

	// arrange
	param := &CreateDmeVolumeParameter{AuthClient: "test", AllSquash: "test"}

	// act
	err := param.validate(constants.ProtocolNfs)

	// assert
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), constants.AllSquash))
}

func TestCreateDmeVolumeParameter_validate_RootSquashError(t *testing.T) {

	// arrange
	param := &CreateDmeVolumeParameter{AuthClient: "test", RootSquash: "test"}

	// act
	err := param.validate(constants.ProtocolNfs)

	// assert
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), constants.RootSquash))
}
