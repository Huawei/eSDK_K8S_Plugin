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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestDeleter_Delete_GlobalNfs_Success(t *testing.T) {
	// arrange
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	ctx := context.Background()
	handler := &GlobalVolumeHandler{Cli: cli, Name: fakeFsName, Protocol: constants.ProtocolNfs}
	deleter := NewDeleter(ctx, handler)

	// mock - GlobalVolumeHandler.Delete calls
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nfsShare, nil)
	cli.EXPECT().DeleteNfsShare(ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(dtfsShare, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx, dtfsShare.ID).Return(nil)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName).Return(fsInfo, nil)
	cli.EXPECT().DeleteFileSystem(ctx, fsInfo.ID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestDeleter_Delete_GlobalNfs_Error(t *testing.T) {
	// arrange
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	ctx := context.Background()
	handler := &GlobalVolumeHandler{Cli: cli, Name: fakeFsName, Protocol: constants.ProtocolNfs}
	deleter := NewDeleter(ctx, handler)

	// mock - GlobalVolumeHandler.Delete calls
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nfsShare, nil)
	cli.EXPECT().DeleteNfsShare(ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(dtfsShare, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx, dtfsShare.ID).Return(nil)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName).Return(fsInfo, nil)
	cli.EXPECT().DeleteFileSystem(ctx, fsInfo.ID).Return(mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_Delete_GlobalDtfs_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	handler := &GlobalVolumeHandler{Cli: cli, Name: fakeFsName, Protocol: constants.ProtocolDtfs}
	deleter := NewDeleter(ctx, handler)

	// mock - GlobalVolumeHandler.Delete calls
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nfsShare, nil)
	cli.EXPECT().DeleteNfsShare(ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(dtfsShare, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx, dtfsShare.ID).Return(nil)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName).Return(fsInfo, nil)
	cli.EXPECT().DeleteFileSystem(ctx, fsInfo.ID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestDeleter_Delete_GlobalDtfs_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	handler := &GlobalVolumeHandler{Cli: cli, Name: fakeFsName, Protocol: constants.ProtocolDtfs}
	deleter := NewDeleter(ctx, handler)

	// mock - GlobalVolumeHandler.Delete calls
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nfsShare, nil)
	cli.EXPECT().DeleteNfsShare(ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(dtfsShare, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx, dtfsShare.ID).Return(nil)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName).Return(fsInfo, nil)
	cli.EXPECT().DeleteFileSystem(ctx, fsInfo.ID).Return(mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_Delete_LocalKVCache_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()
	handler := &LocalVolumeHandler{Cli: mockCli, KvCacheStoreId: "kv-id-1"}
	deleter := NewDeleter(ctx, handler)

	// KVCache exists → proceed with one-stop delete
	mockCli.EXPECT().QueryKVCache(ctx,
		&client.QueryKVCacheParams{ID: "kv-id-1"}).Return(&client.KVCacheStore{ID: "kv-id-1"}, nil)
	mockCli.EXPECT().DeleteKVCache(ctx, "kv-id-1").Return(nil)

	err := deleter.Delete()
	assert.NoError(t, err)
}

func TestDeleter_Delete_LocalKVCache_AlreadyDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()
	handler := &LocalVolumeHandler{Cli: mockCli, KvCacheStoreId: "kv-id-1"}
	deleter := NewDeleter(ctx, handler)

	// KVCache absent → skip deletion
	mockCli.EXPECT().QueryKVCache(ctx, &client.QueryKVCacheParams{ID: "kv-id-1"}).Return(nil, nil)

	err := deleter.Delete()
	assert.NoError(t, err)
}

func TestDeleter_Delete_LocalKVCache_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()
	handler := &LocalVolumeHandler{Cli: mockCli, KvCacheStoreId: "kv-id-1"}
	deleter := NewDeleter(ctx, handler)

	mockCli.EXPECT().QueryKVCache(ctx, &client.QueryKVCacheParams{ID: "kv-id-1"}).Return(nil, mockErr)

	err := deleter.Delete()
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_Delete_GlobalWithoutShares(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()
	handler := &GlobalVolumeHandler{Cli: mockCli, Name: "test-vol", Protocol: constants.ProtocolNfs}
	deleter := NewDeleter(ctx, handler)

	// No shares, no filesystem
	mockCli.EXPECT().GetNfsShareByPath(ctx, "/test-vol/").Return(nil, nil)
	mockCli.EXPECT().GetDataTurboShareByPath(ctx, "/test-vol/").Return(nil, nil)
	mockCli.EXPECT().GetFileSystemByName(ctx, "test-vol").Return(nil, nil)

	err := deleter.Delete()
	assert.NoError(t, err)
}
