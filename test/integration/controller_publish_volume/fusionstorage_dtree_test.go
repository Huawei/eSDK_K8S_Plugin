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

// Package controller_publish_volume includes the integration tests of ControllerPublishVolume interface
package controller_publish_volume

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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestControllerPublishVolume_FusionstorageDTree_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeControllerPublishVolumeSuccessData()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.ControllerPublishVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func TestControllerPublishVolume_FusionstorageDTree_EmptyDTreeParentNameSuccess(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeControllerPublishVolumeSuccessData()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().Logout(ctx)

	// action
	resp, err := csiServer.ControllerPublishVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
	require.Equal(t, data.response(), resp)
}

func fakeControllerPublishVolumeSuccessData() *fusionDTree {
	return &fusionDTree{
		VolumeName:            "test-dtree-volume",
		NodeId:                "{\"HostName\":\"test-node\"}",
		DTreeParentName:       "",
		DisableVerifyCapacity: "",
		Permission:            "755",
		BackendName:           "test-fusionstorage-dtree",
		Protocol:              constants.ProtocolNfs,

		ExpectedDTreeParentname: "",
	}
}

type fusionDTree struct {
	VolumeName            string
	NodeId                string
	DTreeParentName       string
	DisableVerifyCapacity string
	Permission            string

	BackendName       string
	BackendParentName string
	Protocol          string

	ExpectedDTreeParentname string
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

func (dtree *fusionDTree) request() *csi.ControllerPublishVolumeRequest {
	req := &csi.ControllerPublishVolumeRequest{
		VolumeId: dtree.BackendName + "." + dtree.VolumeName,
		NodeId:   dtree.NodeId,
		VolumeContext: map[string]string{
			"backend":                          dtree.BackendName,
			"name":                             dtree.VolumeName,
			"fsPermission":                     dtree.Permission,
			constants.DTreeParentKey:           dtree.DTreeParentName,
			constants.DisableVerifyCapacityKey: dtree.DisableVerifyCapacity,
		},
	}
	return req
}

func (dtree *fusionDTree) response() *csi.ControllerPublishVolumeResponse {
	publishInfo := fmt.Sprintf("{\"dTreeParentName\":\"%s\"}", dtree.ExpectedDTreeParentname)
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"publishInfo":    publishInfo,
			"filesystemMode": "",
		},
	}
}
