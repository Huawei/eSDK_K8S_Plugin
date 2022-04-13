/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
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
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"huawei-csi-driver/connector"
	connutils "huawei-csi-driver/connector/utils"
	"huawei-csi-driver/connector/utils/lock"
	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/csi/driver"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/k8sutils"
	"huawei-csi-driver/utils/log"
)

const (
	configFile        = "/etc/huawei/csi.json"
	secretFile        = "/etc/huawei/secret/secret.json"
	controllerLogFile = "huawei-csi-controller"
	nodeLogFile       = "huawei-csi-node"
	csiLogFile        = "huawei-csi"

	csiVersion        = "2.2.16"
	csiBetaVersion    = "B060"
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
		300,
		"Timeout interval in seconds for stale device cleanup")
	scanVolumeTimeout = flag.Int("scan-volume-timeout",
		3,
		"The timeout for waiting for multipath aggregation "+
			"when DM-multipath is used on the host")

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
		raisePanic("Read config file %s error: %v", configFile, err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		raisePanic("Unmarshal config file %s error: %v", configFile, err)
	}

	if len(config.Backends) <= 0 {
		raisePanic("Must configure at least one backend")
	}

	secretData, err := ioutil.ReadFile(secretFile)
	if err != nil {
		raisePanic("Read config file %s error: %v", secretFile, err)
	}

	err = json.Unmarshal(secretData, &secret)
	if err != nil {
		raisePanic("Unmarshal config file %s error: %v", secretFile, err)
	}

	err = mergeData(config, secret)
	if err != nil {
		raisePanic("Merge configs error: %v", err)
	}

	// nodeName flag is only considered for node plugin
	if "" == *nodeName && !*controller {
		log.Warningln("Node name is empty. Topology aware volume provisioning feature may not behave normal")
	}

	if *scanVolumeTimeout < 1 || *scanVolumeTimeout > 600 {
		raisePanic("The value of scanVolumeTimeout ranges from 1 to 600,%d", *scanVolumeTimeout)
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
		getSecret(backendSecret, backendConfig, "keyText")
	}
	return nil
}

func updateBackendCapabilities() {
	err := backend.SyncUpdateCapabilities()
	if err != nil {
		raisePanic("Update backend capabilities error: %v", err)
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

func ensureRuntimePanicLogging() {
	if r := recover(); r != nil {
		log.Errorf("Runtime error caught in main routine: %v", r)
		log.Errorf("%s", debug.Stack())
	}

	log.Flush()
	log.Close()
}

func releaseStorageClient(keepLogin bool) {
	if keepLogin {
		backend.LogoutBackend()
	}
}
func raisePanic(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Errorln(msg)
	panic(msg)
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
	defer func() {
		ensureRuntimePanicLogging()
		releaseStorageClient(controllerService)
	}()

	// parse configurations
	parseConfig()
	if !controllerService {
		doNodeAction()
	}

	err = backend.RegisterBackend(config.Backends, controllerService, *driverName)
	if err != nil {
		raisePanic("Register backends error: %v", err)
	}

	if controllerService {
		go updateBackendCapabilities()
	}

	k8sUtils, err := k8sutils.NewK8SUtils(*kubeconfig)
	if err != nil {
		raisePanic("Kubernetes client initialization failed %v", err)
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
			log.Fatalf("Error creating directory %s. error: %v", endpoint, err)
		}
	} else {
		_, err = os.Stat(endpoint)
		if err == nil {
			log.Infof("Gonna remove old sock file %s", endpoint)
			err = os.Remove(endpoint)
			if err != nil {
				log.Fatalf("Error removing directory %s. error: %v", endpoint, err)
			}
		}
	}
	listener, err := net.Listen("unix", endpoint)
	if err != nil {
		log.Fatalf("Listen on %s error: %v", endpoint, err)
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
		raisePanic("Start Huawei CSI driver error: %v", err)
	}
}

func checkMultiPathType() {
	if *volumeUseMultiPath {
		if !(*scsiMultiPathType == connector.DMMultiPath || *scsiMultiPathType == connector.HWUltraPath ||
			*scsiMultiPathType == connector.HWUltraPathNVMe) {
			log.Fatalf("The scsi-multipath-type=%v configuration is incorrect.", scsiMultiPathType)
		}
		if *nvmeMultiPathType != connector.HWUltraPathNVMe {
			log.Fatalf("The nvme-multipath-type=%v configuration is incorrect.", nvmeMultiPathType)
		}
	}
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
		log.Fatalf("Get required multipath services failed. Error: %v", err)
	}

	err = connutils.VerifyMultipathService(requiredServices,
		utils.GetForbiddenMultipath(context.Background(), multipathConfig, config.Backends))
	if err != nil {
		log.Fatalf("Check multipath service failed. error:%v", err)
	}
	log.Infof("Check multipath service success.")
}

func doNodeAction() {
	err := lock.InitLock(*driverName)
	if err != nil {
		log.Fatalf("Init Lock error for driver %s: %v", *driverName, err)
	}

	checkMultiPathType()
	checkMultiPathService()
}

func triggerGarbageCollector(k8sUtils k8sutils.Interface) {
	// Trigger stale device clean up and exit after cleanup completion or during timeout
	log.Debugf("Enter func triggerGarbageCollector")
	cleanupReport := make(chan error, 1)
	defer func() {
		close(cleanupReport)
	}()
	go func(ch chan error) {
		res := nodeStaleDeviceCleanup(context.Background(), k8sUtils, *kubeletRootDir, *driverName, *nodeName)
		ch <- res
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
