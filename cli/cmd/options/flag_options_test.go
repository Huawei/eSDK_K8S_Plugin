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

// Package options defines options which user can input
package options

import (
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/spf13/cobra"
)

func TestFlagsOptions_WithAuthenticationMode(t *testing.T) {
	// arrange
	required := true
	flagsOptions := &FlagsOptions{cmd: &cobra.Command{}}

	// mock
	mock := gomonkey.ApplyPrivateMethod(flagsOptions, "markPersistentFlagRequired", func(name string) {})

	// act
	get := flagsOptions.WithAuthenticationMode(required)

	// assert
	if !reflect.DeepEqual(get, flagsOptions) {
		t.Errorf("TestFlagsOptions_WithAuthenticationMode failed, get %v, want %v", get, flagsOptions)
	}

	// clean
	t.Cleanup(func() {
		mock.Reset()
	})
}
