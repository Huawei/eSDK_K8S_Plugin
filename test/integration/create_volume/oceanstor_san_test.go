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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/utils"
)

func TestCreateVolume_OceanstorSan_FullFeaturesSuccess(t *testing.T) {
	// arrange
	data := fakeOceanstorSanDataWithSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli, constants.OceanStorDoradoV6))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration", map[string]string{}, nil)
	defer p.Reset()
	cli.EXPECT().GetPoolByName(ctx, data.ExpectedPoolName).Return(map[string]any{"ID": data.FakePoolID}, nil)
	cli.EXPECT().MakeLunName(data.ExpectedLunName).Return(data.ExpectedLunName)
	cli.EXPECT().GetLunByName(ctx, data.ExpectedLunName).Return(nil, nil)
	cli.EXPECT().CreateLun(ctx, data.expectedCreateLunParams()).Return(map[string]any{"ID": data.FakeLunID,
		"WWN": data.FakeWwn}, nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_OceanstorSan_SuccessWithScVolumeName(t *testing.T) {
	// arrange
	data := fakeOceanstorSanDataWithSuccess()
	data.ScVolumeName = "prefix-{{ .PVCNamespace }}-{{ .PVCName }}"
	data.ExpectedLunName = "prefix-pvcTestNamespace-pvcTestName-pvTestName"
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
	cli.EXPECT().GetPoolByName(ctx, data.ExpectedPoolName).Return(map[string]any{"ID": data.FakePoolID}, nil)
	cli.EXPECT().MakeLunName(data.ExpectedLunName).Return(data.ExpectedLunName)
	cli.EXPECT().GetLunByName(ctx, data.ExpectedLunName).Return(nil, nil)
	cli.EXPECT().CreateLun(ctx, data.expectedCreateLunParams()).Return(map[string]any{"ID": data.FakeLunID,
		"WWN": data.FakeWwn}, nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, createVolumeReq)

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func fakeOceanstorSanDataWithSuccess() *oceanstorSan {
	return &oceanstorSan{
		// pvc parameters
		Name:     "test-pvc-san",
		Capacity: 1024 * 1024 * 1024,

		// sc parameters
		AllocType:             "thin",
		VolumeType:            "lun",
		Permission:            "777",
		DisableVerifyCapacity: "false",
		AdvancedOptions:       `{"CAPACITYTHRESHOLD": 90}`,

		// backend parameters
		BackendName: "san-backend",
		PoolName:    "StoragePool001",
		Protocol:    "iscsi",

		// expected data
		ExpectedPoolName:  "StoragePool001",
		ExpectedLunName:   "test-pvc-san",
		ExpectedCapacity:  1024 * 1024 * 1024 / constants.AllocationUnitBytes,
		ExpectedAllocType: 1,

		// fake data
		FakePoolID: "fake-pool-id",
		FakeLunID:  "fake-lun-id",
		FakeWwn:    "fake-wwn",
	}
}

type oceanstorSan struct {
	Name     string
	Capacity int64

	AllocType             string `sc:"allocType"`
	VolumeType            string `sc:"volumeType"`
	Permission            string `sc:"fsPermission"`
	DisableVerifyCapacity string `sc:"disableVerifyCapacity"`
	AdvancedOptions       string `sc:"advancedOptions"`
	ScVolumeName          string `sc:"volumeName"`

	BackendName string `sc:"backend"`
	PoolName    string
	Protocol    string

	ExpectedPoolName  string
	ExpectedLunName   string
	ExpectedCapacity  int64
	ExpectedAllocType int

	FakePoolID string
	FakeLunID  string
	FakeWwn    string
}

func (data *oceanstorSan) backend(cli client.OceanstorClientInterface,
	product constants.OceanstorVersion) model.Backend {
	p := &plugin.OceanstorSanPlugin{}
	p.SetCli(cli)
	p.SetProduct(product)
	return model.Backend{
		Name:        data.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorSan,
		Available:   true,
		Plugin:      p,
		Parameters:  map[string]any{"protocol": data.Protocol},
		Pools: []*model.StoragePool{
			{
				Name:    data.PoolName,
				Storage: constants.OceanStorSan,
				Parent:  data.BackendName,
				Capabilities: map[string]bool{
					"SupportApplicationType": true,
					"SupportClone":           true,
					"SupportMetro":           false,
					"SupportMetroNAS":        true,
					"SupportQoS":             true,
					"SupportReplication":     false,
					"SupportThick":           false,
					"SupportThin":            true,
				},
				Capacities: map[string]string{},
				Plugin:     p,
			},
		},
	}
}

func (data *oceanstorSan) request() *csi.CreateVolumeRequest {
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

func (data *oceanstorSan) response() *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: data.Capacity,
			VolumeId:      data.BackendName + "." + data.ExpectedLunName,
			VolumeContext: map[string]string{
				"backend":                          data.BackendName,
				"name":                             data.ExpectedLunName,
				"fsPermission":                     data.Permission,
				"lunWWN":                           data.FakeWwn,
				constants.DisableVerifyCapacityKey: data.DisableVerifyCapacity,
				constants.DTreeParentKey:           "",
			},
			ContentSource:      data.request().GetVolumeContentSource(),
			AccessibleTopology: make([]*csi.Topology, 0),
		},
	}
}

func (data *oceanstorSan) expectedCreateLunParams() map[string]any {
	params := map[string]any{
		"alloctype":                  data.ExpectedAllocType,
		"backend":                    data.BackendName,
		"capacity":                   data.ExpectedCapacity,
		"description":                "Created from Kubernetes CSI",
		"fspermission":               data.Permission,
		"name":                       data.ExpectedLunName,
		"parentid":                   data.FakePoolID,
		"poolID":                     data.FakePoolID,
		"storagepool":                data.PoolName,
		"vstoreId":                   "0",
		constants.AdvancedOptionsKey: data.AdvancedOptions,
	}

	return params
}
