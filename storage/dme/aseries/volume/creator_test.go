/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

// Package volume defines operations of volumes
package volume

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging("volumeTest")
	defer log.MockStopLogging("volumeTest")

	m.Run()
}

var (
	fakeFsName         = "test-fs-name"
	fakeFsID           = "test-fs-id"
	fakePoolName       = "test-pool-name"
	fakeShareID        = "test-share-id"
	fakePoolRawID      = "1"
	fakeStorageID      = "aaa"
	fakeAuthClient     = "test-client"
	fakeAllocationType = "thin"
	fakeAuthUser       = "test-user"
	mockErr            = errors.New("mock err")
	fakeCreateNfsModel = &CreateVolumeModel{
		Protocol:           constants.ProtocolNfs,
		SnapshotDirVisible: false,
		Name:               fakeFsName,
		PoolName:           fakePoolName,
		Capacity:           1024 * 1024,
		Description:        "test-description",
		AllSquash:          constants.AllSquashValue,
		RootSquash:         constants.RootSquashValue,
		AllocationType:     fakeAllocationType,
		AuthClients:        []string{fakeAuthClient},
	}
	fakeCreateDtfsModel = &CreateVolumeModel{
		Protocol:           constants.ProtocolDtfs,
		SnapshotDirVisible: false,
		Name:               fakeFsName,
		PoolName:           fakePoolName,
		Capacity:           1024 * 1024,
		Description:        "test-description",
		AllSquash:          constants.AllSquashValue,
		RootSquash:         constants.RootSquashValue,
		AllocationType:     fakeAllocationType,
		AuthUsers:          []string{fakeAuthUser},
	}
)

func TestCreator_CreateWithNfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	handler := &GlobalVolumeHandler{Cli: cli, Name: fakeFsName, Protocol: constants.ProtocolNfs}
	creator := NewCreator(ctx, cli, fakeCreateNfsModel, handler)

	// mock
	pool := &client.HyperScalePool{RawId: fakePoolRawID}
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetHyperScalePoolByName(creator.ctx, creator.params.PoolName).Return(pool, nil)
	cli.EXPECT().GetFileSystemByName(creator.ctx, creator.params.Name).Return(nil, nil).Times(1)
	cli.EXPECT().GetFileSystemByName(creator.ctx, creator.params.Name).Return(fsInfo, nil).Times(1)

	createFsParam := &client.CreateFilesystemParams{
		SnapshotDirVisible: creator.params.SnapshotDirVisible,
		StorageID:          fakeStorageID,
		PoolRawID:          fakePoolRawID,
		ZoneID:             fakeStorageID,
		FilesystemSpecs: []*client.FilesystemSpec{
			{Capacity: transDmeCapacityFromByteIoGb(creator.params.Capacity), Name: fakeFsName, Count: 1,
				Description: creator.params.Description}},
		CreateNfsShareParam: &client.CreateNfsShareParam{
			StorageId:   fakeStorageID,
			SharePath:   creator.params.sharePath(),
			Description: creator.params.Description,
			NfsClientAddition: []*client.NfsClientAddition{
				{Name: creator.params.AuthClients[0], Permission: nfsShareReadWrite,
					WriteMode: nfsShareWriteModeSync, PermissionConstraint: allSquashMap[creator.params.AllSquash],
					RootPermissionConstraint: rootSquashMap[creator.params.RootSquash],
				},
			},
		},
		CreateDpcShareParam: nil,
		Tuning:              &client.Tuning{AllocationType: creator.params.AllocationType},
	}
	cli.EXPECT().CreateFileSystem(creator.ctx, createFsParam).Return(nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID).AnyTimes()
	cli.EXPECT().GetZoneID().Return(fakeStorageID).AnyTimes()
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(creator.ctx, creator.params.sharePath()).Return(nfsShare, nil).AnyTimes()
	cli.EXPECT().SyncDeleteNfsShare(creator.ctx, nfsShare.ID).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeFsName, volume.GetVolumeName())
	assert.Equal(t, fakeFsID, creator.fsId)
}

