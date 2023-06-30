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

// Package options control the service configurations, include env and config
package options

import (
	"flag"
	"os"
	"time"

	"huawei-csi-driver/csi/app/config"
)

const (
	defaultDriverName = "csi.huawei.com"
	nodeNameEnv       = "CSI_NODENAME"
)

// serviceOptions include service's configuration
type serviceOptions struct {
	controller           bool
	enableLeaderElection bool

	driverName       string
	endpoint         string
	drEndpoint       string
	kubeConfig       string
	nodeName         string
	kubeletRootDir   string
	volumeNamePrefix string

	maxVolumesPerNode     int
	webHookPort           int
	backendUpdateInterval int
	workerThreads         int

	leaderLeaseDuration time.Duration
	leaderRenewDeadline time.Duration
	leaderRetryPeriod   time.Duration
	reSyncPeriod        time.Duration
	timeout             time.Duration
}

// NewServiceOptions returns service configurations
func NewServiceOptions() *serviceOptions {
	return &serviceOptions{}
}

// AddFlags add the service flags
func (opt *serviceOptions) AddFlags(ff *flag.FlagSet) {
	ff.StringVar(&opt.endpoint, "endpoint",
		"/var/lib/kubelet/plugins/huawei.csi.driver/csi.sock",
		"CSI endpoint")
	ff.StringVar(&opt.drEndpoint, "dr-endpoint",
		"/var/lib/kubelet/plugins/huawei.csi.driver/dr-csi.sock",
		"DR CSI endpoint")
	ff.BoolVar(&opt.controller, "controller",
		false,
		"Run as a controller service")
	ff.StringVar(&opt.driverName, "driver-name",
		defaultDriverName,
		"CSI driver name")
	ff.IntVar(&opt.backendUpdateInterval, "backend-update-interval",
		60,
		"The interval seconds to update backends status. Default is 60 seconds")
	ff.StringVar(&opt.kubeConfig, "kubeconfig",
		"",
		"absolute path to the kubeconfig file")
	ff.StringVar(&opt.nodeName, "nodename",
		os.Getenv(nodeNameEnv),
		"node name in kubernetes cluster")
	ff.StringVar(&opt.kubeletRootDir, "kubeletRootDir",
		"/var/lib",
		"kubelet root directory")
	ff.StringVar(&opt.volumeNamePrefix, "volume-name-prefix", "pvc",
		"Prefix to apply to the name of a created volume.")
	ff.IntVar(&opt.maxVolumesPerNode, "max-volumes-per-node",
		0,
		"The number of volumes that controller can publish to the node")
	ff.IntVar(&opt.webHookPort, "web-hook-port",
		0,
		"The number of volumes that controller can publish to the node")
	ff.BoolVar(&opt.enableLeaderElection, "enable-leader-election",
		false,
		"backend enable leader election")
	ff.DurationVar(&opt.leaderLeaseDuration, "leader-lease-duration",
		8*time.Second,
		"backend leader lease duration")
	ff.DurationVar(&opt.leaderRenewDeadline, "leader-renew-deadline",
		6*time.Second,
		"backend leader renew deadline")
	ff.DurationVar(&opt.leaderRetryPeriod, "leader-retry-period",
		2*time.Second,
		"backend leader retry period")
	ff.DurationVar(&opt.reSyncPeriod, "re-sync-period",
		15*time.Minute,
		"reSync interval of the controller")
	ff.IntVar(&opt.workerThreads, "worker-threads",
		10,
		"number of worker threads.")
	ff.DurationVar(&opt.timeout, "timeout",
		1*time.Minute,
		"timeout for any RPCs")
}

// ApplyFlags assign the service flags
func (opt *serviceOptions) ApplyFlags(cfg *config.Config) {
	cfg.Endpoint = opt.endpoint
	cfg.DrEndpoint = opt.drEndpoint
	cfg.Controller = opt.controller
	cfg.DriverName = opt.driverName
	cfg.BackendUpdateInterval = opt.backendUpdateInterval
	cfg.KubeConfig = opt.kubeConfig
	cfg.NodeName = opt.nodeName
	cfg.KubeletRootDir = opt.kubeletRootDir
	cfg.VolumeNamePrefix = opt.volumeNamePrefix
	cfg.MaxVolumesPerNode = opt.maxVolumesPerNode
	cfg.WebHookPort = opt.webHookPort
	cfg.EnableLeaderElection = opt.enableLeaderElection
	cfg.LeaderRetryPeriod = opt.leaderRetryPeriod
	cfg.LeaderLeaseDuration = opt.leaderLeaseDuration
	cfg.LeaderRenewDeadline = opt.leaderRenewDeadline
	cfg.ReSyncPeriod = opt.reSyncPeriod
	cfg.WorkerThreads = opt.workerThreads
	cfg.Timeout = opt.timeout
}

// ValidateFlags validate the service flags
func (opt *serviceOptions) ValidateFlags() []error {
	return nil
}
