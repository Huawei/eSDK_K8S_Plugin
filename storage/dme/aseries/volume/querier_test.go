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
	fakeQueryFilesystemModel = &QueryVolumeModel{
		Name: fakeFsName,
	}

	fakeCapacity = int64(52521566208) // The unit is byte.
)

func TestQuerier_Query_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeQueryFilesystemModel)

	// mock
	fsInfo := &client.FileSystemInfo{ID: fakeFsID,
		TotalCapacityInByte: fakeCapacity}
	cli.EXPECT().GetFileSystemByName(querier.ctx, querier.params.Name).Return(fsInfo, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeFsName, volume.GetVolumeName())
	assert.Equal(t, fakeCapacity, volume.GetSize())
}

func TestQuerier_Query_Error(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockDMEASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeQueryFilesystemModel)

	// mock
	cli.EXPECT().GetFileSystemByName(querier.ctx, querier.params.Name).Return(nil, mockErr)

	// action
	volume, err := querier.Query()

	// assert
	assert.Error(t, err)
	assert.Nil(t, volume)
	assert.ErrorIs(t, err, mockErr)
}
