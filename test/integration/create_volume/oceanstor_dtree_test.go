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

// Package create_volume includes the integration tests of creating volume
package create_volume

import (
	"context"
	"encoding/json"
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
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/utils"
)

func TestCreateVolume_OceanstorDTree_FullFeaturesSuccess(t *testing.T) {
	// arrange
	data := fakeOceanstorDtreeDataWithSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedParentName).Return(map[string]any{"ID": data.FakeFsID}, nil)
	cli.EXPECT().CreateDTree(ctx, data.expectedCreateDTreeParams(t)).Return(map[string]any{"ID": data.FakeDTreeID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath(), data.FakeVStoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, data.expectedCreateNfsShareParams()).
		Return(map[string]any{"ID": data.FakeShareID}, nil)
	cli.EXPECT().GetNfsShareAccessCount(ctx, data.FakeShareID, data.FakeVStoreID).Return(int64(0), nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, data.expectedAllowNfsShareRequest()).Return(nil)
	cli.EXPECT().CreateQuota(ctx, data.expectedCreateQuotaParam()).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_OceanstorDTree_SuccessWithScVolumeName(t *testing.T) {
	// arrange
	data := fakeOceanstorDtreeDataWithSuccess()
	data.ScVolumeName = "prefix-{{ .PVCNamespace }}-{{ .PVCName }}"
	data.ExpectedDTreeName = "prefix-pvcTestNamespace-pvcTestName-pvTestName"
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)
	createVolumeReq := data.request()
	createVolumeReq.Parameters[constants.PVCNamespaceKey] = "pvcTestNamespace"
	createVolumeReq.Parameters[constants.PVCNameKey] = "pvcTestName"
	createVolumeReq.Parameters[constants.PVNameKey] = "pvc-pvTestName"
	originPrefix := app.GetGlobalConfig().VolumeNamePrefix
	defer func() { app.GetGlobalConfig().VolumeNamePrefix = originPrefix }()
	app.GetGlobalConfig().VolumeNamePrefix = "pvc-"

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedParentName).Return(map[string]any{"ID": data.FakeFsID}, nil)
	cli.EXPECT().CreateDTree(ctx, data.expectedCreateDTreeParams(t)).Return(map[string]any{"ID": data.FakeDTreeID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath(), data.FakeVStoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, data.expectedCreateNfsShareParams()).
		Return(map[string]any{"ID": data.FakeShareID}, nil)
	cli.EXPECT().GetNfsShareAccessCount(ctx, data.FakeShareID, data.FakeVStoreID).Return(int64(0), nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, data.expectedAllowNfsShareRequest()).Return(nil)
	cli.EXPECT().CreateQuota(ctx, data.expectedCreateQuotaParam()).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, createVolumeReq)

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_OceanstorDTree_FailedWithInvalidScVolumeName(t *testing.T) {
	// arrange
	data := fakeOceanstorDtreeDataWithSuccess()
	data.ScVolumeName = "prefix-{{ .PVCNamespace }}"
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)
	createVolumeReq := data.request()

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.CreateVolume(ctx, createVolumeReq)

	// assert
	require.ErrorContains(t, err,
		"{{.PVCNamespace}} or {{.PVCName}} must be configured in the volumeName parameter at the same time")
}

func TestCreateVolume_OceanstorDTreeV5_SuccessWithScVolumeName(t *testing.T) {
	// arrange
	data := fakeOceanstorDtreeDataWithSuccess()
	data.ScVolumeName = "prefix-{{ .PVCNamespace }}-{{ .PVCName }}"
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorV5))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)
	createVolumeReq := data.request()
	createVolumeReq.Parameters[constants.PVCNamespaceKey] = "pvcTestNamespace"
	createVolumeReq.Parameters[constants.PVCNameKey] = "pvcTestName"
	createVolumeReq.Parameters[constants.PVNameKey] = "pvc-pvTestName"
	originPrefix := app.GetGlobalConfig().VolumeNamePrefix
	defer func() { app.GetGlobalConfig().VolumeNamePrefix = originPrefix }()
	app.GetGlobalConfig().VolumeNamePrefix = "pvc-"

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedParentName).Return(map[string]any{"ID": data.FakeFsID}, nil)
	cli.EXPECT().CreateDTree(ctx, data.expectedCreateDTreeParams(t)).Return(map[string]any{"ID": data.FakeDTreeID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath(), data.FakeVStoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, data.expectedCreateNfsShareParams()).
		Return(map[string]any{"ID": data.FakeShareID}, nil)
	cli.EXPECT().GetNfsShareAccessCount(ctx, data.FakeShareID, data.FakeVStoreID).Return(int64(0), nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, data.expectedAllowNfsShareRequest()).Return(nil)
	cli.EXPECT().CreateQuota(ctx, data.expectedCreateQuotaParam()).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, createVolumeReq)

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_OceanstorDTree_FailedWithInvalidCapacity(t *testing.T) {
	// arrange
	data := fakeOceanstorDTreeDataWithInvalidCapacity()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.ErrorContains(t, err, "not multiple of 512")
	require.Nil(t, resp)
}

