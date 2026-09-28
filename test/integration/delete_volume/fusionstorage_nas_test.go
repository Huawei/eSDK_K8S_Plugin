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

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/types"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestDeleteVolume_FusionStorageNas_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeFusionNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(data.fakeFsInfo(), nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath()).Return(data.fakeShareInfo(), nil)
	cli.EXPECT().DeleteNfsShare(ctx, data.FakeShareID).Return(nil)
	cli.EXPECT().GetQuotaByFileSystemById(ctx, data.FakeFsIDString).Return(data.fakeQuotaInfo(), nil)
	cli.EXPECT().DeleteQuota(ctx, data.FakeQuotaID).Return(nil)
	cli.EXPECT().GetQoSPolicyIdByFsName(ctx, data.FsName).Return(types.NoQoSPolicyId, nil)
	cli.EXPECT().DeleteFileSystem(ctx, data.FakeFsIDString).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestDeleteVolume_FusionStorageNas_FsNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeFusionNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestDeleteVolume_FusionStorageNas_GetFsFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeFusionNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(nil, fmt.Errorf("internal error"))
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.Error(t, err)
	require.Contains(t, err.Error(), "internal error")
}

func TestDeleteVolume_FusionStorageNas_ShareNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeFusionNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(data.fakeFsInfo(), nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath()).Return(nil, nil)
	cli.EXPECT().GetQuotaByFileSystemById(ctx, data.FakeFsIDString).Return(nil, nil)
	cli.EXPECT().GetQoSPolicyIdByFsName(ctx, data.FsName).Return(types.NoQoSPolicyId, nil)
	cli.EXPECT().DeleteFileSystem(ctx, data.FakeFsIDString).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestDeleteVolume_FusionStorageNas_QuotaNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeFusionNasDeleteSuccess()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(data.fakeFsInfo(), nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath()).Return(data.fakeShareInfo(), nil)
	cli.EXPECT().DeleteNfsShare(ctx, data.FakeShareID).Return(nil)
	cli.EXPECT().GetQuotaByFileSystemById(ctx, data.FakeFsIDString).Return(nil, nil)
	cli.EXPECT().GetQoSPolicyIdByFsName(ctx, data.FsName).Return(types.NoQoSPolicyId, nil)
	cli.EXPECT().DeleteFileSystem(ctx, data.FakeFsIDString).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestDeleteVolume_FusionStorageNas_WithQoS(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeFusionNasDeleteWithQoS()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(data.fakeFsInfo(), nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath()).Return(data.fakeShareInfo(), nil)
	cli.EXPECT().DeleteNfsShare(ctx, data.FakeShareID).Return(nil)
	cli.EXPECT().GetQuotaByFileSystemById(ctx, data.FakeFsIDString).Return(nil, nil)
	cli.EXPECT().GetQoSPolicyIdByFsName(ctx, data.FsName).Return(data.FakeQoSID, nil)
	cli.EXPECT().DisassociateConvergedQoSWithVolume(ctx, data.FsName).Return(nil)
	cli.EXPECT().GetQoSPolicyAssociationCount(ctx, data.FakeQoSID).Return(0, nil)
	cli.EXPECT().GetConvergedQoSNameByID(ctx, data.FakeQoSID).Return(data.FakeQoSName, nil)
	cli.EXPECT().DeleteConvergedQoS(ctx, data.FakeQoSName).Return(nil)
	cli.EXPECT().DeleteFileSystem(ctx, data.FakeFsIDString).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestDeleteVolume_FusionStorageNas_QoSWithRemainingAssociations(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeFusionNasDeleteWithQoS()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock - QoS still has other associations, should skip QoS deletion
	cli.EXPECT().GetFileSystemByName(ctx, data.FsName).Return(data.fakeFsInfo(), nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath()).Return(data.fakeShareInfo(), nil)
	cli.EXPECT().DeleteNfsShare(ctx, data.FakeShareID).Return(nil)
	cli.EXPECT().GetQuotaByFileSystemById(ctx, data.FakeFsIDString).Return(nil, nil)
	cli.EXPECT().GetQoSPolicyIdByFsName(ctx, data.FsName).Return(data.FakeQoSID, nil)
	cli.EXPECT().DisassociateConvergedQoSWithVolume(ctx, data.FsName).Return(nil)
	cli.EXPECT().GetQoSPolicyAssociationCount(ctx, data.FakeQoSID).Return(3, nil)
	cli.EXPECT().DeleteFileSystem(ctx, data.FakeFsIDString).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func fakeFusionNasDeleteSuccess() *fusionNasDelete {
	return &fusionNasDelete{
		FsName:         "pvc-test-name",
		BackendName:    "test-fusion-nas-backend",
		Protocol:       "nfs",
		FakeFsID:       float64(11),
		FakeFsIDString: "11",
		FakeShareID:    "fake-share-id",
		FakeQuotaID:    "fake-quota-id",
	}
}

func fakeFusionNasDeleteWithQoS() *fusionNasDelete {
	data := fakeFusionNasDeleteSuccess()
	data.FakeQoSID = 2
	data.FakeQoSName = "csi-qos-fs-20250101"
	return data
}

type fusionNasDelete struct {
	FsName      string
	BackendName string
	Protocol    string

	FakeFsID       float64
	FakeFsIDString string
	FakeShareID    string
	FakeQuotaID    string
	FakeQoSID      int
	FakeQoSName    string
}

func (f *fusionNasDelete) request() *csi.DeleteVolumeRequest {
	return &csi.DeleteVolumeRequest{
		VolumeId: f.BackendName + "." + f.FsName,
	}
}

func (f *fusionNasDelete) response() *csi.DeleteVolumeResponse {
	return &csi.DeleteVolumeResponse{}
}

func (f *fusionNasDelete) fakeFsInfo() map[string]any {
	return map[string]any{
		"id": f.FakeFsID,
	}
}

func (f *fusionNasDelete) expectedSharePath() string {
	return "/" + f.FsName + "/"
}

func (f *fusionNasDelete) fakeShareInfo() map[string]any {
	return map[string]any{
		"id":             f.FakeShareID,
		"file_system_id": f.FakeFsIDString,
	}
}

func (f *fusionNasDelete) fakeQuotaInfo() map[string]any {
	return map[string]any{
		"id": f.FakeQuotaID,
	}
}

func (f *fusionNasDelete) backend(cli client.IRestClient) model.Backend {
	p := &plugin.FusionStorageNasPlugin{}
	p.SetCli(cli)
	return model.Backend{
		Name:        f.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.FusionNas,
		Available:   true,
		Plugin:      p,
		Parameters:  map[string]any{"protocol": f.Protocol},
		Pools: []*model.StoragePool{
			{
				Name:    f.BackendName,
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
