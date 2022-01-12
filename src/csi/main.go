package main

import (
	"csi/backend"
	"csi/driver"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
	"utils"
	"utils/k8sutils"
	"utils/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	configFile        = "/etc/huawei/csi.json"
	secretFile        = "/etc/huawei/secret/secret.json"
	controllerLogFile = "huawei-csi-controller"
	nodeLogFile       = "huawei-csi-node"
	csiLogFile        = "huawei-csi"

	csiVersion        = "2.2.15"
	defaultDriverName = "csi.huawei.com"

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
		backendName := backendConfig["name"].(string)
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
	// parse command line flags
	flag.Parse()

	// ensure flags status
	if *containerized {
		*controllerFlagFile = ""
	}

	keepLogin := *controller || *controllerFlagFile != ""
	err := log.InitLogging(getLogFileName())
	if err != nil {
		logrus.Fatalf("Init log error: %v", err)
	}
	defer func() {
		ensureRuntimePanicLogging()
		releaseStorageClient(keepLogin)
	}()

	// parse configurations
	parseConfig()
	err = backend.RegisterBackend(config.Backends, keepLogin, *driverName)
	if err != nil {
		raisePanic("Register backends error: %v", err)
	}
	if keepLogin {
		go updateBackendCapabilities()
	}

	endpointDir := filepath.Dir(*endpoint)
	_, err = os.Stat(endpointDir)
	if err != nil && os.IsNotExist(err) {
		os.Mkdir(endpointDir, 0755)
	} else {
		_, err := os.Stat(*endpoint)
		if err == nil {
			log.Infof("Gonna remove old sock file %s", *endpoint)
			os.Remove(*endpoint)
		}
	}

	k8sUtils, err := k8sutils.NewK8SUtils(*kubeconfig)
	if err != nil {
		raisePanic("Kubernetes client initialization failed  %v", err)
	}

	// For device cleanup before bootup only in node plugin
	if !keepLogin {
		triggerGarbageCollector(k8sUtils)
	}

	registerServer(k8sUtils)
}

func registerServer(k8sUtils k8sutils.Interface) {
	listener, err := net.Listen("unix", *endpoint)
	if err != nil {
		raisePanic("Listen on %s error: %v", *endpoint, err)
	}

	isNeedMultiPath := utils.NeedMultiPath(config.Backends)
	d := driver.NewDriver(*driverName, csiVersion, *volumeUseMultiPath, isNeedMultiPath, k8sUtils, *nodeName)

	server := grpc.NewServer()
	csi.RegisterIdentityServer(server, d)
	csi.RegisterControllerServer(server, d)
	csi.RegisterNodeServer(server, d)

	log.Infof("Starting Huawei CSI driver, listening on %s", *endpoint)
	if err := server.Serve(listener); err != nil {
		raisePanic("Start Huawei CSI driver error: %v", err)
	}
}

func triggerGarbageCollector(k8sUtils k8sutils.Interface) {
	// Trigger stale device clean up and exit after cleanup completion or during timeout
	cleanupReport := make(chan error, 1)

	defer func() {
		close(cleanupReport)
	}()

	go func(ch chan error) {
		res := nodeStaleDeviceCleanup(k8sUtils, *kubeletRootDir, *driverName, *nodeName)
		ch <- res
	}(cleanupReport)

	timeoutInterval := time.Second * time.Duration(*deviceCleanupTimeout)
	select {
	case report := <-cleanupReport:
		if nil == report {
			log.Infof("Successfully completed stale device garbage collection")
		} else {
			log.Infof("Stale device garbage collection exited with error %s", report)
		}
	case <-time.After(timeoutInterval):
		log.Infof("Stale device garbage collection incomplete, exited due to timeout")
	}
	return
}