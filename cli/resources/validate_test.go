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

// Package resources defines resources handle
package resources

import (
	"fmt"
	"testing"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

func TestValidatorBuilder_ValidateAuthenticationMode_WrongInput(t *testing.T) {
	// arrange
	builder := &ValidatorBuilder{&Validator{errs: make([]error, 0)}}

	// mock
	config.AuthenticationMode = "testMode"
	validParams := []string{constants.AuthModeLDAP, constants.AuthModeLocal}
	wantErr := fmt.Errorf("invalid value for --authenticationMode=%s, "+
		"allowed values are: %+v", config.AuthenticationMode, validParams)

	// act
	builder.ValidateAuthenticationMode()

	// assert
	if err := builder.Validate(); err == nil || err.Error() != wantErr.Error() {
		t.Errorf("TestValidatorBuilder_ValidateAuthenticationMode_WrongInput, wantErr %v, get err %v", wantErr, err)
	}

	// clean
	t.Cleanup(func() {
		config.AuthenticationMode = ""
	})
}

func TestValidatorBuilder_ValidateAuthenticationMode_RightInput(t *testing.T) {
	// arrange
	builder := &ValidatorBuilder{&Validator{errs: make([]error, 0)}}

	// mock
	config.AuthenticationMode = "LDap"

	// act
	builder.ValidateAuthenticationMode()

	// assert
	if err := builder.Validate(); err != nil {
		t.Errorf("TestValidatorBuilder_ValidateAuthenticationMode_RightInput,  err is %v", err)
	}

	// clean
	t.Cleanup(func() {
		config.AuthenticationMode = ""
	})
}
