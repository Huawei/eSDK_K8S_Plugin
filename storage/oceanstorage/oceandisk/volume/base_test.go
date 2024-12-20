/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package volume is used for OceanDisk san test
package volume

import (
	"context"
	"testing"
)

func TestBase_getQoS_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	b := &Base{}
	param := map[string]interface{}{
		"qos": map[string]int{"maxIOPS": 999, "maxMBPS": 999},
	}

	// action
	gotErr := b.getQoS(ctx, param)

	// assert
	if gotErr != nil {
		t.Errorf("TestBase_getQoS_Success failed, gotErr = %v, wantErr = %v.", gotErr, nil)
	}
}

func TestBase_getQoS_Empty(t *testing.T) {
	// arrange
	ctx := context.Background()
	b := &Base{}
	param := map[string]interface{}{}

	// action
	gotErr := b.getQoS(ctx, param)

	// assert
	if gotErr != nil {
		t.Errorf("TestBase_getQoS_Empty failed, gotErr = %v, wantErr = %v.", gotErr, nil)
	}
}
