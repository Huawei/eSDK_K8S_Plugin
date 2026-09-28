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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestExpander_Expand_UpdateExistingQuota(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:    fakeDtreeID,
		RawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(&client.QuotaInfo{
		ID:             fakeQuotaID,
		SpaceHardQuota: 536870912,
	}, nil)
	cli.EXPECT().UpdateDTreeQuota(ctx, fakeQuotaID, &client.UpdateQuotaParams{
		SpaceHardQuota: fakeCapacity,
	}).Return(nil)

	// action
	err := expander.Expand()

	// assert
	assert.NoError(t, err)

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_CreateNewQuota(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:    fakeDtreeID,
		RawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, &client.CreateQuotaParams{
		ParentID:       fakeDtreeID,
		ParentType:     parentType,
		QuotaType:      directoryQuota,
		SpaceHardQuota: fakeCapacity,
	}).Return(&client.QuotaInfo{ID: "new-quota"}, nil)

	// action
	err := expander.Expand()

	// assert
	assert.NoError(t, err)

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_ParentFSNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, nil)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorContains(t, err, "parent filesystem "+fakeParentName+" does not exist")

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_DTreeNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorContains(t, err, "DTree "+fakeDtreeName+" does not exist")

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_QueryParentFSError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, dtreeMockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_QueryDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, dtreeMockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_QueryQuotaError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID: fakeDtreeID, RawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, dtreeMockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_UpdateQuotaError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID: fakeDtreeID, RawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(&client.QuotaInfo{
		ID: fakeQuotaID, SpaceHardQuota: 536870912,
	}, nil)
	cli.EXPECT().UpdateDTreeQuota(ctx, fakeQuotaID, &client.UpdateQuotaParams{
		SpaceHardQuota: fakeCapacity,
	}).Return(dtreeMockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_CreateQuotaError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   fakeCapacity,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID: fakeDtreeID, RawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(nil, dtreeMockErr)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestExpander_Expand_QuotaUsesByteUnit(t *testing.T) {
	// arrange — verify Expand uses Byte unit (not KB like CreateDTree)
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	capacityInBytes := int64(2147483648) // 2 GiB
	params := &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDtreeName,
		Capacity:   capacityInBytes,
	}
	expander := NewExpander(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID: fakeDtreeID, RawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(&client.QuotaInfo{
		ID: fakeQuotaID, SpaceHardQuota: 1073741824,
	}, nil)
	cli.EXPECT().UpdateDTreeQuota(ctx, fakeQuotaID, &client.UpdateQuotaParams{
		SpaceHardQuota: capacityInBytes, // bytes, not KB
	}).Return(nil)

	// action
	err := expander.Expand()

	// assert
	assert.NoError(t, err)

	// clean
	mockCtrl.Finish()
}
