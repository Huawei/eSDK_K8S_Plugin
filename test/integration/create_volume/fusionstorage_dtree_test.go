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

// Package delete_volume includes the integration tests of creating volume
package create_volume

import (
	"context"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/accessmodes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/utils"
)

func TestCreateVolume_FusionDTree_FullFeaturesSuccess(t *testing.T) {
	// arrange
	data := fakeFusionDtreeDataWithSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetDTreeByName(ctx, data.ExpectedParentName, data.Name).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, data.ExpectedParentName, data.Name, data.Permission).
		Return(&client.DTreeResponse{Id: data.FakeDTreeID}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, data.FakeDTreeID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, data.FakeDTreeID,
		data.Capacity).Return(&client.DTreeQuotaResponse{Id: data.FakeQuotaID, ParentId: data.FakeDTreeID,
		SpaceHardQuota: 1024 * 1024, SpaceUnitType: 0}, nil)
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, data.expectedSharePath()).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx,
		&client.CreateDTreeNfsShareRequest{DtreeId: data.FakeDTreeID, Sharepath: data.expectedSharePath(),
			Description: data.Description}).Return(&client.CreateDTreeNfsShareResponse{Id: data.FakeShareID}, nil)
	cli.EXPECT().AddNfsShareAuthClient(ctx,
		&client.AddNfsShareAuthClientRequest{AccessName: data.AuthClient, ShareId: data.FakeShareID, AccessValue: 1,
			Sync: 0, AllSquash: data.ExpectedAllSquashParam, RootSquash: data.ExpectedRootSquashParam})
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_FusionDTree_NfsShareExistsSuccess(t *testing.T) {
	// arrange
	data := fakeFusionDtreeDataWithSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetDTreeByName(ctx, data.ExpectedParentName, data.Name).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, data.ExpectedParentName, data.Name, data.Permission).
		Return(&client.DTreeResponse{Id: data.FakeDTreeID}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, data.FakeDTreeID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, data.FakeDTreeID,
		data.Capacity).Return(&client.DTreeQuotaResponse{Id: data.FakeQuotaID, ParentId: data.FakeDTreeID,
		SpaceHardQuota: 1024 * 1024, SpaceUnitType: 0}, nil)
	// Simulate that NFS share already exists
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, data.expectedSharePath()).Return(&client.GetDTreeNfsShareResponse{
		Id:        data.FakeShareID,
		SharePath: data.expectedSharePath(),
	}, nil)
	// Delete old nfs share first
	cli.EXPECT().DeleteDTreeNfsShare(ctx, data.FakeShareID).Return(nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx,
		&client.CreateDTreeNfsShareRequest{DtreeId: data.FakeDTreeID, Sharepath: data.expectedSharePath(),
			Description: data.Description}).Return(&client.CreateDTreeNfsShareResponse{Id: data.FakeShareID}, nil)
	cli.EXPECT().AddNfsShareAuthClient(ctx,
		&client.AddNfsShareAuthClientRequest{AccessName: data.AuthClient, ShareId: data.FakeShareID, AccessValue: 1,
			Sync: 0, AllSquash: data.ExpectedAllSquashParam, RootSquash: data.ExpectedRootSquashParam})
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_FusionDTree_FullFeaturesOtherParamsSuccess(t *testing.T) {
	// arrange
	data := fakeFusionDtreeDataWithOtherParamsSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetDTreeByName(ctx, data.ExpectedParentName, data.Name).Return(nil, nil)
	cli.EXPECT().CreateDTree(ctx, data.ExpectedParentName, data.Name, data.Permission).
		Return(&client.DTreeResponse{Id: data.FakeDTreeID}, nil)
	cli.EXPECT().GetQuotaByDTreeId(ctx, data.FakeDTreeID).Return(nil, nil)
	cli.EXPECT().CreateDTreeQuota(ctx, data.FakeDTreeID,
		data.Capacity).Return(&client.DTreeQuotaResponse{Id: data.FakeQuotaID, ParentId: data.FakeDTreeID,
		SpaceHardQuota: 1024 * 1024, SpaceUnitType: 0}, nil)
	cli.EXPECT().GetDTreeNfsShareByPath(ctx, data.expectedSharePath()).Return(nil, nil)
	cli.EXPECT().CreateDTreeNfsShare(ctx,
		&client.CreateDTreeNfsShareRequest{DtreeId: data.FakeDTreeID, Sharepath: data.expectedSharePath(),
			Description: data.Description}).Return(&client.CreateDTreeNfsShareResponse{Id: data.FakeShareID}, nil)
	cli.EXPECT().AddNfsShareAuthClient(ctx,
		&client.AddNfsShareAuthClientRequest{AccessName: data.AuthClient, ShareId: data.FakeShareID, AccessValue: 1,
			Sync: 0, AllSquash: data.ExpectedAllSquashParam, RootSquash: data.ExpectedRootSquashParam})
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func fakeFusionDtreeDataWithOtherParamsSuccess() *fusionDTree {
	return &fusionDTree{
		Name:                  "pvc-test-dtree",
		Capacity:              1025,
		AllocType:             "thin",
		VolumeType:            "dtree",
		AuthClient:            "*",
		Permission:            "755",
		DisableVerifyCapacity: "true",
		AllSquash:             "no_all_squash",
		RootSquash:            "no_root_squash",
		ScParentName:          "",
		Description:           "test-description",

		BackendName:       "test-fusionstorage-dtree-backend",
		BackendParentName: "test-backend-parentname",
		Protocol:          constants.ProtocolNfs,

		ExpectedAllSquashParam:  1,
		ExpectedRootSquashParam: 1,
		ExpectedParentName:      "test-backend-parentname",

		FakeVStoreID: "",
		FakeFsID:     "fake-fs-id",
		FakeDTreeID:  "fake-dtree-id",
		FakeShareID:  "fake-share-id",
		FakeQuotaID:  "fake-quota-id",
	}
}

func fakeFusionDtreeDataWithSuccess() *fusionDTree {
	return &fusionDTree{
		Name:                  "pvc-test-dtree",
		Capacity:              1024,
		AllocType:             "thin",
		VolumeType:            "dtree",
		AuthClient:            "*",
		Permission:            "777",
		DisableVerifyCapacity: "false",
		AllSquash:             "all_squash",
		RootSquash:            "root_squash",
		ScParentName:          "test-sc-parent-name",
		Description:           "test-description",

		BackendName:       "test-fusionstorage-dtree-backend",
		BackendParentName: "",
		Protocol:          constants.ProtocolNfs,

		ExpectedAllSquashParam:  0,
		ExpectedRootSquashParam: 0,
		ExpectedParentName:      "test-sc-parent-name",

		FakeVStoreID: "",
		FakeFsID:     "fake-fs-id",
		FakeDTreeID:  "fake-dtree-id",
		FakeShareID:  "fake-share-id",
		FakeQuotaID:  "fake-quota-id",
	}
}

type fusionDTree struct {
	Name     string
	Capacity int64

	AllocType             string `sc:"allocType"`
	VolumeType            string `sc:"volumeType"`
	AuthClient            string `sc:"authClient"`
	Permission            string `sc:"fsPermission"`
	DisableVerifyCapacity string `sc:"disableVerifyCapacity"`
	AllSquash             string `sc:"allSquash"`
	RootSquash            string `sc:"rootSquash"`
	ScParentName          string `sc:"parentname"`
	Description           string `sc:"description"`

	BackendName       string `sc:"backend"`
	BackendParentName string
	Protocol          string

	ExpectedAllSquashParam  int
	ExpectedRootSquashParam int
	ExpectedParentName      string

	FakeVStoreID string
	FakeFsID     string
	FakeDTreeID  string
	FakeShareID  string
	FakeQuotaID  string
}

func (dtree *fusionDTree) backend(cli client.IRestClient) model.Backend {
	p := &plugin.FusionStorageDTreePlugin{}
	p.SetCli(cli)
	p.SetParentName(dtree.BackendParentName)
	p.SetProtocol(dtree.Protocol)
	return model.Backend{
		Name:        dtree.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.FusionDTree,
		Available:   true,
		Plugin:      p,
		Pools: []*model.StoragePool{
			{
				Name:    dtree.BackendName,
				Storage: constants.FusionDTree,
				Parent:  dtree.BackendName,
				Capabilities: map[string]bool{
					"SupportNFS3":            true,
					"SupportNFS4":            false,
					"SupportNFS41":           true,
					"SupportQoS":             false,
					"SupportReplication":     false,
					"SupportThick":           false,
					"SupportThin":            true,
					"SupportApplicationType": false,
					"SupportClone":           false,
					"SupportMetro":           false,
					"SupportMetroNAS":        false,
				},
				Capacities: map[string]string{},
				Plugin:     p,
			},
		},
		Parameters: map[string]any{"parentname": dtree.BackendParentName},
	}
}

func (dtree *fusionDTree) request() *csi.CreateVolumeRequest {
	accessMode, _ := accessmodes.ToCSIAccessMode([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, false)

	req := &csi.CreateVolumeRequest{
		Name:       dtree.Name,
		Parameters: utils.StructToStringMap(dtree, "sc"),
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						FsType:     "",
						MountFlags: nil,
					},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: accessMode},
			},
		},
		CapacityRange: &csi.CapacityRange{RequiredBytes: dtree.Capacity},
	}

	return req
}

func (dtree *fusionDTree) response() *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: dtree.Capacity,
			VolumeId:      dtree.BackendName + "." + dtree.Name,
			VolumeContext: map[string]string{
				"backend":                          dtree.BackendName,
				"name":                             dtree.Name,
				"fsPermission":                     dtree.Permission,
				constants.DTreeParentKey:           dtree.ExpectedParentName,
				constants.DisableVerifyCapacityKey: dtree.DisableVerifyCapacity,
			},
			ContentSource:      dtree.request().GetVolumeContentSource(),
			AccessibleTopology: make([]*csi.Topology, 0),
		},
	}
}

func (dtree *fusionDTree) expectedCreateNfsShareParams() map[string]any {
	return map[string]any{
		"sharepath":   dtree.expectedSharePath(),
		"fsid":        dtree.FakeFsID,
		"description": "Created from Kubernetes CSI",
		"vStoreID":    dtree.FakeVStoreID,
		"DTREEID":     dtree.FakeDTreeID,
	}
}

func (dtree *fusionDTree) expectedSharePath() string {
	return fmt.Sprintf("/%s/%s", dtree.ExpectedParentName, dtree.Name)
}
