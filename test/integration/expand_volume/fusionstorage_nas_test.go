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

// Package expand_volume includes the integration tests of volume expanding
package expand_volume

import (
	"context"
	"math"
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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

const invalidQuota = math.MaxUint64

func TestControllerExpandVolume_FusionStorageNas_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	cases := []struct {
		name string
		data *fusionStorageNas
	}{
		{name: "expand nas with hard quota", data: fakeFusionStorageNasExpandHardQuotaSuccess()},
		{name: "expand nas with soft quota", data: fakeFusionStorageNasExpandSoftQuotaSuccess()},
		{name: "expand nas with both hard and soft quota", data: fakeFusionStorageNasExpandHardAndSoftQuotaSuccess()},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.data
			mockCtrl := gomock.NewController(t)
			cli := mock_client.NewMockIRestClient(mockCtrl)
			cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
			defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

			// mock
			p := gomonkey.NewPatches().ApplyMethodReturn(
				app.GetGlobalConfig().K8sUtils, "GetVolumeAttrByVolumeId",
				data.fakeVolumeAttributes(), nil)
			defer p.Reset()
			cli.EXPECT().GetQuotaByFileSystemName(ctx, data.FsName).Return(data.fakeFsQuota(), nil)
			cli.EXPECT().UpdateQuota(ctx, data.expectedUpdateQuotaParams()).Return(nil)
			cli.EXPECT().Logout(ctx)

			// action
			resp, err := csiServer.ControllerExpandVolume(ctx, data.request())

			// assert
			require.NoError(t, err)
			require.Equal(t, data.response(), resp)
		})
	}
}

func TestControllerExpandVolume_FusionStorageNas_FailedWithEmptyQuota(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeFusionStorageNasExpandWithEmptyQuota()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().ApplyMethodReturn(
		app.GetGlobalConfig().K8sUtils, "GetVolumeAttrByVolumeId",
		data.fakeVolumeAttributes(), nil)
	defer p.Reset()
	cli.EXPECT().GetQuotaByFileSystemName(ctx, data.FsName).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.ControllerExpandVolume(ctx, data.request())

	// assert
	require.ErrorContains(t, err, "does not contain id field")
}

func fakeFusionStorageNasExpandHardQuotaSuccess() *fusionStorageNas {
	var expandedCapacity int64 = 1024 * 1024 * 1024
	return &fusionStorageNas{
		FsName:   "test-fs-name",
		Capacity: expandedCapacity,

		BackendName: "test-fusion-nas-name",
		PoolName:    "test-pool-name",
		Protocol:    "nfs",
		AccountName: "system",

		FakeHardQuota: 1024 * 1024,
		FakeSoftQuota: invalidQuota,
		FakeQuotaID:   "101@1",

		ExpectedSpaceHardQuota: expandedCapacity,
	}
}

func fakeFusionStorageNasExpandSoftQuotaSuccess() *fusionStorageNas {
	data := fakeFusionStorageNasExpandHardQuotaSuccess()
	data.FakeHardQuota = invalidQuota
	data.FakeSoftQuota = 1024 * 1024
	data.ExpectedSpaceHardQuota = 0
	data.ExpectedSpaceSoftQuota = data.Capacity

	return data
}

func fakeFusionStorageNasExpandHardAndSoftQuotaSuccess() *fusionStorageNas {
	data := fakeFusionStorageNasExpandHardQuotaSuccess()
	data.FakeHardQuota = 1024 * 1024
	data.FakeSoftQuota = 1024 * 1024
	data.ExpectedSpaceHardQuota = data.Capacity
	data.ExpectedSpaceSoftQuota = 0

	return data
}

func fakeFusionStorageNasExpandWithEmptyQuota() *fusionStorageNas {
	var expandedCapacity int64 = 1024 * 1024 * 1024
	return &fusionStorageNas{
		FsName:   "test-fs-name",
		Capacity: expandedCapacity,

		BackendName: "test-fusion-nas-name",
		PoolName:    "test-pool-name",
		Protocol:    "nfs",
		AccountName: "system",

		FakeHardQuota: 1024 * 1024,
		FakeSoftQuota: invalidQuota,
		FakeQuotaID:   "101@1",

		ExpectedSpaceHardQuota: expandedCapacity,
	}
}

type fusionStorageNas struct {
	FsName   string
	Capacity int64

	BackendName string
	PoolName    string
	Protocol    string
	AccountName string

	DisableVerifyCapacity string

	FakeHardQuota uint64
	FakeSoftQuota uint64
	FakeQuotaID   string

	ExpectedSpaceHardQuota int64
	ExpectedSpaceSoftQuota int64
}

func (f *fusionStorageNas) expectedUpdateQuotaParams() map[string]any {
	res := map[string]any{"id": f.FakeQuotaID}

	if f.ExpectedSpaceHardQuota != 0 {
		res["space_hard_quota"] = f.ExpectedSpaceHardQuota
	}

	if f.ExpectedSpaceSoftQuota != 0 {
		res["space_soft_quota"] = f.ExpectedSpaceSoftQuota
	}

	return res
}

func (f *fusionStorageNas) fakeVolumeAttributes() map[string]string {
	return map[string]string{
		"backend":                          f.BackendName,
		constants.DTreeParentKey:           "",
		constants.DisableVerifyCapacityKey: f.DisableVerifyCapacity,
		"fsPermission":                     "",
		"name":                             f.FsName,
	}
}

func (f *fusionStorageNas) fakeFsQuota() *client.QueryQuotaResponse {
	return &client.QueryQuotaResponse{
		Id:             f.FakeQuotaID,
		SpaceHardQuota: f.FakeHardQuota,
		SpaceSoftQuota: f.FakeSoftQuota,
		SpaceUnitType:  0,
	}
}

func (f *fusionStorageNas) request() *csi.ControllerExpandVolumeRequest {
	return &csi.ControllerExpandVolumeRequest{
		VolumeId:      f.BackendName + "." + f.FsName,
		CapacityRange: &csi.CapacityRange{RequiredBytes: f.Capacity},
	}
}

func (f *fusionStorageNas) response() *csi.ControllerExpandVolumeResponse {
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         f.Capacity,
		NodeExpansionRequired: false,
	}
}

func (f *fusionStorageNas) backend(cli client.IRestClient) model.Backend {
	p := &plugin.FusionStorageNasPlugin{}
	p.SetCli(cli)
	return model.Backend{
		Name:        f.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.FusionNas,
		Available:   true,
		Plugin:      p,
		Parameters:  map[string]any{"protocol": f.Protocol},
		AccountName: f.AccountName,
		Pools: []*model.StoragePool{
			{
				Name:    f.PoolName,
				Storage: constants.FusionNas,
				Parent:  f.BackendName,
				Capabilities: map[string]bool{
					"SupportClone": false,
					"SupportNFS3":  true,
					"SupportNFS4":  false,
					"SupportNFS41": true,
					"SupportQoS":   true,
					"SupportQuota": true,
					"SupportThick": false,
					"SupportThin":  true,
				},
				Capacities: map[string]string{},
				Plugin:     p,
			},
		},
	}
}
