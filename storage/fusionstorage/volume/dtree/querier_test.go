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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

var (
	fakeDtreeQuotaSpaceHard = int64(1024 * 1024)
	fakeDTreeQuotaId        = "fake-dtree-quota-id"
)

func TestQuerier_Query_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDTreeName, fakeParentName)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).
		Return(&client.DTreeQuotaResponse{Id: fakeDTreeQuotaId, SpaceHardQuota: fakeDtreeQuotaSpaceHard}, nil)

	// action
	volume, err := querier.Query()

	// assert
	require.NoError(t, err)
	require.NotNil(t, volume)
	require.Equal(t, fakeDTreeName, volume.GetVolumeName())
	require.Equal(t, fakeDtreeQuotaSpaceHard, volume.GetSize())
}

func TestQuerier_Query_Fail(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	querier := NewQuerier(ctx, cli, fakeDTreeName, fakeParentName)

	t.Run("test query dtree failed", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, fmt.Errorf("query dtree failed"))
		// action
		_, err := querier.Query()
		// assert
		require.EqualError(t, err, "query dtree failed")
	})

	t.Run("test nil dtree", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, nil)
		// action
		_, err := querier.Query()
		// assert
		require.EqualError(t, err, fmt.Sprintf("the dtree %q of parent %q is not exist", fakeDTreeName, fakeParentName))
	})

	t.Run("test query quota failed", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
		cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, fmt.Errorf("query dtree quota failed"))
		// action
		_, err := querier.Query()
		// assert
		require.EqualError(t, err, "query dtree quota failed")
	})

	t.Run("test nil quota", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
		cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, nil)
		// action
		_, err := querier.Query()
		// assert
		require.EqualError(t, err,
			fmt.Sprintf("the quota of dtree %q of parent %q is not exist", fakeDTreeName, fakeParentName))

	})

	t.Run("test zero hard quota", func(t *testing.T) {
		// mock
		cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(&client.DTreeResponse{Id: fakeDTreeId}, nil)
		cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).
			Return(&client.DTreeQuotaResponse{Id: fakeDTreeQuotaId}, nil)
		// action
		_, err := querier.Query()
		// assert
		require.EqualError(t, err,
			fmt.Sprintf("the SpaceHardQuota of dtree %q of parent %q is 0", fakeDTreeName, fakeParentName))
	})
}
