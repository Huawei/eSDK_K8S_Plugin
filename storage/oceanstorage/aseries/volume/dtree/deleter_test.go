/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.com/licenses/LICENSE-2.0
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestNewDeleter(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolNfs)

	assert.NotNil(t, deleter)
	assert.Equal(t, ctx, deleter.ctx)
	assert.Equal(t, cli, deleter.cli)
	assert.Equal(t, fakeParentName, deleter.parentName)
	assert.Equal(t, fakeDtreeName, deleter.dtreeName)
	assert.Equal(t, constants.ProtocolNfs, deleter.protocol)
}

// NFS protocol deletion tests

func TestDeleter_DeleteWithNfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolNfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, sharePath, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().DeleteNfsShare(ctx, fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestDeleter_DeleteWithNfsProtocol_NfsShareNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolNfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestDeleter_DeleteWithNfsProtocol_GetNfsShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolNfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_DeleteWithNfsProtocol_DeleteNfsShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolNfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, sharePath, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().DeleteNfsShare(ctx, fakeShareID, fakeVstoreID).Return(mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_DeleteWithNfsProtocol_DeleteDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolNfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_DeleteWithNfsProtocol_GetDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolNfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

// Dtfs (DataTurbo) protocol deletion tests

func TestDeleter_DeleteWithDtfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolDtfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx, fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestDeleter_DeleteWithDtfsProtocol_DataTurboShareNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolDtfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	assert.NoError(t, err)
}

func TestDeleter_DeleteWithDtfsProtocol_GetDataTurboShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolDtfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_DeleteWithDtfsProtocol_DeleteDataTurboShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolDtfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx, fakeShareID, fakeVstoreID).Return(mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_DeleteWithDtfsProtocol_DeleteDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolDtfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, mockErr)
}

func TestDeleter_DeleteWithDtfsProtocol_GetDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDtreeName, constants.ProtocolDtfs)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, mockErr)

	// action
	err := deleter.Delete()

	// assert
	assert.ErrorIs(t, err, mockErr)
}
