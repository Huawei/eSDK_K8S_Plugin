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

func TestDeleter_Delete_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1: delete NFS share
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(&client.DTreeNfsShareInfo{
		ID:        fakeNfsShareID,
		SharePath: sharePath,
	}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeNfsShareID).Return(nil)

	// mock — Step 2: delete DataTurbo share
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(&client.DTreeDpcShareInfo{
		ID:        fakeDpcShareID,
		SharePath: sharePath,
	}, nil)
	cli.EXPECT().DeleteDTreeDataTurboShare(ctx, fakeDpcShareID).Return(nil)

	// mock — Step 3: delete DTree
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:   fakeDtreeID,
		Name: fakeDtreeName,
	}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_NoNfsShare(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1: NFS share not found, skip
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(nil, nil)

	// mock — Step 2: DataTurbo share not found, skip
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)

	// mock — Step 3: delete DTree
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID: fakeDtreeID, Name: fakeDtreeName,
	}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_NoDataTurboShare(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1: delete NFS share
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(&client.DTreeNfsShareInfo{
		ID: fakeNfsShareID,
	}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeNfsShareID).Return(nil)

	// mock — Step 2: DataTurbo share not found, skip
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)

	// mock — Step 3: delete DTree
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID: fakeDtreeID, Name: fakeDtreeName,
	}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_ParentFSNotExist(t *testing.T) {
	// arrange — parent FS already deleted, DTree considered cleaned up
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1 & 2: no shares
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(nil, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)

	// mock — Step 3: parent FS not found
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_DTreeNotExist(t *testing.T) {
	// arrange — DTree already deleted
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1 & 2: no shares
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(nil, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)

	// mock — Step 3: DTree not found
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_QueryNfsShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1: query NFS share fails
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(nil, dtreeMockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_DeleteNfsShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1: NFS share found but delete fails
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(&client.DTreeNfsShareInfo{
		ID: fakeNfsShareID,
	}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeNfsShareID).Return(dtreeMockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_QueryDataTurboShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1: NFS share deleted successfully
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(&client.DTreeNfsShareInfo{
		ID: fakeNfsShareID,
	}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeNfsShareID).Return(nil)

	// mock — Step 2: query DataTurbo share fails
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, dtreeMockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_DeleteDataTurboShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1: NFS share deleted
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(&client.DTreeNfsShareInfo{
		ID: fakeNfsShareID,
	}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeNfsShareID).Return(nil)

	// mock — Step 2: DataTurbo share found but delete fails
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(&client.DTreeDpcShareInfo{
		ID: fakeDpcShareID,
	}, nil)
	cli.EXPECT().DeleteDTreeDataTurboShare(ctx, fakeDpcShareID).Return(dtreeMockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_QueryParentFSError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1 & 2: no shares
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(nil, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)

	// mock — Step 3: query parent FS fails
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, dtreeMockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_QueryDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1 & 2: no shares
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(nil, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)

	// mock — Step 3: query DTree fails
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, dtreeMockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}

func TestDeleter_Delete_DeleteDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName)

	// mock — Step 1 & 2: no shares
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(nil, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)

	// mock — Step 3: delete DTree fails
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID: fakeDtreeID, Name: fakeDtreeName,
	}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(dtreeMockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, dtreeMockErr)

	// clean
	mockCtrl.Finish()
}
