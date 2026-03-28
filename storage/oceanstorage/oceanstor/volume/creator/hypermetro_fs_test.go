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

// Package creator provides creator of volume
package creator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

func TestNewHyperMetroCreatorFromParams(t *testing.T) {
	// arrange
	activeCli := &client.OceanstorClient{}
	standbyCli := &client.OceanstorClient{}
	params := &Parameter{
		params: map[string]any{
			WaitForSplitKey: false,
			CloneFromKey:    "test",
		},
	}

	// act
	creator := NewHyperMetroCreatorFromParams(activeCli, standbyCli, params)

	// assert
	assert.NotNil(t, creator)
	active, ok := creator.active.(*CloneFsCreator)
	assert.True(t, ok)
	assert.Equal(t, true, active.waitForSplit)
}
