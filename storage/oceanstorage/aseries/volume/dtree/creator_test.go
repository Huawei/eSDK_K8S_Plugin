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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	fakeDtreeName      = "test-dtree-name"
	fakeDtreeID        = "test-dtree-id"
	fakeParentName     = "test-parent-fs"
	fakeFsID           = "test-fs-id"
	fakeQuotaID        = "test-quota-id"
	fakeShareID        = "test-share-id"
	fakeVstoreID       = "0"
	fakeAuthClient     = "test-client"
	fakeAuthUser       = "test-user"
	mockErr            = errors.New("mock err")
	fakeCapacity       = int64(1024 * 1024)
	fakeCreateNfsModel = &CreateDTreeModel{
		Protocol:     constants.ProtocolNfs,
		DTreeName:    fakeDtreeName,
		ParentName:   fakeParentName,
		AllSquash:    constants.AllSquashValue,
		RootSquash:   constants.RootSquashValue,
		FsPermission: "755",
		Capacity:     fakeCapacity,
		AuthClients:  []string{fakeAuthClient},
	}
	fakeCreateDtfsModel = &CreateDTreeModel{
		Protocol:     constants.ProtocolDtfs,
		DTreeName:    fakeDtreeName,
		ParentName:   fakeParentName,
		AllSquash:    constants.AllSquashValue,
		RootSquash:   constants.RootSquashValue,
		FsPermission: "755",
		Capacity:     fakeCapacity,
		AuthUsers:    []string{fakeAuthUser},
	}
)

func TestMain(m *testing.M) {
	log.MockInitLogging("dtreeTest")
	defer log.MockStopLogging("dtreeTest")

	m.Run()
}

func TestNewCreator(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)

	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	assert.NotNil(t, creator)
	assert.Equal(t, ctx, creator.ctx)
	assert.Equal(t, cli, creator.cli)
	assert.Equal(t, fakeCreateNfsModel, creator.params)
}

func TestCreator_CreateWithNfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeQuotaID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeCreateNfsModel.sharePath(), fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, &base.AllowNfsShareAccessRequest{
		Name:       fakeAuthClient,
		ParentID:   fakeShareID,
		VStoreID:   fakeVstoreID,
		AccessVal:  readWriteAccessValue,
		Sync:       synchronize,
		AllSquash:  fakeCreateNfsModel.AllSquash,
		RootSquash: fakeCreateNfsModel.RootSquash,
	}).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeDtreeName, volume.GetVolumeName())
	assert.Equal(t, fakeCapacity, volume.GetSize())
}

func TestCreator_CreateWithNfsProtocol_DTreeAlreadyExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock - DTree already exists
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	// Quota already exists
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeQuotaID, "SPACEHARDQUOTA": "1048576"}, nil)
	// NFS share already exists - delete then recreate
	cli.EXPECT().GetNfsShareByPath(ctx, fakeCreateNfsModel.sharePath(), fakeVstoreID).
		Return(map[string]interface{}{"ID": "old-share-id"}, nil)
	cli.EXPECT().DeleteNfsShare(ctx, "old-share-id", fakeVstoreID).Return(nil)
	cli.EXPECT().CreateNfsShare(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, gomock.Any()).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
}

func TestCreator_CreateWithNfsProtocol_ParentFSNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).Return(nil, nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.ErrorContains(t, err, "parent filesystem "+fakeParentName+" does not exist")
	assert.Nil(t, volume)
}

func TestCreator_CreateWithNfsProtocol_CreateDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(nil, mockErr)

	// action
	volume, err := creator.Create()

	// assert
	assert.ErrorIs(t, err, mockErr)
	assert.Nil(t, volume)
}

func TestCreator_CreateWithNfsProtocol_CreateQuotaError_Rollback(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(nil, mockErr)
	// Rollback: delete DTree
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.ErrorIs(t, err, mockErr)
	assert.Nil(t, volume)
}

