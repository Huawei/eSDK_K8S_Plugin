/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume/creator"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestCreator_autoManageAuthClient_CreateClient(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeVStoreID := "fake-vstore-id"
	fakeShareID := "fake-share-id"
	fakeSharePath := "/fake-volume/"
	fakeVolume := "fake-volume"
	fakeAuthClient := "192.168.1.1"
	fakeClients := []string{fakeAuthClient}
	wantShareReq := &base.AllowNfsShareAccessRequest{
		Name:        fakeAuthClient,
		ParentID:    fakeShareID,
		VStoreID:    fakeVStoreID,
		AccessVal:   int(constants.AuthClientReadWrite),
		AllSquash:   constants.NoAllSquashValue,
		RootSquash:  constants.NoRootSquashValue,
		AccessKrb5:  creator.AccessKrb5ReadNoneInt,
		AccessKrb5i: creator.AccessKrb5ReadNoneInt,
		AccessKrb5p: creator.AccessKrb5ReadNoneInt,
	}
	plugin := &Base{cli: cli}

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVStoreID).AnyTimes()
	cli.EXPECT().GetNfsShareByPath(ctx, fakeSharePath, fakeVStoreID).Return(map[string]any{"ID": fakeShareID}, nil)
	cli.EXPECT().GetNfsShareAccess(ctx, fakeShareID, fakeAuthClient, fakeVStoreID).Return(nil, nil)
	cli.EXPECT().GetNfsShareAccessRange(ctx, fakeShareID, fakeVStoreID, int64(0), int64(1)).Return(nil, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, wantShareReq).Return(nil)

	// action
	gotErr := plugin.autoManageAuthClient(ctx, fakeVolume, fakeClients, constants.AuthClientReadWrite)

	// assert
	require.NoError(t, gotErr)
}

func TestCreator_autoManageAuthClient_GetShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeVStoreID := "fake-vstore-id"
	fakeSharePath := "/fake-volume/"
	fakeErr := errors.New("get share error")
	fakeVolume := "fake-volume"
	fakeAuthClient := "192.168.1.1"
	fakeClients := []string{fakeAuthClient}
	plugin := &Base{cli: cli}

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVStoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeSharePath, fakeVStoreID).Return(nil, fakeErr)

	// action
	gotErr := plugin.autoManageAuthClient(ctx, fakeVolume, fakeClients, constants.AuthClientReadWrite)

	// assert
	require.ErrorContains(t, gotErr, fakeErr.Error())
}

func TestCreator_autoManageAuthClient_ShareNotExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeVStoreID := "fake-vstore-id"
	fakeSharePath := "/fake-volume/"
	fakeVolume := "fake-volume"
	fakeAuthClient := "192.168.1.1"
	fakeClients := []string{fakeAuthClient}
	plugin := &Base{cli: cli}

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVStoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeSharePath, fakeVStoreID).Return(nil, nil)

	// action
	gotErr := plugin.autoManageAuthClient(ctx, fakeVolume, fakeClients, constants.AuthClientReadWrite)

	// assert
	require.ErrorContains(t, gotErr, "not exist")
}

func TestCreator_autoManageAuthClient_NoAccessAndShareNotExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeVStoreID := "fake-vstore-id"
	fakeSharePath := "/fake-volume/"
	fakeVolume := "fake-volume"
	fakeAuthClient := "192.168.1.1"
	fakeClients := []string{fakeAuthClient}
	plugin := &Base{cli: cli}

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVStoreID)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeSharePath, fakeVStoreID).Return(nil, nil)

	// action
	gotErr := plugin.autoManageAuthClient(ctx, fakeVolume, fakeClients, constants.AuthClientNoAccess)

	// assert
	require.NoError(t, gotErr)
}

func TestCreator_autoManageAuthClient_GetAccessError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeVStoreID := "fake-vstore-id"
	fakeSharePath := "/fake-volume/"
	fakeShareID := "fake-share-id"
	fakeVolume := "fake-volume"
	fakeAuthClient := "192.168.1.1"
	fakeClients := []string{fakeAuthClient}
	fakeErr := errors.New("fake error")
	plugin := &Base{cli: cli}

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVStoreID).Times(2)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeSharePath, fakeVStoreID).Return(map[string]any{"ID": fakeShareID}, nil)
	cli.EXPECT().GetNfsShareAccess(ctx, fakeShareID, fakeAuthClient, fakeVStoreID).Return(nil, fakeErr)

	// action
	gotErr := plugin.autoManageAuthClient(ctx, fakeVolume, fakeClients, constants.AuthClientReadWrite)

	// assert
	require.ErrorContains(t, gotErr, fakeErr.Error())
}

