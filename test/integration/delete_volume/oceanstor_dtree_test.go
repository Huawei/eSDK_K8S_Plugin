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

// Package delete_volume includes the integration tests of deleting volume
package delete_volume

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestDeleteVolume_OceanstorDTree_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorDtreeSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().ApplyMethodReturn(
		app.GetGlobalConfig().K8sUtils, "GetVolumeAttrByVolumeId",
		data.fakeVolumeAttributes(), nil)
	defer p.Reset()
	cli.EXPECT().GetFileSystemByName(ctx, data.ParentName).Return(map[string]any{"ID": "1"}, nil)
	cli.EXPECT().GetDTreeByName(ctx, "0", data.ParentName, data.FakeVstoreID,
		data.DTreeName).Return(data.fakeDtreeInfo(), nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.sharePath(), data.FakeVstoreID).
		Return(map[string]any{"ID": data.FakeShareID}, nil)
	cli.EXPECT().DeleteNfsShare(ctx, data.FakeShareID, data.FakeVstoreID).Return(nil)
	cli.EXPECT().DeleteDTreeByName(ctx, data.ParentName, data.DTreeName, data.FakeVstoreID).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func fakeOceanstorDtreeSuccess() *oceanstorDTree {
	return &oceanstorDTree{
		ParentName:     "test-parent-name",
		DTreeName:      "test-dtree-name",
		BackendName:    "test-oceanstor-dtree-backend",
		Protocol:       "nfs",
		FakeDTreeQuota: 1024 * 1024,
		FakeShareID:    "test-share-id",
		FakeQuotaID:    "test-quota-id",
		FakeDTreeID:    "test-dtree-id",
	}
}

type oceanstorDTree struct {
	ParentName string
	DTreeName  string

	BackendName string
	Protocol    string

	FakeDTreeQuota int64
	FakeShareID    string
	FakeQuotaID    string
	FakeDTreeID    string
	FakeVstoreID   string
}

func (f *oceanstorDTree) sharePath() string {
	return "/" + f.ParentName + "/" + f.DTreeName
}

func (f *oceanstorDTree) fakeBatchGetQuotaResponse() []any {
	return []any{
		map[string]any{
			"ID":             f.FakeQuotaID,
			"SPACEHARDQUOTA": f.FakeDTreeQuota,
		},
	}
}

func (f *oceanstorDTree) expectedBatchGetQuotaReq() map[string]any {
	return map[string]any{
		"PARENTTYPE":    client.ParentTypeDTree,
		"PARENTID":      f.FakeDTreeID,
		"range":         "[0-100]",
		"vstoreId":      f.FakeVstoreID,
		"QUERYTYPE":     "2",
		"SPACEUNITTYPE": client.SpaceUnitTypeBytes,
	}
}

func (f *oceanstorDTree) fakeDtreeInfo() map[string]any {
	return map[string]any{"ID": f.FakeDTreeID}
}

func (f *oceanstorDTree) fakeVolumeAttributes() map[string]string {
	return map[string]string{
		"backend":                f.BackendName,
		constants.DTreeParentKey: f.ParentName,
		"name":                   f.DTreeName,
	}
}

func (f *oceanstorDTree) request() *csi.DeleteVolumeRequest {
	return &csi.DeleteVolumeRequest{
		VolumeId: f.BackendName + "." + f.DTreeName,
	}
}

func (f *oceanstorDTree) response() *csi.DeleteVolumeResponse {
	return &csi.DeleteVolumeResponse{}
}

func (f *oceanstorDTree) backend(cli client.OceanstorClientInterface) model.Backend {
	p := &plugin.OceanstorDTreePlugin{}
	p.SetCli(cli)
	return model.Backend{
		Name:        f.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorDtree,
		Available:   true,
		Plugin:      p,
		Parameters:  map[string]any{"protocol": f.Protocol},
		Pools: []*model.StoragePool{
			{
				Name:    f.BackendName,
				Storage: constants.OceanStorDtree,
				Parent:  f.BackendName,
				Capabilities: map[string]bool{
					"SupportClone": false,
					"SupportNFS3":  true,
					"SupportNFS4":  true,
					"SupportNFS41": true,
					"SupportQoS":   false,
					"SupportQuota": false,
					"SupportThick": false,
					"SupportThin":  true,
				},
				Capacities: map[string]string{},
				Plugin:     p,
			},
		},
	}
}