func TestCreator_CreateWithNfsProtocol_AllowNfsShareAccessError_Rollback(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeQuotaID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeCreateNfsModel.sharePath(), fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, gomock.Any()).Return(mockErr)
	// Rollback: delete NFS share, delete quota, delete DTree
	cli.EXPECT().DeleteNfsShare(ctx, fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().DeleteDTreeQuota(ctx, fakeQuotaID, fakeVstoreID).Return(nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(nil)

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
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeQuotaID}, nil)

	// Based on new createDataTurboShare logic
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, nil)

	cli.EXPECT().CreateDataTurboShare(ctx, &client.CreateDataTurboShareParams{
		SharePath:   sharePath,
		FsId:        fakeFsID,
		Description: "Created from Kubernetes CSI",
		VstoreId:    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)

	cli.EXPECT().AddDataTurboShareUser(ctx, &client.AddDataTurboShareUserParams{
		UserName:   fakeAuthUser,
		ShareId:    fakeShareID,
		Permission: readWriteAccessValue,
		VstoreId:   fakeVstoreID,
	}).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeDtreeID, volume.GetID())
}

func TestCreator_CreateWithDtfsProtocol_DataTurboShareAlreadyExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeQuotaID}, nil)

	// DataTurbo share already exists - delete then recreate
	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).
		Return(map[string]interface{}{"ID": "old-share-id"}, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx, "old-share-id", fakeVstoreID).Return(nil)

	cli.EXPECT().CreateDataTurboShare(ctx, &client.CreateDataTurboShareParams{
		SharePath:   sharePath,
		FsId:        fakeFsID,
		Description: "Created from Kubernetes CSI",
		VstoreId:    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)

	cli.EXPECT().AddDataTurboShareUser(ctx, &client.AddDataTurboShareUserParams{
		UserName:   fakeAuthUser,
		ShareId:    fakeShareID,
		Permission: readWriteAccessValue,
		VstoreId:   fakeVstoreID,
	}).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
}

func TestCreator_CreateWithDtfsProtocol_AddAuthUserError_Rollback(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeQuotaID}, nil)

	sharePath := "/" + fakeParentName + "/" + fakeDtreeName
	cli.EXPECT().GetDataTurboShareByPath(ctx, sharePath, fakeVstoreID).Return(nil, nil)

	cli.EXPECT().CreateDataTurboShare(ctx, &client.CreateDataTurboShareParams{
		SharePath:   sharePath,
		FsId:        fakeFsID,
		Description: "Created from Kubernetes CSI",
		VstoreId:    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)

	cli.EXPECT().AddDataTurboShareUser(ctx, &client.AddDataTurboShareUserParams{
		UserName:   fakeAuthUser,
		ShareId:    fakeShareID,
		Permission: readWriteAccessValue,
		VstoreId:   fakeVstoreID,
	}).Return(mockErr)

	// Rollback expectations: add user error triggers rollback
	cli.EXPECT().DeleteDataTurboShare(ctx, fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().DeleteDTreeQuota(ctx, fakeQuotaID, fakeVstoreID).Return(nil)
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, mockErr)
	assert.Nil(t, volume)
}
func TestCreator_CreateWithNfsProtocol_QuotaAlreadyExistsDifferentCapacity(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDtreeName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, gomock.Any()).Return(map[string]interface{}{"ID": fakeDtreeID}, nil)
	// Quota already exists with different capacity
	cli.EXPECT().GetDTreeQuota(ctx, fakeDtreeID, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeQuotaID, "SPACEHARDQUOTA": "999999"}, nil)
	// Rollback: delete DTree
	cli.EXPECT().DeleteDTreeByID(ctx, fakeVstoreID, fakeDtreeID).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.ErrorContains(t, err, "quota already exists with different capacity")
	assert.Nil(t, volume)
}

func TestParseQuotaValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
		wantErr  bool
	}{
		{name: "valid number", input: "1048576", expected: 1048576, wantErr: false},
		{name: "empty string", input: "", expected: 0, wantErr: false},
		{name: "invalid string", input: "abc", expected: 0, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseQuotaValue(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
