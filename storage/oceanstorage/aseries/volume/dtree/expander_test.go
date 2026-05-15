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

func TestNewExpander(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}

	expander := NewExpander(ctx, cli, params)

	assert.NotNil(t, expander)
	assert.Equal(t, ctx, expander.ctx)
	assert.Equal(t, cli, expander.cli)
	assert.Equal(t, params, expander.params)
}

func TestExpander_Expand_UpdateQuota_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   2 * fakeCapacity, // expand to 2x
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeQuotaID, "SPACEHARDQUOTA": "1048576"}, nil)
	cli.EXPECT().UpdateDTreeQuota(ctx, fakeQuotaID, gomock.Any()).Return(nil)

	// action
	err := expander.Expand()

	// assert
	assert.NoError(t, err)
}

func TestExpander_Expand_CreateQuota_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock - quota does not exist, need to create
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeQuotaID}, nil)

	// action
	err := expander.Expand()

	// assert
	assert.NoError(t, err)
}

func TestExpander_Expand_DTreeNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorContains(t, err, "to be expanded does not exist")
}

func TestExpander_Expand_CapacityDecrease(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   100, // less than existing quota
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeQuotaID, "SPACEHARDQUOTA": "1048576"}, nil)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorContains(t, err, "must be greater than the current capacity")
}

func TestExpander_Expand_GetDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, mockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestExpander_Expand_GetDTreeQuotaError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, mockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestExpander_Expand_UpdateQuotaError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   2 * fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeQuotaID, "SPACEHARDQUOTA": "1048576"}, nil)
	cli.EXPECT().UpdateDTreeQuota(ctx, fakeQuotaID, gomock.Any()).Return(mockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestExpander_Expand_CreateQuotaError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock - quota does not exist, create fails
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(nil, mockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestExpander_Expand_QuotaNoID(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   2 * fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock - quota exists but has no ID
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"SPACEHARDQUOTA": "1048576"}, nil) // no "ID" field

	// action
	err := expander.Expand()

	// assert
	assert.ErrorContains(t, err, "get quota ID failed")
}

func TestExpander_Expand_DTreeNoID(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock - DTree found but has no ID
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"NAME": fakeDtreeName}, nil) // no "ID" field

	// action
	err := expander.Expand()

	// assert
	assert.ErrorContains(t, err, "get DTree ID failed")
}
