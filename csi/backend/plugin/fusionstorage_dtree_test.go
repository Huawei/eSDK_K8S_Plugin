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

package plugin

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestFusionStorageDTreePlugin_Init(t *testing.T) {
	// arrange
	ctx := context.Background()
	p := GetPlugin(constants.FusionDTree)
	backendName := "test-fusionstorage-dtree"
	backendId := "huawei-csi/" + backendName
	parentName := "test-parentname"
	config := map[string]any{"backendID": backendId, "contentName": "test-content-name",
		"maxClientThreads": "30", "name": backendName, "namespace": "huawei-csi",
		"parameters":  map[string]any{"portals": []any{"test-portals"}, "protocol": "nfs"},
		"provisioner": constants.DefaultDriverName, "secretName": backendId, "secretNamespace": "huawei-csi",
		"storage": constants.FusionDTree, "urls": []any{"test-urls"}, "user": "test-user", "userCert": false}
	parameters := map[string]any{"parentname": parentName, "portals": []interface{}{"test-portals"}, "protocol": "nfs"}
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)

	// mock
	patch := gomonkey.NewPatches().ApplyFuncReturn(client.NewIRestClient, cli)
	defer patch.Reset()
	cli.EXPECT().Login(ctx).Return(nil)
	cli.EXPECT().SetAccountId(ctx)

	// mock

	// action
	err := p.Init(ctx, config, parameters, true)

	// assert
	require.NoError(t, err)
	pp, ok := p.(*FusionStorageDTreePlugin)
	if !ok {
		return
	}
	require.Equal(t, "nfs", pp.protocol)
	require.Equal(t, parentName, pp.parentname)
}

func TestFusionStorageDTreePlugin_UpdateBackendCapabilities(t *testing.T) {
	// arrange
	ctx := context.Background()
	p := GetPlugin(constants.FusionDTree)
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	pp, ok := p.(*FusionStorageDTreePlugin)
	if !ok {
		return
	}
	pp.cli = cli
	expectedCapabilities := map[string]any{
		"SupportClone": false,
		"SupportNFS3":  true,
		"SupportNFS4":  false,
		"SupportNFS41": true,
		"SupportQoS":   false,
		"SupportQuota": false,
		"SupportThick": false,
		"SupportThin":  true,
	}

	// mock
	cli.EXPECT().GetNFSServiceSetting(ctx).Return(map[string]bool{"SupportNFS41": true}, nil)

	// action
	capabilities, specifications, err := p.UpdateBackendCapabilities(ctx)

	// assert
	require.NoError(t, err)
	require.Nil(t, specifications)
	require.Equal(t, expectedCapabilities, capabilities)
}

func TestFusionStorageDTreePlugin_UpdatePoolCapabilities(t *testing.T) {
	// arrange
	ctx := context.Background()
	p := GetPlugin(constants.FusionDTree)

	// action
	capacities, err := p.UpdatePoolCapabilities(ctx, []string{})

	// assert
	require.NoError(t, err)
	require.Len(t, capacities, 0)
}

func TestFusionStorageDTreePlugin_Validate(t *testing.T) {
	// arrange
	ctx := context.Background()
	p := GetPlugin(constants.FusionDTree)
	backendName := "test-fusionstorage-dtree"
	backendId := "huawei-csi/" + backendName
	parentName := "test-parentname"
	config := map[string]any{"backendID": backendId, "contentName": "test-content-name",
		"maxClientThreads": "30", "name": backendName, "namespace": "huawei-csi",
		"parameters":  map[string]any{"parentname": parentName, "portals": []any{"test-portals"}, "protocol": "nfs"},
		"provisioner": constants.DefaultDriverName, "secretName": backendId, "secretNamespace": "huawei-csi",
		"storage": constants.FusionDTree, "urls": []any{"test-urls"}, "user": "test-user", "userCert": false}
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)

	// mock
	patch := gomonkey.NewPatches().ApplyFuncReturn(client.NewIRestClient, cli)
	defer patch.Reset()
	cli.EXPECT().ValidateLogin(ctx).Return(nil)
	cli.EXPECT().Logout(ctx)

	// action
	err := p.Validate(ctx, config)

	// assert
	require.NoError(t, err)
}
