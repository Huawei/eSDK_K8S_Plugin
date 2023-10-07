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
	"time"

	"github.com/sirupsen/logrus"

	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	"huawei-csi-driver/utils/k8sutils"
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
	EnableLabel          bool

	Endpoint         string
	DrEndpoint       string
	DriverName       string
	KubeConfig       string
	NodeName         string
	KubeletRootDir   string
	VolumeNamePrefix string

	MaxVolumesPerNode     int
	WebHookPort           int
	WorkerThreads         int
	BackendUpdateInterval int

	LeaderLeaseDuration time.Duration
	LeaderRenewDeadline time.Duration
	LeaderRetryPeriod   time.Duration
	ReSyncPeriod        time.Duration
	Timeout             time.Duration
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
}

type k8sConfig struct {
	Namespace string
}

// Config contains the configurations from env
type Config struct {
	loggingConfig
	serviceConfig
	connectorConfig
	k8sConfig
}

// CompletedConfig contains the env and config
type CompletedConfig struct {
	*Config
	K8sUtils     k8sutils.Interface
	BackendUtils clientSet.Interface
}

func (cfg *Config) Complete() (*CompletedConfig, error) {
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
		Config:       cfg,
		K8sUtils:     k8sUtils,
		BackendUtils: backendUtils,
	}, nil
}

// Print the configuration when before the service
func (cfg *CompletedConfig) Print() {
	logrus.Infof("Controller manager config %+v", cfg.Config)
}
