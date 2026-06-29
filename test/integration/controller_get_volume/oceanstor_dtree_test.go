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
	"errors"
	"fmt"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestControllerGetVolume_OceanstorDTree_Success(t *testing.T) {
	// Arrange
	data := fakeOceanstorDTreeData()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
		"GetDTreeParentNameByVolumeId", data.ParentName, nil)
	defer p.Reset()

	cli.EXPECT().GetDTreeByName(gomock.Any(), gomock.Any(), data.ParentName, "", data.DTreeName).Return(nil, nil)
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
}

func TestControllerGetVolume_OceanstorDTree_ParentQueryError(t *testing.T) {
	// Arrange
	data := fakeOceanstorDTreeData()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
		"GetDTreeParentNameByVolumeId", "", errors.New("parent query failed"))
	defer p.Reset()

	cli.EXPECT().Logout(ctx)

	// Act
	_, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.Error(t, err)
	require.Contains(t, err.Error(), "parent query failed")
}

func TestControllerGetVolume_OceanstorDTree_VolumeNotFound(t *testing.T) {
	// Arrange
	data := fakeOceanstorDTreeData()
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	p := gomonkey.ApplyMethodReturn(app.GetGlobalConfig().K8sUtils,
		"GetDTreeParentNameByVolumeId", data.ParentName, nil)
	defer p.Reset()

	cli.EXPECT().GetDTreeByName(gomock.Any(), gomock.Any(), data.ParentName, "", data.DTreeName).Return(nil,
		fmt.Errorf("volume not exist"))
	cli.EXPECT().Logout(ctx)

	// Act
	resp, err := csiServer.ControllerGetVolume(ctx, data.request())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.True(t, resp.Status.VolumeCondition.Abnormal)
}

func fakeOceanstorDTreeData() *oceanstorDTreeData {
	return &oceanstorDTreeData{
		BackendName: "dtree-backend",
		DTreeName:   "test-dtree",
		ParentName:  "test-parent",
		VolumeId:    "dtree-backend.test-dtree",
	}
}

type oceanstorDTreeData struct {
	BackendName string
	DTreeName   string
	ParentName  string
	VolumeId    string
}

func (d *oceanstorDTreeData) request() *csi.ControllerGetVolumeRequest {
	return &csi.ControllerGetVolumeRequest{VolumeId: d.VolumeId}
}

func (d *oceanstorDTreeData) backend(cli client.OceanstorClientInterface) model.Backend {
	p := &plugin.OceanstorDTreePlugin{}
	p.SetCli(cli)
	return model.Backend{
		Name:       d.BackendName,
		Storage:    constants.OceanStorDtree,
		Available:  true,
		Plugin:     p,
		Parameters: map[string]any{},
	}
}
