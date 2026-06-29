/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2026. All rights reserved.
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
	"fmt"
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
	NodeWorkerThreads     int
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
	EnablePerNodeSecret         bool
	HealthMonitorEnabled        bool
	// EnableVolumeModify indicates whether to enable volume modification feature.
	EnableVolumeModify bool

	// KubeAPIQPS is the QPS limit for Kubernetes API requests.
	KubeAPIQPS float32
	// KubeAPIBurst is the burst limit for Kubernetes API requests.
	KubeAPIBurst int
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
	if cfg.KubeAPIQPS < 0 {
		return nil, fmt.Errorf("kube-api-qps must be >= 0, got %.2f", cfg.KubeAPIQPS)
	}
	if cfg.KubeAPIBurst < 0 {
		return nil, fmt.Errorf("kube-api-burst must be >= 0, got %d", cfg.KubeAPIBurst)
	}
	if cfg.KubeAPIBurst > 0 && cfg.KubeAPIQPS >= float32(cfg.KubeAPIBurst) {
		return nil, fmt.Errorf("kube-api-burst (%d) must be > kube-api-qps (%.2f)",
			cfg.KubeAPIBurst, cfg.KubeAPIQPS)
	}

	k8sOpts := []k8sutils.Option{
		k8sutils.WithQPS(cfg.KubeAPIQPS),
		k8sutils.WithBurst(cfg.KubeAPIBurst),
		k8sutils.WithVolumeNamePrefix(cfg.VolumeNamePrefix),
		k8sutils.WithVolumeLabels(map[string]string{"provisioner": cfg.DriverName}),
		k8sutils.WithEnableVolumeModify(cfg.EnableVolumeModify),
	}
	k8sUtils, err := k8sutils.NewK8SUtils(cfg.KubeConfig, k8sOpts...)
	if err != nil {
		logrus.Errorf("k8sutils initialized failed %v", err)
		return nil, err
	}

	backendUtils, err := k8sutils.NewBackendUtils(cfg.KubeConfig,
		k8sutils.QPS(cfg.KubeAPIQPS),
		k8sutils.Burst(cfg.KubeAPIBurst))
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
