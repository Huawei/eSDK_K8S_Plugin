/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

// Package driver provides csi driver with controller, node, identity services
package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/handler"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
)

func TestCsiDriver_DeleteVolume_KVCacheSuccess(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeDmeKVCache()

	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
		"GetKvCacheStoreIdByVolumeId", "fake-kvcacheStoreId", nil).
		ApplyMethodReturn(&plugin.DMEASeriesPlugin{}, "DeleteVolume", nil).
		ApplyMethodReturn(&handler.BackendSelector{}, "SelectBackend", data.backend(), nil)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

type dmeKVCache struct {
	volName     string
	backendName string
}

func fakeDmeKVCache() *dmeKVCache {
	return &dmeKVCache{
		volName:     "test-vol-name",
		backendName: "test-dme-backend",
	}
}

func (f *dmeKVCache) request() *csi.DeleteVolumeRequest {
	return &csi.DeleteVolumeRequest{
		VolumeId: f.backendName + "." + f.volName,
	}
}

func (f *dmeKVCache) response() *csi.DeleteVolumeResponse {
	return &csi.DeleteVolumeResponse{}
}

func (f *dmeKVCache) backend() *model.Backend {
	return &model.Backend{
		Name:        f.backendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorASeriesNasDme,
		Plugin:      &plugin.DMEASeriesPlugin{},
	}
}

func TestCsiDriver_ControllerGetVolume_Normal(t *testing.T) {
	ctx := context.Background()
	data := fakeControllerGetVolumeData()
	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&handler.BackendSelector{}, "SelectBackend", data.backend(), nil).
		ApplyMethodReturn(&plugin.FusionStorageSanPlugin{}, "GetVolumeStatus", utils.VolumeStatus{Abnormal: false})

	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Status.VolumeCondition.Abnormal)
}

type controllerGetVolumeData struct {
	volName     string
	backendName string
}

func fakeControllerGetVolumeData() *controllerGetVolumeData {
	return &controllerGetVolumeData{
		volName:     "test-vol",
		backendName: "test-backend",
	}
}

func TestCsiDriver_DeleteVolume_DmeNasSuccess(t *testing.T) {
	// Arrange
	ctx := context.Background()
	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")

	backend := &model.Backend{
		Name:        "test-dme-backend",
		ContentName: "test-content",
		Storage:     constants.OceanStorASeriesNasDme,
		Plugin:      &plugin.DMEASeriesPlugin{},
	}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&handler.BackendSelector{}, "SelectBackend", backend, nil).
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
			"GetKvCacheStoreIdByVolumeId", "kv-id-1", nil).
		ApplyMethodReturn(&plugin.DMEASeriesPlugin{}, "DeleteVolume", nil)

	// Action
	resp, err := csiServer.DeleteVolume(ctx, &csi.DeleteVolumeRequest{
		VolumeId: "test-dme-backend.test-vol",
	})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestCsiDriver_DeleteVolume_DmeNasGetParamsError(t *testing.T) {
	// Arrange
	ctx := context.Background()
	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")

	backend := &model.Backend{
		Name:        "test-dme-backend",
		ContentName: "test-content",
		Storage:     constants.OceanStorASeriesNasDme,
		Plugin:      &plugin.DMEASeriesPlugin{},
	}

	getParamsErr := errors.New("get kvcache store id failed")
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&handler.BackendSelector{}, "SelectBackend", backend, nil).
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
			"GetKvCacheStoreIdByVolumeId", "", getParamsErr)

	// Action
	resp, err := csiServer.DeleteVolume(ctx, &csi.DeleteVolumeRequest{
		VolumeId: "test-dme-backend.test-vol",
	})

	// Assert
	require.Error(t, err)
	require.NotNil(t, resp)
}

func TestCsiDriver_buildDeleteVolumeParams_NasWithKvCacheStoreId(t *testing.T) {
	// Arrange
	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")
	bk := &model.Backend{Storage: constants.OceanStorASeriesNas}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
		"GetKvCacheStoreIdByVolumeId", "kv-id-1", nil)

	// Action
	params, err := csiServer.buildDeleteVolumeParams(bk, "vol-id-1")

	// Assert
	require.NoError(t, err)
	require.Equal(t, "kv-id-1", params[constants.KvCacheStoreId])
}

func TestCsiDriver_buildDeleteVolumeParams_DmeWithoutKvCacheStoreId(t *testing.T) {
	// Arrange
	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")
	bk := &model.Backend{Storage: constants.OceanStorASeriesNasDme}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
		"GetKvCacheStoreIdByVolumeId", "", nil)

	// Action
	params, err := csiServer.buildDeleteVolumeParams(bk, "vol-id-1")

	// Assert
	require.NoError(t, err)
	_, exists := params[constants.KvCacheStoreId]
	require.False(t, exists)
}

func TestCsiDriver_buildDeleteVolumeParams_Error(t *testing.T) {
	// Arrange
	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")
	bk := &model.Backend{Storage: constants.OceanStorASeriesNasDme}

	wantErr := errors.New("k8s api failed")
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
		"GetKvCacheStoreIdByVolumeId", "", wantErr)

	// Action
	params, err := csiServer.buildDeleteVolumeParams(bk, "vol-id-1")

	// Assert
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, params)
}

func TestCsiDriver_buildDeleteVolumeParams_OtherStorage(t *testing.T) {
	// Arrange
	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")
	bk := &model.Backend{Storage: constants.OceanStorSan}

	// Action
	params, err := csiServer.buildDeleteVolumeParams(bk, "vol-id-1")

	// Assert
	require.NoError(t, err)
	require.Nil(t, params)
}

func (d *controllerGetVolumeData) request() *csi.ControllerGetVolumeRequest {
	return &csi.ControllerGetVolumeRequest{
		VolumeId: d.backendName + "." + d.volName,
	}
}

func (d *controllerGetVolumeData) backend() *model.Backend {
	return &model.Backend{
		Name:        d.backendName,
		ContentName: "test-content",
		Storage:     "fusionstorage-san",
		Plugin:      &plugin.FusionStorageSanPlugin{},
	}
}
