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

package dtree

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	fakeDTreeName      = "test-dtree-name"
	fakeDTreeId        = "test-dtree-id"
	fakeParentName     = "test-parent-name"
	fakeCreateNfsModel = &CreateDTreeModel{
		Protocol:    constants.ProtocolNfs,
		DTreeName:   fakeDTreeName,
		ParentName:  fakeParentName,
		AllSquash:   constants.AllSquashValue,
		RootSquash:  constants.RootSquashValue,
		Description: "test-description",
		Capacity:    1024 * 1024,
		AuthClients: []string{"*"},
	}
	fakeCreateDpcModel = &CreateDTreeModel{
		Protocol:    constants.ProtocolDpc,
		DTreeName:   fakeDTreeName,
		ParentName:  fakeParentName,
		AllSquash:   constants.AllSquashValue,
		RootSquash:  constants.RootSquashValue,
		Description: "test-description",
		Capacity:    1024 * 1024,
		AuthClients: []string{"*"},
	}
)

func TestMain(m *testing.M) {
	log.MockInitLogging("test")
	defer log.MockStopLogging("test")

	m.Run()
}

func TestCreator_CreateNfsProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, fakeParentName, fakeDTreeName, fakeCreateNfsModel.FsPermission).
		Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx,
		fakeDTreeId, fakeCreateNfsModel.Capacity).Return(&client.DTreeQuotaResponse{Id: "test-quota-id",
		ParentId: fakeDTreeId, SpaceHardQuota: 1024 * 1024, SpaceUnitType: 0}, nil)
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, fakeCreateNfsModel.sharePath()).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx,
		&client.CreateDTreeNfsShareRequest{DtreeId: fakeDTreeId, Sharepath: fakeCreateNfsModel.sharePath(),
			Description: fakeCreateNfsModel.Description}).Return(&client.CreateDTreeNfsShareResponse{Id: "test-share-id"},
		nil)
	cli.EXPECT().AddNfsShareAuthClient(ctx,
		&client.AddNfsShareAuthClientRequest{AccessName: "*", ShareId: "test-share-id",
			AccessValue: readWriteAccessValue, Sync: synchronize, AllSquash: fakeCreateNfsModel.AllSquash,
			RootSquash: fakeCreateNfsModel.RootSquash})

	// action
	volume, err := creator.Create()

	// assert
	require.NoError(t, err)
	require.NotNil(t, volume)
	require.Equal(t, fakeDTreeName, volume.GetVolumeName())
}

func TestCreator_CreateNfsProtocol_SuccessWithResourcesExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(&client.DTreeQuotaResponse{Id: "test-quota-id",
		ParentId: fakeDTreeId, SpaceHardQuota: 1024 * 1024, SpaceUnitType: 0}, nil)
	cli.EXPECT().GetDTreeNfsShareByPath(ctx,
		fakeCreateNfsModel.sharePath()).Return(&client.GetDTreeNfsShareResponse{Id: "test-share-id"}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, "test-share-id").Return(nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx, gomock.Any()).Return(
		&client.CreateDTreeNfsShareResponse{Id: "test-share-id"}, nil)
	cli.EXPECT().AddNfsShareAuthClient(ctx,
		&client.AddNfsShareAuthClientRequest{AccessName: "*", ShareId: "test-share-id",
			AccessValue: readWriteAccessValue, Sync: synchronize, AllSquash: fakeCreateNfsModel.AllSquash,
			RootSquash: fakeCreateNfsModel.RootSquash})

	// action
	volume, err := creator.Create()

	// assert
	require.NoError(t, err)
	require.NotNil(t, volume)
	require.Equal(t, fakeDTreeName, volume.GetVolumeName())
}

