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

// --- LocalVolumeHandler.GetPool ---

func TestLocalVolumeHandler_GetPool_Success(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()
	pool := &client.StoragePool{Name: fakePoolName, RawID: fakePoolRawID}

	mockCli.EXPECT().GetZoneID().Return("zone-1")
	mockCli.EXPECT().GetStoragePoolByName(ctx, fakePoolName, "zone-1").Return(pool, nil)

	handler := &LocalVolumeHandler{Cli: mockCli}

	// action
	result, err := handler.GetPool(ctx, fakePoolName)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, pool, result)
}

func TestLocalVolumeHandler_GetPool_Error(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetZoneID().Return("zone-1")
	mockCli.EXPECT().GetStoragePoolByName(ctx, fakePoolName, "zone-1").Return(nil, mockErr)

	handler := &LocalVolumeHandler{Cli: mockCli}

	// action
	result, err := handler.GetPool(ctx, fakePoolName)

	// assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, mockErr)
}

func TestLocalVolumeHandler_GetPool_NotFound(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetZoneID().Return("zone-1")
	mockCli.EXPECT().GetStoragePoolByName(ctx, fakePoolName, "zone-1").Return(nil, nil)

	handler := &LocalVolumeHandler{Cli: mockCli}

	// action
	result, err := handler.GetPool(ctx, fakePoolName)

	// assert
	assert.Nil(t, result)
	assert.EqualError(t, err, "pool test-pool-name does not exist")
}

// --- LocalVolumeHandler.Delete ---

func TestLocalVolumeHandler_Delete_Success(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().QueryKVCache(ctx, &client.QueryKVCacheParams{ID: "kv-id-1"}).
		Return(&client.KVCacheStore{ID: "kv-id-1"}, nil)
	mockCli.EXPECT().DeleteKVCache(ctx, "kv-id-1").Return(nil)

	handler := &LocalVolumeHandler{Cli: mockCli, KvCacheStoreId: "kv-id-1"}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.NoError(t, err)
}

func TestLocalVolumeHandler_Delete_AlreadyDeleted(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().QueryKVCache(ctx, &client.QueryKVCacheParams{ID: "kv-id-1"}).Return(nil, nil)

	handler := &LocalVolumeHandler{Cli: mockCli, KvCacheStoreId: "kv-id-1"}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.NoError(t, err)
}

func TestLocalVolumeHandler_Delete_QueryError(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().QueryKVCache(ctx, &client.QueryKVCacheParams{ID: "kv-id-1"}).Return(nil, mockErr)

	handler := &LocalVolumeHandler{Cli: mockCli, KvCacheStoreId: "kv-id-1"}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestLocalVolumeHandler_Delete_DeleteError(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().QueryKVCache(ctx, &client.QueryKVCacheParams{ID: "kv-id-1"}).
		Return(&client.KVCacheStore{ID: "kv-id-1"}, nil)
	mockCli.EXPECT().DeleteKVCache(ctx, "kv-id-1").Return(mockErr)

	handler := &LocalVolumeHandler{Cli: mockCli, KvCacheStoreId: "kv-id-1"}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.ErrorIs(t, err, mockErr)
}

// --- GlobalVolumeHandler.GetPool ---

func TestGlobalVolumeHandler_GetPool_Success(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()
	pool := &client.HyperScalePool{Name: fakePoolName, RawId: fakePoolRawID}

	mockCli.EXPECT().GetHyperScalePoolByName(ctx, fakePoolName).Return(pool, nil)

	handler := &GlobalVolumeHandler{Cli: mockCli}

	// action
	result, err := handler.GetPool(ctx, fakePoolName)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, pool, result)
}

func TestGlobalVolumeHandler_GetPool_Error(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetHyperScalePoolByName(ctx, fakePoolName).Return(nil, mockErr)

	handler := &GlobalVolumeHandler{Cli: mockCli}

	// action
	result, err := handler.GetPool(ctx, fakePoolName)

	// assert
	assert.Nil(t, result)
	assert.ErrorIs(t, err, mockErr)
}

