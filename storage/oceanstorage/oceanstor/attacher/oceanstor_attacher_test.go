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

// Package attacher provide base operations for volume attach
package attacher

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base/attacher"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

func TestOceanStorAttacher_ControllerAttach_Success(t *testing.T) {
	// arrange
	namespaceName, hostName := "namespace1", "host1"
	parameters := map[string]interface{}{
		"HostName": "host",
	}
	newClient := &client.OceanstorClient{}

	volumeAttacher := &OceanStorAttacher{
		VolumeAttacher: VolumeAttacher{
			AttachmentManager: &attacher.AttachmentManager{
				Cli:      newClient,
				Protocol: constants.ProtocolRoceNVMe,
				Portals:  []string{"127.0.0.1"},
			},
			Cli: newClient,
		},
	}

	wantRes := map[string]interface{}{
		"tgtPortals": []string{"127.0.0.1"},
		"tgtLunGuid": "guid1",
	}

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&client.OceanstorClient{}, "GetHostByName",
		map[string]interface{}{"ID": "1", "NAME": hostName}, nil).
		ApplyFuncReturn(attacher.GetSingleInitiator, "initiator1", nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetInitiatorByID",
			map[string]interface{}{"ISFREE": "false", "PARENTID": "1"}, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetLunByName",
			map[string]interface{}{"ID": "1", "NGUID": "guid1"}, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetMappingByName",
			map[string]interface{}{"ID": "1"}, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "QueryAssociateHostGroup", []interface{}{}, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetHostGroupByName",
			map[string]interface{}{"ID": "1"}, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "AddHostToGroup", nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "QueryAssociateLunGroup", []interface{}{}, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetLunGroupByName",
			map[string]interface{}{"ID": "1"}, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "AddLunToGroup", nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "AddGroupToMapping", nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetHostLunId", "1", nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetPortalByIP",
			map[string]interface{}{"SUPPORTPROTOCOL": "64"}, nil)

	// action
	getRes, getErr := volumeAttacher.ControllerAttach(context.Background(), namespaceName, parameters)

	// assert
	assert.Nil(t, getErr)
	assert.Equal(t, wantRes, getRes)
}