func TestCreateVolume_OceanstorDTree_FailedWithEmptyParentName(t *testing.T) {
	// arrange
	data := fakeOceanstorDTreeDataWithEmptyParentName()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.ErrorContains(t, err, "no found any available dtree backend for volume")
	require.Nil(t, resp)
}

func TestCreateVolume_OceanstorDTree_FailedWithDifferentParentName(t *testing.T) {
	// arrange
	data := fakeOceanstorDTreeDataWithDifferentParentName()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.ErrorContains(t, err,
		fmt.Sprintf("parentname %q in StorageClass is not equal to %q in backend", data.ScParentName,
			data.BackendParentName))
	require.Nil(t, resp)
}

func TestCreateVolume_OceanstorDTree_AdvancedOptionsUnmarshalFailed(t *testing.T) {
	// arrange
	data := fakeOceanstorDtreeDataWithSuccess()
	data.AdvancedOptions = `{"CAPACITYTHRESHOLD": 90`
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedParentName).Return(map[string]any{"ID": data.FakeFsID}, nil)
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.ErrorContains(t, err, "failed to unmarshal advancedOptions")
}

func fakeOceanstorDTreeDataWithDifferentParentName() *oceanstorDTree {
	return &oceanstorDTree{
		Name:         "pvc-test-dtree",
		Capacity:     1024,
		AllocType:    "thin",
		VolumeType:   "dtree",
		AuthClient:   "*",
		ScParentName: "test-sc-parentname",

		BackendName:       "test-dtree-backend",
		BackendParentName: "test-bk-parentname",
	}
}

func fakeOceanstorDTreeDataWithEmptyParentName() *oceanstorDTree {
	return &oceanstorDTree{
		Name:       "pvc-test-dtree",
		Capacity:   1024,
		AllocType:  "thin",
		VolumeType: "dtree",
		AuthClient: "*",

		BackendName:       "test-dtree-backend",
		BackendParentName: "",
	}
}

func fakeOceanstorDTreeDataWithInvalidCapacity() *oceanstorDTree {
	return &oceanstorDTree{
		Name:                  "pvc-test-dtree",
		Capacity:              999, // Invalid capacity
		AllocType:             "thin",
		VolumeType:            "dtree",
		AuthClient:            "*",
		DisableVerifyCapacity: "false", // Enable verify capacity

		BackendName:       "test-dtree-backend",
		BackendParentName: "test-backend-parent-name",
	}
}

func fakeOceanstorDtreeDataWithSuccess() *oceanstorDTree {
	return &oceanstorDTree{
		Name:                  "pvc-test-dtree",
		Capacity:              1024,
		AllocType:             "thin",
		VolumeType:            "dtree",
		AuthClient:            "*",
		Permission:            "777",
		DisableVerifyCapacity: "false",
		AllSquash:             "all_squash",
		RootSquash:            "root_squash",
		AccessKrb5:            "read_write",
		AccessKrb5i:           "read_write",
		AccessKrb5p:           "read_write",
		AdvancedOptions:       `{"CAPACITYTHRESHOLD": 90}`,

		BackendName:       "test-dtree-backend",
		BackendParentName: "test-backend-parent-name",

		ExpectedDTreeName:        "pvc-test-dtree",
		ExpectedParentName:       "test-backend-parent-name",
		ExpectedAllSquashParam:   0,
		ExpectedRootSquashParam:  0,
		ExpectedAccessKrb5Param:  1,
		ExpectedAccessKrb5iParam: 1,
		ExpectedAccessKrb5pParam: 1,

		FakeVStoreID: "",
		FakeFsID:     "fake-fs-id",
		FakeDTreeID:  "fake-dtree-id",
		FakeShareID:  "fake-share-id",
	}
}

