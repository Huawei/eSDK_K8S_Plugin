/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
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

// Package expand_volume includes the integration tests of volume expanding
package expand_volume

import (
	"context"
	"fmt"
	"strconv"
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

func TestControllerExpandVolume_OceanstorDTree_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	cases := []struct {
		name string
		data *oceanstorDTree
	}{
		{name: "expand success", data: fakeOceanstorDTreeSuccessData()},
		{name: "expand success with equivalent capacity", data: fakeOceanstorDtreeSuccessWithEquivalentCapacityData()},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.data
			mockCtrl := gomock.NewController(t)
			cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
			cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
			defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

			// mock
			p := gomonkey.NewPatches().ApplyMethodReturn(
				app.GetGlobalConfig().K8sUtils, "GetVolumeAttrByVolumeId",
				data.fakeVolumeAttributes(), nil)
			defer p.Reset()
			cli.EXPECT().GetDTreeByName(ctx, "", data.ParentName, data.FakeVstoreID,
				data.DTreeName).Return(data.fakeDtreeInfo(), nil)
			cli.EXPECT().BatchGetQuota(ctx, data.expectedBatchGetQuotaReq()).Return(data.fakeBatchGetQuotaResponse(),
				nil)
			cli.EXPECT().UpdateQuota(ctx, data.FakeQuotaID, data.expectedUpdateQuotaReq()).Return(nil)
			cli.EXPECT().Logout(ctx)

			// action
			resp, err := csiServer.ControllerExpandVolume(ctx, data.request())

			// assert
			require.NoError(t, err)
			require.Equal(t, data.response(), resp)
		})
	}
}

func TestControllerExpandVolume_OceanstorNas_FailedWithWrongCapacity(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorDtreeFailedWithWrongCapacityData()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().ApplyMethodReturn(
		app.GetGlobalConfig().K8sUtils, "GetVolumeAttrByVolumeId",
		data.fakeVolumeAttributes(), nil)
	defer p.Reset()
	cli.EXPECT().GetDTreeByName(ctx, "", data.ParentName, data.FakeVstoreID,
		data.DTreeName).Return(data.fakeDtreeInfo(), nil)
	cli.EXPECT().BatchGetQuota(ctx, data.expectedBatchGetQuotaReq()).Return(data.fakeBatchGetQuotaResponse(),
		nil)
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.ControllerExpandVolume(ctx, data.request())

	// assert
	require.ErrorContains(t, err,
		fmt.Sprintf("new quota size %d must be larger than old size %s", data.Capacity, data.FakeOldQuota))
}

func fakeOceanstorDtreeFailedWithWrongCapacityData() *oceanstorDTree {
	return &oceanstorDTree{
		ParentName:  "test-parent-name",
		DTreeName:   "test-dtree-name",
		Capacity:    1024 * 1024 * 1024,
		BackendName: "test-oceanstor-dtree-backend",
		Protocol:    "nfs",

		FakeOldQuota: strconv.Itoa(2 * 1024 * 1024 * 1024),
		FakeQuotaID:  "fake-quota-id",
		FakeDTreeID:  "fake-dtree-id",
	}
}

func fakeOceanstorDtreeSuccessWithEquivalentCapacityData() *oceanstorDTree {
	return &oceanstorDTree{
		ParentName:  "test-parent-name",
		DTreeName:   "test-dtree-name",
		Capacity:    1024 * 1024 * 1024,
		BackendName: "test-oceanstor-dtree-backend",
		Protocol:    "nfs",

		FakeOldQuota: strconv.Itoa(1024 * 1024 * 1024),
		FakeQuotaID:  "fake-quota-id",
		FakeDTreeID:  "fake-dtree-id",
	}
}

func fakeOceanstorDTreeSuccessData() *oceanstorDTree {
	return &oceanstorDTree{
		ParentName:  "test-parent-name",
		DTreeName:   "test-dtree-name",
		Capacity:    2 * 1024 * 1024 * 1024,
		BackendName: "test-oceanstor-dtree-backend",
		Protocol:    "nfs",

		FakeOldQuota: strconv.Itoa(1024 * 1024 * 1024),
		FakeQuotaID:  "fake-quota-id",
		FakeDTreeID:  "fake-dtree-id",
	}
}

type oceanstorDTree struct {
	ParentName string
	DTreeName  string
	Capacity   int64

	BackendName string
	Protocol    string

	DisableVerifyCapacity string

	FakeOldQuota string
	FakeQuotaID  string
	FakeDTreeID  string
	FakeVstoreID string
}

func (f *oceanstorDTree) expectedUpdateQuotaReq() map[string]any {
	return map[string]any{
		"SPACEHARDQUOTA": f.Capacity,
		"vstoreId":       f.FakeVstoreID,
	}
}

func (f *oceanstorDTree) fakeBatchGetQuotaResponse() []any {
	return []any{
		map[string]any{
			"ID":             f.FakeQuotaID,
			"SPACEHARDQUOTA": f.FakeOldQuota,
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
		"backend":                          f.BackendName,
		constants.DTreeParentKey:           f.ParentName,
		constants.DisableVerifyCapacityKey: f.DisableVerifyCapacity,
		"fsPermission":                     "",
		"name":                             f.DTreeName,
	}
}

func (f *oceanstorDTree) request() *csi.ControllerExpandVolumeRequest {
	return &csi.ControllerExpandVolumeRequest{
		VolumeId:      f.BackendName + "." + f.DTreeName,
		CapacityRange: &csi.CapacityRange{RequiredBytes: f.Capacity},
	}
}

func (f *oceanstorDTree) response() *csi.ControllerExpandVolumeResponse {
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         f.Capacity,
		NodeExpansionRequired: false,
	}
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
