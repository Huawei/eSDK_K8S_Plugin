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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	fakeFsName         = "test-fs-name"
	fakeFsID           = "test-fs-id"
	fakePoolName       = "test-pool-name"
	fakePoolID         = "test-pool-id"
	fakeWorkloadType   = "test-workload-type"
	fakeWorkloadTypeID = "test-workload-type-id"
	fakeShareID        = "test-share-id"
	fakeQosID          = "test-qos-id"
	fakeVstoreID       = "0"
	fakeAuthClient     = "test-client"
	fakeAuthUser       = "test-user"
	mockErr            = errors.New("mock err")
	fakeCreateNfsModel = &CreateFilesystemModel{
		Protocol:        constants.ProtocolNfs,
		Name:            fakeFsName,
		PoolName:        fakePoolName,
		WorkloadType:    fakeWorkloadType,
		Capacity:        1024 * 1024,
		Description:     "test-description",
		UnixPermissions: "755",
		AllSquash:       constants.AllSquashValue,
		RootSquash:      constants.RootSquashValue,
		Qos:             "{\"IOTYPE\": 2, \"MAXIOPS\": 1000}",
		AuthClients:     []string{fakeAuthClient},
	}
	fakeCreateDtfsModel = &CreateFilesystemModel{
		Protocol:        constants.ProtocolDtfs,
		Name:            fakeFsName,
		PoolName:        fakePoolName,
		WorkloadType:    fakeWorkloadType,
		Capacity:        1024 * 1024,
		Description:     "test-description",
		UnixPermissions: "755",
		AllSquash:       constants.AllSquashValue,
		RootSquash:      constants.RootSquashValue,
		AuthUsers:       []string{fakeAuthUser},
	}
)

func TestMain(m *testing.M) {
	log.MockInitLogging("volumeTest")
	defer log.MockStopLogging("volumeTest")

	m.Run()
}

func TestCreator_CreateWithNfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(time.Time{}, "Format", "20060102150405")
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetPoolByName(ctx, fakePoolName).Return(map[string]interface{}{"ID": fakePoolID}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return(fakeWorkloadTypeID, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateFileSystem(ctx, &client.CreateFilesystemParams{
		Name:            fakeCreateNfsModel.Name,
		ParentId:        fakePoolID,
		Capacity:        fakeCreateNfsModel.Capacity,
		Description:     fakeCreateNfsModel.Description,
		WorkLoadTypeId:  fakeWorkloadTypeID,
		UnixPermissions: fakeCreateNfsModel.UnixPermissions,
		VstoreId:        fakeVstoreID,
	}, nil).Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeCreateNfsModel.sharePath(), fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, map[string]interface{}{
		"sharepath":   fakeCreateNfsModel.sharePath(),
		"fsid":        fakeFsID,
		"description": fakeCreateNfsModel.Description,
		"vStoreID":    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx,
		&base.AllowNfsShareAccessRequest{
			Name:       fakeAuthClient,
			ParentID:   fakeShareID,
			VStoreID:   fakeVstoreID,
			AccessVal:  readWriteAccessValue,
			Sync:       synchronize,
			AllSquash:  fakeCreateNfsModel.AllSquash,
			RootSquash: fakeCreateNfsModel.RootSquash}).Return(nil)
	qosName := fmt.Sprintf("k8s_%s%s_%s", "fs", fakeFsID, "20060102150405")
	cli.EXPECT().CreateQos(ctx, base.CreateQoSArgs{
		Name:     qosName,
		ObjID:    fakeFsID,
		ObjType:  "fs",
		VStoreID: fakeVstoreID,
		Params:   map[string]int{"IOTYPE": 2, "MAXIOPS": 1000},
	}).Return(map[string]interface{}{"ID": fakeQosID, "ENABLESTATUS": "false"}, nil)
	cli.EXPECT().ActivateQos(ctx, fakeQosID, fakeVstoreID).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeFsName, volume.GetVolumeName())
}

