/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026. All rights reserved.
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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
)

func TestBaseCreator_CreateQoS_UsesDefaultClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCli := mock_client.NewMockOceanstorClientInterface(ctrl)
	ctx := context.Background()

	// no MIN/LATENCY prefix, so UpdateFileSystem is not called
	mockCli.EXPECT().CreateQos(ctx, gomock.Any()).Return(
		map[string]any{"ID": "qos-001", "ENABLESTATUS": "true"}, nil)

	creator := &BaseCreator{
		cli:         mockCli,
		isCreateQoS: true,
		qos:         map[string]int{"MAXIOPS": 1000},
	}

	qosID, err := creator.CreateQoS(ctx, "fs-001", "vstore-1")
	assert.NoError(t, err)
	assert.Equal(t, "qos-001", qosID)
}

func TestBaseCreator_CreateQoS_UsesOverrideClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultCli := mock_client.NewMockOceanstorClientInterface(ctrl)
	overrideCli := mock_client.NewMockOceanstorClientInterface(ctrl)
	ctx := context.Background()

	// only overrideCli should be called, defaultCli should NOT
	overrideCli.EXPECT().CreateQos(ctx, gomock.Any()).Return(
		map[string]any{"ID": "qos-override", "ENABLESTATUS": "true"}, nil)

	creator := &BaseCreator{
		cli:         defaultCli,
		isCreateQoS: true,
		qos:         map[string]int{"MAXIOPS": 2000},
	}

	qosID, err := creator.CreateQoS(ctx, "fs-001", "vstore-1", overrideCli)
	assert.NoError(t, err)
	assert.Equal(t, "qos-override", qosID)
}

func TestBaseCreator_RollbackQoS_UsesDefaultClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCli := mock_client.NewMockOceanstorClientInterface(ctrl)
	ctx := context.Background()

	// DeleteQos flow: GetQosByID -> (single obj, no UpdateQos) -> DeactivateQos -> DeleteQos
	mockCli.EXPECT().GetQosByID(ctx, "qos-001", "vstore-1").Return(
		map[string]any{"FSLIST": `["fs-001"]`}, nil)
	mockCli.EXPECT().DeactivateQos(ctx, "qos-001", "vstore-1").Return(nil)
	mockCli.EXPECT().DeleteQos(ctx, "qos-001", "vstore-1").Return(nil)

	creator := &BaseCreator{
		cli:         mockCli,
		isCreateQoS: true,
		qos:         map[string]int{"MAXIOPS": 1000},
	}

	err := creator.RollbackQoS(ctx, "qos-001", "fs-001", "vstore-1")
	assert.NoError(t, err)
}

func TestBaseCreator_RollbackQoS_UsesOverrideClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultCli := mock_client.NewMockOceanstorClientInterface(ctrl)
	overrideCli := mock_client.NewMockOceanstorClientInterface(ctrl)
	ctx := context.Background()

	// only overrideCli should be called
	overrideCli.EXPECT().GetQosByID(ctx, "qos-002", "vstore-standby").Return(
		map[string]any{"FSLIST": `["fs-002"]`}, nil)
	overrideCli.EXPECT().DeactivateQos(ctx, "qos-002", "vstore-standby").Return(nil)
	overrideCli.EXPECT().DeleteQos(ctx, "qos-002", "vstore-standby").Return(nil)

	creator := &BaseCreator{
		cli:         defaultCli,
		isCreateQoS: true,
		qos:         map[string]int{"MAXIOPS": 1000},
	}

	err := creator.RollbackQoS(ctx, "qos-002", "fs-002", "vstore-standby", overrideCli)
	assert.NoError(t, err)
}

func TestBaseCreator_AddQoSTransactionStep_WithOverrideClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultCli := mock_client.NewMockOceanstorClientInterface(ctrl)
	overrideCli := mock_client.NewMockOceanstorClientInterface(ctrl)
	ctx := context.Background()

	// QoS on override client
	overrideCli.EXPECT().CreateQos(ctx, gomock.Any()).Return(
		map[string]any{"ID": "qos-standby", "ENABLESTATUS": "true"}, nil)

	creator := &BaseCreator{
		cli:         defaultCli,
		isCreateQoS: true,
		qos:         map[string]int{"MAXIOPS": 1000},
	}
	creator.transaction = newTransaction()

	fsId := "fs-standby"
	creator.addQoSTransactionStep(ctx, &fsId, "vstore-standby", overrideCli)

	err := creator.transaction.Commit()
	assert.NoError(t, err)
}

func newTransaction() *flow.Transaction {
	return flow.NewTransaction()
}