func TestCreator_CreateDpcProtocol_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateDpcModel)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, fakeParentName, fakeDTreeName, fakeCreateDpcModel.FsPermission).
		Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx,
		fakeDTreeId, fakeCreateDpcModel.Capacity).Return(&client.DTreeQuotaResponse{Id: "test-quota-id",
		ParentId: fakeDTreeId, SpaceHardQuota: 1024 * 1024, SpaceUnitType: 0}, nil)

	// action
	volume, err := creator.Create()

	// assert
	require.NoError(t, err)
	require.NotNil(t, volume)
	require.Equal(t, fakeDTreeName, volume.GetVolumeName())
}

func TestCreator_CreateNfsProtocol_CreateAuthClientFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, fakeParentName, fakeDTreeName, fakeCreateNfsModel.FsPermission).
		Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx,
		fakeDTreeId, fakeCreateNfsModel.Capacity).Return(&client.DTreeQuotaResponse{Id: "test-quota-id",
		ParentId: fakeDTreeId, SpaceHardQuota: 1024 * 1024, SpaceUnitType: 0}, nil)
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, fakeCreateNfsModel.sharePath()).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx,
		&client.CreateDTreeNfsShareRequest{DtreeId: fakeDTreeId, Sharepath: fakeCreateNfsModel.sharePath(),
			Description: fakeCreateNfsModel.Description}).Return(&client.CreateDTreeNfsShareResponse{Id: "test-share-id"},
		nil)
	cli.EXPECT().AddNfsShareAuthClient(ctx,
		&client.AddNfsShareAuthClientRequest{AccessName: "*", ShareId: "test-share-id",
			AccessValue: readWriteAccessValue, Sync: synchronize, AllSquash: fakeCreateNfsModel.AllSquash,
			RootSquash: fakeCreateNfsModel.RootSquash}).Return(errors.New("fake error"))
	cli.EXPECT().DeleteDTreeNfsShare(ctx, "test-share-id")
	cli.EXPECT().DeleteDTreeQuota(ctx, "test-quota-id")
	cli.EXPECT().DeleteDTree(ctx, fakeDTreeId)

	// action
	volume, err := creator.Create()

	// assert
	require.Error(t, err)
	require.Nil(t, volume)
}

func TestCreator_CreateNfsProtocol_CreateNfsShareFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, fakeParentName, fakeDTreeName, fakeCreateNfsModel.FsPermission).
		Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx,
		fakeDTreeId, fakeCreateNfsModel.Capacity).Return(&client.DTreeQuotaResponse{Id: "test-quota-id",
		ParentId: fakeDTreeId, SpaceHardQuota: 1024 * 1024, SpaceUnitType: 0}, nil)
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, fakeCreateNfsModel.sharePath()).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx,
		&client.CreateDTreeNfsShareRequest{DtreeId: fakeDTreeId, Sharepath: fakeCreateNfsModel.sharePath(),
			Description: fakeCreateNfsModel.Description}).Return(nil, errors.New("fake error"))
	cli.EXPECT().DeleteDTreeQuota(ctx, "test-quota-id")
	cli.EXPECT().DeleteDTree(ctx, fakeDTreeId)

	// action
	volume, err := creator.Create()

	// assert
	require.Error(t, err)
	require.Nil(t, volume)
}

func TestCreator_CreateNfsProtocol_CreateQuotaFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, fakeParentName, fakeDTreeName, fakeCreateNfsModel.FsPermission).
		Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx,
		fakeDTreeId, fakeCreateNfsModel.Capacity).Return(nil, errors.New("fake error"))
	cli.EXPECT().DeleteDTree(ctx, fakeDTreeId)

	// action
	volume, err := creator.Create()

	// assert
	require.Error(t, err)
	require.Nil(t, volume)
}

func TestCreator_CreateNfsProtocol_QuotaNotEqualFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	creator := NewCreator(ctx, cli, fakeCreateNfsModel)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(&client.DTreeQuotaResponse{Id: "test-quota-id",
		ParentId: fakeDTreeId, SpaceHardQuota: 1024 * 1024 * 1024, SpaceUnitType: 0}, nil)
	cli.EXPECT().DeleteDTree(ctx, fakeDTreeId)

	// action
	volume, err := creator.Create()

	// assert
	require.Error(t, err)
	require.Nil(t, volume)
}