func TestCreator_CreateWithNfsProtocol_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	handler := &GlobalVolumeHandler{Cli: cli, Name: fakeFsName, Protocol: constants.ProtocolNfs}
	creator := NewCreator(ctx, cli, fakeCreateNfsModel, handler)

	// mock
	pool := &client.HyperScalePool{RawId: fakePoolRawID}
	cli.EXPECT().GetStorageID().Return(fakeStorageID).AnyTimes()
	cli.EXPECT().GetZoneID().Return(fakeStorageID).AnyTimes()
	cli.EXPECT().GetHyperScalePoolByName(creator.ctx, creator.params.PoolName).Return(pool, nil)
	cli.EXPECT().GetFileSystemByName(creator.ctx, creator.params.Name).Return(nil, nil).AnyTimes()
	cli.EXPECT().GetNfsShareByPath(creator.ctx, creator.params.sharePath()).Return(nil, nil).AnyTimes()
	cli.EXPECT().GetDataTurboShareByPath(creator.ctx, creator.params.sharePath()).Return(nil, nil).AnyTimes()
	cli.EXPECT().SyncDeleteNfsShare(creator.ctx, gomock.Any()).Return(nil).AnyTimes()

	createFsParam := &client.CreateFilesystemParams{
		SnapshotDirVisible: creator.params.SnapshotDirVisible,
		StorageID:          fakeStorageID,
		PoolRawID:          fakePoolRawID,
		ZoneID:             fakeStorageID,
		FilesystemSpecs: []*client.FilesystemSpec{
			{Capacity: transDmeCapacityFromByteIoGb(creator.params.Capacity), Name: fakeFsName, Count: 1,
				Description: creator.params.Description}},
		CreateNfsShareParam: &client.CreateNfsShareParam{
			StorageId:   fakeStorageID,
			SharePath:   creator.params.sharePath(),
			Description: creator.params.Description,
			NfsClientAddition: []*client.NfsClientAddition{
				{Name: creator.params.AuthClients[0], Permission: nfsShareReadWrite,
					WriteMode: nfsShareWriteModeSync, PermissionConstraint: allSquashMap[creator.params.AllSquash],
					RootPermissionConstraint: rootSquashMap[creator.params.RootSquash],
				},
			},
		},
		CreateDpcShareParam: nil,
		Tuning:              &client.Tuning{AllocationType: creator.params.AllocationType},
	}
	cli.EXPECT().CreateFileSystem(creator.ctx, createFsParam).Return(mockErr)

	// action
	volume, err := creator.Create()

	// assert
	assert.ErrorIs(t, err, mockErr)
	assert.Nil(t, volume)
}

func TestCreator_CreateWithDtfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	handler := &GlobalVolumeHandler{Cli: cli, Name: fakeFsName, Protocol: constants.ProtocolDtfs}
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel, handler)

	// mock
	pool := &client.HyperScalePool{RawId: fakePoolRawID}
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetHyperScalePoolByName(creator.ctx, creator.params.PoolName).Return(pool, nil)
	cli.EXPECT().GetFileSystemByName(creator.ctx, creator.params.Name).Return(nil, nil).Times(1)
	cli.EXPECT().GetFileSystemByName(creator.ctx, creator.params.Name).Return(fsInfo, nil).Times(1)

	createDtfsParam := &client.CreateFilesystemParams{
		SnapshotDirVisible: creator.params.SnapshotDirVisible,
		StorageID:          fakeStorageID,
		PoolRawID:          fakePoolRawID,
		ZoneID:             fakeStorageID,
		FilesystemSpecs: []*client.FilesystemSpec{
			{Capacity: transDmeCapacityFromByteIoGb(creator.params.Capacity), Name: fakeFsName, Count: 1,
				Description: creator.params.Description}},
		CreateNfsShareParam: nil,
		CreateDpcShareParam: &client.CreateDpcShareParam{Charset: storage.CharsetUtf8,
			Description: creator.params.Description,
			DpcAuth:     []*client.DpcAuth{{DpcUserID: fakeAuthUser, Permission: dpcShareReadWrite}}},
		Tuning: &client.Tuning{AllocationType: creator.params.AllocationType},
	}
	cli.EXPECT().CreateFileSystem(creator.ctx, createDtfsParam).Return(nil)
	cli.EXPECT().GetStorageID().Return(fakeStorageID).AnyTimes()
	cli.EXPECT().GetZoneID().Return(fakeStorageID).AnyTimes()
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(creator.ctx, creator.params.sharePath()).Return(dtfsShare, nil).AnyTimes()
	cli.EXPECT().DeleteDataTurboShare(creator.ctx, dtfsShare.ID).Return(nil)
	adminInfo := &client.DataTurboAdmin{ID: fakeAuthUser}
	cli.EXPECT().GetDataTurboUserByName(creator.ctx, creator.params.AuthUsers[0]).Return(adminInfo, nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeFsName, volume.GetVolumeName())
	assert.Equal(t, fakeFsID, creator.fsId)
}

