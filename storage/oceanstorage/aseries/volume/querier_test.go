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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

var (
	fakeQueryFilesystemModel = &QueryFilesystemModel{
		Name:         fakeFsName,
		WorkloadType: fakeWorkloadType,
	}
)

func TestQuerier_Query_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeQueryFilesystemModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(
		map[string]interface{}{
			"ID":             fakeFsID,
			"CAPACITY":       "1024",
			"workloadTypeId": fakeWorkloadTypeID,
		}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return(fakeWorkloadTypeID, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, fakeFsName, volume.GetVolumeName())
	assert.Equal(t, int64(1024*constants.AllocationUnitBytes), volume.GetSize())
}

func TestQuerier_Query_NotFoundError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeQueryFilesystemModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(nil, nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorContains(t, err, "does not exist")
	assert.Nil(t, volume)
}

func TestQuerier_Query_WorkloadNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeQueryFilesystemModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(
		map[string]interface{}{
			"ID":             fakeFsID,
			"CAPACITY":       "1024",
			"workloadTypeId": fakeWorkloadTypeID,
		}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return("", nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorContains(t, err, "does not exist")
	assert.Nil(t, volume)
}

func TestQuerier_Query_WorkloadNotMatchError(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	cli := mock_client.NewMockOceanASeriesClientInterface(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeQueryFilesystemModel)

	// mock
	cli.EXPECT().GetvStoreID().Return(fakeVstoreID)
	cli.EXPECT().GetFileSystemByName(ctx, fakeFsName, fakeVstoreID).Return(
		map[string]interface{}{
			"ID":             fakeFsID,
			"CAPACITY":       "1024",
			"workloadTypeId": fakeWorkloadTypeID,
		}, nil)
	cli.EXPECT().GetApplicationTypeByName(ctx, fakeWorkloadType).Return("wrong-id", nil)

	// action
	volume, err := querier.Query()

	// assert
	assert.ErrorContains(t, err, fakeWorkloadTypeID)
	assert.Nil(t, volume)
}