type oceanstorDTree struct {
	Name     string
	Capacity int64

	AllocType             string `sc:"allocType"`
	VolumeType            string `sc:"volumeType"`
	AuthClient            string `sc:"authClient"`
	Permission            string `sc:"fsPermission"`
	DisableVerifyCapacity string `sc:"disableVerifyCapacity"`
	AllSquash             string `sc:"allSquash"`
	RootSquash            string `sc:"rootSquash"`
	AccessKrb5            string `sc:"accesskrb5"`
	AccessKrb5i           string `sc:"accesskrb5i"`
	AccessKrb5p           string `sc:"accesskrb5p"`
	ScParentName          string `sc:"parentname"`
	AdvancedOptions       string `sc:"advancedOptions"`
	ScVolumeName          string `sc:"volumeName"`

	BackendName       string `sc:"backend"`
	BackendParentName string

	ExpectedDTreeName        string
	ExpectedParentName       string
	ExpectedAllSquashParam   int
	ExpectedRootSquashParam  int
	ExpectedAccessKrb5Param  int
	ExpectedAccessKrb5pParam int
	ExpectedAccessKrb5iParam int

	FakeVStoreID string
	FakeFsID     string
	FakeDTreeID  string
	FakeShareID  string
}

func (dtree *oceanstorDTree) backend(cli client.OceanstorClientInterface,
	product constants.OceanstorVersion) model.Backend {
	p := &plugin.OceanstorDTreePlugin{}
	p.SetCli(cli)
	p.SetParentName(dtree.BackendParentName)
	p.SetProduct(product)
	return model.Backend{
		Name:        dtree.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorDtree,
		Available:   true,
		Plugin:      p,
		Pools: []*model.StoragePool{
			{
				Name:    dtree.BackendName,
				Storage: constants.OceanStorDtree,
				Parent:  dtree.BackendName,
				Capabilities: map[string]bool{
					"SupportNFS3":            true,
					"SupportNFS4":            true,
					"SupportNFS41":           true,
					"SupportNFS42":           true,
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

func (dtree *oceanstorDTree) request() *csi.CreateVolumeRequest {
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

func (dtree *oceanstorDTree) response() *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: dtree.Capacity,
			VolumeId:      dtree.BackendName + "." + dtree.ExpectedDTreeName,
			VolumeContext: map[string]string{
				"backend":                          dtree.BackendName,
				"name":                             dtree.ExpectedDTreeName,
				"fsPermission":                     dtree.Permission,
				constants.DTreeParentKey:           dtree.BackendParentName,
				constants.DisableVerifyCapacityKey: dtree.DisableVerifyCapacity,
			},
			ContentSource:      dtree.request().GetVolumeContentSource(),
			AccessibleTopology: make([]*csi.Topology, 0),
		},
	}
}

func (dtree *oceanstorDTree) expectedCreateQuotaParam() map[string]any {
	res := map[string]any{
		"PARENTTYPE":     client.ParentTypeDTree,
		"PARENTID":       dtree.FakeDTreeID,
		"QUOTATYPE":      client.QuotaTypeDir,
		"SPACEHARDQUOTA": dtree.Capacity,
		"vstoreId":       nil,
	}
	if dtree.FakeVStoreID != "" {
		res["vstoreId"] = dtree.FakeVStoreID
	}

	return res
}

func (dtree *oceanstorDTree) expectedAllowNfsShareRequest() *client.AllowNfsShareAccessRequest {
	return &client.AllowNfsShareAccessRequest{
		Name:        dtree.AuthClient,
		ParentID:    dtree.FakeShareID,
		AccessVal:   1,
		Sync:        0,
		AllSquash:   dtree.ExpectedAllSquashParam,
		RootSquash:  dtree.ExpectedRootSquashParam,
		VStoreID:    dtree.FakeVStoreID,
		AccessKrb5:  dtree.ExpectedAccessKrb5Param,
		AccessKrb5i: dtree.ExpectedAccessKrb5iParam,
		AccessKrb5p: dtree.ExpectedAccessKrb5pParam,
	}
}

func (dtree *oceanstorDTree) expectedCreateNfsShareParams() map[string]any {
	return map[string]any{
		"sharepath":   dtree.expectedSharePath(),
		"fsid":        dtree.FakeFsID,
		"description": "Created from Kubernetes CSI",
		"vStoreID":    dtree.FakeVStoreID,
		"DTREEID":     dtree.FakeDTreeID,
	}
}

func (dtree *oceanstorDTree) expectedSharePath() string {
	return fmt.Sprintf("/%s/%s", dtree.BackendParentName, dtree.ExpectedDTreeName)
}

func (dtree *oceanstorDTree) expectedCreateDTreeParams(t *testing.T) map[string]any {
	params := map[string]any{
		"unixPermissions": dtree.Permission,
		"NAME":            dtree.ExpectedDTreeName,
		"PARENTNAME":      dtree.BackendParentName,
		"PARENTTYPE":      client.ParentTypeFS,
		"securityStyle":   client.SecurityStyleUnix,
	}
	if dtree.AdvancedOptions != "" {
		advancedOptions := make(map[string]any)
		err := json.Unmarshal([]byte(dtree.AdvancedOptions), &advancedOptions)
		require.NoError(t, err)
		params = pkgUtils.CombineMap(params, advancedOptions)
	}
	return params
}
