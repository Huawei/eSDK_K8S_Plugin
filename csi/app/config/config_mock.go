/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

// Package config includes the configurations from env
package config

import (
	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	"huawei-csi-driver/utils/k8sutils"
)

// MockCompletedConfig for unit test
func MockCompletedConfig() *CompletedConfig {
	return &CompletedConfig{
		Config: &Config{
			mockLoggingConfig(),
			mockServiceConfig(),
			mockConnectorConfig(),
			mockK8sConfig(),
		},
		K8sUtils:     &k8sutils.KubeClient{},
		BackendUtils: &clientSet.Clientset{},
	}
}

func mockLoggingConfig() loggingConfig {
	return loggingConfig{
		LogFileSize:   "1024",
		LoggingModule: "file",
		LogLevel:      "info",
		LogFileDir:    "fake-dir",
		MaxBackups:    5,
	}
}

func mockServiceConfig() serviceConfig {
	return serviceConfig{
		Controller:           false,
		EnableLeaderElection: false,

		Endpoint:         "",
		DrEndpoint:       "",
		DriverName:       "",
		KubeConfig:       "",
		NodeName:         "",
		KubeletRootDir:   "",
		VolumeNamePrefix: "",

		MaxVolumesPerNode:     0,
		WebHookPort:           0,
		WorkerThreads:         0,
		BackendUpdateInterval: 0,
	}
}

func mockConnectorConfig() connectorConfig {
	return connectorConfig{
		VolumeUseMultiPath:   false,
		ScsiMultiPathType:    "DM-multipath",
		NvmeMultiPathType:    "HW-UltraPath-NVMe",
		DeviceCleanupTimeout: 5,
		ScanVolumeTimeout:    5,
		ConnectorThreads:     5,
		AllPathOnline:        true,
	}
}

func mockK8sConfig() k8sConfig {
	return k8sConfig{
		Namespace: "mock-namespace",
	}
}