func TestCreator_autoManageAuthClient_CreateClientError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeVStoreID := "fake-vstore-id"
	fakeShareID := "fake-share-id"
	fakeSharePath := "/fake-volume/"
	fakeVolume := "fake-volume"
	fakeAuthClient := "192.168.1.1"
	fakeClients := []string{fakeAuthClient}
	wantShareReq := &base.AllowNfsShareAccessRequest{
		Name:        fakeAuthClient,
		ParentID:    fakeShareID,
		VStoreID:    fakeVStoreID,
		AccessVal:   int(constants.AuthClientReadWrite),
		AllSquash:   constants.NoAllSquashValue,
		RootSquash:  constants.NoRootSquashValue,
		AccessKrb5:  creator.AccessKrb5ReadNoneInt,
		AccessKrb5i: creator.AccessKrb5ReadNoneInt,
		AccessKrb5p: creator.AccessKrb5ReadNoneInt,
	}
	fakeErr := errors.New("fake error")
	plugin := &Base{cli: cli}

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVStoreID).AnyTimes()
	cli.EXPECT().GetNfsShareByPath(ctx, fakeSharePath, fakeVStoreID).Return(map[string]any{"ID": fakeShareID}, nil)
	cli.EXPECT().GetNfsShareAccess(ctx, fakeShareID, fakeAuthClient, fakeVStoreID).Return(nil, nil)
	cli.EXPECT().GetNfsShareAccessRange(ctx, fakeShareID, fakeVStoreID, int64(0), int64(1)).Return(nil, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, wantShareReq).Return(fakeErr)

	// action
	gotErr := plugin.autoManageAuthClient(ctx, fakeVolume, fakeClients, constants.AuthClientReadWrite)

	// assert
	require.ErrorContains(t, gotErr, fakeErr.Error())
}

func TestCreator_autoManageAuthClient_UpdateClientError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeVStoreID := "fake-vstore-id"
	fakeShareID := "fake-share-id"
	fakeSharePath := "/fake-volume/"
	fakeAccessID := "fake-access-id"
	fakeVolume := "fake-volume"
	fakeAuthClient := "192.168.1.1"
	fakeClients := []string{fakeAuthClient}
	fakeErr := errors.New("fake error")
	plugin := &Base{cli: cli}

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVStoreID).Times(2)
	cli.EXPECT().GetNfsShareByPath(ctx, fakeSharePath, fakeVStoreID).Return(map[string]any{"ID": fakeShareID}, nil)
	cli.EXPECT().GetNfsShareAccess(ctx, fakeShareID, fakeAuthClient, fakeVStoreID).
		Return(map[string]any{"ID": fakeAccessID}, nil)
	cli.EXPECT().ModifyNfsShareAccess(ctx, fakeAccessID, fakeVStoreID, constants.AuthClientReadWrite).Return(fakeErr)

	// action
	gotErr := plugin.autoManageAuthClient(ctx, fakeVolume, fakeClients, constants.AuthClientReadWrite)

	// assert
	require.ErrorContains(t, gotErr, fakeErr.Error())
}

func TestBase_checkAllClientsStatus_WithEmptyClients(t *testing.T) {
	// arrange
	ctx := context.Background()
	plugin := &Base{}

	// action
	gotErr := plugin.checkAllClientsStatus(ctx, "volume", []string{}, true)

	// assert
	assert.NoError(t, gotErr)
}

func TestBase_checkAllClientsStatus_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	volume := "test-volume"
	authClients := []string{"client1", "client2"}
	sharePath := "/test-volume"
	vStoreID := "fake-vstore-id"

	// mock
	cli.EXPECT().GetvStoreID().Return(vStoreID).AnyTimes()
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client1", vStoreID,
		constants.AuthClientReadWrite).Return(true, nil)
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client2", vStoreID,
		constants.AuthClientReadWrite).Return(true, nil)

	// action
	gotErr := plugin.checkAllClientsStatus(ctx, volume, authClients, true)

	// assert
	assert.NoError(t, gotErr)
}

