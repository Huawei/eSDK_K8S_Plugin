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

// Package options control the service configurations, include env and config
package options

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

const (
	defaultRpcTimeout                   = 1 * time.Minute
	defaultWorkerThreads                = 10
	defaultNodeWorkerThreads            = 4
	defaultReSyncPeriods                = 2 * time.Minute
	defaultLeaderRetryPeriod            = 2 * time.Second
	defaultLeaderRenewDeadline          = 6 * time.Second
	defaultLeaderLeaseDuration          = 8 * time.Second
	defaultBackendUpdateIntervalSeconds = 60
	defaultExportCsiServerPort          = 9090
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
	webHookAddress        string
	backendUpdateInterval int
	workerThreads         int
	nodeWorkerThreads     int

	exportCsiServerAddress string
	exportCsiServerPort    int

	leaderLeaseDuration time.Duration
	leaderRenewDeadline time.Duration
	leaderRetryPeriod   time.Duration
	reSyncPeriod        time.Duration
	timeout             time.Duration

	kubeletVolumeDevicesDirName string
	reportNodeIP                bool
	enablePerNodeSecret         bool
	healthMonitorEnabled        bool
	enableVolumeModify          bool

	kubeApiQps   float64
	kubeApiBurst int
}

// NewServiceOptions returns service configurations
func NewServiceOptions() *serviceOptions {
	return &serviceOptions{}
}

// AddFlags add the service flags
func (opt *serviceOptions) AddFlags(ff *flag.FlagSet) {
	opt.addCSIEndpointFlags(ff)
	opt.addDriverFlags(ff)
	opt.addK8sConnectionFlags(ff)
	opt.addVolumeFlags(ff)
	opt.addWebhookFlags(ff)
	opt.addLeaderElectionFlags(ff)
	opt.addWorkerAndTimeoutFlags(ff)
	opt.addKubeletFlags(ff)
	opt.addExportServiceFlags(ff)
	opt.addFeatureFlags(ff)
	opt.addRateLimitingFlags(ff)
	opt.addHealthMonitorFlag(ff)
}

func (opt *serviceOptions) addCSIEndpointFlags(ff *flag.FlagSet) {
	ff.StringVar(&opt.endpoint, "endpoint",
		"/var/lib/kubelet/plugins/huawei.csi.driver/csi.sock", "CSI endpoint")
	ff.StringVar(&opt.drEndpoint, "dr-endpoint",
		"/var/lib/kubelet/plugins/huawei.csi.driver/dr-csi.sock",
		"DR CSI endpoint")
	ff.BoolVar(&opt.controller, "controller",
		false, "Run as a controller service")
}

func (opt *serviceOptions) addDriverFlags(ff *flag.FlagSet) {
	ff.StringVar(&opt.driverName, "driver-name",
		constants.DefaultDriverName,
		"CSI driver name")
	ff.IntVar(&opt.backendUpdateInterval, "backend-update-interval", defaultBackendUpdateIntervalSeconds,
		"The interval seconds to update backends status. Default is 60 seconds")
}

func (opt *serviceOptions) addK8sConnectionFlags(ff *flag.FlagSet) {
	ff.StringVar(&opt.kubeConfig, "kubeconfig", "",
		"absolute path to the kubeconfig file")
	ff.StringVar(&opt.nodeName, "nodename",
		os.Getenv(constants.NodeNameEnv),
		"node name in kubernetes cluster")
}

func (opt *serviceOptions) addVolumeFlags(ff *flag.FlagSet) {
	ff.StringVar(&opt.kubeletRootDir, "kubeletRootDir", "/var/lib",
		"kubelet root directory")
	ff.StringVar(&opt.volumeNamePrefix, "volume-name-prefix", "pvc",
		"Prefix to apply to the name of a created volume.")
	ff.IntVar(&opt.maxVolumesPerNode, "max-volumes-per-node", 0,
		"The number of volumes that controller can publish to the node")
}

func (opt *serviceOptions) addWebhookFlags(ff *flag.FlagSet) {
	ff.IntVar(&opt.webHookPort, "web-hook-port", 0,
		"The port of webhook server")
	ff.StringVar(&opt.webHookAddress, "web-hook-address", "",
		"The Address of webhook server")
}

func (opt *serviceOptions) addLeaderElectionFlags(ff *flag.FlagSet) {
	ff.BoolVar(&opt.enableLeaderElection, "enable-leader-election", false,
		"backend enable leader election")
	ff.DurationVar(&opt.leaderLeaseDuration, "leader-lease-duration", defaultLeaderLeaseDuration,
		"backend leader lease duration")
	ff.DurationVar(&opt.leaderRenewDeadline, "leader-renew-deadline", defaultLeaderRenewDeadline,
		"backend leader renew deadline")
	ff.DurationVar(&opt.leaderRetryPeriod, "leader-retry-period", defaultLeaderRetryPeriod,
		"backend leader retry period")
	ff.DurationVar(&opt.reSyncPeriod, "re-sync-period", defaultReSyncPeriods, "reSync interval of the controller")
}