func TestCreator_CreateWithDtfsProtocol_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	handler := &GlobalVolumeHandler{Cli: cli, Name: fakeFsName, Protocol: constants.ProtocolDtfs}
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel, handler)

	// mock
	pool := &client.HyperScalePool{RawId: fakePoolRawID}
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	adminInfo := &client.DataTurboAdmin{ID: fakeAuthUser}
	cli.EXPECT().GetStorageID().Return(fakeStorageID).AnyTimes()
	cli.EXPECT().GetZoneID().Return(fakeStorageID).AnyTimes()
	cli.EXPECT().GetHyperScalePoolByName(creator.ctx, creator.params.PoolName).Return(pool, nil)
	cli.EXPECT().GetFileSystemByName(creator.ctx, creator.params.Name).Return(nil, nil).AnyTimes()
	cli.EXPECT().GetNfsShareByPath(creator.ctx, creator.params.sharePath()).Return(nil, nil).AnyTimes()
	cli.EXPECT().GetDataTurboShareByPath(creator.ctx, creator.params.sharePath()).Return(dtfsShare, nil).AnyTimes()
	cli.EXPECT().DeleteDataTurboShare(creator.ctx, dtfsShare.ID).Return(nil).AnyTimes()
	cli.EXPECT().GetDataTurboUserByName(creator.ctx, creator.params.AuthUsers[0]).Return(adminInfo, nil)

	createDtfsParam := &client.CreateFilesystemParams{
		SnapshotDirVisible: creator.params.SnapshotDirVisible,
		StorageID:          fakeStorageID,
		PoolRawID:          fakePoolRawID,
		ZoneID:             fakeStorageID,
		FilesystemSpecs: []*client.FilesystemSpec{
			{Capacity: transDmeCapacityFromByteIoGb(creator.params.Capacity), Name: fakeFsName, Count: 1,
				Description: creator.params.Description}},
		CreateNfsShareParam: nil,
		CreateDpcShareParam: &client.CreateDpcShareParam{Charset: storage.CharsetUtf8,
			Description: creator.params.Description,
			DpcAuth:     []*client.DpcAuth{{DpcUserID: fakeAuthUser, Permission: dpcShareReadWrite}}},
		Tuning: &client.Tuning{AllocationType: creator.params.AllocationType},
	}
	cli.EXPECT().CreateFileSystem(creator.ctx, createDtfsParam).Return(mockErr)

	// action
	volume, err := creator.Create()

	// assert
	assert.Error(t, err)
	assert.Nil(t, volume)
	assert.ErrorIs(t, err, mockErr)
}

func Test_validateAndPrepareParams_WithNFS_NoAuthClients(t *testing.T) {
	// arrange
	c := NewCreator(context.Background(), nil, &CreateVolumeModel{
		Protocol:    constants.ProtocolNfs,
		AuthClients: nil,
		AuthUsers:   nil,
	}, nil)
	wantErr := fmt.Errorf("authClient parameter must be provided in StorageClass for nfs protocol")

	// action
	gotErr := c.validateAndPrepareParams()

	// assert
	assert.EqualError(t, gotErr, wantErr.Error())
}

