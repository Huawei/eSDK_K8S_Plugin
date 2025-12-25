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

var (
	fakeDeleteNfsModel = &DeleteVolumeModel{
		Protocol: constants.ProtocolNfs,
		Name:     fakeFsName,
	}
	fakeDeleteDtfsModel = &DeleteVolumeModel{
		Protocol: constants.ProtocolDtfs,
		Name:     fakeFsName,
	}
)

func TestDeleter_DeleteWithNfsProtocol_Success(t *testing.T) {
	// arrange
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	ctx := context.Background()
	deleter := NewDeleter(ctx, cli, fakeDeleteNfsModel)

	// mock
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(deleter.ctx, deleter.params.sharePath()).Return(nfsShare, nil)
	cli.EXPECT().DeleteNfsShare(deleter.ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(deleter.ctx, deleter.params.sharePath()).Return(dtfsShare, nil)
	cli.EXPECT().DeleteDataTurboShare(deleter.ctx, dtfsShare.ID).Return(nil)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetFileSystemByName(deleter.ctx, deleter.params.Name).Return(fsInfo, nil)
	cli.EXPECT().DeleteFileSystem(deleter.ctx, fsInfo.ID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)

}
func TestDeleter_DeleteWithNfsProtocol_Error(t *testing.T) {
	// arrange
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	ctx := context.Background()
	deleter := NewDeleter(ctx, cli, fakeDeleteNfsModel)

	// mock
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(deleter.ctx, deleter.params.sharePath()).Return(nfsShare, nil)
	cli.EXPECT().DeleteNfsShare(deleter.ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(deleter.ctx, deleter.params.sharePath()).Return(dtfsShare, nil)
	cli.EXPECT().DeleteDataTurboShare(deleter.ctx, dtfsShare.ID).Return(nil)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetFileSystemByName(deleter.ctx, deleter.params.Name).Return(fsInfo, nil)
	cli.EXPECT().DeleteFileSystem(deleter.ctx, fsInfo.ID).Return(mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, mockErr)
}
func TestDeleter_DeleteWithDtfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeDeleteDtfsModel)

	// mock
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(deleter.ctx, deleter.params.sharePath()).Return(nfsShare, nil)
	cli.EXPECT().DeleteNfsShare(deleter.ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(deleter.ctx, deleter.params.sharePath()).Return(dtfsShare, nil)
	cli.EXPECT().DeleteDataTurboShare(deleter.ctx, dtfsShare.ID).Return(nil)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetFileSystemByName(deleter.ctx, deleter.params.Name).Return(fsInfo, nil)
	cli.EXPECT().DeleteFileSystem(deleter.ctx, fsInfo.ID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}
func TestDeleter_DeleteWithDtfsProtocol_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeDeleteDtfsModel)

	// mock
	nfsShare := &client.NfsShareInfo{ID: fakeShareID}
	cli.EXPECT().GetNfsShareByPath(deleter.ctx, deleter.params.sharePath()).Return(nfsShare, nil)
	cli.EXPECT().DeleteNfsShare(deleter.ctx, nfsShare.ID).Return(nil)
	dtfsShare := &client.DataTurboShare{ID: fakeShareID}
	cli.EXPECT().GetDataTurboShareByPath(deleter.ctx, deleter.params.sharePath()).Return(dtfsShare, nil)
	cli.EXPECT().DeleteDataTurboShare(deleter.ctx, dtfsShare.ID).Return(nil)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID}
	cli.EXPECT().GetFileSystemByName(deleter.ctx, deleter.params.Name).Return(fsInfo, nil)
	cli.EXPECT().DeleteFileSystem(deleter.ctx, fsInfo.ID).Return(mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, mockErr)
}
