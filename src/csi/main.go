package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/src/csi/backend"
	"github.com/Huawei/eSDK_K8S_Plugin/src/csi/driver"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	configFile        = "/etc/huawei/csi.json"
	controllerLogFile = "huawei-csi-controller"
	nodeLogFile       = "huawei-csi-node"
	csiLogFile        = "huawei-csi"

	csiVersion        = "2.2.11"
	defaultDriverName = "csi.huawei.com"
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

	config CSIConfig
)

type CSIConfig struct {
	Backends []map[string]interface{} `json:"backends"`
}

func init() {
	_ = flag.Set("log_dir", "/var/log/huawei")
	flag.Parse()

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		logrus.Fatalf("Read config file %s error: %v", configFile, err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		logrus.Fatalf("Unmarshal config file %s error: %v", configFile, err)
	}

	if len(config.Backends) <= 0 {
		logrus.Fatalf("Must configure at least one backend")
	}

	if *containerized {
		*controllerFlagFile = ""
	}

	var logFilePrefix string
	if len(*controllerFlagFile) > 0 {
		logFilePrefix = csiLogFile
	} else if *controller {
		logFilePrefix = controllerLogFile
	} else {
		logFilePrefix = nodeLogFile
	}

	err = log.Init(map[string]string{
		"logFilePrefix": logFilePrefix,
	})
	if err != nil {
		logrus.Fatalf("Init log error: %v", err)
	}
}

func updateBackendCapabilities() {
	err := backend.SyncUpdateCapabilities()
	if err != nil {
		log.Fatalf("Update backend capabilities error: %v", err)
	}

	ticker := time.NewTicker(time.Second * time.Duration(*backendUpdateInterval))
	for range ticker.C {
		backend.AsyncUpdateCapabilities(*controllerFlagFile)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Runtime error caught in main routine: %v", r)
			log.Errorf("%s", debug.Stack())
		}

		log.Flush()
		log.Close()
	}()

	if *controller || *controllerFlagFile != "" {
		err := backend.RegisterBackend(config.Backends, true)
		if err != nil {
			log.Fatalf("Register backends error: %v", err)
		}

		go updateBackendCapabilities()
	} else {
		err := backend.RegisterBackend(config.Backends, false)
		if err != nil {
			log.Fatalf("Register backends error: %v", err)
		}
	}

	endpointDir := filepath.Dir(*endpoint)
	_, err := os.Stat(endpointDir)
	if err != nil && os.IsNotExist(err) {
		os.Mkdir(endpointDir, 0755)
	} else {
		_, err := os.Stat(*endpoint)
		if err == nil {
			log.Infof("Gonna remove old sock file %s", *endpoint)
			os.Remove(*endpoint)
		}
	}

	listener, err := net.Listen("unix", *endpoint)
	if err != nil {
		log.Fatalf("Listen on %s error: %v", *endpoint, err)
	}

	d := driver.NewDriver(*driverName, csiVersion)
	server := grpc.NewServer()

	csi.RegisterIdentityServer(server, d)
	csi.RegisterControllerServer(server, d)
	csi.RegisterNodeServer(server, d)

	log.Infof("Starting Huawei CSI driver, listening on %s", *endpoint)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("Start Huawei CSI driver error: %v", err)
	}
}
