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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/utils"
)

func TestCreateVolume_OceanstorNas_FullFeaturesSuccess(t *testing.T) {
	// arrange
	data := fakeOceanstorNasDataWithSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetvStoreID().Return(data.FakeVStoreID).AnyTimes()
	cli.EXPECT().GetCurrentLifWwn().Return(data.ExpectedCurrentLifWwn).AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return(data.ExpectedCurrentSiteWwn).AnyTimes()
	cli.EXPECT().GetPoolByName(ctx, data.ExpectedPoolName).Return(map[string]any{"ID": data.FakePoolID}, nil)
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedFsName).Return(nil, nil)
	cli.EXPECT().CreateFileSystem(ctx, data.expectedCreateFsParams(t)).Return(map[string]any{"ID": data.FakeFsID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath(), data.FakeVStoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, data.expectedCreateNfsShareParams()).Return(map[string]any{"ID": data.FakeShareID},
		nil)
	cli.EXPECT().GetNfsShareAccessCount(ctx, data.FakeShareID, data.FakeVStoreID).Return(int64(0), nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, data.expectedAllowNfsShareRequest()).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_OceanstorNas_SuccessWithScVolumeName(t *testing.T) {
	// arrange
	data := fakeOceanstorNasDataWithSuccess()
	data.ScVolumeName = "prefix-{{ .PVCNamespace }}-{{ .PVCName }}"
	data.ExpectedFsName = "prefix-pvcTestNamespace-pvcTestName-pvTestName"
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
	cli.EXPECT().GetvStoreID().Return(data.FakeVStoreID).AnyTimes()
	cli.EXPECT().GetCurrentLifWwn().Return(data.ExpectedCurrentLifWwn).AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return(data.ExpectedCurrentSiteWwn).AnyTimes()
	cli.EXPECT().GetPoolByName(ctx, data.ExpectedPoolName).Return(map[string]any{"ID": data.FakePoolID}, nil)
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedFsName).Return(nil, nil)
	cli.EXPECT().CreateFileSystem(ctx, data.expectedCreateFsParams(t)).Return(map[string]any{"ID": data.FakeFsID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath(), data.FakeVStoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, data.expectedCreateNfsShareParams()).Return(map[string]any{"ID": data.FakeShareID},
		nil)
	cli.EXPECT().GetNfsShareAccessCount(ctx, data.FakeShareID, data.FakeVStoreID).Return(int64(0), nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, data.expectedAllowNfsShareRequest()).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, createVolumeReq)

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_OceanstorNasV5_SuccessWithScVolumeName(t *testing.T) {
	// arrange
	data := fakeOceanstorNasDataWithSuccess()
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
	cli.EXPECT().GetvStoreID().Return(data.FakeVStoreID).AnyTimes()
	cli.EXPECT().GetCurrentLifWwn().Return(data.ExpectedCurrentLifWwn).AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return(data.ExpectedCurrentSiteWwn).AnyTimes()
	cli.EXPECT().GetPoolByName(ctx, data.ExpectedPoolName).Return(map[string]any{"ID": data.FakePoolID}, nil)
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedFsName).Return(nil, nil)
	cli.EXPECT().CreateFileSystem(ctx, data.expectedCreateFsParams(t)).Return(map[string]any{"ID": data.FakeFsID}, nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.expectedSharePath(), data.FakeVStoreID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, data.expectedCreateNfsShareParams()).Return(map[string]any{"ID": data.FakeShareID},
		nil)
	cli.EXPECT().GetNfsShareAccessCount(ctx, data.FakeShareID, data.FakeVStoreID).Return(int64(0), nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, data.expectedAllowNfsShareRequest()).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, createVolumeReq)

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_OceanstorNas_FailedWithInvalidScVolumeName(t *testing.T) {
	// arrange
	data := fakeOceanstorNasDataWithSuccess()
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
	cli.EXPECT().GetvStoreID().Return(data.FakeVStoreID).AnyTimes()
	cli.EXPECT().GetCurrentLifWwn().Return(data.ExpectedCurrentLifWwn).AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return(data.ExpectedCurrentSiteWwn).AnyTimes()
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.CreateVolume(ctx, createVolumeReq)

	// assert
	require.ErrorContains(t, err,
		"{{.PVCNamespace}} or {{.PVCName}} must be configured in the volumeName parameter at the same time")
}

func TestCreateVolume_OceanstorNas_UnmarshalAdvancedOptionsFailed(t *testing.T) {
	// arrange
	data := fakeOceanstorNasDataWithSuccess()
	data.AdvancedOptions = `{"CAPACITYTHRESHOLD": 90`
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetvStoreID().Return(data.FakeVStoreID).AnyTimes()
	cli.EXPECT().GetCurrentLifWwn().Return(data.ExpectedCurrentLifWwn).AnyTimes()
	cli.EXPECT().GetCurrentSiteWwn().Return(data.ExpectedCurrentSiteWwn).AnyTimes()
	cli.EXPECT().GetPoolByName(ctx, data.ExpectedPoolName).Return(map[string]any{"ID": data.FakePoolID}, nil)
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedFsName).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.ErrorContains(t, err, "failed to unmarshal advancedOptions")
}

