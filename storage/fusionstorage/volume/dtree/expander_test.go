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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

var (
	fakeExpanderParam = &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDTreeName,
		Capacity:   1200,
	}

	fakeQuotaRes = &client.DTreeQuotaResponse{
		Id:             fakeDTreeId,
		SpaceHardQuota: 1000,
	}
)

func TestExpander_Expand_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpanderParam)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).
		Return(&client.DTreeResponse{Id: fakeDTreeId, Name: fakeDTreeName}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(fakeQuotaRes, nil)
	cli.EXPECT().UpdateDTreeQuota(ctx, fakeQuotaRes.Id, fakeExpanderParam.Capacity).Return(nil)

	// action
	err := expander.Expand()

	// assert
	require.NoError(t, err)
}

func TestExpander_Expand_Success_EmptyQuota(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpanderParam)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).
		Return(&client.DTreeResponse{Id: fakeDTreeId, Name: fakeDTreeName}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, fakeDTreeId, fakeExpanderParam.Capacity).Return(fakeQuotaRes, nil)

	// action
	err := expander.Expand()

	// assert
	require.NoError(t, err)
}

func TestExpander_Expand_Failed_CapacityIllegal(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	expander := NewExpander(ctx, cli, &ExpandDTreeModel{
		ParentName: fakeParentName,
		DTreeName:  fakeDTreeName,
		Capacity:   800,
	})

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).
		Return(&client.DTreeResponse{Id: fakeDTreeId, Name: fakeDTreeName}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(fakeQuotaRes, nil)

	// action
	err := expander.Expand()

	// assert
	require.Error(t, err)
}

func TestExpander_Expand_Failed_QuotaIdEmpty(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpanderParam)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).
		Return(&client.DTreeResponse{Id: fakeDTreeId, Name: fakeDTreeName}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, fakeDTreeId, fakeExpanderParam.Capacity).Return(&client.DTreeQuotaResponse{},
		nil)

	// action
	err := expander.Expand()

	// assert
	require.Error(t, err)
}

func TestExpander_Expand_Failed_GetDTreeFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpanderParam)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, fmt.Errorf("fake-error"))

	// action
	err := expander.Expand()

	// assert
	require.Error(t, err)
}

func TestExpander_Expand_Failed_DTreeNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpanderParam)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).Return(nil, nil)

	// action
	err := expander.Expand()

	// assert
	require.Error(t, err)
}

func TestExpander_Expand_Failed_GetQuotaFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	expander := NewExpander(ctx, cli, fakeExpanderParam)

	// mock
	cli.EXPECT().GetDTreeByName(ctx, fakeParentName, fakeDTreeName).
		Return(&client.DTreeResponse{Id: fakeDTreeId, Name: fakeDTreeName}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, fakeDTreeId).Return(nil, fmt.Errorf("fake-error"))

	// action
	err := expander.Expand()

	// assert
	require.Error(t, err)
}
