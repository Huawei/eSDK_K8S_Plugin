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

// Package backend includes the integration tests of backend
package backend

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	v1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestBuildBackend_FusionStorageDTree_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	data := getFusionDTreeBuildBackendData()
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)

	// mock
	p := gomonkey.NewPatches().
		ApplyFuncReturn(backend.GetStorageBackendInfo, data.fakeBackendConfig(), nil).
		ApplyFuncReturn(client.NewIRestClient, cli)
	defer p.Reset()
	cli.EXPECT().Login(ctx).Return(nil)
	cli.EXPECT().SetAccountId(ctx)

	// action
	got, err := backend.BuildBackend(ctx, data.getStorageBackendContent())

	// assert
	require.NoError(t, err)
	data.assertBackend(t, got)
}

func getFusionDTreeDPCBuildBackendData() *BuildBackendData {
	return &BuildBackendData{
		BackendName: "test-fusionstorage-dtree",
		ContentName: "test-content-name",
		ParentName:  "test-parentname",
		Namespace:   "huawei-csi",
		StorageType: constants.FusionDTree,
		Urls:        []any{"test-urls"},
		Protocol:    constants.ProtocolDpc,
		Portals:     []any{"test-portals"},
		User:        "test-user",
	}
}

func getFusionDTreeBuildBackendData() *BuildBackendData {
	return &BuildBackendData{
		BackendName: "test-fusionstorage-dtree",
		ContentName: "test-content-name",
		ParentName:  "test-parentname",
		Namespace:   "huawei-csi",
		StorageType: constants.FusionDTree,
		Urls:        []any{"test-urls"},
		Protocol:    "nfs",
		Portals:     []any{"test-portals"},
		User:        "test-user",
	}
}

type BuildBackendData struct {
	BackendName string
	ContentName string
	AccountName string
	ParentName  string
	Namespace   string
	StorageType string
	Urls        []any
	Protocol    string
	Portals     []any
	User        string
}

func (data *BuildBackendData) fakeBackendConfig() map[string]any {
	res := map[string]any{
		"name":             data.BackendName,
		"namespace":        data.Namespace,
		"storage":          data.StorageType,
		"urls":             data.Urls,
		"maxClientThreads": "30",
		"provisioner":      constants.DefaultDriverName,
		"parameters": map[string]any{
			"protocol": data.Protocol,
			"portals":  data.Portals,
		},
		"secretNamespace": data.Namespace,
		"secretName":      data.fullName(),
		"user":            data.User,
		"backendID":       data.fullName(),
		"userCert":        false,
		"contentName":     data.ContentName,
	}

	if data.ParentName != "" {
		params, ok := res["parameters"].(map[string]any)
		if ok {
			params["parentname"] = data.ParentName
		}
	}

	if data.AccountName != "" {
		res["accountName"] = data.AccountName
	}

	return res
}

func (data *BuildBackendData) assertBackend(t *testing.T, actualBackend *model.Backend) {
	require.Equal(t, actualBackend.Name, data.BackendName)
	require.Equal(t, actualBackend.ContentName, data.ContentName)
	require.Equal(t, actualBackend.Storage, data.StorageType)
	require.Equal(t, actualBackend.Parameters,
		map[string]any{"parentname": "test-parentname", "protocol": data.Protocol, "portals": data.Portals})
	require.Equal(t, actualBackend.AccountName, data.AccountName)
}

func (data *BuildBackendData) fullName() string {
	return data.Namespace + "/" + data.BackendName
}

func (data *BuildBackendData) getStorageBackendContent() v1.StorageBackendContent {
	return v1.StorageBackendContent{
		Spec: v1.StorageBackendContentSpec{
			Provider:         constants.DefaultDriverName,
			ConfigmapMeta:    data.fullName(),
			SecretMeta:       data.fullName(),
			BackendClaim:     data.fullName(),
			MaxClientThreads: "30",
			UseCert:          false,
		},
		Status: &v1.StorageBackendContentStatus{
			ContentName:     data.ContentName,
			ProviderVersion: "test-version",
			Capabilities: map[string]bool{
				"SupportClone": false,
				"SupportQoS":   false,
				"SupportQuota": false,
				"SupportThick": false,
				"SupportThin":  true,
				"SupportNFS3":  true,
				"SupportNFS4":  false,
				"SupportNFS41": true,
			},
			ConfigmapMeta:    data.fullName(),
			SecretMeta:       data.fullName(),
			Online:           true,
			MaxClientThreads: "30",
			SN:               "test-sn",
		},
	}
}
