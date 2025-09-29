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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

var (
	fakeExpandFilesystemModel = &ExpandFilesystemModel{
		Capacity: 1024,
		Name:     fakeFsName,
	}
)

func TestQuerier_Expand_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpandFilesystemModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID, "CAPACITY": "512", "PARENTNAME": fakePoolName}, nil)
	cli.EXPECT().GetPoolByName(ctx, fakePoolName).Return(map[string]interface{}{"ID": fakePoolID}, nil)
	cli.EXPECT().UpdateFileSystem(ctx, fakeFsID,
		map[string]interface{}{"CAPACITY": fakeExpandFilesystemModel.Capacity, "vstoreId": fakeVstoreID}).Return(nil)

	// action
	err := expander.Expand()

	// assert
	assert.NoError(t, err)
}

func TestQuerier_Expand_SuccessWithNoCapacityChange(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpandFilesystemModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID, "CAPACITY": "1024", "PARENTNAME": fakePoolName}, nil)

	// action
	err := expander.Expand()

	// assert
	assert.NoError(t, err)
}

func TestQuerier_Expand_InvalidCapacity(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpandFilesystemModel)
	wantErr := "is less than current size"

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID, "CAPACITY": "2048", "PARENTNAME": fakePoolName}, nil)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorContains(t, err, wantErr)
}

func TestQuerier_Expand_PoolNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpandFilesystemModel)
	wantErr := "is not exist"

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).
		Return(map[string]interface{}{"ID": fakeFsID, "CAPACITY": "512", "PARENTNAME": fakePoolName}, nil)
	cli.EXPECT().GetPoolByName(ctx, fakePoolName).Return(nil, nil)

	// action
	err := expander.Expand()

	// assert
	assert.ErrorContains(t, err, wantErr)
}
