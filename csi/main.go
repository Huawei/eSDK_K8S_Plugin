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
	"runtime/debug"
	"time"
	"utils"
	"utils/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"google.golang.org/grpc"
)

const (
	ENDPOINT           = "huawei.csi.driver"
	PLUGIN_DIR         = "/var/lib/kubelet/plugins/" + ENDPOINT
	PROVIDER_FLAG_FILE = "/var/lib/kubelet/plugins/" + ENDPOINT + "/provider_running"
)

var (
	configFile = flag.String("config-file", "/etc/huawei/csi.json", "Config file of CSI driver")

	config CSIConfig
	flock  = utils.NewFlock("/var/lock/huawei-csi-driver")
)

type CSIConfig struct {
	Backends       []map[string]interface{} `json:"backends"`
	LogFilePrefix  string                   `json:"logFilePrefix"`
	MaxLogFileSize string                   `json:"maxLogFileSize"`
	LogDir         string                   `json:"logDir"`
}

func init() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(utils.GetCSIVersion())
		os.Exit(0)
	}

	flag.Set("log_dir", "/var/log/huawei")
	flag.Parse()

	data, err := ioutil.ReadFile(*configFile)
	if err != nil {
		glog.Fatalf("Read config file %s error: %v", configFile, err)
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		glog.Fatalf("Unmarshal config file %s error: %v", configFile, err)
	}

	if len(config.Backends) <= 0 {
		glog.Fatalf("Must configure at least one backend")
	}

	logFilePrefix := "huawei-csi"
	if config.LogFilePrefix != "" {
		logFilePrefix = config.LogFilePrefix
	}

	err = log.Init(map[string]string{
		"logFilePrefix": logFilePrefix,
		"logFileMaxCap": config.MaxLogFileSize,
		"logDir":        config.LogDir,
	})
	if err != nil {
		glog.Fatalf("Init log error: %v", err)
	}
}

func updateBackends() {
	err := backend.SyncUpdateCapabilities(PROVIDER_FLAG_FILE)
	if err != nil {
		log.Fatalf("Update backend capabilities error: %v", err)
	}

	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		backend.AsyncUpdateCapabilities(PROVIDER_FLAG_FILE)
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

		flock.UnLock()
	}()

	err := flock.Lock()
	if err != nil {
		log.Fatalf("Lock error: %v, Huawei CSI driver may already be running", err)
	}

	if _, err := os.Stat(PLUGIN_DIR); err != nil && os.IsNotExist(err) {
		os.Mkdir(PLUGIN_DIR, 0755)
	}

	sockFile := fmt.Sprintf("%s/csi.sock", PLUGIN_DIR)
	_, err = os.Stat(sockFile)
	if err == nil {
		err := os.Remove(sockFile)
		if err != nil {
			log.Fatalf("Delete %s error: %v", sockFile, err)
		}
	}

	listener, err := net.Listen("unix", sockFile)
	if err != nil {
		log.Fatalf("Listen on %s error: %v", sockFile, err)
	}

	err = backend.RegisterBackend(config.Backends)
	if err != nil {
		log.Fatalf("Register backends error: %v", err)
	}

	go updateBackends()

	d := driver.NewDriver()

	server := grpc.NewServer()
	csi.RegisterIdentityServer(server, d)
	csi.RegisterControllerServer(server, d)
	csi.RegisterNodeServer(server, d)

	log.Infof("Starting Huawei CSI driver, listening on %s", sockFile)
	server.Serve(listener)
}