func TestCreator_CreateWithNfsProtocol_SuccessWithResourceExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetPoolByName(ctx, fakePoolName).Return(map[string]interface{}{"ID": fakePoolID}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return(fakeWorkloadTypeID, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID, "IOCLASSID": fakeQosID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeCreateNfsModel.sharePath(), fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().DeleteNfsShare(ctx,
		fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().CreateNfsShare(ctx, map[string]interface{}{
		"sharepath":   fakeCreateNfsModel.sharePath(),
		"fsid":        fakeFsID,
		"description": fakeCreateNfsModel.Description,
		"vStoreID":    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx,
		&base.AllowNfsShareAccessRequest{
			Name:       fakeAuthClient,
			ParentID:   fakeShareID,
			VStoreID:   fakeVstoreID,
			AccessVal:  readWriteAccessValue,
			Sync:       synchronize,
			AllSquash:  fakeCreateNfsModel.AllSquash,
			RootSquash: fakeCreateNfsModel.RootSquash}).Return(nil)
	cli.EXPECT().GetQosByID(ctx, fakeQosID, fakeVstoreID).Return(map[string]interface{}{"ID": fakeQosID}, nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeFsName, volume.GetVolumeName())
}

func TestCreator_CreateWithNfsProtocol_AddAuthClientError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetPoolByName(ctx, fakePoolName).Return(map[string]interface{}{"ID": fakePoolID}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return(fakeWorkloadTypeID, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateFileSystem(ctx, &client.CreateFilesystemParams{
		Name:            fakeCreateNfsModel.Name,
		ParentId:        fakePoolID,
		Capacity:        fakeCreateNfsModel.Capacity,
		Description:     fakeCreateNfsModel.Description,
		WorkLoadTypeId:  fakeWorkloadTypeID,
		UnixPermissions: fakeCreateNfsModel.UnixPermissions,
		VstoreId:        fakeVstoreID,
	}, nil).Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeCreateNfsModel.sharePath(), fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, map[string]interface{}{
		"sharepath":   fakeCreateNfsModel.sharePath(),
		"fsid":        fakeFsID,
		"description": fakeCreateNfsModel.Description,
		"vStoreID":    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx,
		&base.AllowNfsShareAccessRequest{
			Name:       fakeAuthClient,
			ParentID:   fakeShareID,
			VStoreID:   fakeVstoreID,
			AccessVal:  readWriteAccessValue,
			Sync:       synchronize,
			AllSquash:  fakeCreateNfsModel.AllSquash,
			RootSquash: fakeCreateNfsModel.RootSquash}).Return(mockErr)
	cli.EXPECT().DeleteNfsShare(ctx, fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().DeleteFileSystem(ctx, map[string]interface{}{"ID": fakeFsID}).Return(nil)

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
	cli.EXPECT().GetPoolByName(ctx, fakePoolName).Return(map[string]interface{}{"ID": fakePoolID}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return(fakeWorkloadTypeID, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateFileSystem(ctx, &client.CreateFilesystemParams{
		Name:            fakeCreateDtfsModel.Name,
		ParentId:        fakePoolID,
		Capacity:        fakeCreateDtfsModel.Capacity,
		Description:     fakeCreateDtfsModel.Description,
		WorkLoadTypeId:  fakeWorkloadTypeID,
		UnixPermissions: fakeCreateDtfsModel.UnixPermissions,
		VstoreId:        fakeVstoreID,
	}, nil).Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDataTurboShareByPath(ctx, fakeCreateDtfsModel.sharePath(), fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDataTurboShare(ctx, &client.CreateDataTurboShareParams{
		SharePath:   fakeCreateDtfsModel.sharePath(),
		FsId:        fakeFsID,
		Description: fakeCreateDtfsModel.Description,
		VstoreId:    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AddDataTurboShareUser(ctx,
		&client.AddDataTurboShareUserParams{
			UserName:   fakeAuthUser,
			ShareId:    fakeShareID,
			Permission: readWriteAccessValue,
			VstoreId:   fakeVstoreID}).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeFsName, volume.GetVolumeName())
}

func TestCreator_CreateWithDtfsProtocol_SuccessWithResourceExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetPoolByName(ctx, fakePoolName).Return(map[string]interface{}{"ID": fakePoolID}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return(fakeWorkloadTypeID, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDataTurboShareByPath(ctx, fakeCreateDtfsModel.sharePath(), fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().DeleteDataTurboShare(ctx,
		fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().CreateDataTurboShare(ctx, &client.CreateDataTurboShareParams{
		SharePath:   fakeCreateDtfsModel.sharePath(),
		FsId:        fakeFsID,
		Description: fakeCreateDtfsModel.Description,
		VstoreId:    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AddDataTurboShareUser(ctx,
		&client.AddDataTurboShareUserParams{
			UserName:   fakeAuthUser,
			ShareId:    fakeShareID,
			Permission: readWriteAccessValue,
			VstoreId:   fakeVstoreID}).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeFsName, volume.GetVolumeName())
}

func TestCreator_CreateWithDtfsProtocol_AddAuthUserError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateDtfsModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetPoolByName(ctx, fakePoolName).Return(map[string]interface{}{"ID": fakePoolID}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return(fakeWorkloadTypeID, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateFileSystem(ctx, &client.CreateFilesystemParams{
		Name:            fakeCreateDtfsModel.Name,
		ParentId:        fakePoolID,
		Capacity:        fakeCreateDtfsModel.Capacity,
		Description:     fakeCreateDtfsModel.Description,
		WorkLoadTypeId:  fakeWorkloadTypeID,
		UnixPermissions: fakeCreateDtfsModel.UnixPermissions,
		VstoreId:        fakeVstoreID,
	}, nil).Return(map[string]interface{}{"ID": fakeFsID}, nil)
	cli.EXPECT().GetDataTurboShareByPath(ctx, fakeCreateDtfsModel.sharePath(), fakeVstoreID).Return(nil, nil)
	cli.EXPECT().CreateDataTurboShare(ctx, &client.CreateDataTurboShareParams{
		SharePath:   fakeCreateDtfsModel.sharePath(),
		FsId:        fakeFsID,
		Description: fakeCreateDtfsModel.Description,
		VstoreId:    fakeVstoreID,
	}).Return(map[string]interface{}{"ID": fakeShareID}, nil)
	cli.EXPECT().AddDataTurboShareUser(ctx,
		&client.AddDataTurboShareUserParams{
			UserName:   fakeAuthUser,
			ShareId:    fakeShareID,
			Permission: readWriteAccessValue,
			VstoreId:   fakeVstoreID}).Return(mockErr)
	cli.EXPECT().DeleteDataTurboShare(ctx, fakeShareID, fakeVstoreID).Return(nil)
	cli.EXPECT().DeleteFileSystem(ctx, map[string]interface{}{"ID": fakeFsID}).Return(nil)

	// action
	volume, err := creator.Create()

	// assert
	assert.ErrorIs(t, err, mockErr)
	assert.Nil(t, volume)
}