func (opt *serviceOptions) addWorkerAndTimeoutFlags(ff *flag.FlagSet) {
	ff.IntVar(&opt.workerThreads, "worker-threads", defaultWorkerThreads, "number of worker threads.")
	ff.IntVar(&opt.nodeWorkerThreads, "node-worker-threads", defaultNodeWorkerThreads, "number of node worker threads.")
	ff.DurationVar(&opt.timeout, "timeout", defaultRpcTimeout, "timeout for any RPCs")
}

func (opt *serviceOptions) addKubeletFlags(ff *flag.FlagSet) {
	ff.StringVar(&opt.kubeletVolumeDevicesDirName, "kubelet-volume-devices-dir-name",
		constants.DefaultKubeletVolumeDevicesDirName, "The dir name of volume devices")
}

func (opt *serviceOptions) addExportServiceFlags(ff *flag.FlagSet) {
	ff.IntVar(&opt.exportCsiServerPort, "export-csi-service-port", defaultExportCsiServerPort,
		"The port of exported csi server")
	ff.StringVar(&opt.exportCsiServerAddress, "export-csi-service-address", "",
		"The address of exported csi server")
}

func (opt *serviceOptions) addFeatureFlags(ff *flag.FlagSet) {
	ff.BoolVar(&opt.reportNodeIP, "report-node-ip", false, "Whether to report node IP")
	ff.BoolVar(&opt.enablePerNodeSecret, "enable-per-node-secret", false, `Whether to enable per-node create secret`)
	ff.BoolVar(&opt.enableVolumeModify, "enable-volume-modify", false, `Whether to enable volume modify feature`)
}

func (opt *serviceOptions) addRateLimitingFlags(ff *flag.FlagSet) {
	ff.Float64Var(&opt.kubeApiQps, "kube-api-qps", constants.DefaultKubeAPIQPS, "QPS for Kubernetes API requests")
	ff.IntVar(&opt.kubeApiBurst, "kube-api-burst", constants.DefaultKubeAPIBurst, "Burst for Kubernetes API requests")
}

func (opt *serviceOptions) addHealthMonitorFlag(ff *flag.FlagSet) {
	ff.BoolVar(&opt.healthMonitorEnabled, "health-monitor-enabled", false, "Whether to enable health monitor")
}

// ApplyFlags assign the service flags
func (opt *serviceOptions) ApplyFlags(cfg *config.AppConfig) {
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
	cfg.WebHookAddress = opt.webHookAddress
	cfg.EnableLeaderElection = opt.enableLeaderElection
	cfg.LeaderRetryPeriod = opt.leaderRetryPeriod
	cfg.LeaderLeaseDuration = opt.leaderLeaseDuration
	cfg.LeaderRenewDeadline = opt.leaderRenewDeadline
	cfg.ReSyncPeriod = opt.reSyncPeriod
	cfg.WorkerThreads = opt.workerThreads
	cfg.NodeWorkerThreads = opt.nodeWorkerThreads
	cfg.Timeout = opt.timeout
	cfg.KubeletVolumeDevicesDirName = opt.kubeletVolumeDevicesDirName
	cfg.ExportCsiServerAddress = opt.exportCsiServerAddress
	cfg.ExportCsiServerPort = opt.exportCsiServerPort
	cfg.ReportNodeIP = opt.reportNodeIP
	cfg.EnablePerNodeSecret = opt.enablePerNodeSecret
	cfg.EnableVolumeModify = opt.enableVolumeModify
	cfg.HealthMonitorEnabled = opt.healthMonitorEnabled
	cfg.KubeAPIQPS = float32(opt.kubeApiQps)
	cfg.KubeAPIBurst = opt.kubeApiBurst
}

// ValidateFlags validate the service flags
func (opt *serviceOptions) ValidateFlags() []error {
	var errs []error

	qps := opt.kubeApiQps
	burst := opt.kubeApiBurst

	if qps < 0 {
		errs = append(errs, fmt.Errorf("kube-api-qps must be >= 0, got %.2f", qps))
	}
	if burst < 0 {
		errs = append(errs, fmt.Errorf("kube-api-burst must be >= 0, got %d", burst))
	}
	if burst > 0 && qps >= float64(burst) {
		errs = append(errs, fmt.Errorf("kube-api-burst (%d) must be > kube-api-qps (%.2f)",
			burst, qps))
	}

	return errs
}
