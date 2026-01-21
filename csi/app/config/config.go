/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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
	"time"

	"github.com/sirupsen/logrus"

	clientSet "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
)

type loggingConfig struct {
	LogFileSize   string
	LoggingModule string
	LogLevel      string
	LogFileDir    string
	MaxBackups    uint
}

type serviceConfig struct {
	Controller           bool
	EnableLeaderElection bool

	Endpoint         string
	DrEndpoint       string
	DriverName       string
	KubeConfig       string
	NodeName         string
	KubeletRootDir   string
	VolumeNamePrefix string

	MaxVolumesPerNode int
	WebHookPort       int
	// address of webhook server
	WebHookAddress        string
	WorkerThreads         int
	BackendUpdateInterval int

	ExportCsiServerAddress string
	ExportCsiServerPort    int

	LeaderLeaseDuration time.Duration
	LeaderRenewDeadline time.Duration
	LeaderRetryPeriod   time.Duration
	ReSyncPeriod        time.Duration
	Timeout             time.Duration

	// kubeletVolumeDevicesDirName, default is /volumeDevices/
	KubeletVolumeDevicesDirName string
	ReportNodeIP                bool
}

type connectorConfig struct {
	VolumeUseMultiPath   bool
	ScsiMultiPathType    string
	NvmeMultiPathType    string
	DeviceCleanupTimeout int
	ScanVolumeTimeout    int
	ConnectorThreads     int
	AllPathOnline        bool
	ExecCommandTimeout   int
	EnableRoCEConnect    bool
}

type k8sConfig struct {
	Namespace string
}

type extenderConfig struct {
	// VolumeModifyReSyncPeriod volume modify re-sync period
	VolumeModifyReSyncPeriod time.Duration

	// VolumeModifyRetryBaseDelay retry base delay
	VolumeModifyRetryBaseDelay time.Duration

	// VolumeModifyRetryMaxDelay retry max delay
	VolumeModifyRetryMaxDelay time.Duration

	// VolumeModifyReconcileDelay reconcile delay
	VolumeModifyReconcileDelay time.Duration
}

// AppConfig contains the configurations from env
type AppConfig struct {
	loggingConfig
	serviceConfig
	connectorConfig
	k8sConfig
	extenderConfig
}

// CompletedConfig contains the env and config
type CompletedConfig struct {
	*AppConfig
	K8sUtils     k8sutils.Interface
	BackendUtils clientSet.Interface
}

// Complete the AppConfig and return the CompletedConfig
func (cfg *AppConfig) Complete() (*CompletedConfig, error) {
	k8sUtils, err := k8sutils.NewK8SUtils(cfg.KubeConfig, cfg.VolumeNamePrefix,
		map[string]string{"provisioner": cfg.DriverName})
	if err != nil {
		logrus.Errorf("k8sutils initialized failed %v", err)
		return nil, err
	}

	backendUtils, err := clientSet.NewBackendUtils(cfg.KubeConfig)
	if err != nil {
		logrus.Errorf("BackendUtils initialized failed %v", err)
		return nil, err
	}

	return &CompletedConfig{
		AppConfig:    cfg,
		K8sUtils:     k8sUtils,
		BackendUtils: backendUtils,
	}, nil
}

// Print the configuration when before the service
func (cfg *CompletedConfig) Print() {
	logrus.Infof("Controller manager config %+v", cfg.AppConfig)
}