func TestBase_checkAllClientsStatus_WithCheckTwice(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	volume := "test-volume"
	authClients := []string{"client1", "client2"}
	sharePath := "/test-volume"
	vStoreID := "fake-vstore-id"

	// mock
	cli.EXPECT().GetvStoreID().Return(vStoreID).AnyTimes()
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client1", vStoreID,
		constants.AuthClientReadWrite).Return(false, nil)
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client1", vStoreID,
		constants.AuthClientReadWrite).Return(true, nil)
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client2", vStoreID,
		constants.AuthClientReadWrite).Return(true, nil)

	// action
	gotErr := plugin.checkAllClientsStatus(ctx, volume, authClients, true)

	// assert
	assert.NoError(t, gotErr)
}

func TestBase_checkAllClientsStatus_WithClientCheckFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	volume := "test-volume"
	authClients := []string{"client1", "client2"}
	sharePath := "/test-volume"
	vStoreID := "fake-vstore-id"

	// mock
	cli.EXPECT().GetvStoreID().Return(vStoreID).AnyTimes()
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client1", vStoreID,
		constants.AuthClientReadWrite).Return(false, nil).AnyTimes()
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client2", vStoreID,
		constants.AuthClientReadWrite).Return(true, nil).AnyTimes()

	// action
	gotErr := plugin.checkAllClientsStatus(ctx, volume, authClients, true)

	// assert
	assert.ErrorContains(t, gotErr, "timeout")
}

func TestBase_checkAllClientsStatus_WithClientCheckError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	volume := "test-volume"
	authClients := []string{"client1", "client2"}
	sharePath := "/test-volume"
	vStoreID := "fake-vstore-id"
	fakeErr := errors.New("check error")

	// mock
	cli.EXPECT().GetvStoreID().Return(vStoreID)
	cli.EXPECT().GetvStoreID().Return(vStoreID)
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client1", vStoreID,
		constants.AuthClientReadWrite).Return(false, fakeErr)
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client2", vStoreID,
		constants.AuthClientReadWrite).Return(true, nil)

	// action
	gotErr := plugin.checkAllClientsStatus(ctx, volume, authClients, true)

	// assert
	assert.ErrorContains(t, gotErr, "check error")
}

func TestBase_checkAllClientsStatus_NotSupport(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	volume := "test-volume"
	authClients := []string{"client1", "client2"}
	sharePath := "/test-volume"
	vStoreID := "fake-vstore-id"
	NotFoundError := errors.New("invalid character 'S' looking for beginning of value")

	// mock
	cli.EXPECT().GetvStoreID().Return(vStoreID)
	cli.EXPECT().GetvStoreID().Return(vStoreID)
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client1", vStoreID,
		constants.AuthClientReadWrite).Return(false, NotFoundError)
	cli.EXPECT().CheckNfsShareAccessStatus(ctx, sharePath, "client2", vStoreID,
		constants.AuthClientReadWrite).Return(false, NotFoundError)

	// action
	gotErr := plugin.checkAllClientsStatus(ctx, volume, authClients, true)

	// assert
	assert.NoError(t, gotErr)
}

func TestBase_getExistingAuthClientAttr(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	fakeVStoreID := "fake-vstore-id"
	name := "fake-client"
	accessVal := 1
	wantReq := &base.AllowNfsShareAccessRequest{
		Name:        name,
		ParentID:    shareID,
		VStoreID:    fakeVStoreID,
		AccessVal:   accessVal,
		AllSquash:   constants.NoAllSquashValue,
		RootSquash:  constants.NoRootSquashValue,
		AccessKrb5:  creator.AccessKrb5ReadNoneInt,
		AccessKrb5i: creator.AccessKrb5ReadNoneInt,
		AccessKrb5p: creator.AccessKrb5ReadNoneInt,
	}

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVStoreID).AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, fakeVStoreID, int64(0), int64(1)).Return([]interface{}{}, nil)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, wantReq, gotReq)
}

func TestBase_getExistingAuthClientAttr_GetClientsError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	name := "fake-client"
	accessVal := 1
	fakeErr := errors.New("get clients error")

	// mock
	cli.EXPECT().GetvStoreID().Return("fake-vstore-id").AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, "fake-vstore-id", int64(0), int64(1)).Return(nil, fakeErr)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotReq)
	assert.Contains(t, gotErr.Error(), "failed to get auth client")
}

func TestBase_getExistingAuthClientAttr_ClientConvertFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	name := "fake-client"
	accessVal := 1
	clients := []interface{}{"invalid-client"}

	// mock
	cli.EXPECT().GetvStoreID().Return("fake-vstore-id").AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, "fake-vstore-id", int64(0), int64(1)).Return(clients, nil)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotReq)
	assert.Contains(t, gotErr.Error(), "convert client")
}

