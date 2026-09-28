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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/fcnvme"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

func TestAttachmentManager_GetMappingProperties(t *testing.T) {
	// arrange
	manager := AttachmentManager{
		Cli:      &client.OceanstorClient{},
		Protocol: constants.ProtocolFCNVMe,
	}
	params := map[string]interface{}{}
	initiator := "initiator1"
	wwn := "wwn1"
	want := map[string]interface{}{
		"portWWNList": []fcnvme.PortWWNPair{{
			InitiatorPortWWN: initiator,
			TargetPortWWN:    wwn,
		}},
		"tgtLunGuid": "wwn",
	}

	// mock
	p := gomonkey.NewPatches()
	defer p.Reset()
	p.ApplyFuncReturn(GetMultipleInitiators, []string{initiator}, nil).
		ApplyMethodReturn(&client.OceanstorClient{}, "GetFCTargetWWNs", []string{wwn}, nil)

	// action
	got, err := manager.GetMappingProperties(context.Background(), "wwn", "1", params)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, want, got)
}

func TestAttachmentManager_AttachFC_Success(t *testing.T) {
	// arrange
	manager := AttachmentManager{Cli: &client.OceanstorClient{}}
	hostID := "1"
	params := map[string]interface{}{"HostName": "node1"}
	wwn := "20:00:00:00:00:00:00:01"
	initiatorInfo := map[string]interface{}{
		"ID":            wwn,
		"RUNNINGSTATUS": "27",
		"ISFREE":        "true",
		"PARENTID":      "",
	}
	want := []map[string]interface{}{initiatorInfo}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(GetMultipleInitiators, []string{wwn}, nil)
	mock.ApplyMethodReturn(&client.OceanstorClient{}, "GetFCInitiator", initiatorInfo, nil)
	mock.ApplyMethodReturn(&client.OceanstorClient{}, "AddFCInitiatorToHost", nil)

	// action
	got, err := manager.AttachFC(context.Background(), hostID, params)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, want, got)
}

func TestAttachmentManager_AttachFC_NoValidInitiator(t *testing.T) {
	// arrange
	manager := AttachmentManager{Cli: &client.OceanstorClient{}}
	hostID := "1"
	params := map[string]interface{}{"HostName": "node1"}
	wwn := "20:00:00:00:00:00:00:01"

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyFuncReturn(GetMultipleInitiators, []string{wwn}, nil)
	mock.ApplyMethodReturn(&client.OceanstorClient{}, "GetFCInitiator", nil, nil)

	// action
	got, err := manager.AttachFC(context.Background(), hostID, params)

	// assert
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no valid FC initiator found")
	assert.Nil(t, got)
}
