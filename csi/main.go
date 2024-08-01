/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"huawei-csi-driver/connector"
	connutils "huawei-csi-driver/connector/utils"
	"huawei-csi-driver/connector/utils/lock"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/csi/driver"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/k8sutils"
	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/notify"
	"huawei-csi-driver/utils/version"
)

const (
	configFile        = "/etc/huawei/csi.json"
	secretFile        = "/etc/huawei/secret/secret.json"
	versionFile       = "/csi/version"
	controllerLogFile = "huawei-csi-controller"
	nodeLogFile       = "huawei-csi-node"
	csiLogFile        = "huawei-csi"

	csiVersion        = "3.2.3"
	defaultDriverName = "csi.huawei.com"
	endpointDirPerm   = 0755

	nodeNameEnv = "CSI_NODENAME"
)

var (
	endpoint = flag.String("endpoint",
		"/var/lib/kubelet/plugins/huawei.csi.driver/csi.sock",
		"CSI endpoint")
	controller = flag.Bool("controller",
		false,
		"Run as a controller service")
	controllerFlagFile = flag.String("controller-flag-file",
		"/var/lib/kubelet/plugins/huawei.csi.driver/provider_running",
		"The flag file path to specify controller service. Privilege is higher than controller")
	driverName = flag.String("driver-name",
		defaultDriverName,
		"CSI driver name")
	containerized = flag.Bool("containerized",
		false,
		"Run as a containerized service")
	backendUpdateInterval = flag.Int("backend-update-interval",
		60,
		"The interval seconds to update backends status. Default is 60 seconds")
	volumeUseMultiPath = flag.Bool("volume-use-multipath",
		true,
		"Whether to use multipath when attach block volume")
	scsiMultiPathType = flag.String("scsi-multipath-type",
		connector.DMMultiPath,
		"Multipath software for fc/iscsi block volumes")
	nvmeMultiPathType = flag.String("nvme-multipath-type",
		connector.HWUltraPathNVMe,
		"Multipath software for roce/fc-nvme block volumes")
	kubeconfig = flag.String("kubeconfig",
		"",
		"absolute path to the kubeconfig file")
	nodeName = flag.String("nodename",
		os.Getenv(nodeNameEnv),
		"node name in kubernetes cluster")
	kubeletRootDir = flag.String("kubeletRootDir",
		"/var/lib",
		"kubelet root directory")
	deviceCleanupTimeout = flag.Int("deviceCleanupTimeout",
		240,
		"Timeout interval in seconds for stale device cleanup")
	scanVolumeTimeout = flag.Int("scan-volume-timeout",
		3,
		"The timeout for waiting for multipath aggregation "+
			"when DM-multipath is used on the host")
	allPathOnline = flag.Bool("all-path-online",
		false,
		"Whether to check the number of online paths for DM-multipath aggregation, default false")
	kubeletVolumeDevicesDirName = flag.String("kubelet-volume-devices-dir-name", "/volumeDevices/",
		"The dir name of volume devices")

	config CSIConfig
	secret CSISecret
)

type CSIConfig struct {
	Backends []map[string]interface{} `json:"backends"`
}

type CSISecret struct {
	Secrets map[string]interface{} `json:"secrets"`
}

func parseConfig() {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		notify.Stop("Read config file %s error: %v", configFile, err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		notify.Stop("Unmarshal config file %s error: %v", configFile, err)
	}

	if len(config.Backends) <= 0 {
		notify.Stop("Must configure at least one backend")
	}

	secretData, err := ioutil.ReadFile(secretFile)
	if err != nil {
		notify.Stop("Read config file %s error: %v", secretFile, err)
	}

	err = json.Unmarshal(secretData, &secret)
	if err != nil {
		notify.Stop("Unmarshal config file %s error: %v", secretFile, err)
	}

	err = mergeData(config, secret)
	if err != nil {
		notify.Stop("Merge configs error: %v", err)
	}

	// nodeName flag is only considered for node plugin
	if "" == *nodeName && !*controller {
		log.Warningln("Node name is empty. Topology aware volume provisioning feature may not behave normal")
	}

	if *scanVolumeTimeout < 1 || *scanVolumeTimeout > 600 {
		notify.Stop("The value of scanVolumeTimeout ranges from 1 to 600,%d", *scanVolumeTimeout)
	}

	connector.ScanVolumeTimeout = time.Second * time.Duration(*scanVolumeTimeout)
}

