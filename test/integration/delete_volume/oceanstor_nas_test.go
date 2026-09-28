/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026. All rights reserved.
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
	"fmt"
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

func TestDeleteVolume_OceanstorNas_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetKvCacheStoreIdByVolumeId", "", nil)
	defer p.Reset()
	cli.EXPECT().GetCurrentLifWwn().Return("").AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return("").AnyTimes()
	// NAS.Delete calls GetFileSystemByName, then DeleteFS also calls GetFileSystemByName
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(data.fakeFsInfo(), nil).Times(2)
	cli.EXPECT().GetFSSnapshotCountByParentId(ctx, data.FakeFsID).Return(0, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath(), data.FakeVStoreID).
		Return(map[string]any{"ID": data.FakeShareID}, nil)
	cli.EXPECT().DeleteNfsShare(ctx, data.FakeShareID, data.FakeVStoreID).Return(nil)
	cli.EXPECT().DeleteFileSystem(ctx, data.expectedDeleteFsParams()).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestDeleteVolume_OceanstorNas_FsNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetKvCacheStoreIdByVolumeId", "", nil)
	defer p.Reset()
	cli.EXPECT().GetCurrentLifWwn().Return("").AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return("").AnyTimes()
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestDeleteVolume_OceanstorNas_SnapshotExists(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetKvCacheStoreIdByVolumeId", "", nil)
	defer p.Reset()
	cli.EXPECT().GetCurrentLifWwn().Return("").AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return("").AnyTimes()
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(data.fakeFsInfo(), nil)
	cli.EXPECT().GetFSSnapshotCountByParentId(ctx, data.FakeFsID).Return(2, nil)
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.Error(t, err)
}

func TestDeleteVolume_OceanstorNas_ShareNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetKvCacheStoreIdByVolumeId", "", nil)
	defer p.Reset()
	cli.EXPECT().GetCurrentLifWwn().Return("").AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return("").AnyTimes()
	// NAS.Delete calls GetFileSystemByName, then DeleteFS also calls GetFileSystemByName
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(data.fakeFsInfo(), nil).Times(2)
	cli.EXPECT().GetFSSnapshotCountByParentId(ctx, data.FakeFsID).Return(0, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath(), data.FakeVStoreID).Return(nil, nil)
	cli.EXPECT().DeleteFileSystem(ctx, data.expectedDeleteFsParams()).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestDeleteVolume_OceanstorNas_LogicPortFailover(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetKvCacheStoreIdByVolumeId", "", nil)
	defer p.Reset()
	cli.EXPECT().GetCurrentLifWwn().Return("site-wwn-a").AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return("site-wwn-b").AnyTimes()
	cli.EXPECT().GetCurrentLif(ctx).Return("test-lif").AnyTimes()
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.Error(t, err)
}

func TestDeleteVolume_OceanstorNas_GetFileSystemFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetKvCacheStoreIdByVolumeId", "", nil)
	defer p.Reset()
	cli.EXPECT().GetCurrentLifWwn().Return("").AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return("").AnyTimes()
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(nil, fmt.Errorf("internal error"))
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.Error(t, err)
}

func fakeOceanstorNasDeleteSuccess() *oceanstorNasDelete {
	return &oceanstorNasDelete{
		BackendName:  "test-nfs-backend",
		FsName:       "pvc_test_nas",
		FakeFsID:     "fake-fs-id",
		FakeShareID:  "fake-share-id",
		FakeVStoreID: "",
	}
}

type oceanstorNasDelete struct {
	BackendName string
	FsName      string

	FakeFsID     string
	FakeShareID  string
	FakeVStoreID string
}

func (f *oceanstorNasDelete) request() *csi.DeleteVolumeRequest {
	return &csi.DeleteVolumeRequest{
		VolumeId: f.BackendName + "." + f.FsName,
	}
}

func (f *oceanstorNasDelete) response() *csi.DeleteVolumeResponse {
	return &csi.DeleteVolumeResponse{}
}

func (f *oceanstorNasDelete) fakeFsInfo() map[string]any {
	return map[string]any{
		"ID":       f.FakeFsID,
		"vstoreId": f.FakeVStoreID,
	}
}

func (f *oceanstorNasDelete) expectedSharePath() string {
	return "/" + f.FsName + "/"
}

func (f *oceanstorNasDelete) expectedDeleteFsParams() map[string]any {
	return map[string]any{"ID": f.FakeFsID}
}

func (f *oceanstorNasDelete) backend(cli client.OceanstorClientInterface) model.Backend {
	p := &plugin.OceanstorNasPlugin{}
	p.SetCli(cli)
	p.SetProduct(constants.OceanStorDoradoV6)
	return model.Backend{
		Name:        f.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorNas,
		Available:   true,
		Plugin:      p,
		Parameters:  map[string]any{"protocol": "nfs"},
		Pools: []*model.StoragePool{
			{
				Name:    f.BackendName,
				Storage: constants.OceanStorNas,
				Parent:  f.BackendName,
				Capabilities: map[string]bool{
					"SupportApplicationType":    true,
					"SupportClone":              true,
					"SupportConsistentSnapshot": true,
					"SupportMetro":              false,
					"SupportNFS3":               true,
					"SupportNFS4":               true,
					"SupportNFS41":              true,
					"SupportNFS42":              true,
					"SupportQoS":                true,
					"SupportReplication":        false,
					"SupportThick":              false,
					"SupportThin":               true,
				},
				Capacities: map[string]string{},
				Plugin:     p,
			},
		},
	}
}
