package main

import (
	"dev"
	"encoding/json"
	"errors"
	"flexvolume/config"
	"flexvolume/types"
	"fmt"
	"storage/oceanstor/attacher"
	"storage/oceanstor/client"
	"strings"
	"utils"
	"utils/log"
)

const (
	HYPERMETROPAIR_RUNNING_STATUS_NORMAL = "1"
)

type BlockVolumeDriver struct {
	cli     *client.Client
	portals []string

	remoteCli     *client.Client
	remotePortals []string
}

func (d *BlockVolumeDriver) initBackend(backend string) error {
	backendConf, exist := config.Backends[backend]
	if !exist {
		msg := fmt.Sprintf("Backend %s is not configured", backend)
		log.Errorln(msg)
		return errors.New(msg)
	}

	cli := client.NewClient(backendConf.Urls, backendConf.User, backendConf.Password, backendConf.VstoreName)
	err := cli.Login()
	if err != nil {
		log.Errorf("Login backend %s error: %v", backend, err)
		return err
	}

	d.cli = cli
	d.portals, _ = backendConf.Options["portals"].([]string)

	return nil
}

func (d *BlockVolumeDriver) releaseBackend() {
	d.cli.Logout()
}

func (d *BlockVolumeDriver) initMetroRemoteBackend(backend string) error {
	backendConf, _ := config.Backends[backend]

	if backendConf.HyperMetroDomain != "" {
		for name, b := range config.Backends {
			if name != backend && b.HyperMetroDomain == backendConf.HyperMetroDomain {
				d.remoteCli = client.NewClient(b.Urls, b.User, b.Password, b.VstoreName)
				d.remotePortals, _ = b.Options["portals"].([]string)
			}
		}
	}

	if d.remoteCli == nil {
		msg := "No hypermetro remote backend exists"
		log.Errorln(msg)
		return errors.New(msg)
	}

	err := d.remoteCli.Login()
	if err != nil {
		log.Errorf("Login hypermetro remote backend error: %v", err)
		return err
	}

	return nil
}

func (d *BlockVolumeDriver) releaseMetroRemoteBackend() {
	d.remoteCli.Logout()
}

func (d *BlockVolumeDriver) init() types.Result {
	return types.Result{
		Status: "Success",
	}
}

func (d *BlockVolumeDriver) attachLun(lun map[string]interface{}, backend string) (string, error) {
	var lunWWN string
	var rss map[string]string

	rssStr := lun["HASRSSOBJECT"].(string)
	err := json.Unmarshal([]byte(rssStr), &rss)
	if err != nil {
		log.Errorf("Unmarshal RSS %s error: %v", rssStr, err)
		return "", err
	}

	lunName := lun["NAME"].(string)
	localAttacher := attacher.NewAttacher(d.cli, config.Config.Proto, "flexvolume", d.portals)

	if rss["HyperMetro"] == "TRUE" {
		localLunID := lun["ID"].(string)
		pair, err := d.cli.GetHyperMetroPairByLocalObjID(localLunID)
		if err != nil {
			log.Errorf("Get hypermetro pair by local obj ID %s error: %v", localLunID, err)
			return "", err
		}
		if pair == nil || pair["RUNNINGSTATUS"].(string) != HYPERMETROPAIR_RUNNING_STATUS_NORMAL {
			msg := "Hypermetro pair doesn't exist or status is not normal"
			log.Errorln(msg)
			return "", errors.New(msg)
		}

		err = d.initMetroRemoteBackend(backend)
		if err != nil {
			return "", err
		}

		defer func() {
			d.releaseMetroRemoteBackend()
		}()

		remoteAttacher := attacher.NewAttacher(d.remoteCli, config.Config.Proto, "flexvolume", d.remotePortals)
		metroAttacher := attacher.NewMetroAttacher(localAttacher, remoteAttacher)

		lunWWN, err = metroAttacher.ControllerAttach(lunName, nil)
		if err != nil {
			log.Errorf("Attach hypermetro volume %s error: %v", lunName, err)
			return "", err
		}
	} else {
		lunWWN, err = localAttacher.ControllerAttach(lunName, nil)
		if err != nil {
			log.Errorf("Attach volume %s error: %v", lunName, err)
			return "", err
		}
	}

	return lunWWN, nil
}

func (d *BlockVolumeDriver) attach(opts *types.CmdOptions) types.Result {
	var err error
	var lunName, lunWWN string
	var lun map[string]interface{}

	backend, volName := utils.GetBackendAndVolume(opts.VolumeName)
	err = d.initBackend(backend)
	if err != nil {
		log.Errorf("Connect backend %s error: %v", backend, err)
		goto ERROR
	}

	defer func() {
		d.releaseBackend()
	}()

	lunName = utils.GetLunName(volName)

	lun, err = d.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		goto ERROR
	}
	if lun == nil {
		msg := fmt.Sprintf("LUN %s doesn't exist for attach", lunName)
		log.Errorln(msg)
		err = errors.New(msg)
		goto ERROR
	}

	lunWWN, err = d.attachLun(lun, backend)
	if err != nil {
		goto ERROR
	}

	return types.Result{
		Status: "Success",
		Device: lunWWN,
	}

ERROR:
	return types.Result{
		Status:  "Failure",
		Message: err.Error(),
	}
}

