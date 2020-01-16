package main

import (
	"errors"
	"flexvolume/config"
	"flexvolume/types"
	"fmt"
	"strings"
	"utils"
	"utils/log"
)

type NfsVolumeDriver struct {
	portal string
}

func (d *NfsVolumeDriver) init() types.Result {
	return types.Result{
		Status: "Success",
		Capabilities: map[string]bool{
			"attach": false,
		},
	}
}

func (d *NfsVolumeDriver) update(backend string) error {
	backendConf, exist := config.Backends[backend]
	if !exist {
		msg := fmt.Sprintf("Backend %s is not configured", backend)
		log.Errorln(msg)
		return errors.New(msg)
	}

	d.portal = backendConf.Options["portal"].(string)
	return nil
}

func (d *NfsVolumeDriver) mount(mountDir string, opts *types.CmdOptions) types.Result {
	var err error
	var output string
	var exportPath string

	backend, fsName := utils.GetBackendAndVolume(opts.VolumeName)
	err = d.update(backend)
	if err != nil {
		log.Errorf("Connect backend %s error: %v", backend, err)
		goto ERROR
	}

	exportPath = d.portal + ":/" + utils.GetFileSystemName(fsName)

	output, err = utils.ExecShellCmd("mount %s %s", exportPath, mountDir)
	if err != nil {
		log.Errorf("Couldn't mount %s to %s: %s", exportPath, mountDir, output)
		goto ERROR
	}

	return types.Result{
		Status: "Success",
	}

ERROR:
	return types.Result{
		Status:  "Failure",
		Message: err.Error(),
	}
}

func (d *NfsVolumeDriver) unmount(mountDir string) types.Result {
	output, err := utils.ExecShellCmd("umount %s", mountDir)
	if err != nil && !strings.Contains(output, "not mounted") {
		log.Errorf("Couldn't umount %s: %s", mountDir, output)
		return types.Result{
			Status:  "Failure",
			Message: err.Error(),
		}
	}

	return types.Result{
		Status: "Success",
	}
}