func Test_validateAndPrepareParams_WithNFS_WithAuthClients(t *testing.T) {
	// arrange
	c := NewCreator(context.Background(), nil, &CreateVolumeModel{
		Protocol:    constants.ProtocolNfs,
		AuthClients: []string{"fake-auth-client"},
		AuthUsers:   nil,
	}, nil)

	// mock setPool to return nil
	patches := gomonkey.ApplyPrivateMethod(c, "setPool", func(c *Creator) error {
		return nil
	})
	defer patches.Reset()

	// action
	gotErr := c.validateAndPrepareParams()

	// assert
	assert.NoError(t, gotErr)
}

func Test_validateAndPrepareParams_WithDTFS_NoAuthUsers(t *testing.T) {
	// arrange
	c := NewCreator(context.Background(), nil, &CreateVolumeModel{
		Protocol:    constants.ProtocolDtfs,
		AuthClients: nil,
		AuthUsers:   nil,
	}, nil)
	wantErr := fmt.Errorf("authUser parameter must be provided in StorageClass for dtfs protocol")

	// action
	gotErr := c.validateAndPrepareParams()

	// assert
	assert.EqualError(t, gotErr, wantErr.Error())
}

func Test_validateAndPrepareParams_WithDTFS_WithAuthUsers(t *testing.T) {
	// arrange
	c := NewCreator(context.Background(), nil, &CreateVolumeModel{
		Protocol:    constants.ProtocolDtfs,
		AuthClients: nil,
		AuthUsers:   []string{"fake-auth-user"},
	}, nil)

	// mock setPool to return nil
	patches := gomonkey.ApplyPrivateMethod(c, "setPool", func(c *Creator) error {
		return nil
	})
	defer patches.Reset()

	// action
	gotErr := c.validateAndPrepareParams()

	// assert
	assert.NoError(t, gotErr)
}

func TestCreator_Create_WithKVCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	model := &CreateVolumeModel{
		Protocol:          constants.ProtocolNfs,
		Name:              "test-vol",
		PoolName:          "pool1",
		Capacity:          10 * 1024 * 1024 * 1024,
		EnableKVCache:     true,
		EnableTimeAwareGC: false,
		AuthClients:       []string{"client1"},
		VstoreName:        "myVstore",
	}

	handler := &LocalVolumeHandler{Cli: mockCli}
	mockCli.EXPECT().GetStoragePoolByName(gomock.Any(), "pool1", gomock.Any()).Return(&client.StoragePool{Name: "pool1",
		RawID: "1"}, nil)
	mockCli.EXPECT().GetStorageID().Return("storage-1").AnyTimes()
	mockCli.EXPECT().GetZoneID().Return("zone-1").AnyTimes()
	mockCli.EXPECT().QueryVstores(gomock.Any(), gomock.Any()).Return([]*client.VstoreInfo{{ID: "vstore-1",
		Name: "myVstore"}}, nil)
	mockCli.EXPECT().QueryKVCache(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockCli.EXPECT().CreateKVCache(gomock.Any(), gomock.Any()).Return(&client.KVCacheStore{ID: "kv-id-1"}, nil)

	creator := NewCreator(context.Background(), mockCli, model, handler)
	vol, err := creator.Create()
	assert.NoError(t, err)
	assert.Equal(t, "kv-id-1", vol.GetKvcacheStoreId())
}

