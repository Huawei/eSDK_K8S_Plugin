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
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/host"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestControllerPublishVolume_OceanstorNas_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorNasData()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().Logout(ctx)

	// action
	_, err := csiServer.ControllerPublishVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
}

func TestControllerPublishVolume_OceanstorNas_NfsAutoAuthClient(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorNasDataNfsAutoAuthClient()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	fakeWwn := "fake-wwn"
	fakeHostInfo := &host.NodeHostInfo{HostName: "test-host", HostIPs: []string{"192.168.1.2"}}
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().Logout(ctx)
	cli.EXPECT().GetCurrentLifWwn().Return(fakeWwn).Times(2)
	cli.EXPECT().GetCurrentSiteWwn().Return(fakeWwn).Times(2)
	patches := gomonkey.ApplyFuncReturn(host.GetNodeHostInfosFromSecret, fakeHostInfo, nil).
		ApplyMethodReturn(&volume.NAS{}, "AutoManageAuthClient", nil)
	defer patches.Reset()

	// action
	_, err := csiServer.ControllerPublishVolume(ctx, data.request())

	// assert
	require.NoError(t, err)
}

func fakeOceanstorNasData() *oceanstorNas {
	return &oceanstorNas{
		VolumeName:               "test-nas-volume",
		NodeId:                   `{"HostName": "test-node"}`,
		DisableVerifyCapacity:    "",
		Permission:               "755",
		BackendName:              "test-oceanstor-nas",
		EnabledNfsAutoAuthClient: false,
		NfsAutoAuthClientCIDRs:   nil,
	}
}

func fakeOceanstorNasDataNfsAutoAuthClient() *oceanstorNas {
	return &oceanstorNas{
		VolumeName:               "test-nas-volume",
		NodeId:                   `{"HostName": "test-node"}`,
		DisableVerifyCapacity:    "",
		Permission:               "755",
		BackendName:              "test-oceanstor-nas",
		EnabledNfsAutoAuthClient: true,
		NfsAutoAuthClientCIDRs:   []string{"192.168.1.0/24"},
	}
}

type oceanstorNas struct {
	VolumeName            string
	NodeId                string
	DisableVerifyCapacity string
	Permission            string

	BackendName              string
	BackendParentName        string
	EnabledNfsAutoAuthClient bool
	NfsAutoAuthClientCIDRs   []string
}

func (nas *oceanstorNas) backend(cli client.OceanstorClientInterface) model.Backend {
	p := &plugin.OceanstorNasPlugin{}
	p.SetCli(cli)
	p.SetNfsAutoAuthClient(nas.EnabledNfsAutoAuthClient, nas.NfsAutoAuthClientCIDRs)
	return model.Backend{
		Name:        nas.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorNas,
		Available:   true,
		Plugin:      p,
		Pools: []*model.StoragePool{
			{
				Name:    nas.BackendName,
				Storage: constants.OceanStorNas,
				Parent:  nas.BackendName,
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
		Parameters: map[string]any{"parentname": nas.BackendParentName},
	}
}

func (nas *oceanstorNas) request() *csi.ControllerPublishVolumeRequest {
	req := &csi.ControllerPublishVolumeRequest{
		VolumeId: nas.BackendName + "." + nas.VolumeName,
		NodeId:   nas.NodeId,
		VolumeContext: map[string]string{
			"backend":                          nas.BackendName,
			"name":                             nas.VolumeName,
			"fsPermission":                     nas.Permission,
			constants.DisableVerifyCapacityKey: nas.DisableVerifyCapacity,
		},
	}
	return req
}
