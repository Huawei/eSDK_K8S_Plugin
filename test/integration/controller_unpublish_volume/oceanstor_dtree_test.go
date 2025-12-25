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

// Package controller_unpublish_volume includes the integration tests of ControllerUnpublishVolume interface
package controller_unpublish_volume

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/host"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func Test_ControllerUnpublishVolume_OceanstorDTree_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorDtree()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().Logout(ctx)
	patches := gomonkey.ApplyMethodReturn(
		app.GetGlobalConfig().K8sUtils, "GetDTreeParentNameByVolumeId", data.DTreeParentName, nil)
	defer patches.Reset()

	// action
	_, gotErr := csiServer.ControllerUnpublishVolume(ctx, data.request())

	// assert
	assert.NoError(t, gotErr)
}

func Test_ControllerUnpublishVolume_OceanstorDTree_NfsAutoAuthClient(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorDtreeWithNfsAutoAuthClient()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeHostInfo := &host.NodeHostInfo{HostName: "test-host", HostIPs: []string{"192.168.1.2"}}
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().Logout(ctx)
	patches := gomonkey.ApplyFuncReturn(host.GetNodeHostInfosFromSecret, fakeHostInfo, nil).
		ApplyMethodReturn(&volume.DTree{}, "AutoManageAuthClient", nil).
		ApplyMethodReturn(app.GetGlobalConfig().K8sUtils, "GetDTreeParentNameByVolumeId", data.DTreeParentName, nil)
	defer patches.Reset()

	// action
	_, gotErr := csiServer.ControllerUnpublishVolume(ctx, data.request())

	// assert
	assert.NoError(t, gotErr)
}

func fakeOceanstorDtree() *oceanstorDtree {
	return &oceanstorDtree{
		VolumeName:               "test-dtree-volume",
		NodeId:                   `{"HostName": "test-node"}`,
		DTreeParentName:          "test-parent-name",
		DisableVerifyCapacity:    "",
		Permission:               "755",
		BackendName:              "test-oceanstor-dtree",
		EnabledNfsAutoAuthClient: false,
		NfsAutoAuthClientCIDRs:   nil,
		ExpectedDTreeParentname:  "test-parent-name",
	}
}

func fakeOceanstorDtreeWithNfsAutoAuthClient() *oceanstorDtree {
	return &oceanstorDtree{
		VolumeName:               "test-dtree-volume",
		NodeId:                   `{"HostName": "test-node"}`,
		DTreeParentName:          "test-parent-name",
		DisableVerifyCapacity:    "",
		Permission:               "755",
		BackendName:              "test-oceanstor-dtree",
		EnabledNfsAutoAuthClient: true,
		NfsAutoAuthClientCIDRs:   []string{"192.168.1.0/24"},
		ExpectedDTreeParentname:  "test-parent-name",
	}
}

type oceanstorDtree struct {
	VolumeName            string
	NodeId                string
	DTreeParentName       string
	DisableVerifyCapacity string
	Permission            string

	BackendName              string
	BackendParentName        string
	EnabledNfsAutoAuthClient bool
	NfsAutoAuthClientCIDRs   []string

	ExpectedDTreeParentname string
}

func (dtree *oceanstorDtree) backend(cli client.OceanstorClientInterface) model.Backend {
	p := &plugin.OceanstorDTreePlugin{}
	p.SetCli(cli)
	p.SetParentName(dtree.BackendParentName)
	p.SetNfsAutoAuthClient(dtree.EnabledNfsAutoAuthClient, dtree.NfsAutoAuthClientCIDRs)
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

func (dtree *oceanstorDtree) request() *csi.ControllerUnpublishVolumeRequest {
	req := &csi.ControllerUnpublishVolumeRequest{
		VolumeId: dtree.BackendName + "." + dtree.VolumeName,
		NodeId:   dtree.NodeId,
	}
	return req
}
