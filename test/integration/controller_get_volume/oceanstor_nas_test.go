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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestControllerGetVolume_OceanstorNas_Success_NormalStatus(t *testing.T) {
	// Arrange
	data := fakeOceanstorNasData()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	cli.EXPECT().GetFileSystemByName(gomock.Any(), data.VolumeName).Return(
		map[string]any{"HEALTHSTATUS": "1"}, nil)
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.False(t, resp.Status.VolumeCondition.Abnormal)
}

func TestControllerGetVolume_OceanstorNas_FaultStatus(t *testing.T) {
	// Arrange
	data := fakeOceanstorNasData()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	cli.EXPECT().GetFileSystemByName(gomock.Any(), data.VolumeName).Return(
		map[string]any{"HEALTHSTATUS": "2"}, nil)
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.True(t, resp.Status.VolumeCondition.Abnormal)
}

func TestControllerGetVolume_OceanstorNas_NotFound(t *testing.T) {
	// Arrange
	data := fakeOceanstorNasData()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	cli.EXPECT().GetFileSystemByName(gomock.Any(), data.VolumeName).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.True(t, resp.Status.VolumeCondition.Abnormal)
}

func TestControllerGetVolume_OceanstorNas_APIError(t *testing.T) {
	// Arrange
	data := fakeOceanstorNasData()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	cli.EXPECT().GetFileSystemByName(gomock.Any(), data.VolumeName).Return(
		nil, fmt.Errorf("backend down"))
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.True(t, resp.Status.VolumeCondition.Abnormal)
}

func fakeOceanstorNasData() *oceanstorNasData {
	return &oceanstorNasData{
		BackendName:  "nas-backend",
		VolumeName:   "test-vol",
		VolumeId:     "nas-backend.test-vol",
		HealthStatus: "1",
	}
}

type oceanstorNasData struct {
	BackendName  string
	VolumeName   string
	VolumeId     string
	HealthStatus string
}

func (d *oceanstorNasData) request() *csi.ControllerGetVolumeRequest {
	return &csi.ControllerGetVolumeRequest{VolumeId: d.VolumeId}
}

func (d *oceanstorNasData) backend(cli client.OceanstorClientInterface) model.Backend {
	p := &plugin.OceanstorNasPlugin{}
	p.SetCli(cli)
	return model.Backend{
		Name:       d.BackendName,
		Storage:    constants.OceanStorNas,
		Available:  true,
		Plugin:     p,
		Parameters: map[string]any{},
	}
}
