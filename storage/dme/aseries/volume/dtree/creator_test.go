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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestCreator_Create_Success_NFS(t *testing.T) {
	// arrange — task flow: checkParentFS → createDTree → createQuota → createNfsShare
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		Protocol:     "nfs",
		DTreeName:    fakeDtreeName,
		ParentName:   fakeParentName,
		AuthClients:  []string{fakeAuthClient},
		AllSquash:    "all_squash",
		RootSquash:   "no_root_squash",
		FsPermission: "750",
		Capacity:     fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID:    fakeDtreeID,
		DtreeRawID: fakeDtreeRawID,
		DtreeName:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	cli.EXPECT().GetNfsShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx, gomock.Any()).Return(&client.DTreeNfsShareInfo{ID: fakeNfsShareID}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, fakeDtreeName, vol.GetVolumeName())
	assert.Equal(t, fakeDtreeID, vol.GetID())
	assert.Equal(t, fakeCapacity, vol.GetSize())

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_Success_DataTurbo(t *testing.T) {
	// arrange — task flow: checkParentFS → createDTree → createQuota → createDpcShare
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		Protocol:   "dtfs",
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		AuthUsers:  []string{fakeAuthUser},
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID:    fakeDtreeID,
		DtreeRawID: fakeDtreeRawID,
		DtreeName:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetDataTurboUserByName(ctx, fakeAuthUser).Return(&client.DataTurboAdmin{ID: "user-001"}, nil)
	cli.EXPECT().CreateDTreeDpcShare(ctx, gomock.Any()).Return(&client.DTreeDpcShareInfo{ID: fakeDpcShareID}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, fakeDtreeID, vol.GetID())

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_QuotaMatch_NoShare(t *testing.T) {
	// arrange — dtree exists, quota matches, no authClients/authUsers
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

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
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, fakeDtreeID, vol.GetID())

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_QuotaMismatch(t *testing.T) {
	// arrange — dtree exists, quota capacity mismatch → no rollback (we didn't create dtree or quota)
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:    fakeDtreeID,
		RawID: fakeDtreeRawID,
		Name:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(&client.QuotaInfo{
		ID:             fakeQuotaID,
		SpaceHardQuota: 999, // mismatched
	}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.Error(t, err)
	assert.ErrorContains(t, err, "quota capacity")
	assert.ErrorContains(t, err, "does not match expected")
	assert.Nil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_QuotaNotExist_CreateQuota(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:    fakeDtreeID,
		RawID: fakeDtreeRawID,
		Name:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, fakeDtreeID, vol.GetID())

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_QuotaNotExist_CreateQuotaByteUnit(t *testing.T) {
	// Ensure standalone quota creation uses Byte (not KB conversion)
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	capacityInBytes := int64(1073741824) // 1 GiB
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   capacityInBytes,
	}
	creator := NewCreator(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:    fakeDtreeID,
		RawID: fakeDtreeRawID,
		Name:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, p *client.CreateQuotaParams) (*client.QuotaInfo, error) {
			// Standalone quota API uses Byte, NOT KB
			assert.Equal(t, capacityInBytes, p.SpaceHardQuota)
			return &client.QuotaInfo{ID: fakeQuotaID}, nil
		})

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_NfsShareNotExist_CreateShare(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:   fakeDtreeName,
		ParentName:  fakeParentName,
		AuthClients: []string{fakeAuthClient},
		AllSquash:   "all_squash",
		RootSquash:  "no_root_squash",
		Capacity:    fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

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
	cli.EXPECT().GetNfsShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx, gomock.Any()).Return(&client.DTreeNfsShareInfo{ID: fakeNfsShareID}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, fakeDtreeID, vol.GetID())

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_NfsShareExist_DeleteAndRecreate(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:   fakeDtreeName,
		ParentName:  fakeParentName,
		AuthClients: []string{fakeAuthClient},
		AllSquash:   "all_squash",
		RootSquash:  "no_root_squash",
		Capacity:    fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

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
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(&client.DTreeNfsShareInfo{
		ID:        fakeNfsShareID,
		SharePath: sharePath,
	}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeNfsShareID).Return(nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx, gomock.Any()).Return(&client.DTreeNfsShareInfo{ID: "new-nfs-id"}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_DpcShareNotExist_CreateShare(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		AuthUsers:  []string{fakeAuthUser},
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

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
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)
	cli.EXPECT().GetDataTurboUserByName(ctx, fakeAuthUser).Return(&client.DataTurboAdmin{ID: "user-001"}, nil)
	cli.EXPECT().CreateDTreeDpcShare(ctx, gomock.Any()).Return(&client.DTreeDpcShareInfo{ID: fakeDpcShareID}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_DpcShareExist_DeleteAndRecreate(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		AuthUsers:  []string{fakeAuthUser},
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

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
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(&client.DTreeDpcShareInfo{
		ID:        fakeDpcShareID,
		SharePath: sharePath,
	}, nil)
	cli.EXPECT().DeleteDTreeDataTurboShare(ctx, fakeDpcShareID).Return(nil)
	cli.EXPECT().GetDataTurboUserByName(ctx, fakeAuthUser).Return(&client.DataTurboAdmin{ID: "user-001"}, nil)
	cli.EXPECT().CreateDTreeDpcShare(ctx, gomock.Any()).Return(&client.DTreeDpcShareInfo{ID: "new-dpc-id"}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_AlreadyExists_FullIdempotentRepair(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:   fakeDtreeName,
		ParentName:  fakeParentName,
		AuthClients: []string{fakeAuthClient},
		AuthUsers:   []string{fakeAuthUser},
		AllSquash:   "all_squash",
		RootSquash:  "no_root_squash",
		Capacity:    fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(&client.DTreeInfo{
		ID:    fakeDtreeID,
		RawID: fakeDtreeRawID,
		Name:  fakeDtreeName,
	}, nil)
	// Quota not exists → create
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	// NFS share exists → delete + recreate
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetNfsShareByDTreePath(ctx, sharePath).Return(&client.DTreeNfsShareInfo{
		ID: fakeNfsShareID, SharePath: sharePath,
	}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeNfsShareID).Return(nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx, gomock.Any()).Return(&client.DTreeNfsShareInfo{ID: "new-nfs"}, nil)
	// DPC share not exists → create
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, sharePath).Return(nil, nil)
	cli.EXPECT().GetDataTurboUserByName(ctx, fakeAuthUser).Return(&client.DataTurboAdmin{ID: "user-001"}, nil)
	cli.EXPECT().CreateDTreeDpcShare(ctx, gomock.Any()).Return(&client.DTreeDpcShareInfo{ID: fakeDpcShareID}, nil)

	// action
	vol, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, vol)
	assert.Equal(t, fakeDtreeID, vol.GetID())

	// clean
	mockCtrl.Finish()
}

func TestCreator_Create_ParentFSNotExist(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, nil)

	vol, err := creator.Create()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "parent filesystem "+fakeParentName+" does not exist")
	assert.Nil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_QueryParentFSError(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, dtreeMockErr)

	vol, err := creator.Create()

	assert.Error(t, err)
	assert.ErrorIs(t, err, dtreeMockErr)
	assert.Nil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_QueryExistingDTreeError(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, dtreeMockErr)

	vol, err := creator.Create()

	assert.ErrorIs(t, err, dtreeMockErr)
	assert.Nil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_CreateDTreeError(t *testing.T) {
	// arrange — createDTree fails, no dtreeId set → rollbackDTree is no-op
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(nil, dtreeMockErr)

	// rollback: checkParentFS has no rollback, createDTree has no dtreeId → no-op

	vol, err := creator.Create()

	assert.ErrorIs(t, err, dtreeMockErr)
	assert.Nil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_CreateDTreeSuccess_QuotaCreateError(t *testing.T) {
	// arrange — DTree created, quota creation fails → rollback quota(no-op) + dtree
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID:    fakeDtreeID,
		DtreeRawID: fakeDtreeRawID,
		DtreeName:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(nil, dtreeMockErr)

	// rollback: quotaId is "" → no-op; dtreeId is set → delete
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(nil)

	vol, err := creator.Create()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "create quota for DTree "+fakeDtreeName+" failed")
	assert.Nil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_CreateDTreeSuccess_NfsShareCreateError(t *testing.T) {
	// arrange — DTree + quota created, NFS share creation fails → rollback NFS(no-op) + quota + dtree
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:   fakeDtreeName,
		ParentName:  fakeParentName,
		AuthClients: []string{fakeAuthClient},
		AllSquash:   "all_squash",
		RootSquash:  "no_root_squash",
		Capacity:    fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID:    fakeDtreeID,
		DtreeRawID: fakeDtreeRawID,
		DtreeName:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	cli.EXPECT().GetNfsShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx, gomock.Any()).Return(nil, dtreeMockErr)

	// rollback: nfsShareId "" → no-op; quotaId set → delete; dtreeId set → delete
	cli.EXPECT().DeleteDTreeQuota(ctx, fakeQuotaID).Return(nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(nil)

	vol, err := creator.Create()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "create NFS share for DTree "+fakeDtreeName+" failed")
	assert.Nil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_DataTurboUserError(t *testing.T) {
	// arrange — DTree + quota created, DPC share fails on user query → rollback quota + dtree
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		Protocol:   "dtfs",
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		AuthUsers:  []string{fakeAuthUser},
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID:    fakeDtreeID,
		DtreeRawID: fakeDtreeRawID,
		DtreeName:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetDataTurboUserByName(ctx, fakeAuthUser).Return(nil, dtreeMockErr)

	// rollback: dpcShareId "" → no-op; quotaId set → delete; dtreeId set → delete
	cli.EXPECT().DeleteDTreeQuota(ctx, fakeQuotaID).Return(nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(nil)

	vol, err := creator.Create()

	assert.ErrorContains(t, err, "get DataTurbo user "+fakeAuthUser+" failed")
	assert.Nil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_DataTurboUserNotExist(t *testing.T) {
	// arrange — DTree + quota created, DPC share fails on user not found → rollback quota + dtree
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		Protocol:   "dtfs",
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		AuthUsers:  []string{fakeAuthUser},
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID:    fakeDtreeID,
		DtreeRawID: fakeDtreeRawID,
		DtreeName:  fakeDtreeName,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetDataTurboUserByName(ctx, fakeAuthUser).Return(nil, nil)

	// rollback: quotaId → delete; dtreeId → delete
	cli.EXPECT().DeleteDTreeQuota(ctx, fakeQuotaID).Return(nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(nil)

	vol, err := creator.Create()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "dataTurbo user "+fakeAuthUser+" does not exist")
	assert.Nil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_NoShareNoAuth(t *testing.T) {
	// arrange — create dtree + quota only, no authClients/authUsers
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:  fakeDtreeName,
		ParentName: fakeParentName,
		Capacity:   fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, p *client.CreateDTreeParams) (*client.CreateDTreeResponse, error) {
			assert.Nil(t, p.CreateNfsShareParam)
			assert.Nil(t, p.DataturboShare)
			assert.Nil(t, p.CreateQuotaParam)
			assert.True(t, p.QuotaSwitch)
			return &client.CreateDTreeResponse{DtreeID: fakeDtreeID, DtreeRawID: fakeDtreeRawID}, nil
		})
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)

	vol, err := creator.Create()

	assert.NoError(t, err)
	assert.NotNil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_NfsShareParams(t *testing.T) {
	// arrange — verify NFS share creation params in task flow
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		Protocol:     "nfs",
		DTreeName:    fakeDtreeName,
		ParentName:   fakeParentName,
		AuthClients:  []string{fakeAuthClient},
		AllSquash:    "all_squash",
		RootSquash:   "no_root_squash",
		FsPermission: "750",
		Capacity:     fakeCapacity,
		Description:  "my NFS desc",
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID: fakeDtreeID, DtreeRawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	cli.EXPECT().GetNfsShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, p client.CreateNfsShareRequestBody) (*client.DTreeNfsShareInfo, error) {
			assert.Equal(t, "/"+fakeParentName+"/"+fakeDtreeName, p.CreateNfsShareParam.SharePath)
			assert.Equal(t, fakeFsID, p.CreateNfsShareParam.FsID)
			assert.Equal(t, "my NFS desc", p.CreateNfsShareParam.Description)
			assert.Len(t, p.CreateNfsShareParam.NfsClientAddition, 1)
			assert.Equal(t, fakeAuthClient, p.CreateNfsShareParam.NfsClientAddition[0].Name)
			assert.Equal(t, nfsShareReadWriteStandalone, p.CreateNfsShareParam.NfsClientAddition[0].Permission)
			assert.Equal(t, nfsShareWriteModeSync, p.CreateNfsShareParam.NfsClientAddition[0].WriteMode)
			return &client.DTreeNfsShareInfo{ID: fakeNfsShareID}, nil
		})

	vol, err := creator.Create()

	assert.NoError(t, err)
	assert.NotNil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_DpcShareParams(t *testing.T) {
	// arrange — verify DPC share creation params in task flow
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		Protocol:    "dtfs",
		DTreeName:   fakeDtreeName,
		ParentName:  fakeParentName,
		AuthUsers:   []string{fakeAuthUser},
		Capacity:    fakeCapacity,
		Description: "my DPC desc",
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID: fakeDtreeID, DtreeRawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetDataTurboUserByName(ctx, fakeAuthUser).Return(&client.DataTurboAdmin{ID: "user-001"}, nil)
	cli.EXPECT().CreateDTreeDpcShare(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, p client.CreateDpcShareParams) (*client.DTreeDpcShareInfo, error) {
			assert.Equal(t, fakeFsID, p.FsID)
			assert.Equal(t, fakeDtreeID, p.DtreeID)
			assert.Equal(t, "my DPC desc", p.Description)
			assert.Equal(t, storage.CharsetUtf8, p.Charset)
			assert.Len(t, p.DpcShareAuth, 1)
			assert.Equal(t, "user-001", p.DpcShareAuth[0].DpcUserID)
			assert.Equal(t, dpcShareReadWrite, p.DpcShareAuth[0].Permission)
			return &client.DTreeDpcShareInfo{ID: fakeDpcShareID}, nil
		})

	vol, err := creator.Create()

	assert.NoError(t, err)
	assert.NotNil(t, vol)

	mockCtrl.Finish()
}

func TestCreator_Create_DpcShareCreateError_RollbackAll(t *testing.T) {
	// arrange — DTree + quota + NFS share created, DPC share fails → rollback all
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	params := &CreateDTreeVolumeModel{
		DTreeName:   fakeDtreeName,
		ParentName:  fakeParentName,
		AuthClients: []string{fakeAuthClient},
		AuthUsers:   []string{fakeAuthUser},
		AllSquash:   "all_squash",
		RootSquash:  "no_root_squash",
		Capacity:    fakeCapacity,
	}
	creator := NewCreator(ctx, cli, params)

	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeFsID, fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(&client.CreateDTreeResponse{
		DtreeID: fakeDtreeID, DtreeRawID: fakeDtreeRawID,
	}, nil)
	cli.EXPECT().GetDTreeQuotaByRawID(ctx, fakeDtreeRawID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(&client.QuotaInfo{ID: fakeQuotaID}, nil)
	cli.EXPECT().GetNfsShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx, gomock.Any()).Return(&client.DTreeNfsShareInfo{ID: fakeNfsShareID}, nil)
	cli.EXPECT().GetDataTurboShareByDTreePath(ctx, "/"+fakeParentName+"/"+fakeDtreeName).Return(nil, nil)
	cli.EXPECT().GetDataTurboUserByName(ctx, fakeAuthUser).Return(&client.DataTurboAdmin{ID: "user-001"}, nil)
	cli.EXPECT().CreateDTreeDpcShare(ctx, gomock.Any()).Return(nil, dtreeMockErr)

	// rollback: dpcShareId "" → no-op; nfsShareId → delete; quotaId → delete; dtreeId → delete
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeNfsShareID).Return(nil)
	cli.EXPECT().DeleteDTreeQuota(ctx, fakeQuotaID).Return(nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeDtreeID).Return(nil)

	vol, err := creator.Create()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "create DPC share for DTree "+fakeDtreeName+" failed")
	assert.Nil(t, vol)

	mockCtrl.Finish()
}
