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

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func Test_ControllerUnpublishVolume_OceanstorSan_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := fakeOceanstorSanData()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockOceanstorClientInterface(mockCtrl)
	cache.BackendCacheProvider.Store(ctx, data.BackendName, data.backend(cli))
	defer cache.BackendCacheProvider.Delete(ctx, data.BackendName)

	// mock
	cli.EXPECT().Logout(ctx)
	cli.EXPECT().MakeLunName(data.LunName).Return(data.LunName)
	cli.EXPECT().GetLunByName(ctx, data.LunName).Return(data.Lun, nil).AnyTimes()
	cli.EXPECT().GetHostByName(ctx, data.HostName).Return(data.Host, nil)
	cli.EXPECT().QueryAssociateLunGroup(ctx, 11, data.LunId).Return(data.LunGroups, nil)

	// action
	_, gotErr := csiServer.ControllerUnpublishVolume(ctx, data.request())

	// assert
	assert.NoError(t, gotErr)
}

type oceanstorSan struct {
	VolumeName  string
	NodeId      string
	BackendName string
	Lun         map[string]any
	LunId       string
	LunGroups   []interface{}
	LunName     string
	Host        map[string]any
	HostName    string
	HostId      string
}

func fakeOceanstorSanData() *oceanstorSan {
	return &oceanstorSan{
		VolumeName:  "test-san-volume",
		NodeId:      `{"HostName": "test-node"}`,
		BackendName: "test-oceanstor-san",
		LunName:     "test-san-volume",
		HostName:    "k8s_test-node",
		LunId:       "1",
		Lun:         map[string]any{"NAME": "test-san-volume", "ID": "1", "WWN": "xxx"},
		Host:        map[string]any{"hostname": "k8s_test-node", "ID": "1"},
		LunGroups:   make([]interface{}, 0),
	}
}

func (san *oceanstorSan) backend(cli client.OceanstorClientInterface) model.Backend {
	p := &plugin.OceanstorSanPlugin{}
	p.SetCli(cli)
	p.SetStorageOnline(true)
	return model.Backend{
		Name:        san.BackendName,
		ContentName: "test-content-name",
		Storage:     constants.OceanStorSan,
		Available:   true,
		Plugin:      p,
		Pools: []*model.StoragePool{
			{
				Name:    san.BackendName,
				Storage: constants.OceanStorSan,
				Parent:  san.BackendName,
				Capabilities: map[string]bool{
					"SupportQoS":             true,
					"SupportReplication":     false,
					"SupportThick":           false,
					"SupportThin":            true,
					"SupportApplicationType": true,
					"SupportClone":           true,
					"SupportMetro":           false,
				},
				Capacities: map[string]string{},
				Plugin:     p,
			},
		},
	}
}

func (san *oceanstorSan) request() *csi.ControllerUnpublishVolumeRequest {
	req := &csi.ControllerUnpublishVolumeRequest{
		VolumeId: san.BackendName + "." + san.VolumeName,
		NodeId:   san.NodeId,
	}
	return req
}