func (d *BlockVolumeDriver) detachLun(lun map[string]interface{}, backend string) error {
	var rss map[string]string

	rssStr := lun["HASRSSOBJECT"].(string)
	err := json.Unmarshal([]byte(rssStr), &rss)
	if err != nil {
		log.Errorf("Unmarshal RSS %s error: %v", rssStr, err)
		return err
	}

	lunName := lun["NAME"].(string)
	localAttacher := attacher.NewAttacher(d.cli, config.Config.Proto, "flexvolume", nil)

	if rss["HyperMetro"] == "TRUE" {
		err := d.initMetroRemoteBackend(backend)
		if err != nil {
			return err
		}

		defer func() {
			d.releaseMetroRemoteBackend()
		}()

		remoteAttacher := attacher.NewAttacher(d.remoteCli, config.Config.Proto, "flexvolume", nil)
		metroAttacher := attacher.NewMetroAttacher(localAttacher, remoteAttacher)

		err = metroAttacher.NodeUnstage(lunName, nil)
		if err != nil {
			log.Errorf("Detach hypermetro volume %s error: %v", lunName, err)
			return err
		}
	} else {
		err := localAttacher.NodeUnstage(lunName, nil)
		if err != nil {
			log.Errorf("Detach volume %s error: %v", lunName, err)
			return err
		}
	}

	return nil
}

func (d *BlockVolumeDriver) detach(volumeName string) types.Result {
	var err error
	var lunName string
	var lun map[string]interface{}

	backend, volName := utils.GetBackendAndVolume(volumeName)
	err = d.initBackend(backend)
	if err != nil {
		log.Errorf("Connect backend %s error: %v", backend, err)
		goto ERROR
	}

	defer func() {
		d.releaseBackend()
	}()

	lunName = utils.GetLunName(volName)

	lun, err = d.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		goto ERROR
	}
	if lun == nil {
		log.Warningf("LUN %s doesn't exist for detach", lunName)
		goto SUCCESS
	}

	err = d.detachLun(lun, backend)
	if err != nil {
		goto ERROR
	}

SUCCESS:
	return types.Result{
		Status: "Success",
	}

ERROR:
	return types.Result{
		Status:  "Failure",
		Message: err.Error(),
	}
}

func (d *BlockVolumeDriver) isAttached(opts *types.CmdOptions) types.Result {
	var err error
	var lun map[string]interface{}
	var lunName, lunWWN, device string
	var attached bool

	backend, volName := utils.GetBackendAndVolume(opts.VolumeName)
	err = d.initBackend(backend)
	if err != nil {
		log.Errorf("Connect backend %s error: %v", backend, err)
		goto ERROR
	}

	defer func() {
		d.releaseBackend()
	}()

	lunName = utils.GetLunName(volName)

	lun, err = d.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get LUN %s info error: %v", lunName, err)
		goto ERROR
	}
	if lun == nil {
		log.Warningf("LUN %s doesn't exist for attached check", lunName)
		goto SUCCESS
	}

	lunWWN = lun["WWN"].(string)
	device, err = dev.GetDev(lunWWN)
	if err != nil {
		log.Errorf("Check device of WWN %s exist error: %v", lunWWN, err)
		goto ERROR
	}

	attached = device != ""

SUCCESS:
	return types.Result{
		Status:   "Success",
		Attached: attached,
	}

ERROR:
	return types.Result{
		Status:  "Failure",
		Message: err.Error(),
	}
}

func (d *BlockVolumeDriver) waitForAttach(lunWWN string) types.Result {
	device := dev.ScanDev(lunWWN, config.Config.Proto)
	if device == "" {
		msg := fmt.Sprintf("Device of WWN %s wasn't found", lunWWN)
		log.Errorln(msg)
		return types.Result{
			Status:  "Failure",
			Message: msg,
		}
	}

	return types.Result{
		Status: "Success",
		Device: lunWWN,
	}
}

func (d *BlockVolumeDriver) mountDevice(mountDir string, lunWWN string, opts *types.CmdOptions) types.Result {
	var fsType, devPath, output string

	device, err := dev.GetDev(lunWWN)
	if err != nil {
		log.Errorf("Get device of WWN %s error: %v", lunWWN, err)
		goto ERROR
	}

	fsType = opts.FsType
	if fsType == "" {
		fsType = "ext4"
	}

	devPath = fmt.Sprintf("/dev/%s", device)

	output, err = utils.ExecShellCmd(`blkid -o udev %s | grep "ID_FS_UUID" | cut -d "=" -f2`, devPath)
	if err != nil {
		log.Errorf("Query fs of %s error: %s", devPath, output)
		goto ERROR
	}
	if output == "" {
		output, err = utils.ExecShellCmd("mkfs -t %s -F %s", fsType, devPath)
		if err != nil {
			log.Errorf("Couldn't mkfs %s to type %s: %s", devPath, fsType, output)
			goto ERROR
		}
	}

	output, err = utils.ExecShellCmd("mount %s %s", devPath, mountDir)
	if err != nil {
		log.Errorf("Couldn't mount %s to %s: %s", devPath, mountDir, output)
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

func (d *BlockVolumeDriver) unmountDevice(mountDir string) types.Result {
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