func TestCreator_Create_WithKVCache_AlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	model := &CreateVolumeModel{
		Protocol:      constants.ProtocolNfs,
		Name:          "test-vol",
		PoolName:      "pool1",
		Capacity:      10 * 1024 * 1024 * 1024,
		EnableKVCache: true,
		AuthClients:   []string{"client1"},
		VstoreName:    "myVstore",
	}

	handler := &LocalVolumeHandler{Cli: mockCli}
	mockCli.EXPECT().GetStoragePoolByName(gomock.Any(), "pool1", gomock.Any()).Return(&client.StoragePool{Name: "pool1",
		RawID: "1"}, nil)
	mockCli.EXPECT().GetStorageID().Return("storage-1").AnyTimes()
	mockCli.EXPECT().GetZoneID().Return("zone-1").AnyTimes()
	mockCli.EXPECT().QueryVstores(gomock.Any(), gomock.Any()).Return([]*client.VstoreInfo{{ID: "vstore-1",
		Name: "myVstore"}}, nil)
	mockCli.EXPECT().QueryKVCache(gomock.Any(), gomock.Any()).Return(&client.KVCacheStore{ID: "kv-id-1"}, nil)

	creator := NewCreator(context.Background(), mockCli, model, handler)
	vol, err := creator.Create()
	assert.NoError(t, err)
	assert.Equal(t, "kv-id-1", vol.GetKvcacheStoreId())
}

func TestCreator_Create_KVCache_WithoutVstoreName(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	model := &CreateVolumeModel{
		Protocol:      constants.ProtocolNfs,
		Name:          "test-vol",
		PoolName:      "pool1",
		Capacity:      10 * 1024 * 1024 * 1024,
		EnableKVCache: true,
		AuthClients:   []string{"client1"},
		VstoreName:    "",
	}
	model.VstoreName = storage.DefaultVStore

	handler := &LocalVolumeHandler{Cli: mockCli}
	mockCli.EXPECT().GetStoragePoolByName(gomock.Any(), "pool1", gomock.Any()).Return(&client.StoragePool{Name: "pool1",
		RawID: "1"}, nil)
	mockCli.EXPECT().GetStorageID().Return("storage-1").AnyTimes()
	mockCli.EXPECT().GetZoneID().Return("zone-1").AnyTimes()
	mockCli.EXPECT().QueryVstores(gomock.Any(), &client.VstoreQueryParams{
		Name:      storage.DefaultVStore,
		StorageID: "storage-1",
		ZoneID:    "zone-1",
	}).Return([]*client.VstoreInfo{{ID: "vstore-default", Name: storage.DefaultVStore}}, nil)
	mockCli.EXPECT().QueryKVCache(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockCli.EXPECT().CreateKVCache(gomock.Any(), gomock.Any()).Return(&client.KVCacheStore{ID: "kv-id-default"}, nil)

	creator := NewCreator(context.Background(), mockCli, model, handler)
	vol, err := creator.Create()
	assert.NoError(t, err)
	assert.Equal(t, "kv-id-default", vol.GetKvcacheStoreId())
	assert.Equal(t, storage.DefaultVStore, creator.params.VstoreName)
}

func TestCreator_Create_WithoutKVCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	model := &CreateVolumeModel{
		Protocol:      constants.ProtocolNfs,
		Name:          "test-vol",
		PoolName:      "pool1",
		Capacity:      10 * 1024 * 1024 * 1024,
		EnableKVCache: false,
		AuthClients:   []string{"client1"},
	}

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: "test-vol", Protocol: constants.ProtocolNfs}
	mockCli.EXPECT().GetHyperScalePoolByName(gomock.Any(), "pool1").Return(&client.HyperScalePool{Name: "pool1"}, nil)
	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), gomock.Any()).Return(&client.FileSystemInfo{ID: "fs-1"},
		nil).Times(1)
	mockCli.EXPECT().CreateFileSystem(gomock.Any(), gomock.Any()).Return(nil)
	mockCli.EXPECT().GetStorageID().Return("storage-1").AnyTimes()
	mockCli.EXPECT().GetZoneID().Return("storage-1").AnyTimes()
	mockCli.EXPECT().GetNfsShareByPath(gomock.Any(), gomock.Any()).Return(&client.NfsShareInfo{ID: "share-1"},
		nil).AnyTimes()
	mockCli.EXPECT().SyncDeleteNfsShare(gomock.Any(), gomock.Any()).Return(nil)

	creator := NewCreator(context.Background(), mockCli, model, handler)
	vol, err := creator.Create()
	assert.NoError(t, err)
	assert.Equal(t, "", vol.GetKvcacheStoreId())
}