func getSecret(backendSecret, backendConfig map[string]interface{}, secretKey string) {
	if secretValue, exist := backendSecret[secretKey].(string); exist {
		backendConfig[secretKey] = secretValue
	} else {
		log.Fatalln(fmt.Sprintf("The key %s is not in secret %v.", secretKey, backendSecret))
	}
}

func mergeData(config CSIConfig, secret CSISecret) error {
	for _, backendConfig := range config.Backends {
		backendName, exist := backendConfig["name"].(string)
		if !exist {
			return fmt.Errorf("the key name does not exist in backend")
		}
		Secret, exist := secret.Secrets[backendName]
		if !exist {
			return fmt.Errorf("the key %s is not in secret", backendName)
		}

		backendSecret := Secret.(map[string]interface{})
		getSecret(backendSecret, backendConfig, "user")
		getSecret(backendSecret, backendConfig, "password")
	}
	return nil
}

func updateBackendCapabilities() {
	err := backend.SyncUpdateCapabilities()
	if err != nil {
		notify.Stop("Update backend capabilities error: %v", err)
	}

	ticker := time.NewTicker(time.Second * time.Duration(*backendUpdateInterval))
	for range ticker.C {
		backend.AsyncUpdateCapabilities(*controllerFlagFile)
	}
}

func getLogFileName() string {
	// check log file name
	logFileName := nodeLogFile
	if len(*controllerFlagFile) > 0 {
		logFileName = csiLogFile
	} else if *controller {
		logFileName = controllerLogFile
	}

	return logFileName
}

func ensureRuntimePanicLogging(ctx context.Context) {
	utils.RecoverPanic(ctx)

	log.Flush()
	log.Close()
}

func releaseStorageClient() {
	backend.LogoutBackend()
}

func main() {
	flag.Parse()
	// ensure flags status
	if *containerized {
		*controllerFlagFile = ""
	}

	controllerService := *controller || *controllerFlagFile != ""
	err := log.InitLogging(getLogFileName())
	if err != nil {
		logrus.Fatalf("Init log error: %v", err)
	}

	go exitClean(controllerService)
	// parse configurations
	parseConfig()
	if !controllerService {
		doNodeAction()
		// init version file on node
		err := version.InitVersion(versionFile, csiVersion)
		if err != nil {
			logrus.Warningf("Init version error: %v", err)
		}
	}

	err = backend.RegisterBackend(config.Backends, controllerService, *driverName)
	if err != nil {
		notify.Stop("Register backends error: %v", err)
	}

	if controllerService {
		go updateBackendCapabilities()
	}

	k8sUtils, err := k8sutils.NewK8SUtils(*kubeconfig)
	if err != nil {
		notify.Stop("Kubernetes client initialization failed %v", err)
	}

	if !controllerService {
		triggerGarbageCollector(k8sUtils)
	}

	d := driver.NewDriver(*driverName, csiVersion, *volumeUseMultiPath, *scsiMultiPathType,
		*nvmeMultiPathType, k8sUtils, *nodeName)

	listener := listenEndpoint(*endpoint)
	registerServer(listener, d)
}

func listenEndpoint(endpoint string) net.Listener {
	endpointDir := filepath.Dir(endpoint)
	_, err := os.Stat(endpointDir)
	if err != nil && os.IsNotExist(err) {
		err = os.Mkdir(endpointDir, endpointDirPerm)
		if err != nil {
			notify.Stop("Error creating directory %s. error: %v", endpoint, err)
		}
	} else {
		_, err = os.Stat(endpoint)
		if err == nil {
			log.Infof("Gonna remove old sock file %s", endpoint)
			err = os.Remove(endpoint)
			if err != nil {
				notify.Stop("Error removing directory %s. error: %v", endpoint, err)
			}
		}
	}
	listener, err := net.Listen("unix", endpoint)
	if err != nil {
		notify.Stop("Listen on %s error: %v", endpoint, err)
	}
	return listener
}

