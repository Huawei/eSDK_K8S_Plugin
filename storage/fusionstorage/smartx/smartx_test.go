/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2025. All rights reserved.
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

package smartx

import (
	"context"
	"errors"
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func TestMain(m *testing.M) {
	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	log.MockInitLogging("smartx_test.log")
	defer log.MockStopLogging("smartx_test.log")

	m.Run()
}

func TestQoS_AddQoS_RevertOnAssociateFailure(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockIRestClient(mockCtrl)
	qos := NewQoS(cli)

	volName := "test-volume"
	params := map[string]int{"IOPS": 1000}

	associateErr := errors.New("associate qos failed")
	removeQoSErr := errors.New("remove qos failed")

	// mock
	cli.EXPECT().CreateQoS(ctx, gomock.Any(), gomock.Any()).Return(nil)
	cli.EXPECT().AssociateQoSWithVolume(ctx, volName, gomock.Any()).Return(associateErr)
	cli.EXPECT().GetQoSNameByVolume(ctx, volName).Return("qos-name", nil)
	cli.EXPECT().DisassociateQoSWithVolume(ctx, volName, "qos-name").Return(removeQoSErr)

	// action
	result, err := qos.AddQoS(ctx, volName, params)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remove qos failed")
	assert.Empty(t, result)
}

func TestQoS_AddQoS_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockIRestClient(mockCtrl)
	qos := NewQoS(cli)

	volName := "test-volume"
	params := map[string]int{"IOPS": 1000}

	// mock
	cli.EXPECT().CreateQoS(ctx, gomock.Any(), gomock.Any()).Return(nil)
	cli.EXPECT().AssociateQoSWithVolume(ctx, volName, gomock.Any()).Return(nil)

	// action
	result, err := qos.AddQoS(ctx, volName, params)

	// assert
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestQoS_AddQoS_CreateQoSFailure(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockIRestClient(mockCtrl)
	qos := NewQoS(cli)

	volName := "test-volume"
	params := map[string]int{"IOPS": 1000}
	createErr := errors.New("create qos failed")

	// mock
	cli.EXPECT().CreateQoS(ctx, gomock.Any(), gomock.Any()).Return(createErr)

	// action
	result, err := qos.AddQoS(ctx, volName, params)

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create qos")
	assert.Empty(t, result)
}
