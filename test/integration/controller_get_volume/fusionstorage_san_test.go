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

// Package controller_publish_volume includes the integration tests of ControllerGetVolume interface
package controller_get_volume

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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestControllerGetVolume_FusionStorageSan_Success_NormalStatus(t *testing.T) {
	// Arrange
	data := fakeFusionStorageSanDataWithNormalStatus()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	cli.EXPECT().QueryVolume(gomock.Any(), data.VolumeName).Return(
		map[string]any{"health_status": float64(1)}, nil)
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.False(t, resp.Status.VolumeCondition.Abnormal)
}

func TestControllerGetVolume_FusionStorageSan_FaultStatus(t *testing.T) {
	// Arrange
	data := fakeFusionStorageSanDataWithNormalStatus()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	cli.EXPECT().QueryVolume(gomock.Any(), data.VolumeName).Return(
		map[string]any{"health_status": float64(2)}, nil)
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.True(t, resp.Status.VolumeCondition.Abnormal)
	require.Contains(t, resp.Status.VolumeCondition.Message, "fault")
}

func TestControllerGetVolume_FusionStorageSan_VolumeNotFound(t *testing.T) {
	// Arrange
	data := fakeFusionStorageSanDataWithNormalStatus()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	cli.EXPECT().QueryVolume(gomock.Any(), data.VolumeName).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.True(t, resp.Status.VolumeCondition.Abnormal)
}

func TestControllerGetVolume_FusionStorageSan_APIError(t *testing.T) {
	// Arrange
	data := fakeFusionStorageSanDataWithNormalStatus()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	cli.EXPECT().QueryVolume(gomock.Any(), data.VolumeName).Return(
		nil, fmt.Errorf("backend down"))
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.True(t, resp.Status.VolumeCondition.Abnormal)
}

func fakeFusionStorageSanDataWithNormalStatus() *fusionstorageSanData {
	return &fusionstorageSanData{
		BackendName:  "fusionstorage-san-backend",
		VolumeName:   "test-volume",
		VolumeId:     "fusionstorage-san-backend.test-volume",
		HealthStatus: 1,
	}
}

type fusionstorageSanData struct {
	BackendName  string
	VolumeName   string
	VolumeId     string
	HealthStatus int
}

func (d *fusionstorageSanData) request() *csi.ControllerGetVolumeRequest {
	return &csi.ControllerGetVolumeRequest{VolumeId: d.VolumeId}
}

func (d *fusionstorageSanData) backend(cli client.IRestClient) model.Backend {
	p := &plugin.FusionStorageSanPlugin{}
	p.SetCli(cli)
	return model.Backend{
		Name:       d.BackendName,
		Storage:    "fusionstorage-san",
		Available:  true,
		Plugin:     p,
		Parameters: map[string]any{"protocol": "iscsi"},
	}
}