func TestCreator_rollbackKVCache_EmptyStoreId(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	creator := NewCreator(context.Background(), mockCli, &CreateVolumeModel{
		Name: "test-vol",
	}, nil)
	creator.kvcacheStoreId = ""

	// Should not call any CLI methods
	creator.rollbackKVCache()
}

func TestCreator_rollbackKVCache_DeleteKVCacheFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	creator := NewCreator(context.Background(), mockCli, &CreateVolumeModel{
		Name: "test-vol",
	}, nil)
	creator.kvcacheStoreId = "kv-id-1"

	mockCli.EXPECT().DeleteKVCache(gomock.Any(), "kv-id-1").Return(mockErr)

	creator.rollbackKVCache()
}

func TestCreator_Create_KVCache_CreateFailed_TriggersCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	model := &CreateVolumeModel{
		Protocol:      constants.ProtocolNfs,
		Name:          "test-vol",
		PoolName:      "pool1",
		Capacity:      10 * 1024 * 1024 * 1024,
		EnableKVCache: true,
		AuthClients:   []string{"client1"},
		VstoreName:    "myVstore",
	}

	handler := &LocalVolumeHandler{Cli: mockCli}

	// Step 0: validateAndPrepareParams
	mockCli.EXPECT().GetStoragePoolByName(gomock.Any(), "pool1", gomock.Any()).Return(&client.StoragePool{
		Name: "pool1", RawID: "1"}, nil)
	mockCli.EXPECT().GetStorageID().Return("storage-1").AnyTimes()
	mockCli.EXPECT().GetZoneID().Return("zone-1").AnyTimes()
	mockCli.EXPECT().QueryVstores(gomock.Any(), gomock.Any()).Return([]*client.VstoreInfo{
		{ID: "vstore-1", Name: "myVstore"}}, nil)

	// Step 2: createKVCache - idempotency check finds nothing, CreateKVCache fails
	mockCli.EXPECT().QueryKVCache(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockCli.EXPECT().CreateKVCache(gomock.Any(), gomock.Any()).Return(nil, mockErr)

	// Rollback (step 1's onRollback): cleanupKVCacheFilesystem cleans up partial resources
	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), "test-vol").Return(&client.FileSystemInfo{
		ID: "fs-partial"}, nil)
	mockCli.EXPECT().GetNfsShareByPath(gomock.Any(), "/test-vol/").Return(&client.NfsShareInfo{
		ID: "share-partial"}, nil)
	mockCli.EXPECT().DeleteNfsPrivateShare(gomock.Any(), "share-partial").Return(nil)
	mockCli.EXPECT().SyncDeleteFileSystem(gomock.Any(), "fs-partial").Return(nil)

	creator := NewCreator(context.Background(), mockCli, model, handler)
	vol, err := creator.Create()
	assert.Error(t, err)
	assert.Nil(t, vol)
}

func TestCreator_Create_KVCache_CreateFailed_CleanupNoResidual(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	model := &CreateVolumeModel{
		Protocol:      constants.ProtocolNfs,
		Name:          "test-vol",
		PoolName:      "pool1",
		Capacity:      10 * 1024 * 1024 * 1024,
		EnableKVCache: true,
		AuthClients:   []string{"client1"},
		VstoreName:    "myVstore",
	}

	handler := &LocalVolumeHandler{Cli: mockCli}

	// Step 0: validateAndPrepareParams
	mockCli.EXPECT().GetStoragePoolByName(gomock.Any(), "pool1", gomock.Any()).Return(&client.StoragePool{
		Name: "pool1", RawID: "1"}, nil)
	mockCli.EXPECT().GetStorageID().Return("storage-1").AnyTimes()
	mockCli.EXPECT().GetZoneID().Return("zone-1").AnyTimes()
	mockCli.EXPECT().QueryVstores(gomock.Any(), gomock.Any()).Return([]*client.VstoreInfo{
		{ID: "vstore-1", Name: "myVstore"}}, nil)

	// Step 2: createKVCache - idempotency check finds nothing, CreateKVCache fails
	mockCli.EXPECT().QueryKVCache(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockCli.EXPECT().CreateKVCache(gomock.Any(), gomock.Any()).Return(nil, mockErr)

	// Rollback: cleanupKVCacheFilesystem finds no partial resources
	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), "test-vol").Return(nil, nil)

	creator := NewCreator(context.Background(), mockCli, model, handler)
	vol, err := creator.Create()
	assert.Error(t, err)
	assert.Nil(t, vol)
}