func TestGlobalVolumeHandler_GetPool_NotFound(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetHyperScalePoolByName(ctx, fakePoolName).Return(nil, nil)

	handler := &GlobalVolumeHandler{Cli: mockCli}

	// action
	result, err := handler.GetPool(ctx, fakePoolName)

	// assert
	assert.Nil(t, result)
	assert.EqualError(t, err, "pool test-pool-name does not exist")
}

// --- GlobalVolumeHandler.Delete ---

func TestGlobalVolumeHandler_Delete_NfsWithShares_Success(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(&client.NfsShareInfo{ID: fakeShareID}, nil)
	mockCli.EXPECT().DeleteNfsShare(ctx, fakeShareID).Return(nil)
	mockCli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(&client.DataTurboShare{ID: fakeShareID},
		nil)
	mockCli.EXPECT().DeleteDataTurboShare(ctx, fakeShareID).Return(nil)
	mockCli.EXPECT().GetFileSystemByName(ctx, fakeFsName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	mockCli.EXPECT().DeleteFileSystem(ctx, fakeFsID).Return(nil)

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: fakeFsName, Protocol: constants.ProtocolNfs}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.NoError(t, err)
}

func TestGlobalVolumeHandler_Delete_NoShares_NoFilesystem(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, nil)
	mockCli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, nil)
	mockCli.EXPECT().GetFileSystemByName(ctx, fakeFsName).Return(nil, nil)

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: fakeFsName, Protocol: constants.ProtocolNfs}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.NoError(t, err)
}

func TestGlobalVolumeHandler_Delete_NfsShareDeleteError(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(&client.NfsShareInfo{ID: fakeShareID}, nil)
	mockCli.EXPECT().DeleteNfsShare(ctx, fakeShareID).Return(mockErr)

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: fakeFsName, Protocol: constants.ProtocolNfs}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestGlobalVolumeHandler_Delete_DtShareDeleteError(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, nil)
	mockCli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(&client.DataTurboShare{ID: fakeShareID},
		nil)
	mockCli.EXPECT().DeleteDataTurboShare(ctx, fakeShareID).Return(mockErr)

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: fakeFsName, Protocol: constants.ProtocolNfs}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestGlobalVolumeHandler_Delete_GetNfsShareError(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, mockErr)

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: fakeFsName, Protocol: constants.ProtocolNfs}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestGlobalVolumeHandler_Delete_GetDtShareError(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, nil)
	mockCli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, mockErr)

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: fakeFsName, Protocol: constants.ProtocolNfs}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestGlobalVolumeHandler_Delete_GetFsError(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, nil)
	mockCli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, nil)
	mockCli.EXPECT().GetFileSystemByName(ctx, fakeFsName).Return(nil, mockErr)

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: fakeFsName, Protocol: constants.ProtocolNfs}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestGlobalVolumeHandler_Delete_DeleteFsError(t *testing.T) {
	// arrange
	ctrl := gomock.NewController(t)
	mockCli := mock_client.NewMockDMEASeriesClientInterface(ctrl)
	ctx := context.Background()

	mockCli.EXPECT().GetNfsShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, nil)
	mockCli.EXPECT().GetDataTurboShareByPath(ctx, "/"+fakeFsName+"/").Return(nil, nil)
	mockCli.EXPECT().GetFileSystemByName(ctx, fakeFsName).Return(&client.FileSystemInfo{ID: fakeFsID}, nil)
	mockCli.EXPECT().DeleteFileSystem(ctx, fakeFsID).Return(mockErr)

	handler := &GlobalVolumeHandler{Cli: mockCli, Name: fakeFsName, Protocol: constants.ProtocolNfs}

	// action
	err := handler.Delete(ctx)

	// assert
	assert.ErrorIs(t, err, mockErr)
}
