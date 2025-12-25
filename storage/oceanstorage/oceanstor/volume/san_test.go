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

package volume

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

func TestSAN_Query_success(t *testing.T) {
	// arrange
	ctx := context.Background()
	cli, _ := client.NewClient(ctx, &client.NewClientConfig{})
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)
	param := map[string]interface{}{
		"applicationtype": "testApp",
	}

	lun := map[string]interface{}{
		"WORKLOADTYPEID": "1",
		"CAPACITY":       "1024",
	}

	// mock
	m := gomonkey.NewPatches()
	defer m.Reset()
	m.ApplyMethodReturn(&client.OceanstorClient{}, "GetLunByName", lun, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetApplicationTypeByName", "1", nil)

	// action
	gotVolume, gotErr := san.Query(ctx, "testName", param)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, "testName", gotVolume.GetVolumeName())
}

func TestSAN_Query_WorkLoadTypeUnmatched(t *testing.T) {
	// arrange
	ctx := context.Background()
	cli, _ := client.NewClient(ctx, &client.NewClientConfig{})
	san := NewSAN(cli, nil, nil, constants.OceanStorDoradoV6)
	param := map[string]interface{}{
		"applicationtype": "testApp",
	}

	lun := map[string]interface{}{
		"WORKLOADTYPEID": "1",
		"CAPACITY":       "1024",
	}

	// mock
	m := gomonkey.NewPatches()
	defer m.Reset()
	m.ApplyMethodReturn(&client.OceanstorClient{}, "GetLunByName", lun, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetApplicationTypeByName", "2", nil)

	// action
	gotVolume, gotErr := san.Query(ctx, "testName", param)

	// assert
	assert.Nil(t, gotVolume)
	assert.ErrorContains(t, gotErr, "the workload type is different between")
}
