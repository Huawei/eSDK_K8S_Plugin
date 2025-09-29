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

package volume

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

var (
	fakeDeleteNfsModel = &DeleteFilesystemModel{
		Protocol: constants.ProtocolNfs,
		Name:     fakeFsName,
	}
	fakeDeleteDtfsModel = &DeleteFilesystemModel{
		Protocol: constants.ProtocolDtfs,
		Name:     fakeFsName,
	}
)

func TestQuerier_DeleteWithNfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeDeleteNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeDeleteDtfsModel.sharePath(), fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().DeleteNfsShare(ctx, fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID, "IOCLASSID": fakeQosID}, nil)
	cli.EXPECT().GetQosByID(ctx, fakeQosID, fakeVstoreID).
		Return(map[string]interface{}{"FSLIST": fmt.Sprintf("[%q]", fakeFsID)}, nil)
	cli.EXPECT().DeactivateQos(ctx, fakeQosID, fakeVstoreID).Return(nil)
	cli.EXPECT().DeleteQos(ctx, fakeQosID, fakeVstoreID).Return(nil)
	cli.EXPECT().DeleteFileSystem(ctx, map[string]interface{}{"ID": fakeFsID}).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestQuerier_DeleteWithNfsProtocol_SuccessWithResourceNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeDeleteNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeDeleteDtfsModel.sharePath(), fakeVstoreID).Return(nil, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(nil, nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestQuerier_DeleteWithNfsProtocol_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeDeleteNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeDeleteDtfsModel.sharePath(), fakeVstoreID).
		Return(map[string]interface{}{"id": fakeShareID}, nil)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorContains(t, err, "empty ID")
}

func TestQuerier_DeleteWithDtfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeDeleteDtfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, fakeDeleteDtfsModel.sharePath(), fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx, fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().DeleteFileSystem(ctx, map[string]interface{}{"ID": fakeFsID}).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestQuerier_DeleteWithDtfsProtocol_SuccessWithResourceNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeDeleteDtfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, fakeDeleteDtfsModel.sharePath(), fakeVstoreID).Return(nil, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(nil, nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestQuerier_DeleteWithDtfsProtocol_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeDeleteDtfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, fakeDeleteDtfsModel.sharePath(), fakeVstoreID).
		Return(map[string]interface{}{"id": fakeShareID}, nil)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorContains(t, err, "empty ID")
}
