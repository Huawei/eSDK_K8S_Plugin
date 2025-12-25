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
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

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
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(creator.ctx, creator.params.sharePath()).Return(nfsShare, nil).AnyTimes()
	cli.EXPECT().DeleteNfsShare(creator.ctx, nfsShare.ID).Return(nil)

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
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	pool := &client.HyperScalePool{RawId: fakePoolRawID}
	cli.EXPECT().GetHyperScalePoolByName(creator.ctx, creator.params.PoolName).Return(pool, nil)
	cli.EXPECT().GetFileSystemByName(creator.ctx, creator.params.Name).Return(nil, nil).Times(2)

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
	cli.EXPECT().GetStorageID().Return(fakeStorageID).AnyTimes()
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(creator.ctx, creator.params.sharePath()).Return(nfsShare, nil).Times(2)
	cli.EXPECT().DeleteNfsShare(creator.ctx, nfsShare.ID).Return(nil).Times(2)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(creator.ctx, creator.params.sharePath()).Return(dtfsShare, nil).AnyTimes()
	cli.EXPECT().DeleteDataTurboShare(creator.ctx, dtfsShare.ID).Return(nil)

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
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel)

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
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel)

	// mock
	pool := &client.HyperScalePool{RawId: fakePoolRawID}
	cli.EXPECT().GetHyperScalePoolByName(creator.ctx, creator.params.PoolName).Return(pool, nil)
	cli.EXPECT().GetFileSystemByName(creator.ctx, creator.params.Name).Return(nil, nil).Times(2)

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
	cli.EXPECT().GetStorageID().Return(fakeStorageID).AnyTimes()
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(creator.ctx, creator.params.sharePath()).Return(nfsShare, nil).AnyTimes()
	cli.EXPECT().DeleteNfsShare(creator.ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(creator.ctx, creator.params.sharePath()).Return(dtfsShare, nil).Times(2)
	cli.EXPECT().DeleteDataTurboShare(creator.ctx, dtfsShare.ID).Return(nil).Times(2)
	adminInfo := &client.DataTurboAdmin{ID: fakeAuthUser}
	cli.EXPECT().GetDataTurboUserByName(creator.ctx, creator.params.AuthUsers[0]).Return(adminInfo, nil)

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
	})
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
	})

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
	})
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
	})

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
