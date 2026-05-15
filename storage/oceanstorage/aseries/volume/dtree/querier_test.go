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

package dtree

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestNewQuerier(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	assert.NotNil(t, querier)
	assert.Equal(t, ctx, querier.ctx)
	assert.Equal(t, cli, querier.cli)
	assert.Equal(t, fakeDtreeName, querier.dtreeName)
	assert.Equal(t, fakeParentName, querier.parentName)
}

func TestQuerier_Query_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeQuotaID, "SPACEHARDQUOTA": "1048576"}, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeDtreeName, volume.GetVolumeName())
	assert.Equal(t, int64(1048576), volume.GetSize())
	assert.Equal(t, fakeDtreeID, volume.GetID())
}

func TestQuerier_Query_DTreeNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorContains(t, err, "is not exist")
	assert.Nil(t, volume)
}

func TestQuerier_Query_QuotaNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorContains(t, err, "quota")
	assert.Nil(t, volume)
}

func TestQuerier_Query_QuotaZeroCapacity(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeQuotaID, "SPACEHARDQUOTA": "0"}, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorContains(t, err, "is 0")
	assert.Nil(t, volume)
}

func TestQuerier_Query_QuotaEmptyString(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeQuotaID, "SPACEHARDQUOTA": ""}, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorContains(t, err, "not exist or empty")
	assert.Nil(t, volume)
}

func TestQuerier_Query_GetDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, mockErr)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorIs(t, err, mockErr)
	assert.Nil(t, volume)
}

func TestQuerier_Query_GetDTreeQuotaError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, mockErr)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorIs(t, err, mockErr)
	assert.Nil(t, volume)
}

func TestQuerier_Query_DTreeNoID(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock - DTree found but no ID
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"NAME": fakeDtreeName}, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorContains(t, err, "get DTree ID failed")
	assert.Nil(t, volume)
}