func TestCreator_cleanupKVCacheFilesystem_QueryFsFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	creator := NewCreator(context.Background(), mockCli, &CreateVolumeModel{
		Name: "test-vol",
	}, nil)

	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), "test-vol").Return(nil, mockErr)

	creator.cleanupKVCacheFilesystem()
}

func TestCreator_cleanupKVCacheFilesystem_FsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	creator := NewCreator(context.Background(), mockCli, &CreateVolumeModel{
		Name: "test-vol",
	}, nil)

	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), "test-vol").Return(nil, nil)

	creator.cleanupKVCacheFilesystem()
}

func TestCreator_cleanupKVCacheFilesystem_QueryNfsShareFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	creator := NewCreator(context.Background(), mockCli, &CreateVolumeModel{
		Name: "test-vol",
	}, nil)

	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), "test-vol").Return(&client.FileSystemInfo{
		ID: "fs-id-1",
	}, nil)
	mockCli.EXPECT().GetNfsShareByPath(gomock.Any(), "/test-vol/").Return(nil, mockErr)
	mockCli.EXPECT().SyncDeleteFileSystem(gomock.Any(), "fs-id-1").Return(nil)

	creator.cleanupKVCacheFilesystem()
}

func TestCreator_cleanupKVCacheFilesystem_DeleteNfsShareFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	creator := NewCreator(context.Background(), mockCli, &CreateVolumeModel{
		Name: "test-vol",
	}, nil)

	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), "test-vol").Return(&client.FileSystemInfo{
		ID: "fs-id-1",
	}, nil)
	mockCli.EXPECT().GetNfsShareByPath(gomock.Any(), "/test-vol/").Return(&client.NfsShareInfo{
		ID: "share-id-1",
	}, nil)
	mockCli.EXPECT().DeleteNfsPrivateShare(gomock.Any(), "share-id-1").Return(mockErr)
	mockCli.EXPECT().SyncDeleteFileSystem(gomock.Any(), "fs-id-1").Return(nil)

	creator.cleanupKVCacheFilesystem()
}

func TestCreator_cleanupKVCacheFilesystem_DeleteFsFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	creator := NewCreator(context.Background(), mockCli, &CreateVolumeModel{
		Name: "test-vol",
	}, nil)

	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), "test-vol").Return(&client.FileSystemInfo{
		ID: "fs-id-1",
	}, nil)
	mockCli.EXPECT().GetNfsShareByPath(gomock.Any(), "/test-vol/").Return(&client.NfsShareInfo{
		ID: "share-id-1",
	}, nil)
	mockCli.EXPECT().DeleteNfsPrivateShare(gomock.Any(), "share-id-1").Return(nil)
	mockCli.EXPECT().SyncDeleteFileSystem(gomock.Any(), "fs-id-1").Return(mockErr)

	creator.cleanupKVCacheFilesystem()
}

func TestCreator_cleanupKVCacheFilesystem_NfsShareNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)

	creator := NewCreator(context.Background(), mockCli, &CreateVolumeModel{
		Name: "test-vol",
	}, nil)

	mockCli.EXPECT().GetFileSystemByName(gomock.Any(), "test-vol").Return(&client.FileSystemInfo{
		ID: "fs-id-1",
	}, nil)
	mockCli.EXPECT().GetNfsShareByPath(gomock.Any(), "/test-vol/").Return(nil, nil)
	mockCli.EXPECT().SyncDeleteFileSystem(gomock.Any(), "fs-id-1").Return(nil)

	creator.cleanupKVCacheFilesystem()
}
