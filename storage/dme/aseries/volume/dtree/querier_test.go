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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging("dtreeTest")
	defer log.MockStopLogging("dtreeTest")
	m.Run()
}

const (
	fakeParentName = "parent-fs"
	fakeDtreeName  = "pvc-dtree"
	fakeFsID       = "fs-001"
	fakeDtreeID    = "dtree-001"
	fakeDtreeRawID = "raw-001"
	fakeStorageID  = "storage-001"
	fakeQuotaID    = "quota-001"
	fakeCapacity   = int64(1024 * 1024 * 1024) // 1 GiB in bytes
	fakeNfsShareID = "nfs-001"
	fakeDpcShareID = "dpc-001"
	fakeAuthClient = "10.0.0.1"
	fakeAuthUser   = "admin"
)

var (
	dtreeMockErr = errors.New("mock err")
)

func TestQuerier_Query_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:    fakeDtreeID,
		RawID: fakeDtreeRawID,
		Name:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(&client.QuotaInfo{
		ID:             fakeQuotaID,
		SpaceHardQuota: fakeCapacity,
	}, nil)

	// action
	vol, err := querier.Query()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, fakeDtreeName, vol.GetVolumeName())
	assert.Equal(t, fakeDtreeID, vol.GetID())
	assert.Equal(t, fakeCapacity, vol.GetSize())

	// clean
	mockCtrl.Finish()
}

func TestQuerier_Query_Success_NoQuota(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:    fakeDtreeID,
		RawID: fakeDtreeRawID,
		Name:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)

	// action
	vol, err := querier.Query()

	// assert
	assert.Error(t, err)
	assert.Nil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestQuerier_Query_ParentFSNotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, nil)

	// action
	vol, err := querier.Query()

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent filesystem "+fakeParentName+" does not exist")
	assert.Nil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestQuerier_Query_DTreeNotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)

	// action
	vol, err := querier.Query()

	// assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dtree "+fakeDtreeName+" of parent "+fakeParentName+" does not exist")
	assert.Nil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestQuerier_Query_QueryParentFSError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, dtreeMockErr)

	// action
	vol, err := querier.Query()

	// assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, dtreeMockErr)
	assert.Nil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestQuerier_Query_QueryDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, dtreeMockErr)

	// action
	vol, err := querier.Query()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)
	assert.Nil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestQuerier_Query_QueryQuotaError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDtreeName, fakeParentName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID: fakeDtreeID, RawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, dtreeMockErr)

	// action
	vol, err := querier.Query()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)
	assert.Nil(t, vol)

	// clean
	mockCtrl.Finish()
}