func TestBase_getExistingAuthClientAttr_AllSquashConvertFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	name := "fake-client"
	accessVal := 1
	clients := []interface{}{map[string]interface{}{
		"ALLSQUASH": "invalid-value",
	}}

	// mock
	cli.EXPECT().GetvStoreID().Return("fake-vstore-id").AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, "fake-vstore-id", int64(0), int64(1)).Return(clients, nil)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotReq)
	assert.Contains(t, gotErr.Error(), "convert allSquash")
}

func TestBase_getExistingAuthClientAttr_RootSquashConvertFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	name := "fake-client"
	accessVal := 1
	clients := []interface{}{map[string]interface{}{
		"ALLSQUASH":  "0",
		"ROOTSQUASH": "invalid-value",
	}}

	// mock
	cli.EXPECT().GetvStoreID().Return("fake-vstore-id").AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, "fake-vstore-id", int64(0), int64(1)).Return(clients, nil)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotReq)
	assert.Contains(t, gotErr.Error(), "convert rootSquash")
}

func TestBase_getExistingAuthClientAttr_AccessKrb5ConvertFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	name := "fake-client"
	accessVal := 1
	clients := []interface{}{map[string]interface{}{
		"ALLSQUASH":  "0",
		"ROOTSQUASH": "0",
		"ACCESSKRB5": "invalid-value",
	}}

	// mock
	cli.EXPECT().GetvStoreID().Return("fake-vstore-id").AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, "fake-vstore-id", int64(0), int64(1)).Return(clients, nil)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotReq)
	assert.Contains(t, gotErr.Error(), "convert accessKrb5")
}

func TestBase_getExistingAuthClientAttr_AccessKrb5iConvertFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	name := "fake-client"
	accessVal := 1
	clients := []interface{}{map[string]interface{}{
		"ALLSQUASH":   "0",
		"ROOTSQUASH":  "0",
		"ACCESSKRB5":  "0",
		"ACCESSKRB5I": "invalid-value",
	}}

	// mock
	cli.EXPECT().GetvStoreID().Return("fake-vstore-id").AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, "fake-vstore-id", int64(0), int64(1)).Return(clients, nil)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotReq)
	assert.Contains(t, gotErr.Error(), "convert accessKrb5i")
}

func TestBase_getExistingAuthClientAttr_AccessKrb5pConvertFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	name := "fake-client"
	accessVal := 1
	clients := []interface{}{map[string]interface{}{
		"ALLSQUASH":   "0",
		"ROOTSQUASH":  "0",
		"ACCESSKRB5":  "0",
		"ACCESSKRB5I": "0",
		"ACCESSKRB5P": "invalid-value",
	}}

	// mock
	cli.EXPECT().GetvStoreID().Return("fake-vstore-id").AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, "fake-vstore-id", int64(0), int64(1)).Return(clients, nil)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.Error(t, gotErr)
	assert.Nil(t, gotReq)
	assert.Contains(t, gotErr.Error(), "convert accessKrb5p")
}

func TestBase_getExistingAuthClientAttr_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	plugin := &Base{cli: cli}
	shareID := "fake-share-id"
	name := "fake-client"
	accessVal := 1
	clients := []interface{}{map[string]interface{}{
		"ALLSQUASH":   "1",
		"ROOTSQUASH":  "1",
		"ACCESSKRB5":  "2",
		"ACCESSKRB5I": "3",
		"ACCESSKRB5P": "4",
	}}
	wantReq := &base.AllowNfsShareAccessRequest{
		Name:        name,
		ParentID:    shareID,
		VStoreID:    "fake-vstore-id",
		AccessVal:   accessVal,
		AllSquash:   1,
		RootSquash:  1,
		AccessKrb5:  2,
		AccessKrb5i: 3,
		AccessKrb5p: 4,
	}

	// mock
	cli.EXPECT().GetvStoreID().Return("fake-vstore-id").AnyTimes()
	cli.EXPECT().GetNfsShareAccessRange(ctx, shareID, "fake-vstore-id", int64(0), int64(1)).Return(clients, nil)

	// action
	gotReq, gotErr := plugin.getExistingAuthClientAttr(ctx, shareID, name, accessVal)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, wantReq, gotReq)
}
