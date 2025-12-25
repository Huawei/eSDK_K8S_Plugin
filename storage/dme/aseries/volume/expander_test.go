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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

var (
	fakeExpanderModel = &ExpandVolumeModel{
		Name:     fakeFsName,
		Capacity: fakeCapacity,
	}
)

func TestExpander_Expand_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpanderModel)

	// mock
	var fakeFsCapacity = int64(42521566208)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID, TotalCapacityInByte: fakeFsCapacity, StoragePoolName: fakePoolName}
	cli.EXPECT().GetFileSystemByName(expander.ctx, expander.params.Name).Return(fsInfo, nil)

	pool := &client.HyperScalePool{}
	cli.EXPECT().GetHyperScalePoolByName(expander.ctx, fsInfo.StoragePoolName).Return(pool, nil)

	params := &client.UpdateFileSystemParams{Capacity: transDmeCapacityFromByteIoGb(fakeExpanderModel.Capacity)}
	cli.EXPECT().UpdateFileSystem(expander.ctx, fsInfo.ID, params).Return(nil)

	// action
	err := expander.Expand()

	// assert
	assert.NoError(t, err)
}

func TestExpander_Expand_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpanderModel)

	// mock
	var fakeFsCapacity = int64(42521566208)
	fsInfo := &client.FileSystemInfo{ID: fakeFsID, TotalCapacityInByte: fakeFsCapacity, StoragePoolName: fakePoolName}
	cli.EXPECT().GetFileSystemByName(expander.ctx, expander.params.Name).Return(fsInfo, nil)

	pool := &client.HyperScalePool{}
	cli.EXPECT().GetHyperScalePoolByName(expander.ctx, fsInfo.StoragePoolName).Return(pool, nil)

	params := &client.UpdateFileSystemParams{Capacity: transDmeCapacityFromByteIoGb(fakeExpanderModel.Capacity)}
	cli.EXPECT().UpdateFileSystem(expander.ctx, fsInfo.ID, params).Return(mockErr)

	// action
	err := expander.Expand()

	// assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, mockErr)
}
