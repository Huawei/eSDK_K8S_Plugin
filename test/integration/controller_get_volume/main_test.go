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

// Package controller_publish_volume includes the integration tests of ControllerPublishVolume interface includes
// the integration tests of getting volume
package controller_get_volume

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/driver"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	csiServer *driver.CsiDriver
)

func TestMain(m *testing.M) {
	kubeClient := &k8sutils.KubeClient{}
	csiServer = driver.NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")
	if csiServer == nil {
		return
	}

	log.MockInitLogging("test")
	defer log.MockStopLogging("test")

	cfg := config.MockCompletedConfig()
	p := gomonkey.NewPatches().ApplyFuncReturn(app.GetGlobalConfig, cfg)
	defer p.Reset()

	m.Run()
}

func TestControllerGetVolume_EmptyVolumeId(t *testing.T) {
	if csiServer == nil {
		kubeClient := &k8sutils.KubeClient{}
		csiServer = driver.NewServer(constants.DefaultDriverName, constants.ProviderVersion, kubeClient, "node1")
	}

	req := &csi.ControllerGetVolumeRequest{VolumeId: ""}
	ctx := context.Background()

	_, err := csiServer.ControllerGetVolume(ctx, req)

	require.Error(t, err)
	require.Contains(t, err.Error(), "no volume ID provided")
}
