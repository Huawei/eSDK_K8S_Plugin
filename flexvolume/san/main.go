package main

import (
	"encoding/json"
	"flag"
	"flexvolume/config"
	"flexvolume/types"
	"fmt"
	"os"
	"runtime/debug"
	"utils/log"

	"github.com/golang/glog"
)

const (
	version = "2.2.9"
)

func init() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(version)
		os.Exit(0)
	}

	flag.Set("log_dir", "/var/log/huawei")
	flag.Parse()

	config.ParseSanConfig("/etc/huawei/flexvolume-san.json")

	logFilePrefix := config.Config.LogFilePrefix
	if logFilePrefix == "" {
		logFilePrefix = "flexvolume-san"
	}

	err := log.Init(map[string]string{
		"logFilePrefix": logFilePrefix,
		"logFileMaxCap": config.Config.MaxLogFileSize,
		"logDir":        config.Config.LogDir,
	})
	if err != nil {
		glog.Fatalf("Init log error: %v", err)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Runtime error caught: %v", r)
			log.Errorf("%s", debug.Stack())
		}

		log.Flush()
		log.Close()
	}()

	msg, exitCode := run()
	fmt.Println(msg)

	log.Flush()
	log.Close()
	os.Exit(exitCode)
}

func run() (string, int) {
	log.Infof("Enter san flexvolume for cmd: %v", os.Args)
	drv := &BlockVolumeDriver{}

	var cmdOpts types.CmdOptions
	var r types.Result

	switch os.Args[1] {
	case "init":
		r = drv.init()

	case "attach":
		json.Unmarshal([]byte(os.Args[2]), &cmdOpts)
		r = drv.attach(&cmdOpts)

	case "detach":
		r = drv.detach(os.Args[2])

	case "isattached":
		json.Unmarshal([]byte(os.Args[2]), &cmdOpts)
		r = drv.isAttached(&cmdOpts)

	case "waitforattach":
		r = drv.waitForAttach(os.Args[2])

	case "mountdevice":
		json.Unmarshal([]byte(os.Args[4]), &cmdOpts)
		r = drv.mountDevice(os.Args[2], os.Args[3], &cmdOpts)

	case "unmountdevice":
		r = drv.unmountDevice(os.Args[2])

	default:
		r = types.Result{
			Status: "Not supported",
		}
	}

	exitCode := 0
	if r.Status != "Success" {
		exitCode = 1
	}

	var msg string

	res, err := json.Marshal(r)
	if err != nil {
		msg = `{"status":"Failure","message":"JSON error"}`
		log.Errorln(msg)
		exitCode = 1
	} else {
		msg = string(res)
	}

	log.Infof("Cmd %v result: %s", os.Args, msg)
	return msg, exitCode
}
