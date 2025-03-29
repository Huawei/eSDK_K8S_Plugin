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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	fakeShareId = "fake-share-id"
)

func TestDeleter_Delete_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDTreeName)
	expectedSharePath := "/" + fakeParentName + "/" + fakeDTreeName

	// mock
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, expectedSharePath).
		Return(&client.GetDTreeNfsShareResponse{Id: fakeShareId, SharePath: expectedSharePath}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeShareId).Return(nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(map[string]any{"ID": "fakeId"}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).
		Return(&client.DTreeResponse{Id: fakeDTreeId, Name: fakeDTreeName}, nil)
	cli.EXPECT().DeleteDTree(ctx, fakeDTreeId).Return(nil)

	// action
	err := deleter.Delete()

	// assert
	require.NoError(t, err)
}

func TestDeleter_Delete_SuccessWithEmptyParent(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDTreeName)
	expectedSharePath := "/" + fakeParentName + "/" + fakeDTreeName
	log.MockInitLogging("test")
	defer log.MockStopLogging("test")

	// mock
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, expectedSharePath).Return(nil, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(nil, nil)

	// action
	err := deleter.Delete()

	// assert
	require.NoError(t, err)
}

func TestDeleter_Delete_SuccessWithEmptyDTree(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDTreeName)
	expectedSharePath := "/" + fakeParentName + "/" + fakeDTreeName
	log.MockInitLogging("test")
	defer log.MockStopLogging("test")

	// mock
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, expectedSharePath).Return(nil, nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(map[string]any{"ID": "fakeId"}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, nil)

	// action
	err := deleter.Delete()

	// assert
	require.NoError(t, err)
}

func TestDeleter_Delete_FailedWithGetNfsShareError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDTreeName)
	expectedSharePath := "/" + fakeParentName + "/" + fakeDTreeName

	// mock
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, expectedSharePath).Return(nil, errors.New("fake error"))

	// action
	err := deleter.Delete()

	// assert
	require.Error(t, err)
}

func TestDeleter_Delete_FailedWithGetDTreeError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	deleter := NewDeleter(ctx, cli, fakeParentName, fakeDTreeName)
	expectedSharePath := "/" + fakeParentName + "/" + fakeDTreeName

	// mock
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, expectedSharePath).
		Return(&client.GetDTreeNfsShareResponse{Id: fakeShareId, SharePath: expectedSharePath}, nil)
	cli.EXPECT().DeleteDTreeNfsShare(ctx, fakeShareId).Return(nil)
	cli.EXPECT().GetFileSystemByName(ctx, fakeParentName).Return(map[string]any{"ID": "fakeId"}, nil)
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, errors.New("fake error"))

	// action
	err := deleter.Delete()

	// assert
	require.Error(t, err)
}