func registerServer(listener net.Listener, d *driver.Driver) {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(log.EnsureGRPCContext),
	}
	server := grpc.NewServer(opts...)

	csi.RegisterIdentityServer(server, d)
	csi.RegisterControllerServer(server, d)
	csi.RegisterNodeServer(server, d)

	log.Infof("Starting Huawei CSI driver, listening on %s", *endpoint)
	if err := server.Serve(listener); err != nil {
		notify.Stop("Start Huawei CSI driver error: %v", err)
	}
}

func checkMultiPathType() {
	if *volumeUseMultiPath {
		if !(*scsiMultiPathType == connector.DMMultiPath || *scsiMultiPathType == connector.HWUltraPath ||
			*scsiMultiPathType == connector.HWUltraPathNVMe) {
			notify.Stop("The scsi-multipath-type=%v configuration is incorrect.", scsiMultiPathType)
		}
		if *nvmeMultiPathType != connector.HWUltraPathNVMe {
			notify.Stop("The nvme-multipath-type=%v configuration is incorrect.", nvmeMultiPathType)
		}
	}
	// set connect config to global config.
	app.Builder().WithVolumeUseMultipath(*volumeUseMultiPath).
		WithScsiMultipathType(*scsiMultiPathType).
		WithNvmeMultipathType(*nvmeMultiPathType).
		WithAllPathOnline(*allPathOnline).
		Build()
}

func checkMultiPathService() {
	multipathConfig := map[string]interface{}{
		"SCSIMultipathType":  *scsiMultiPathType,
		"NVMeMultipathType":  *nvmeMultiPathType,
		"volumeUseMultiPath": *volumeUseMultiPath,
	}

	requiredServices, err := utils.GetRequiredMultipath(context.Background(),
		multipathConfig, config.Backends)
	if err != nil {
		notify.Stop("Get required multipath services failed. Error: %v", err)
	}

	err = connutils.VerifyMultipathService(requiredServices,
		utils.GetForbiddenMultipath(context.Background(), multipathConfig, config.Backends))
	if err != nil {
		notify.Stop("Check multipath service failed. error:%v", err)
	}
	log.Infof("Check multipath service success.")
}

func doNodeAction() {
	err := lock.InitLock(*driverName)
	if err != nil {
		notify.Stop("Init Lock error for driver %s: %v", *driverName, err)
	}

	app.Builder().WithKubeletVolumeDevicesDirName(*kubeletVolumeDevicesDirName).Build()
	checkMultiPathType()
	checkMultiPathService()
}

func triggerGarbageCollector(k8sUtils k8sutils.Interface) {
	// Trigger stale device clean up and exit after cleanup completion or during timeout
	log.Debugf("Enter func triggerGarbageCollector")
	cleanupReport := make(chan error, 1)
	go func(ch chan error) {
		res := nodeStaleDeviceCleanup(context.Background(), k8sUtils, *kubeletRootDir, *driverName, *nodeName)
		ch <- res
		close(ch)
	}(cleanupReport)
	timeoutInterval := time.Second * time.Duration(*deviceCleanupTimeout)
	select {
	case report := <-cleanupReport:
		if report == nil {
			log.Infof("Successfully completed stale device garbage collection")
		} else {
			log.Errorf("Stale device garbage collection exited with error %s", report)
		}
	case <-time.After(timeoutInterval):
		log.Warningf("Stale device garbage collection incomplete, exited due to timeout")
	}
	return
}

func exitClean(isController bool) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	stopChan := notify.GetStopChan()
	defer close(signalChan)
	defer close(stopChan)

	select {
	case sign := <-signalChan:
		log.Infof("Receive exit signal %v", sign)
		clean(isController)
	case <-stopChan:
		log.Infof("Receive stop event ")
		clean(isController)
		os.Exit(-1)
	}
}

func clean(isController bool) {
	// flush log
	ensureRuntimePanicLogging(context.TODO())
	if isController {
		// release client
		releaseStorageClient()
	} else {
		// clean version file
		err := version.ClearVersion(versionFile)
		if err != nil {
			logrus.Warningf("clean version file error: %v", err)
		}
	}
}
