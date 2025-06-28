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
	"math"
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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/types"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
	testUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/test/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestCreateVolume_FusionStorageNas_FullFeaturesSuccess(t *testing.T) {
	// arrange
	data := fakeFusionNasDataWithFullFeaturesSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().ApplyMethodReturn(
		app.GetGlobalConfig().K8sUtils, "GetVolumeConfiguration",
		map[string]string{}, nil,
	)
	defer p.Reset()
	cli.EXPECT().GetAccountIdByName(ctx, data.AccountName).Return(data.FakeAccountID, nil)
	cli.EXPECT().GetPoolByName(ctx, data.PoolName).Return(map[string]any{"poolId": data.FakePoolID}, nil)
	firstGetFs := cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedFsName).Return(nil, nil)
	cli.EXPECT().GetFileSystemByName(ctx, data.ExpectedFsName).
		Return(map[string]any{"id": data.FakeFsID, "running_status": float64(0)}, nil).After(firstGetFs)
	cli.EXPECT().CreateFileSystem(ctx, gomock.Cond(data.expectedCreateFsParamsCond())).
		Return(map[string]any{"id": data.FakeFsID}, nil)
	cli.EXPECT().GetQuotaByFileSystemById(ctx, data.FakeFsIDString).Return(nil, nil)
	cli.EXPECT().CreateQuota(ctx, data.expectedCreateQuotaParams()).Return(nil)
	cli.EXPECT().GetQoSPolicyIdByFsName(ctx, data.ExpectedFsName).Return(types.NoQoSPolicyId, nil)
	cli.EXPECT().CreateConvergedQoS(ctx, data.expectedCreateQosRequest()).Return(data.FakeQoSID, nil)
	cli.EXPECT().AssociateConvergedQoSWithVolume(ctx, data.expectedAssociateQoSRequest()).Return(nil)
	cli.EXPECT().GetNfsShareByPath(ctx, data.ExpectedSharePath, data.FakeAccountID).Return(nil, nil)
	cli.EXPECT().CreateNfsShare(ctx, data.expectedCreateNfsShareParams()).
		Return(map[string]any{"id": data.FakeShareID}, nil)
	cli.EXPECT().AllowNfsShareAccess(ctx, data.expectedAllowNfsShareRequest()).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_FusionStorageNas_ManageVolumeSuccess(t *testing.T) {
	// arrange
	data := fakeFusionNasDataWithManageVolumeSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().ApplyMethodReturn(
		app.GetGlobalConfig().K8sUtils,
		"GetVolumeConfiguration", data.manageVolumeParams(), nil,
	)
	defer p.Reset()
	cli.EXPECT().GetQuotaByFileSystemName(ctx, data.FsName).
		Return(&client.QueryQuotaResponse{
			SpaceHardQuota: math.MaxUint64,
			SpaceSoftQuota: uint64(data.Capacity),
			SpaceUnitType:  0,
		}, nil)
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestCreateVolume_FusionStorageNas_ManageVolumeFailedWithEmptyQuota(t *testing.T) {
	// arrange
	data := fakeFusionNasDataWithManageVolumeSuccess()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	p := gomonkey.NewPatches().ApplyMethodReturn(
		app.GetGlobalConfig().K8sUtils,
		"GetVolumeConfiguration", data.manageVolumeParams(), nil,
	)
	defer p.Reset()
	cli.EXPECT().GetQuotaByFileSystemName(ctx, data.FsName).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.CreateVolume(ctx, data.request())

	// assert
	require.ErrorContains(t, err, "the quota of filesystem "+data.FsName+" does not exist")
}

func fakeFusionNasDataWithManageVolumeSuccess() *fusionStorageNas {
	fsName := "pvc-test-name"
	return &fusionStorageNas{
		FsName:     fsName,
		Capacity:   1024 * 1024 * 1024,
		AllocType:  "thin",
		VolumeType: "fs",
		AuthClient: "*",

		BackendName: "test-fusion-nas-name",
		PoolName:    "test-pool-name",
		Protocol:    "nfs",
		AccountName: "system",

		ExpectedFsName: fsName,
	}
}

func fakeFusionNasDataWithFullFeaturesSuccess() *fusionStorageNas {
	fsName := "pvc-test-name"
	return &fusionStorageNas{
		FsName:                fsName,
		Capacity:              1024 * 1024 * 1024,
		AllocType:             "thin",
		VolumeType:            "fs",
		AuthClient:            "*",
		Permission:            "777",
		ShowSnapshotDir:       "invisible",
		DisableVerifyCapacity: "false",
		AllSquash:             "all_squash",
		RootSquash:            "root_squash",
		QoS:                   `{"maxMBPS": 999, "maxIOPS": 999}`,
		StorageQuota:          `{"spaceQuota": "softQuota", "gracePeriod": 100}`,

		BackendName: "test-fusion-nas-name",
		PoolName:    "test-pool-name",
		Protocol:    "nfs",
		AccountName: "system",

		ExpectedFsName:          utils.GetFileSystemName(fsName),
		ExpectedSharePath:       utils.GetFSSharePath(fsName),
		ExpectedShowSnapshotDir: false,
		ExpectedAllSquashParam:  0,
		ExpectedRootSquashParam: 0,
		ExpectedQuotaKey:        "space_soft_quota",
		ExpectedGracePeriodKey:  "soft_grace_time",
		ExpectedGraceTime:       100,
		ExpectedQoSMaxMbps:      999,
		ExpectedQoSMaxIops:      999,

		FakeAccountID:  "fake-account-id",
		FakePoolID:     10,
		FakeFsID:       11,
		FakeFsIDString: "11",
		FakeQoSID:      2,
		FakeShareID:    "fake-share-id",
	}
}

type fusionStorageNas struct {
	FsName   string
	Capacity int64

	AllocType             string `sc:"allocType"`
	VolumeType            string `sc:"volumeType"`
	AuthClient            string `sc:"authClient"`
	Permission            string `sc:"fsPermission"`
	DisableVerifyCapacity string `sc:"disableVerifyCapacity"`
	ShowSnapshotDir       string `sc:"snapshotDirectoryVisibility"`
	AllSquash             string `sc:"allSquash"`
	RootSquash            string `sc:"rootSquash"`
	QoS                   string `sc:"qos"`
	StorageQuota          string `sc:"storageQuota"`

	BackendName string
	PoolName    string
	Protocol    string
	AccountName string

	ExpectedFsName          string
	ExpectedSharePath       string
	ExpectedShowSnapshotDir bool
	ExpectedAllSquashParam  int
	ExpectedRootSquashParam int
	ExpectedQuotaKey        string
	ExpectedGracePeriodKey  string
	ExpectedGraceTime       int
	ExpectedQoSMaxMbps      int
	ExpectedQoSMaxIops      int

	FakeAccountID  string
	FakePoolID     float64
	FakeFsID       float64
	FakeFsIDString string
	FakeQoSID      int
	FakeShareID    string
}

func (data *fusionStorageNas) backend(cli client.IRestClient) model.Backend {
	p := &plugin.FusionStorageNasPlugin{}
	p.SetCli(cli)
	return model.Backend{
		Name:        data.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.FusionNas,
		Available:   true,
		Plugin:      p,
		Parameters:  map[string]any{"protocol": data.Protocol},
		AccountName: data.AccountName,
		Pools: []*model.StoragePool{
			{
				Name:    data.PoolName,
				Storage: constants.FusionNas,
				Parent:  data.BackendName,
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

func (data *fusionStorageNas) request() *csi.CreateVolumeRequest {
	accessMode, _ := accessmodes.ToCSIAccessMode([]v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, false)

	req := &csi.CreateVolumeRequest{
		Name:       data.FsName,
		Parameters: testUtils.StructToStringMap(data, "sc"),
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

func (data *fusionStorageNas) response() *csi.CreateVolumeResponse {
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: data.Capacity,
			VolumeId:      data.BackendName + "." + data.ExpectedFsName,
			VolumeContext: map[string]string{
				"backend":                          data.BackendName,
				"name":                             data.ExpectedFsName,
				"fsPermission":                     data.Permission,
				constants.DTreeParentKey:           "",
				constants.DisableVerifyCapacityKey: data.DisableVerifyCapacity,
			},
			ContentSource:      data.request().GetVolumeContentSource(),
			AccessibleTopology: make([]*csi.Topology, 0),
		},
	}
}

func (data *fusionStorageNas) manageVolumeParams() map[string]string {
	return map[string]string{
		"fake.driver.name/manageVolumeName":  data.FsName,
		"fake.driver.name/manageBackendName": data.BackendName,
	}
}

func (data *fusionStorageNas) expectedCreateFsParamsCond() func(x map[string]any) bool {
	params := data.expectedCreateFsParams()
	return func(got map[string]any) bool {
		for key, val := range params {
			if got[key] != val {
				return false
			}
		}
		return true
	}
}

func (data *fusionStorageNas) expectedCreateFsParams() map[string]any {
	return map[string]any{
		"name":          data.ExpectedFsName,
		"poolId":        int64(data.FakePoolID),
		"accountid":     data.FakeAccountID,
		"fspermission":  data.Permission,
		"isshowsnapdir": data.ExpectedShowSnapshotDir,
	}
}

func (data *fusionStorageNas) expectedCreateQuotaParams() map[string]any {
	res := map[string]any{
		"parent_id":              data.FakeFsIDString,
		"parent_type":            "40",
		"quota_type":             "1",
		"snap_space_switch":      0,
		"space_unit_type":        1,
		"directory_quota_target": 1,
		data.ExpectedQuotaKey:    data.Capacity / constants.FusionFileCapacityUnit,
	}

	if data.ExpectedGracePeriodKey != "" {
		res[data.ExpectedGracePeriodKey] = data.ExpectedGraceTime
	}

	return res
}

func (data *fusionStorageNas) expectedCreateQosRequest() *types.CreateConvergedQoSReq {
	return &types.CreateConvergedQoSReq{
		QosScale: types.QosScaleNamespace,
		Name:     smartx.ConstructQosNameByCurrentTime("fs"),
		QosMode:  types.QosModeManual,
		MaxMbps:  data.ExpectedQoSMaxMbps,
		MaxIops:  data.ExpectedQoSMaxIops,
	}
}

func (data *fusionStorageNas) expectedAssociateQoSRequest() *types.AssociateConvergedQoSWithVolumeReq {
	return &types.AssociateConvergedQoSWithVolumeReq{
		QosScale:    types.QosScaleNamespace,
		ObjectName:  data.ExpectedFsName,
		QoSPolicyID: data.FakeQoSID,
	}
}

func (data *fusionStorageNas) expectedCreateNfsShareParams() map[string]any {
	return map[string]any{
		"sharepath":   data.ExpectedSharePath,
		"fsid":        data.FakeFsIDString,
		"description": "Created from Kubernetes Provisioner",
		"accountid":   data.FakeAccountID,
	}
}

func (data *fusionStorageNas) expectedAllowNfsShareRequest() *client.AllowNfsShareAccessRequest {
	return &client.AllowNfsShareAccessRequest{
		AccessName:  data.AuthClient,
		ShareId:     data.FakeShareID,
		AccessValue: 1,
		AllSquash:   data.ExpectedAllSquashParam,
		RootSquash:  data.ExpectedRootSquashParam,
		AccountId:   data.FakeAccountID,
	}
}