func fakeOceanstorNasDataWithSuccess() *oceanstorNas {
	sameWwn := "test-wwn"
	return &oceanstorNas{
		Name:                  "pvc-test-nas",
		Capacity:              1024 * 1024 * 1024,
		AllocType:             "thin",
		VolumeType:            "fs",
		AuthClient:            "*",
		Permission:            "777",
		DisableVerifyCapacity: "false",
		AllSquash:             "all_squash",
		RootSquash:            "root_squash",
		AccessKrb5:            "read_write",
		AccessKrb5i:           "read_write",
		AccessKrb5p:           "read_write",
		AdvancedOptions:       `{"CAPACITYTHRESHOLD": 90}`,

		BackendName: "test-nfs-backend",
		PoolName:    "StoragePool001",

		ExpectedCurrentLifWwn:    sameWwn,
		ExpectedCurrentSiteWwn:   sameWwn,
		ExpectedPoolName:         "StoragePool001",
		ExpectedFsName:           "pvc_test_nas",
		ExpectedCapacity:         1024 * 1024 * 1024 / constants.AllocationUnitBytes,
		ExpectedAllocType:        1,
		ExpectedAllSquashParam:   0,
		ExpectedRootSquashParam:  0,
		ExpectedAccessKrb5Param:  1,
		ExpectedAccessKrb5iParam: 1,
		ExpectedAccessKrb5pParam: 1,

		FakePoolID:   "fake-pool-id",
		FakeVStoreID: "",
		FakeFsID:     "fake-fs-id",
		FakeShareID:  "fake-share-id",
	}
}

type oceanstorNas struct {
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
	AdvancedOptions       string `sc:"advancedOptions"`
	ScVolumeName          string `sc:"volumeName"`

	BackendName string `sc:"backend"`
	PoolName    string
	Protocol    string

	ExpectedCurrentLifWwn    string
	ExpectedCurrentSiteWwn   string
	ExpectedPoolName         string
	ExpectedFsName           string
	ExpectedCapacity         int64
	ExpectedAllocType        int
	ExpectedAllSquashParam   int
	ExpectedRootSquashParam  int
	ExpectedAccessKrb5Param  int
	ExpectedAccessKrb5pParam int
	ExpectedAccessKrb5iParam int

	FakePoolID   string
	FakeVStoreID string
	FakeFsID     string
	FakeShareID  string
}

func (data *oceanstorNas) backend(cli client.OceanstorClientInterface,
	product constants.OceanstorVersion) model.Backend {
	p := &plugin.OceanstorNasPlugin{}
	p.SetCli(cli)
	p.SetProduct(product)
	return model.Backend{
		Name:        data.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorNas,
		Available:   true,
		Plugin:      p,
		Parameters:  map[string]any{"protocol": data.Protocol},
		Pools: []*model.StoragePool{
			{
				Name:    data.PoolName,
				Storage: constants.OceanStorNas,
				Parent:  data.BackendName,
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

func (data *oceanstorNas) request() *csi.CreateVolumeRequest {
	accessMode, _ := accessmodes.ToCSIAccessMode([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, false)

	req := &csi.CreateVolumeRequest{
		Name:       data.Name,
		Parameters: utils.StructToStringMap(data, "sc"),
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
		CapacityRange: &csi.CapacityRange{RequiredBytes: data.Capacity},
	}

	return req
}

func (data *oceanstorNas) response() *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: data.Capacity,
			VolumeId:      data.BackendName + "." + data.ExpectedFsName,
			VolumeContext: map[string]string{
				"backend":                          data.BackendName,
				"name":                             data.ExpectedFsName,
				"fsPermission":                     data.Permission,
				constants.DisableVerifyCapacityKey: data.DisableVerifyCapacity,
				constants.DTreeParentKey:           "",
			},
			ContentSource:      data.request().GetVolumeContentSource(),
			AccessibleTopology: make([]*csi.Topology, 0),
		},
	}
}

func (data *oceanstorNas) expectedCreateQuotaParam() map[string]any {
	res := map[string]any{
		"PARENTTYPE":     client.ParentTypeDTree,
		"QUOTATYPE":      client.QuotaTypeDir,
		"SPACEHARDQUOTA": data.Capacity,
		"vstoreId":       nil,
	}
	if data.FakeVStoreID != "" {
		res["vstoreId"] = data.FakeVStoreID
	}

	return res
}

func (data *oceanstorNas) expectedAllowNfsShareRequest() *base.AllowNfsShareAccessRequest {
	return &base.AllowNfsShareAccessRequest{
		Name:        data.AuthClient,
		ParentID:    data.FakeShareID,
		AccessVal:   1,
		Sync:        0,
		AllSquash:   data.ExpectedAllSquashParam,
		RootSquash:  data.ExpectedRootSquashParam,
		VStoreID:    data.FakeVStoreID,
		AccessKrb5:  data.ExpectedAccessKrb5Param,
		AccessKrb5i: data.ExpectedAccessKrb5iParam,
		AccessKrb5p: data.ExpectedAccessKrb5pParam,
	}
}

func (data *oceanstorNas) expectedCreateNfsShareParams() map[string]any {
	return map[string]any{
		"sharepath":   data.expectedSharePath(),
		"fsid":        data.FakeFsID,
		"description": "Created from Kubernetes CSI",
		"vStoreID":    data.FakeVStoreID,
	}
}

func (data *oceanstorNas) expectedSharePath() string {
	return fmt.Sprintf("/%s/", data.ExpectedFsName)
}

func (data *oceanstorNas) expectedCreateFsParams(t *testing.T) map[string]any {
	params := map[string]any{
		"ALLOCTYPE":       data.ExpectedAllocType,
		"CAPACITY":        data.ExpectedCapacity,
		"DESCRIPTION":     "Created from Kubernetes CSI",
		"NAME":            data.ExpectedFsName,
		"PARENTID":        "fake-pool-id",
		"fileSystemMode":  "0",
		"unixPermissions": "777",
	}
	if data.AdvancedOptions != "" {
		advancedOptions := make(map[string]any)
		err := json.Unmarshal([]byte(data.AdvancedOptions), &advancedOptions)
		require.NoError(t, err)
		params = pkgUtils.CombineMap(params, advancedOptions)
	}
	return params
}
