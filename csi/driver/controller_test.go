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
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/handler"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
)

func TestCsiDriver_DeleteVolume_KVCacheSuccess(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorSuccessKVCache()

	kubeClient := &k8sutils.KubeClient{}
	csiServer := NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
		"GetKvCacheStoreIdByVolumeId", "fake-kvcacheStoreId", nil).
		ApplyMethodReturn(&plugin.OceanstorASeriesPlugin{}, "DeleteVolume", nil).
		ApplyMethodReturn(&handler.BackendSelector{}, "SelectBackend", data.backendKVCache(), nil)

	// action
	resp, err := csiServer.DeleteVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func (f *oceanstorKVCache) request() *csi.DeleteVolumeRequest {
	return &csi.DeleteVolumeRequest{
		VolumeId: f.BackendName + "." + f.volName,
	}
}

func (f *oceanstorKVCache) response() *csi.DeleteVolumeResponse {
	return &csi.DeleteVolumeResponse{}
}

func fakeOceanstorSuccessKVCache() *oceanstorKVCache {
	return &oceanstorKVCache{
		volName:     "test-vol-name",
		BackendName: "test-oceanstor-dtree-backend",
	}
}

func (f *oceanstorKVCache) fakeKVCacheVolumeAttributes() map[string]string {
	return map[string]string{
		"kvcacheStoreId": "kvcacheStoreId",
	}
}

func (f *oceanstorKVCache) backendKVCache() *model.Backend {
	return &model.Backend{
		Name:        f.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorASeriesNas,
		Plugin:      &plugin.OceanstorDTreePlugin{},
	}
}

type oceanstorKVCache struct {
	volName     string
	BackendName string
}
