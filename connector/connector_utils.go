/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

// Package connector provide methods of interacting with the host
package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	// NotUseMultipath means the device does not use multipath service
	NotUseMultipath = iota
	// UseDMMultipath means the device use DM-Multipath service
	UseDMMultipath
	// UseUltraPath means the device use huawei-UltraPath service
	UseUltraPath
	// UseUltraPathNVMe means the device use huawei-UltraPath-NVMe service
	UseUltraPathNVMe
	// How many times to retry for a consistent read of /proc/mounts.
	maxListTries = 10
	// Location of the mount file to use
	procMountsPath = "/proc/mounts"

	extendDMBlockWaitTime   = 2 * time.Second
	watchDeviceInterval     = 100 * time.Millisecond
	volumeRemovalRetryTimes = 30

	devLineSplitSegment = 2
)

var (
	// DisconnectVolumeTimeOut defines the timeout period for disconnecting volume
	DisconnectVolumeTimeOut = time.Minute
	// DisconnectVolumeTimeInterval defines the time interval for checking if connected volume
	DisconnectVolumeTimeInterval = time.Second
)

type deviceInfo struct {
	lunWWN         string
	deviceName     string
	deviceFullName string
	multipathType  int
}

// DMDeviceInfo is the information of DM device
type DMDeviceInfo struct {
	// Name device name
	Name string
	// Name device file system
	Sysfs string
	// Devices device list
	Devices []string
}

func getDeviceLink(ctx context.Context, tgtLunGUID string) (string, error) {
	output, err := utils.ExecShellCmd(ctx, "ls -l /dev/disk/by-id/ | grep %s", tgtLunGUID)
	if err != nil {
		if strings.TrimSpace(output) == "" || strings.Contains(output, "No such file or directory") {
			return "", nil
		}

		return "", err
	}
	return output, nil
}

func getDevice(findDeviceMap map[string]string, deviceLink string) string {
	var dev string
	devLines := strings.Split(deviceLink, "\n")
	for _, line := range devLines {
		splits := strings.Split(line, "../../")
		if len(splits) >= devLineSplitSegment {
			name := splits[1]

			if strings.HasPrefix(name, "dm") {
				dev = name
				break
			}

			if _, exist := findDeviceMap[name]; !exist && strings.HasPrefix(name, "nvme") {
				dev = name
				break
			}

			if _, exist := findDeviceMap[name]; !exist && strings.HasPrefix(name, "sd") {
				dev = name
				break
			}
		}
	}
	return dev
}

func getDevices(deviceLink string) []string {
	var devices []string

	devLines := strings.Split(deviceLink, "\n")
	for _, line := range devLines {
		splits := strings.Split(line, "../../")
		if len(splits) < devLineSplitSegment || utils.IsContain(splits[1], devices) {
			continue
		}

		name := splits[1]
		if strings.HasPrefix(name, "sd") || strings.HasPrefix(name, "nvme") ||
			strings.HasPrefix(name, "dm") || strings.HasPrefix(name, "ultrapath") {
			devices = append(devices, name)
		}
	}

	return devices
}

func getDMDeviceByAlias(ctx context.Context, dm string) (string, error) {
	output, err := utils.ExecShellCmd(ctx, "ls -l /dev/mapper/ | grep -w %s", dm)
	if err != nil {
		return "", utils.Errorf(ctx, "Get DMDevice by alias: %s failed. error: %v", dm, err)
	}
	const mpathIndex int = 8
	for _, line := range strings.Split(output, "\n") {
		fieldLines := strings.Fields(line)
		if len(fieldLines) > mpathIndex && isMatch(fieldLines[mpathIndex], `^mpath`) {
			return fieldLines[mpathIndex], nil
		}
	}
	return "", utils.Errorf(ctx, "Can not get DMDevice by alias: %s", dm)
}

func isMatch(s, pattern string) bool {
	p := regexp.MustCompile(pattern)
	return p.FindStringSubmatch(s) != nil
}

func isEndWithDigital(s string) bool {
	return isMatch(s, `[\d]+$`)
}

// GetVirtualDevice used to get virtual device by WWN/GUID
var GetVirtualDevice = func(ctx context.Context, tgtLunGUID string) (string, int, error) {
	var virtualDevice string
	var deviceType int

	// Obtain the devices link that contain the WWN in /dev/disk/by-id/
	devices, err := GetDevicesByGUID(ctx, tgtLunGUID)
	if err != nil {
		return virtualDevice, 0, err
	}

	var virtualDevices []string
	var phyDevices []string
	for _, device := range devices {
		device = strings.TrimSpace(device)
		// check whether device is a partition device.
		partitionDev, err := isPartitionDevice(ctx, device)
		if err != nil {
			return "", 0, utils.Errorf(ctx, "check device: %s is a partition device failed. error: %v", device, err)
		} else if partitionDev {
			log.AddContext(ctx).Infof("Device: %s is a partition device，skip", device)
			continue
		}

		if strings.HasPrefix(device, "ultrapath") {
			deviceType = UseUltraPathNVMe
			virtualDevices = append(virtualDevices, device)
		} else if strings.HasPrefix(device, "dm") {
			deviceType = UseDMMultipath
			virtualDevices = append(virtualDevices, device)
		} else if strings.HasPrefix(device, "sd") && isUltraPathDevice(ctx, device) {
			deviceType = UseUltraPath
			virtualDevices = append(virtualDevices, device)
		} else if strings.HasPrefix(device, "sd") || strings.HasPrefix(device, "nvme") {
			phyDevices = append(phyDevices, device)
		} else {
			log.AddContext(ctx).Warningf("Unknown device link: %s", device)
		}
	}

	log.AddContext(ctx).Infof("Find virtual devices: %v, physical devices: %v", virtualDevices, phyDevices)

	return getVirtualDevice(ctx, virtualDevices, phyDevices, tgtLunGUID, deviceType)
}

func getVirtualDevice(ctx context.Context, virtualDevices []string, phyDevices []string, tgtLunGUID string,
	deviceType int) (string, int, error) {
	var virtualDevice string

	if len(virtualDevices) != 0 {
		if len(virtualDevices) > 1 {
			log.AddContext(ctx).Errorf("Virtual device with WWN/GUID:%s in the /dev/disk/by-id/"+
				" is not unique:%v", tgtLunGUID, virtualDevices)
			return "", 0, errors.New("virtual device not unique")
		}

		virtualDevice = virtualDevices[0]
	} else {
		if len(phyDevices) > 1 {
			log.AddContext(ctx).Errorf("Physical device with WWN:%s in the /dev/disk/by-id/ is not unique:%v",
				tgtLunGUID, phyDevices)
			return "", 0, errors.New("physical device not unique")
		} else if len(phyDevices) == 0 {
			log.AddContext(ctx).Warningf("No device find in /dev/disk/by-id/ with WWN:%s", tgtLunGUID)
			return "", 0, nil
		}

		deviceType = NotUseMultipath
		virtualDevice = phyDevices[0]
	}

	log.AddContext(ctx).Infof("Find virtual device: %s.", virtualDevice)

	return virtualDevice, deviceType, nil
}

func isUltraPathDevice(ctx context.Context, device string) bool {
	output, err := utils.ExecShellCmd(ctx, "upadmin show vlun | grep -w %s", device)
	if err != nil {
		return false
	}

	return strings.Contains(output, device)
}

// GetDevicesByGUID query device from host. If revert connect volume, no need to check device available
var GetDevicesByGUID = func(ctx context.Context, tgtLunGUID string) ([]string, error) {
	var devices []string
	deviceLink, err := getDeviceLink(ctx, tgtLunGUID)
	if err != nil {
		return devices, err
	}

	devices = getDevices(deviceLink)

	return devices, nil
}

func reScanNVMe(ctx context.Context, device string) error {
	var err error
	if match, err := regexp.MatchString(`nvme[0-9]+n[0-9]+`, device); err == nil && match {
		output, err := utils.ExecShellCmd(ctx, "echo 1 > /sys/block/%s/device/rescan_controller", device)
		if err != nil {
			log.AddContext(ctx).Warningf("rescan nvme path error: %s", output)
			return err
		}
	} else if match, err = regexp.MatchString(`nvme[0-9]+$`, device); err == nil && match {
		output, err := utils.ExecShellCmd(ctx, "nvme ns-rescan /dev/%s", device)
		if err != nil {
			log.AddContext(ctx).Warningf("rescan nvme path error: %s", output)
			return err
		}
	}

	if err != nil {
		log.AddContext(ctx).Warningf("pattern compile failed, err: %v", err)
		return err
	}

	log.AddContext(ctx).Warningf("device %s match failed, err: %v", device, err)
	return nil
}

// GetPhyDevicesFromDM used to get physical device from dm-multipath
func GetPhyDevicesFromDM(dm string) ([]string, error) {
	return getDeviceFromDM(dm)
}

var getDeviceFromDM = func(dm string) ([]string, error) {
	devPath := fmt.Sprintf("/sys/block/%s/slaves/*", dm)
	paths, err := filepath.Glob(devPath)
	if err != nil {
		return nil, err
	}

	var devices []string
	for _, path := range paths {
		_, dev := filepath.Split(path)
		devices = append(devices, dev)
	}
	return devices, nil
}

// DeleteSDDev is used to delete the sd device
func DeleteSDDev(ctx context.Context, sd string) error {
	output, err := utils.ExecShellCmd(ctx, "echo 1 > /sys/block/%s/device/delete", sd)
	if err != nil {
		if strings.Contains(output, "No such file or directory") {
			return nil
		}

		log.AddContext(ctx).Errorf("Delete SD device %s error: %v", sd, output)
		return err
	}
	return nil
}

// FlushDMDevice is the device of flush DM
var FlushDMDevice = func(ctx context.Context, dm string) error {
	// command awk can always return success, just check the output
	mPath, _ := utils.ExecShellCmd(ctx, "ls -l /dev/mapper/ | grep -w %s | awk '{print $9}'", dm)
	if mPath == "" {
		return fmt.Errorf("get DM device %s", dm)
	}

	var err error
	for i := 0; i < 3; i++ {
		_, err = utils.ExecShellCmd(ctx, "multipath -f %s", mPath)
		if err == nil {
			log.AddContext(ctx).Infof("Flush multipath device %s successful", mPath)
			break
		}
		log.AddContext(ctx).Warningf("Flush multipath device %s error: %v", mPath, err)
		time.Sleep(time.Second * flushMultiPathInternal)
	}

	return err
}

func flushDeviceIO(ctx context.Context, devPath string) error {
	output, err := utils.ExecShellCmd(ctx, "blockdev --flushbufs %s", devPath)
	if err != nil {
		if strings.Contains(output, "No such device") || strings.Contains(output, "No such file") {
			return nil
		}

		log.AddContext(ctx).Warningf("Failed to flush IO buffers prior to removing device %s", devPath)
	}

	return nil
}

func removeSCSIDevice(ctx context.Context, sd string) error {
	devPath := fmt.Sprintf("/dev/%s", sd)
	err := flushDeviceIO(ctx, devPath)
	if err != nil {
		log.AddContext(ctx).Errorf("Flush %s error: %v", devPath, err)
		return err
	}

	err = DeleteSDDev(ctx, sd)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete device [%s] failed, error: %v", sd, err)
		return err
	}

	waitVolumeRemoval(ctx, []string{devPath})
	return nil
}

func waitVolumeRemoval(ctx context.Context, devPaths []string) {
	existPath := devPaths
	for index := 0; index <= volumeRemovalRetryTimes; index++ {
		var exist []string
		for _, dev := range existPath {
			_, err := os.Stat(dev)
			if err != nil && os.IsNotExist(err) {
				log.AddContext(ctx).Infof("The dev %s has been deleted", dev)
			} else {
				exist = append(exist, dev)
			}
		}

		existPath = exist
		if len(existPath) == 0 {
			return
		}

		if index < volumeRemovalRetryTimes {
			time.Sleep(time.Second)
		}
	}

	return
}

func removeSymlinks(devices []string, realPath, link string) error {
	for _, dev := range devices {
		if dev == realPath {
			if err := os.Remove(link); err != nil {
				return fmt.Errorf("failed to unlink: %+v", err)
			}
		}
	}

	return nil
}

func removeSCSISymlinks(devices []string) error {
	links, err := filepath.Glob("/dev/disk/by-id/scsi-*")
	if err != nil {
		return err
	}

	for _, link := range links {
		if _, err := os.Lstat(link); os.IsNotExist(err) {
			return nil
		}

		realPath, err := os.Readlink(link)
		if err != nil {
			return err
		}

		err = removeSymlinks(devices, realPath, link)
		if err != nil {
			return err
		}
	}

	return nil
}

func getSessionIdByDevice(devPath string) (string, error) {
	dev := fmt.Sprintf("/sys/block/%s", devPath)
	realPath, err := os.Readlink(dev)
	if err != nil {
		return "", err
	}

	file := strings.Split(realPath, "/session")
	if len(file) == 0 {
		return "", nil
	}

	return strings.Split(file[1], "/")[0], nil
}

// WatchDMDevice is an aggregate drive letter monitor.
func WatchDMDevice(ctx context.Context, lunWWN string, expectPathNumber int) (DMDeviceInfo, error) {
	log.AddContext(ctx).Infof("Watch DM Disk Generation. lunWWN: %s,expectPathNumber: %d", lunWWN, expectPathNumber)
	var timeout = time.After(time.Second * time.Duration(app.GetGlobalConfig().ScanVolumeTimeout))
	var dm DMDeviceInfo
	var err = errors.New(VolumeNotFound)
	for {
		select {
		case <-timeout:
			return dm, err
		default:
			time.Sleep(watchDeviceInterval)
		}

		dm, err = findDMDeviceByWWN(ctx, lunWWN)
		if err == nil {
			if !app.GetGlobalConfig().AllPathOnline || len(dm.Devices) == expectPathNumber {
				return dm, nil
			}
			log.AddContext(ctx).Warningf("Querying DM Disk Path Information. "+
				"lunWWN: %s, Sysfs: %s, Devices:%v, expectPathNumber:%d", lunWWN, dm.Sysfs, dm.Devices, expectPathNumber)
			err = errors.New(VolumePathIncomplete)
		} else {
			log.AddContext(ctx).Warningf("Failed to query the DM disk. lunWWN: %s error: %v", lunWWN, err)
		}
	}
}

func findDMDeviceByWWN(ctx context.Context, lunWWN string) (dm DMDeviceInfo, err error) {
	var output string
	output, err = utils.ExecShellCmd(ctx, "multipathd show maps")
	if err != nil {
		err = fmt.Errorf("failed to query the multipathing information. error: %s", err)
		return
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasSuffix(line, lunWWN) {
			return getDMDeviceInfo(line)
		}
	}

	err = errors.New(VolumeNotFound)
	return
}

func getDMDeviceInfo(line string) (dm DMDeviceInfo, err error) {
	const colWidth = 3
	column := strings.Fields(line)
	if len(column) != colWidth {
		err = fmt.Errorf("the returned data is incorrect when the DM disk information is queried. line: %s", line)
		return
	}

	dm.Name = column[0]
	dm.Sysfs = column[1]
	dm.Devices, err = getDeviceFromDM(dm.Sysfs)
	return dm, err
}

// FindAvailableMultiPath is to get dm-multipath through sd devices
func FindAvailableMultiPath(ctx context.Context, foundDevices []string) (string, bool) {
	log.AddContext(ctx).Infof("Start to find the dm multipath of devices %v", foundDevices)
	mPathMap, mPath := findMultiPathMaps(foundDevices)
	if len(mPathMap) == 1 {
		return mPath, false
	}

	if len(mPathMap) == 0 {
		return "", false
	}

	for dmPath, devices := range mPathMap {
		log.AddContext(ctx).Infof("Start to clean up the multipath [%s] with devices %s", dmPath, devices)
		if _, err := removeMultiPathDevice(ctx, dmPath, devices); err != nil {
			log.AddContext(ctx).Errorf("clear multipath [%s] and devices %v error %v", dmPath, devices, err)
		}
	}
	return "", true
}

func findMultiPathMaps(foundDevices []string) (map[string][]string, string) {
	mPathMap := make(map[string][]string)
	var mPath string
	for _, device := range foundDevices {
		dmPath := fmt.Sprintf("/sys/block/%s/holders/dm-*", device)
		paths, err := filepath.Glob(dmPath)
		if err != nil || paths == nil {
			continue
		}
		splitPath := strings.Split(paths[0], "/")
		mPath = splitPath[len(splitPath)-1]
		mPathMap[mPath] = append(mPathMap[mPath], device)
	}
	return mPathMap, mPath
}

func getSCSIWwnByScsiID(ctx context.Context, hostDevice string) (string, error) {
	priorityCmd := fmt.Sprintf("/usr/lib/udev/scsi_id --page 0x83 --whitelisted %s", hostDevice)
	output, err := utils.ExecShellCmd(ctx, priorityCmd)
	if err != nil {
		// /bin/sh echo "not found"
		// /bin/bash echo "no such file or directory"
		lowerOutput := strings.ToLower(output)
		if strings.Contains(lowerOutput, "no such file or directory") ||
			strings.Contains(lowerOutput, "not found") {
			alternateCmd := fmt.Sprintf("/lib/udev/scsi_id --page 0x83 --whitelisted %s", hostDevice)
			output, err = utils.ExecShellCmd(ctx, alternateCmd)
		}
		if err != nil {
			return "", utils.Errorf(ctx, "Failed to get scsi id of device %s, err is %v", hostDevice, err)
		}
	}
	return strings.TrimSpace(output), nil
}

func getScsiHostWWid(ctx context.Context, devInfo map[string]string) (string, error) {
	wwIDFile := fmt.Sprintf("/sys/class/scsi_host/host%s/device/session*/target%s:%s:%s/%s:%s:%s:%s/wwid",
		devInfo["host"], devInfo["host"], devInfo["channel"], devInfo["id"], devInfo["host"],
		devInfo["channel"], devInfo["id"], devInfo["lun"])
	output, err := utils.ExecShellCmd(ctx, "cat %s", wwIDFile)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(output, "\n"), nil
}

func getFCHostWWid(ctx context.Context, devInfo map[string]string) (string, error) {
	wwIDFile := fmt.Sprintf("/sys/class/fc_host/host%s/device/rport-%s:%s-%s/target%s:%s:%s/%s:%s:%s:%s/wwid",
		devInfo["host"], devInfo["host"], devInfo["channel"], devInfo["id"],
		devInfo["host"], devInfo["channel"], devInfo["id"],
		devInfo["host"], devInfo["channel"], devInfo["id"], devInfo["lun"])
	output, err := utils.ExecShellCmd(ctx, "cat %s", wwIDFile)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(output, "\n"), nil
}

func getSCSIWwnByWWid(ctx context.Context, hostDevice string) (string, error) {
	devInfo := getDeviceInfo(ctx, strings.Split(hostDevice, "dev/")[1])
	if devInfo == nil {
		return "", utils.Errorln(ctx, "can not get device info")
	}

	var data string
	var err error
	data, err = getScsiHostWWid(ctx, devInfo)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			data, err = getFCHostWWid(ctx, devInfo)
		}

		if err != nil {
			return "", utils.Errorf(ctx, "get wwid from host failed, err: %v", err)
		}
	}

	if !strings.HasPrefix(data, "naa.") {
		return "", errors.New("unRecognized device type")
	}

	if len(data) <= deviceWWidLength {
		return "", utils.Errorf(ctx, "get wwid for device %s failed", hostDevice)
	}
	return data[deviceWWidLength:], nil
}

// GetSCSIWwn to get the device wwn
var GetSCSIWwn = func(ctx context.Context, hostDevice string) (string, error) {
	var wwn string
	var err error
	readable, err := IsDeviceReadable(ctx, hostDevice)
	if readable && err == nil {
		wwn, err = getSCSIWwnByScsiID(ctx, hostDevice)
		if err != nil {
			log.AddContext(ctx).Warningf("get device %s wwn by scsi_id error: %v", hostDevice, err)
		}
	} else {
		if strings.HasPrefix(hostDevice, "/dev/sd") {
			wwn, err = getSCSIWwnByWWid(ctx, hostDevice)
		}
	}

	return wwn, err
}

// GetNVMeWwn get the unique id of the device
var GetNVMeWwn = func(ctx context.Context, device string) (string, error) {
	cmd := fmt.Sprintf("nvme id-ns %s -o json", device)
	output, err := utils.ExecShellCmdFilterLog(ctx, cmd)
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to get nvme id of device %s, err is %v", device, err)
		return "", err
	}

	var deviceInfo map[string]interface{}
	if err = json.Unmarshal([]byte(output), &deviceInfo); err != nil {
		log.AddContext(ctx).Errorf("Failed to unmarshal input %s", output)
		return "", errors.New("failed to unmarshal device info")
	}

	if uuid, exist := deviceInfo["nguid"]; exist {
		return uuid.(string), nil
	}

	return "", errors.New("there is no nguid in device info")
}

// ReadDevice is to check whether the device is readable
var ReadDevice = func(ctx context.Context, dev string) ([]byte, error) {
	log.AddContext(ctx).Infof("Checking to see if %s is readable.", dev)
	out, err := utils.ExecShellCmdFilterLog(ctx, "dd if=%s bs=1024 count=512 status=none", dev)
	if err != nil {
		return nil, err
	}

	output := []byte(out)
	if len(output) != halfMiDataLength {
		return nil, utils.Errorf(ctx, "can not read 512KiB bytes from the device %s, instead read %d bytes",
			dev, len(output))
	}

	if strings.Contains(out, "0+0 records in") {
		return nil, utils.Errorf(ctx, "the size of %s may be zero, it is abnormal device", dev)
	}

	return output, nil
}

// IsDeviceFormatted reads 2MiBs of the device to check the device formatted or not
func IsDeviceFormatted(ctx context.Context, dev string) (bool, error) {
	output, err := ReadDevice(ctx, dev)
	if err != nil {
		return false, err
	}

	// check data is all zero
	if outWithoutZeros := bytes.Trim(output, "\x00"); len(outWithoutZeros) != 0 {
		log.AddContext(ctx).Infof("Device %s is already formatted", dev)
		return true, nil
	}
	log.AddContext(ctx).Infof("Device %s is not formatted", dev)
	return false, nil
}

func removeDevices(ctx context.Context, devices []string) error {
	for _, dev := range devices {
		err := removeSCSIDevice(ctx, dev)
		if err != nil {
			return err
		}
	}
	return nil
}

func removeMultiPathDevice(ctx context.Context, multiPathName string, devices []string) (string, error) {
	err := FlushDMDevice(ctx, multiPathName)
	if err == nil {
		multiPathName = ""
	}

	if err = removeDevices(ctx, devices); err != nil {
		return "", err
	}

	waitVolumeRemoval(ctx, devices)
	err = removeSCSISymlinks(devices)
	if err != nil {
		return "", err
	}
	return multiPathName, nil
}

// RemoveDevice is used to remove specified device
func RemoveDevice(ctx context.Context, device string) (string, error) {
	var multiPathName string
	var err error

	if strings.HasPrefix(device, "dm") {
		devices, err := getDeviceFromDM(device)
		if err != nil {
			log.AddContext(ctx).Errorf("RemoveDevice.getDeviceFromDM failed, device: %s, err: %v", device, err)
			return "", err
		}
		multiPathName, err = removeMultiPathDevice(ctx, device, devices)
	} else if strings.HasPrefix(device, "sd") {
		err = removeSCSIDevice(ctx, device)
	} else {
		log.AddContext(ctx).Warningf("Device %s to delete does not exist anymore", device)
	}

	if err != nil {
		return "", err
	}
	return multiPathName, nil
}

// ResizeBlock  Resize a block device by using the LUN WWN
func ResizeBlock(ctx context.Context, tgtLunWWN string, requiredBytes int64) error {
	virtualDevice, devType, err := GetVirtualDevice(ctx, tgtLunWWN)
	if err != nil {
		return err
	}

	if virtualDevice == "" {
		return utils.Errorf(ctx, "Can not find the device for lun %s", tgtLunWWN)
	}

	showDeviceSize(ctx, virtualDevice)

	err = rescanDevice(ctx, virtualDevice, devType)
	if err != nil {
		return err
	}

	return utils.WaitUntil(func() (bool, error) {
		curSize := showDeviceSize(ctx, virtualDevice)
		if curSize != "" && strconv.FormatInt(requiredBytes, constants.DefaultIntBase) == curSize {
			return true, nil
		}
		return false, nil
	}, time.Second*expandVolumeTimeOut, time.Second*expandVolumeInternal)
}

func rescanUseDMMultipath(ctx context.Context, virtualDevice string) error {
	subDevices, err := getDeviceFromDM(virtualDevice)
	if err != nil {
		log.AddContext(ctx).Errorf("Get device from multiPath %s error: %v", virtualDevice, err)
		return err
	}
	err = extendBlock(ctx, subDevices)
	if err != nil {
		return err
	}

	err = extendDMBlock(ctx, virtualDevice)
	if err != nil {
		return err
	}

	return nil
}

func rescanUseUltraPath(ctx context.Context, device string) error {
	err := rescanUpVirtualDevice(ctx, device)
	if err != nil {
		return err
	}

	err = rescanUpPhyDevice(ctx, device)
	if err != nil {
		return err
	}

	return nil
}

func rescanUpVirtualDevice(ctx context.Context, device string) error {
	_, err := utils.ExecShellCmd(ctx, "echo 1 > /sys/block/%s/device/rescan", device)
	if err != nil {
		log.AddContext(ctx).Errorf("rescan device: %s failed. error: %v", device, err)
		return err
	}

	return nil
}

func rescanSCSIDevices(ctx context.Context, subDevices []string) error {
	for _, subDevice := range subDevices {
		_, err := utils.ExecShellCmd(ctx, "echo 1 > /sys/class/scsi_device/%s/device/rescan", subDevice)
		if err != nil {
			return utils.Errorf(ctx, "rescan device: %s failed. error: %v", subDevice, err)
		}
	}
	return nil
}

func rescanUpPhyDevice(ctx context.Context, virtualDevice string) error {
	vlunID, err := getVLunIDByDeviceName(ctx, virtualDevice, UseUltraPath)
	if err != nil {
		return err
	}

	subDevices, err := getHCTLByVlunID(ctx, vlunID)
	if err != nil {
		return err
	}

	err = rescanSCSIDevices(ctx, subDevices)
	if err != nil {
		return utils.Errorf(ctx, "rescan device %s failed. error: %v", virtualDevice, err)
	}

	return nil
}

func getVLunIDByDeviceName(ctx context.Context, device string, devType int) (string, error) {
	var output string
	var err error

	switch devType {
	case UseUltraPath:
		output, err = utils.ExecShellCmd(ctx, "upadmin show vlun | grep -w %s", device)
	case UseUltraPathNVMe:
		output, err = utils.ExecShellCmd(ctx, "upadmin_plus show vlun | grep -w %s", device)
	default:
		log.AddContext(ctx).Errorf("get vlun ID failed, invalid devType:%d", devType)
		return "", errors.New("get vlun id failed")
	}

	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(output, "\n") {
		patternFormat := fmt.Sprintf(`^[\s]+([\d]+)[\s]+%s`, device)
		pattern := regexp.MustCompile(patternFormat)
		ret := pattern.FindAllStringSubmatch(line, -1)
		if ret != nil {
			return ret[0][1], nil
		}
	}

	return "", errors.New("get vlun id failed")
}

func getHCTLByVlunID(ctx context.Context, vlunID string) ([]string, error) {
	var subDevices []string

	output, err := utils.ExecShellCmd(ctx, "upadmin show vlun id=%s | grep Path", vlunID)
	if err != nil {
		return subDevices, err
	}

	for _, line := range strings.Split(output, "\n") {
		pattern := regexp.MustCompile(`^Path [\d]+ \[([\d:]+)\]`)
		ret := pattern.FindAllStringSubmatch(line, -1)
		if ret != nil {
			subDevices = append(subDevices, ret[0][1])
		}
	}

	return subDevices, nil
}

func rescanUseUltraPathNVMe(ctx context.Context, device string) error {
	output, err := GetUltraPathDetailsByPath(ctx, UltraPathNVMeCommand, device)
	if err != nil {
		return utils.Errorf(ctx, "get ultraPath %s detail info failed", device)
	}

	phyPaths, err := getFieldFromUltraPathInfo(output, "Path")
	if err != nil {
		return utils.Errorf(ctx, "get ultraPath %s detail info failed", device)
	}

	physicalDevices := getNVMePhysicalDevices(phyPaths)
	err = extendBlock(ctx, physicalDevices)
	if err != nil {
		return err
	}

	return nil
}

func rescanDevice(ctx context.Context, virtualDevice string, devType int) error {
	var err error

	switch devType {
	case NotUseMultipath:
		err = rescanNotUseMultipath(ctx, virtualDevice)
	case UseDMMultipath:
		err = rescanUseDMMultipath(ctx, virtualDevice)
	case UseUltraPath:
		err = rescanUseUltraPath(ctx, virtualDevice)
	case UseUltraPathNVMe:
		err = rescanUseUltraPathNVMe(ctx, virtualDevice)
	default:
		log.AddContext(ctx).Errorln("Invalid device type.")
		return errors.New("invalid device type")
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Extend block %s failed, error info: %v", virtualDevice, err)
		return err
	}

	return nil
}

func rescanNotUseMultipath(ctx context.Context, virtualDevice string) error {
	err := extendBlock(ctx, []string{virtualDevice})
	if err != nil {
		log.AddContext(ctx).Errorf("Extend block failed, device %s, error: %v", virtualDevice, err)
		return err
	}

	return nil
}

func showDeviceSize(ctx context.Context, virtualDevice string) string {
	output, err := getDeviceSize(ctx, virtualDevice)
	if err == nil {
		log.AddContext(ctx).Infof("Device %s size is %s", virtualDevice, output)
		return output
	}

	log.AddContext(ctx).Warningf("Get device: %s size failed. error: %v", virtualDevice, err)
	return ""
}

func getDeviceInfo(ctx context.Context, dev string) map[string]string {
	device := "/dev/" + dev
	output, err := utils.ExecShellCmd(ctx, "lsblk -n -S %s -o HCTL", device)
	if err != nil {
		log.AddContext(ctx).Warningf("Failed to get device %s hctl", device)
		return nil
	}

	devLines := strings.Split(output, "\n")
	for _, d := range devLines {
		devString := strings.TrimSpace(d)
		hostChannelInfo := strings.Split(devString, ":")
		if len(hostChannelInfo) != HCTLLength {
			continue
		}

		devInfo := map[string]string{
			"device":  device,
			"host":    hostChannelInfo[0],
			"channel": hostChannelInfo[1],
			"id":      hostChannelInfo[2],
			"lun":     hostChannelInfo[3],
		}
		return devInfo
	}
	return nil
}

func getDeviceSize(ctx context.Context, dev string) (string, error) {
	device := "/dev/" + dev
	output, err := utils.ExecShellCmd(ctx, "blockdev --getsize64 %s", device)
	return strings.TrimSpace(output), err
}

func extendBlock(ctx context.Context, devices []string) error {
	var err error
	for _, dev := range devices {
		if strings.HasPrefix(dev, "sd") {
			err = extendSCSIBlock(ctx, dev)
		} else if strings.HasPrefix(dev, "nvme") {
			err = extendNVMeBlock(ctx, dev)
		}
	}
	return err
}

func multiPathReconfigure(ctx context.Context) {
	output, err := utils.ExecShellCmd(ctx, "multipathd reconfigure")
	if err != nil {
		log.AddContext(ctx).Warningf("Run multipathd reconfigure err. Output: %s, err: %v", output, err)
	}
}

func multiPathResizeMap(ctx context.Context, device string) (string, error) {
	cmd := fmt.Sprintf("multipathd resize map %s", device)
	output, err := utils.ExecShellCmd(ctx, cmd)
	return output, err
}

func extendDMBlock(ctx context.Context, device string) error {
	multiPathReconfigure(ctx)
	oldSize, err := getDeviceSize(ctx, device)
	if err != nil {
		return err
	}
	log.AddContext(ctx).Infof("Original size of block %s is %s", device, oldSize)

	time.Sleep(extendDMBlockWaitTime)
	result, err := multiPathResizeMap(ctx, device)
	if err != nil || strings.Contains(result, "fail") {
		msg := fmt.Sprintf("Resize device %s err, output: %s, err: %v", device, result, err)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	newSize, err := getDeviceSize(ctx, device)
	if err != nil {
		return err
	}
	log.AddContext(ctx).Infof("After scsi device rescan, new size is %s", newSize)
	return nil
}

func extendSCSIBlock(ctx context.Context, device string) error {
	devInfo := getDeviceInfo(ctx, device)
	if devInfo == nil {
		return errors.New("can not get device info")
	}

	oldSize, err := getDeviceSize(ctx, device)
	if err != nil {
		return err
	}
	log.AddContext(ctx).Infof("Original size of block %s is %s", device, oldSize)

	_, err = utils.ExecShellCmd(ctx, "echo 1 > /sys/bus/scsi/drivers/sd/%s:%s:%s:%s/rescan",
		devInfo["host"], devInfo["channel"], devInfo["id"], devInfo["lun"])
	if err != nil {
		return err
	}

	newSize, err := getDeviceSize(ctx, device)
	if err != nil {
		return err
	}
	log.AddContext(ctx).Infof("After scsi device rescan, new size is %s", newSize)
	return nil
}

func extendNVMeBlock(ctx context.Context, device string) error {
	return reScanNVMe(ctx, device)
}

// GetFsTypeByDevPath use blkid to get fsType
func GetFsTypeByDevPath(ctx context.Context, devicePath string) (string, error) {
	fsType, err := utils.ExecShellCmd(ctx, "blkid -p -s TYPE -o value %s", devicePath)
	if err != nil {
		log.AddContext(ctx).Warningf("blkid %s error: %v", devicePath, err)
		return "", err
	}

	return strings.Trim(fsType, "\n"), nil
}

// ResizeMountPath  Resize the mount point by using the volume path
var ResizeMountPath = func(ctx context.Context, volumePath string) error {
	output, err := utils.ExecShellCmd(ctx, "findmnt -o source --noheadings --target %s", volumePath)
	if err != nil {
		return fmt.Errorf("findmnt volumePath: %s error: %v", volumePath, err)
	}

	devicePath := strings.TrimSpace(output)
	if len(devicePath) == 0 {
		return fmt.Errorf("could not get valid device for mount path: %s", volumePath)
	}

	fsType, err := GetFsTypeByDevPath(ctx, devicePath)
	if err != nil {
		return err
	}

	if fsType == "" {
		return nil
	}

	switch fsType {
	case "ext2", "ext3", "ext4":
		return extResize(ctx, devicePath)
	case "xfs":
		return xfsResize(ctx, volumePath)
	default:
		return fmt.Errorf("resize of format %s is not supported for device %s", fsType, devicePath)
	}
}

func extResize(ctx context.Context, devicePath string) error {
	output, err := utils.ExecShellCmd(ctx, "resize2fs -p %s", devicePath)
	if err != nil {
		log.AddContext(ctx).Errorf("Resize %s error: %s", devicePath, output)
		return err
	}

	log.AddContext(ctx).Infof("Resize success for device path : %v", devicePath)
	return nil
}

func xfsResize(ctx context.Context, volumePath string) error {
	output, err := utils.ExecShellCmd(ctx, "xfs_growfs %s", volumePath)
	if err != nil {
		log.AddContext(ctx).Errorf("Resize %s error: %s", volumePath, output)
		return err
	}

	log.AddContext(ctx).Infof("Resize success for mount point: %v", volumePath)
	return nil
}

func findMultiPathWWN(ctx context.Context, mPath string) (string, error) {
	output, err := utils.ExecShellCmd(ctx, "multipathd show maps")
	if err != nil {
		log.AddContext(ctx).Errorf("Show multipath %s error: %s", mPath, output)
		return "", err
	}

	const pathMapsLen = 3
	for _, out := range strings.Split(output, "\n") {
		pathMaps := strings.Fields(out)
		if len(pathMaps) == pathMapsLen && pathMaps[1] == mPath {
			return pathMaps[2], nil
		}
	}

	return "", utils.Errorf(ctx, "Path %s not exist in multipath map", mPath)
}

// Input devices: [sda, sdb, sdc]
func findDeviceWWN(ctx context.Context, devices []string) (string, error) {
	var findWWN, devWWN string
	var err error
	for _, d := range devices {
		dev := fmt.Sprintf("/dev/%s", d)
		if strings.HasPrefix(d, "sd") {
			devWWN, err = GetSCSIWwn(ctx, dev)
		} else if strings.HasPrefix(d, "nvme") {
			devWWN, err = GetNVMeWwn(ctx, dev)
		}

		if err != nil {
			log.AddContext(ctx).Warningf("get device %s wwn failed, error: %v", dev, err)
			continue
		}

		if findWWN != "" && !(strings.Contains(devWWN, findWWN) ||
			strings.Contains(findWWN, devWWN)) {
			return "", errors.New("InconsistentWWN")
		}
		findWWN = devWWN
	}

	log.AddContext(ctx).Infof("find the wwn %s for devices %v", findWWN, devices)
	return findWWN, nil
}

func clearFaultyDevices(ctx context.Context, devices []string) ([]string, error) {
	var normalDevices []string
	for _, d := range devices {
		dev := fmt.Sprintf("/dev/%s", d)
		readable, err := IsDeviceReadable(ctx, dev)
		if readable && err == nil {
			normalDevices = append(normalDevices, d)
			continue
		}

		err = removeSCSIDevice(ctx, d)
		if err != nil {
			return nil, err
		}
	}
	return normalDevices, nil
}

// IsMultiPathAvailable compares the dm device WWN with the lun WWN
func IsMultiPathAvailable(ctx context.Context, mPath, lunWWN string, devices []string) (bool, error) {
	mPathWWN, err := findMultiPathWWN(ctx, mPath)
	if err != nil {
		return false, err
	}

	if !strings.Contains(mPathWWN, lunWWN) {
		return false, utils.Errorf(ctx, "the multipath device WWN %s is not equal to lun WWN %s",
			mPathWWN, lunWWN)
	}

	deviceWWN, err := findDeviceWWN(ctx, devices)
	if err != nil {
		return false, err
	}

	// false means unavailable when scan device, nil means when delete device without check
	if deviceWWN == "" {
		return false, nil
	}

	if !strings.Contains(deviceWWN, lunWWN) {
		return false, errors.New("the device WWN is not equal to lun WWN")
	}

	return true, nil
}

// IsDeviceAvailable compares the sd device WWN with the lun WWN
var IsDeviceAvailable = func(ctx context.Context, device, lunWWN string) (bool, error) {
	var devWWN string
	var err error
	if strings.Contains(device, "sd") {
		devWWN, err = GetSCSIWwn(ctx, device)
	} else if strings.Contains(device, "nvme") {
		devWWN, err = GetNVMeWwn(ctx, device)
	} else {
		// scsi mode, the device is /dev/disk/by-id/wwn-<id>,
		devWWN, err = GetSCSIWwn(ctx, device)
	}

	if err != nil {
		return false, err
	}

	if devWWN == "" {
		return false, nil
	}

	if !strings.Contains(devWWN, lunWWN) {
		return false, errors.New("the device WWN is not equal to lun WWN")
	}
	return true, nil
}

// DisConnectVolume delete all devices which match to lunWWN
func DisConnectVolume(ctx context.Context, tgtLunWWN string, f func(context.Context, string) error) error {
	return utils.WaitUntil(func() (bool, error) {
		err := f(ctx, tgtLunWWN)
		if err != nil {
			if err.Error() == "FindNoDevice" {
				return true, nil
			}
			return false, err
		}
		return false, nil
	}, DisconnectVolumeTimeOut, DisconnectVolumeTimeInterval)
}

// CheckConnectSuccess is to check the sd device available
func CheckConnectSuccess(ctx context.Context, device, tgtLunWWN string) bool {
	devPath := fmt.Sprintf("/dev/%s", device)
	if readable, err := IsDeviceReadable(ctx, devPath); !readable || err != nil {
		return false
	}

	available, err := IsDeviceAvailable(ctx, devPath, tgtLunWWN)
	if err != nil {
		return false
	}
	return available
}

// ClearUnavailableDevice is to check the sd device connect success, otherwise delete the device
func ClearUnavailableDevice(ctx context.Context, device, lunWWN string) string {
	if !CheckConnectSuccess(ctx, device, lunWWN) {
		if err := DeleteSDDev(ctx, device); err != nil {
			log.Warningf("clear device %s for lun %s error: %v", device, lunWWN, err)
		}
		device = ""
	}
	return device
}

// VerifySingleDevice check the sd device whether available
var VerifySingleDevice = func(ctx context.Context,
	device, lunWWN, errCode string,
	f func(context.Context, string) error) error {
	log.AddContext(ctx).Infof("Found the dev %s", device)
	_, err := ReadDevice(ctx, device)
	if err != nil {
		return err
	}

	available, err := IsDeviceAvailable(ctx, device, lunWWN)
	if err != nil && err.Error() != "the device WWN is not equal to lun WWN" {
		return err
	}

	if !available {
		err = f(ctx, lunWWN)
		if err != nil {
			log.AddContext(ctx).Errorf("delete device err while revert connect volume. Err is: %v", err)
		}
		return errors.New(errCode)
	}
	return nil
}

// VerifyDeviceAvailableOfDM used to check whether the DM device is available
func VerifyDeviceAvailableOfDM(ctx context.Context, tgtLunWWN string, expectPathNumber int,
	foundDevices []string,
	f func(context.Context, string) error) (string, error) {

	start := time.Now()
	dm, err := WatchDMDevice(ctx, tgtLunWWN, expectPathNumber)
	log.AddContext(ctx).Infof("WatchDMDevice-%s:%-36s%-8d%-20s%v",
		time.Second*time.Duration(app.GetGlobalConfig().ScanVolumeTimeout),
		tgtLunWWN, expectPathNumber, time.Now().Sub(start), err)
	if err == nil {
		var dev string
		dev, err = VerifyMultiPathDevice(ctx, dm.Sysfs, tgtLunWWN, VolumeDeviceNotFound, f)
		if err != nil {
			return "", utils.Errorf(ctx, "failed to execute connector.VerifyMultiPathDevice. %v", err)
		}
		return dev, nil
	}

	if err.Error() == VolumePathIncomplete {
		_, rmErr := removeMultiPathDevice(ctx, dm.Sysfs, dm.Devices)
		if rmErr != nil {
			log.AddContext(ctx).Warningf("Failed to clear the DM disk. "+
				"Sysfs:%s , devs: %v ,error:%v", dm.Sysfs, dm.Devices, rmErr)
		}
		return "", err
	}

	// No DM disk is found. Delete the corresponding SD disk. Otherwise, residual physical drive letters may occur.
	log.AddContext(ctx).Infof("Start to clean up the devices %s", foundDevices)
	if rmErr := removeDevices(ctx, foundDevices); rmErr != nil {
		log.AddContext(ctx).Errorf("clear devices %v error %v", foundDevices, rmErr)
	}
	return "", err
}

// RemoveDevices is used to remove devices
func RemoveDevices(ctx context.Context, devices []string) error {
	return removeDevices(ctx, devices)
}

// VerifyMultiPathDevice check the dm device whether available
func VerifyMultiPathDevice(ctx context.Context,
	mPath, lunWWN, errCode string,
	f func(context.Context, string) error) (string, error) {
	log.AddContext(ctx).Infof("Found the dm path %s", mPath)
	device := fmt.Sprintf("/dev/%s", mPath)
	_, err := ReadDevice(ctx, device)
	if err != nil {
		return "", err
	}

	devs, err := getDeviceFromDM(mPath)
	if err != nil {
		return "", err
	}

	devices, err := clearFaultyDevices(ctx, devs)
	if err != nil {
		return "", err
	}

	available, err := IsMultiPathAvailable(ctx, mPath, lunWWN, devices)
	if err != nil && err.Error() == "InconsistentWWN" {
		return "", err
	}

	if !available {
		err = f(ctx, lunWWN)
		if err != nil {
			log.AddContext(ctx).Errorf("delete device err while revert connect volume. Err is: %v", err)
		}
		return "", errors.New(errCode)
	}
	return device, nil
}

// GetDeviceSize to get the device size in bytes
var GetDeviceSize = func(ctx context.Context, hostDevice string) (int64, error) {
	// hostDevice is the symbol, such as /dev/sdb, /dev/dm-5, /dev/mapper/mpatha .etc
	output, err := utils.ExecShellCmd(ctx, "blockdev --getsize64 %s", hostDevice)
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to get device %s, err is %v", hostDevice, err)
		return 0, err
	}

	outputLines := strings.Split(output, "\n")
	for _, line := range outputLines {
		if line == "" {
			continue
		}
		size, err := strconv.ParseInt(line, constants.DefaultIntBase, constants.DefaultIntBitSize)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to get device size %s, err is %v", line, err)
			return 0, err
		}
		return size, nil
	}

	return 0, errors.New("failed to get device size")
}

// IsInFormatting is to check the device whether in formatting
var IsInFormatting = func(ctx context.Context, sourcePath, fsType string) (bool, error) {
	var cmd string
	if fsType != "ext2" && fsType != "ext3" && fsType != "ext4" && fsType != "xfs" {
		return false, utils.Errorf(ctx, "Do not support the type %s.", fsType)
	}

	cmd = fmt.Sprintf("ps -aux | grep mkfs | grep -w %s | wc -l |awk '{if($1>1) print 1; else print 0}'",
		sourcePath)
	output, err := utils.ExecShellCmd(ctx, cmd)
	if err != nil {
		return false, err
	}

	outputSplit := strings.Split(output, "\n")
	return len(outputSplit) != 0 && outputSplit[0] == "1", nil
}

// GetVLunIDByDevName to get the vLun Id by using device Name
func GetVLunIDByDevName(ctx context.Context, upType, devName string) (string, error) {
	output, err := GetUltraPathInfoByDevName(ctx, upType, devName)
	if err != nil {
		return "", err
	}

	splitInfo := strings.Fields(strings.TrimSpace(output))
	if len(splitInfo) != lengthOfUltraPathInfo {
		return "", utils.Errorf(ctx, "The result of upadmin is not valid for vlun %s", devName)
	}

	return splitInfo[0], nil
}

func getFieldFromUltraPathInfo(output, field string) ([]string, error) {
	if output == "" || field == "" {
		return nil, errors.New("input error")
	}

	var fieldInfo []string
	splitLines := strings.Split(output, "\n")
	for _, line := range splitLines {
		if !strings.Contains(line, ":") {
			continue
		}
		if strings.HasPrefix(line, field) {
			fieldInfo = append(fieldInfo, line)
		}
	}
	return fieldInfo, nil
}

func getPhysicalDevices(phyPaths []string) []string {
	var phyDevices []string
	for _, path := range phyPaths {
		splitInfo := strings.Split(path, "[")
		if len(splitInfo) != splitDeviceLength {
			continue
		}
		splitInfo = strings.Split(splitInfo[1], "]")
		if len(splitInfo) != splitDeviceLength {
			continue
		}
		phyDevices = append(phyDevices, splitInfo[0])
	}
	return phyDevices
}

var getNVMePhysicalDevices = func(phyPaths []string) []string {
	var phyDevices []string
	for _, path := range phyPaths {
		splitInfo := strings.Split(path, "(")
		if len(splitInfo) != splitDeviceLength {
			continue
		}
		splitInfo = strings.Split(splitInfo[1], ")")
		if len(splitInfo) != splitDeviceLength {
			continue
		}
		phyDevices = append(phyDevices, splitInfo[0])
	}
	return phyDevices
}

func getPhyDev(ctx context.Context, phyPaths []string, deviceType string) ([]string, error) {
	switch deviceType {
	case deviceTypeSCSI:
		return getPhysicalDevices(phyPaths), nil
	case deviceTypeNVMe:
		return getNVMePhysicalDevices(phyPaths), nil
	default:
		return nil, utils.Errorf(ctx, "Invalid device type %s.", deviceType)
	}
}

// GetPhyDev to get the physical device by using the vLun Id
func GetPhyDev(ctx context.Context, upType, vLunID, deviceType string) ([]string, error) {
	output, err := GetUltraPathDetailsByvLunID(ctx, upType, vLunID)
	if err != nil {
		return nil, err
	}

	phyPaths, err := getFieldFromUltraPathInfo(output, "Path")
	if err != nil {
		return nil, err
	}

	return getPhyDev(ctx, phyPaths, deviceType)
}

func deletePhysicalDevice(ctx context.Context, phyDevice string) error {
	output, err := utils.ExecShellCmd(ctx, "echo 1 > /sys/class/scsi_device/%s/device/delete", phyDevice)
	if err != nil {
		if strings.Contains(output, "No such file or directory") {
			return nil
		}

		log.AddContext(ctx).Errorf("Delete physical device %s error: %v", phyDevice, output)
		return err
	}
	return nil
}

func deleteVirtualDevice(ctx context.Context, virtualDevice string) error {
	output, err := utils.ExecShellCmd(ctx, "echo 1 > /sys/block/%s/device/delete", virtualDevice)
	if err != nil {
		if strings.Contains(output, "No such file or directory") {
			return nil
		}

		log.AddContext(ctx).Errorf("Delete virtual device %s error: %v", virtualDevice, output)
		return err
	}
	return nil
}

// RemoveAllDevice to remove the device through virtual device and physical device
var RemoveAllDevice = func(ctx context.Context,
	virtualDevice string,
	phyDevices []string,
	deviceType int) (string, error) {
	switch deviceType {
	case NotUseMultipath, UseDMMultipath:
		return RemoveDevice(ctx, virtualDevice)
	case UseUltraPath:
		return "", RemoveUltraPathDevice(ctx, virtualDevice, phyDevices)
	case UseUltraPathNVMe:
		return "", RemoveUltraPathNVMeDevice(ctx, virtualDevice, phyDevices)
	default:
		return "", utils.Errorln(ctx, "invalid device type")
	}
}

// ClearResidualPath used to clear residual path
func ClearResidualPath(ctx context.Context, lunWWN string, volumeMode any, multiPathType string) error {
	log.AddContext(ctx).Infof("Enter func: ClearResidualPath. lunWWN:[%s]. volumeMode:[%v]", lunWWN, volumeMode)

	v, ok := volumeMode.(string)
	if ok && v == "Block" {
		log.AddContext(ctx).Infof("volumeMode is Block, skip residual device check.")
		return nil
	}

	if err := clearUltraPathResidualPathByWwn(ctx, multiPathType, lunWWN); err != nil {
		return err
	}

	devInfos, err := getDevicesInfosByGUID(ctx, lunWWN)
	if err != nil {
		return err
	}

	if devInfos == nil {
		log.AddContext(ctx).Infof("No link related wwn:%s exist in the /dev/disk/by-id.", lunWWN)
		return nil
	}

	return clearResidualPath(ctx, devInfos)
}

func clearUltraPathResidualPathByWwn(ctx context.Context, multiPathType, lunWWN string) error {
	if multiPathType != HWUltraPath {
		return nil
	}

	log.AddContext(ctx).Infoln("start to clear ultrapath specific residual paths by device WWN.")
	vLun, err := GetUltrapathVLunByWWN(ctx, UltraPathCommand, lunWWN)
	if err != nil {
		return err
	}

	if vLun == nil {
		log.AddContext(ctx).Infoln("no ultrapath specific residual path to clear.")
		return nil
	}

	return vLun.CleanResidualPath(ctx)
}

func isPartitionDevice(ctx context.Context, dev string) (bool, error) {
	if strings.HasPrefix(dev, "dm") {
		// dm-* should convert to mpath* to determine whether it is a partition disk.
		dmDevice, err := getDMDeviceByAlias(ctx, dev)
		if err != nil {
			return false, utils.Errorf(ctx, "Get DMDevice by alias:%s failed. error: %v", dev, err)
		}
		return isEndWithDigital(dmDevice), nil
	} else if strings.HasPrefix(dev, "nvme") {
		// nvme1n1p1 is a partition disk.
		return isMatch(dev, `nvme[\d]+n[\d]+p[\d]+`), nil
	}

	// ultrapathap1, sdc1... is partition disk.
	return isEndWithDigital(dev), nil
}

func getDevicesInfosByGUID(ctx context.Context, tgtLunGUID string) ([]*deviceInfo, error) {
	// Obtain the devices link that contain the WWN in /dev/disk/by-id/.
	devices, err := GetDevicesByGUID(ctx, tgtLunGUID)
	if err != nil {
		return nil, err
	}

	var devInfos []*deviceInfo
	for _, device := range devices {
		device = strings.TrimSpace(device)
		// check whether device is a partition device.
		partitionDev, err := isPartitionDevice(ctx, device)
		if err != nil {
			return nil, utils.Errorf(ctx, "check device: %s is a partition device failed. error: %v", device, err)
		} else if partitionDev {
			log.AddContext(ctx).Infof("Device: %s is a partition device，skip", device)
			continue
		}

		var devInfo = &deviceInfo{deviceName: device, lunWWN: tgtLunGUID, deviceFullName: "/dev/" + device}
		if strings.HasPrefix(device, "ultrapath") {
			devInfo.multipathType = UseUltraPathNVMe
		} else if strings.HasPrefix(device, "dm") {
			devInfo.multipathType = UseDMMultipath
		} else if strings.HasPrefix(device, "sd") && isUltraPathDevice(ctx, device) {
			devInfo.multipathType = UseUltraPath
		} else if strings.HasPrefix(device, "sd") || strings.HasPrefix(device, "nvme") {
			devInfo.multipathType = NotUseMultipath
		} else {
			log.AddContext(ctx).Warningf("Unknown device link: %s", device)
			continue
		}
		devInfos = append(devInfos, devInfo)
	}

	return devInfos, nil
}

func clearResidualPath(ctx context.Context, deviceInfos []*deviceInfo) error {
	log.AddContext(ctx).Infof("Enter func: clearResidualPath. deviceInfos:%v", deviceInfos)
	for _, deviceInfo := range deviceInfos {
		var isResidualDevicePath bool
		var err error
		switch deviceInfo.multipathType {
		case UseDMMultipath:
			isResidualDevicePath, err = isDMResidualPath(ctx, deviceInfo)
		case UseUltraPath:
			isResidualDevicePath, err = isUpResidualPath(ctx, deviceInfo)
		case UseUltraPathNVMe:
			isResidualDevicePath, err = isUpNVMeResidualPath(ctx, deviceInfo)
		case NotUseMultipath:
			isResidualDevicePath, err = isPhyResidualPath(ctx, deviceInfo)
		default:
			// If invalid types exist, the code is incorrect and needs to be modified.
			return utils.Errorf(ctx, "Multipath type:%d invalid. devInfo %v",
				deviceInfo.multipathType, deviceInfo)
		}

		if err != nil {
			log.AddContext(ctx).Errorf("Failed to check whether the device:%s is residual path. error:%v.",
				deviceInfo.deviceName, err)
			return err
		}

		if isResidualDevicePath {
			log.AddContext(ctx).Infof("Find residual device path:%v.", deviceInfo)

			phyDevices, err := GetPhysicalDevices(ctx, deviceInfo.deviceName, deviceInfo.multipathType)
			if err != nil {
				log.AddContext(ctx).Errorf("Failed to get physical devices of:%s. error:%v",
					deviceInfo.deviceName, err)
				return err
			}

			_, err = RemoveAllDevice(ctx, deviceInfo.deviceName, phyDevices, deviceInfo.multipathType)
			if err != nil {
				log.AddContext(ctx).Errorf("Failed to remove residual path:%s. error:%v",
					deviceInfo.deviceName, err)
				return err
			}
		}
	}

	return nil
}

func isDMResidualPath(ctx context.Context, deviceInfo *deviceInfo) (bool, error) {
	readable, err := IsDeviceReadable(ctx, deviceInfo.deviceFullName)
	if err != nil || !readable {
		// dd command not found considered an error
		if strings.Contains(err.Error(), "command not found") {
			return false, err
		}
		return true, nil
	}

	devices, err := getDeviceFromDM(deviceInfo.deviceName)
	if err != nil {
		return false, err
	}

	available, err := IsMultiPathAvailable(ctx, deviceInfo.deviceName, deviceInfo.lunWWN, devices)
	if err != nil || !available {
		// If the device is readable but unavailable, CSI will not clear it. User need to clear the device manually.
		return true, err
	}

	return false, nil
}

func isUpResidualPathCommon(ctx context.Context, multipathType string, deviceInfo *deviceInfo) (bool, error) {
	readable, err := IsDeviceReadable(ctx, deviceInfo.deviceFullName)
	if err != nil || !readable {
		// dd command not found considered an error
		if strings.Contains(err.Error(), "command not found") {
			return false, err
		}
		return true, nil
	}

	isTakeOver, err := isTakeOverByUltraPath(ctx, multipathType, deviceInfo.lunWWN)
	if err != nil || !isTakeOver {
		log.AddContext(ctx).Infof("Device:%s WWN:%s is not take over by UltraPath.",
			deviceInfo.deviceName, deviceInfo.lunWWN)
		return true, err
	}

	available, err := isUpMultiPathAvailable(ctx, multipathType, deviceInfo.deviceName, deviceInfo.lunWWN)
	if err != nil || !available {
		// If the device is readable but unavailable, CSI will not clear it. User need to clear the device manually.
		return true, err
	}

	return false, nil
}

func isUpResidualPath(ctx context.Context, deviceInfo *deviceInfo) (bool, error) {
	return isUpResidualPathCommon(ctx, UltraPathCommand, deviceInfo)
}

func isUpNVMeResidualPath(ctx context.Context, deviceInfo *deviceInfo) (bool, error) {
	return isUpResidualPathCommon(ctx, UltraPathNVMeCommand, deviceInfo)
}

// IsUpNVMeResidualPath used to determine whether the device is residual
var IsUpNVMeResidualPath = func(ctx context.Context, devName, lunWWN string) (bool, error) {
	return isUpNVMeResidualPath(ctx,
		&deviceInfo{deviceName: devName, lunWWN: lunWWN, multipathType: UseUltraPathNVMe,
			deviceFullName: "/dev/" + devName})
}

func isPhyResidualPath(ctx context.Context, deviceInfo *deviceInfo) (bool, error) {
	readable, err := IsDeviceReadable(ctx, deviceInfo.deviceFullName)
	if err != nil || !readable {
		// dd command not found considered an error
		if strings.Contains(err.Error(), "command not found") {
			return false, err
		}
		return true, nil
	}

	available, err := IsDeviceAvailable(ctx, deviceInfo.deviceFullName, deviceInfo.lunWWN)
	if err != nil || !available {
		// If the device is readable but unavailable, CSI will not clear it. User need to clear the device manually.
		return true, err
	}

	return false, nil
}

// IsDeviceReadable to check the device is readable or not
var IsDeviceReadable = func(ctx context.Context, devicePath string) (bool, error) {
	_, err := ReadDevice(ctx, devicePath)
	if err != nil {
		log.AddContext(ctx).Warningf("Device:%s is unreadable.", devicePath)
		return false, err
	}

	return true, nil
}

func isUpMultiPathAvailable(ctx context.Context, multipathType, dev, lunWWN string) (bool, error) {
	devLunWWN, err := GetLunWWNByDevName(ctx, multipathType, dev)
	if err != nil {
		log.AddContext(ctx).Errorf("Get Lun WWN by device name:%s failed. error:%s", dev, err)
		return false, err
	}

	if !strings.Contains(lunWWN, devLunWWN) {
		log.AddContext(ctx).Warningf("Device:%s wwn:%s is inconsistent with target wwn:%s",
			dev, devLunWWN, lunWWN)
		return false, nil
	}

	return true, nil
}

// GetDeviceFromMountFile parse the mount file and get devicePath from targetPath.
// If a device is associated with multiple paths, an error will be returned
func GetDeviceFromMountFile(ctx context.Context, targetPath string, checkDevRef bool) (string, error) {
	mountMap, err := ReadMountPoints(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("read /proc/mounts failed, error: %v", err)
		return "", err
	}

	// If mountPath is symlink, need get its actual path.
	mountPath, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		mountPath = targetPath
	}

	device := getDeviceFromMountMap(mountPath, mountMap)
	if device == "" {
		return "", fmt.Errorf("the path [%s] doesn't referenced device in /proc/mounts", mountPath)
	}

	if checkDevRef {
		if deviceRefPaths := getDevicePathRefs(device, mountMap); len(deviceRefPaths) > 1 {
			return "", fmt.Errorf("the device [%s] referenced multiple paths in /proc/mounts, paths: %v",
				device, deviceRefPaths)
		}
	}

	return device, nil
}

func getDeviceFromMountMap(targetPath string, mountMap map[string]string) string {
	var device string
	for mountPath, devPath := range mountMap {
		if mountPath == targetPath {
			device = devPath
			break
		}
	}
	return device
}

// getDevicePathRefs find all references to the device.
func getDevicePathRefs(device string, mountMap map[string]string) []string {
	var deviceRefPaths []string
	for mountPath, devPath := range mountMap {
		if devPath == device {
			deviceRefPaths = append(deviceRefPaths, mountPath)
		}
	}
	return deviceRefPaths
}

// ReadMountPoints read mount file
// mountMap: key means mountPath; value means devicePath.
func ReadMountPoints(ctx context.Context) (map[string]string, error) {
	data, err := ConsistentRead(procMountsPath, maxListTries)
	if err != nil {
		log.AddContext(ctx).Errorf("Read the mount file error: %v", err)
		return nil, err
	}

	const splitLength = 2
	mountMap := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			splitValue := strings.Split(line, " ")
			if len(splitValue) >= splitLength && splitValue[0] != "#" {
				mountMap[splitValue[1]] = splitValue[0]
			}
		}
	}
	return mountMap, nil
}

// ConsistentRead repeatedly reads a file until it gets the same content twice.
// This is useful when reading files in /proc/mount that are larger than page size
// and kernel may modify them between individual read() syscalls
func ConsistentRead(filename string, attempts int) ([]byte, error) {
	oldContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	for i := 0; i < attempts; i++ {
		newContent, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(oldContent, newContent) {
			return newContent, nil
		}
		// Files are different, continue reading
		oldContent = newContent
	}
	return nil, fmt.Errorf("could not get consistent content of %s after %d attempts", filename, attempts)
}

// MountPathIsExist if mount point exist in /proc/mounts file, this function will return true
func MountPathIsExist(ctx context.Context, mountPoint string) (bool, error) {
	mountMap, err := ReadMountPoints(ctx)
	if err != nil {
		return false, err
	}
	_, ok := mountMap[mountPoint]
	return ok, nil
}

func removeWwnType(wwn string) string {
	wwnTypes := []string{"t10.1", "t10.", "1", "eui.2", "eui.", "2", "naa.3", "naa.", "3"}
	for _, wwnType := range wwnTypes {
		if strings.HasPrefix(wwn, wwnType) && len(wwn) > len(wwnType) {
			return wwn[len(wwnType):]
		}
	}
	return wwn
}

// GetWwnFromTargetPath get wwn form targetPath
func GetWwnFromTargetPath(ctx context.Context, volumeId, targetPath string, checkDevRef bool) (string, error) {
	log.AddContext(ctx).Infof("start getting targetPath link device")
	devicePath, err := GetDeviceFromSymLink(path.Join(targetPath, volumeId))
	if err != nil || devicePath == "" {
		log.AddContext(ctx).Infof("targetPath not found link device, targetPath: %s, error: %v",
			targetPath, err)

		devicePath, err = GetDeviceFromMountFile(ctx, targetPath, checkDevRef)
		if err != nil {
			log.AddContext(ctx).Errorf("not found device in /proc/mounts, targetPath: %s, error: %v",
				targetPath, err)
			return "", err
		}
	}

	wwn, err := GetWwnByDevice(ctx, devicePath)
	if err != nil {
		log.AddContext(ctx).Errorf("get device wwn failed, error: %v", err)
		return "", err
	}

	return removeWwnType(wwn), nil
}

// GetDeviceFromSymLink returns link path if specified file exists and the type is symbolik link.
// If file doesn't exist, or file exists but not symbolic link, returns an empty string with
// error from Lstat().
func GetDeviceFromSymLink(targetPath string) (string, error) {
	info, err := os.Lstat(targetPath)
	if err != nil {
		return "", err
	}

	if info.Mode()&os.ModeSymlink != os.ModeSymlink {
		return "", fmt.Errorf("the file mode is not system link, targetPath: %s", targetPath)
	}

	linkDevice, err := os.Readlink(targetPath)
	if err != nil {
		return "", err
	}
	return linkDevice, nil
}

func getDeviceTypeByName(ctx context.Context, deviceName string) (int, error) {
	var deviceType int
	if strings.HasPrefix(deviceName, "ultrapath") {
		deviceType = UseUltraPathNVMe
	} else if strings.HasPrefix(deviceName, "dm") || strings.HasPrefix(deviceName, "mpath") {
		deviceType = UseDMMultipath
	} else if strings.HasPrefix(deviceName, "sd") && isUltraPathDevice(ctx, deviceName) {
		deviceType = UseUltraPath
	} else if strings.HasPrefix(deviceName, "sd") || strings.HasPrefix(deviceName, "nvme") {
		deviceType = NotUseMultipath
	} else {
		return 0, fmt.Errorf("unknowns device type, deviceName: %s", deviceName)
	}
	return deviceType, nil
}

// GetWwnByDevice get wwn according to multipath and protocol type
func GetWwnByDevice(ctx context.Context, devicePath string) (string, error) {
	deviceName := path.Base(devicePath)
	devType, err := getDeviceTypeByName(ctx, deviceName)
	if err != nil {
		log.AddContext(ctx).Errorf("get device type failed, devicePath: %s, error: %v", devicePath, err)
		return "", err
	}

	if devType == UseUltraPath {
		return GetLunWWNByDevName(ctx, UltraPathCommand, deviceName)
	} else if devType == UseUltraPathNVMe {
		return GetLunWWNByDevName(ctx, UltraPathNVMeCommand, deviceName)
	} else if devType == UseDMMultipath {
		return GetSCSIWwn(ctx, devicePath)
	} else if strings.HasPrefix(deviceName, "nvme") {
		return GetNVMeWwn(ctx, devicePath)
	} else if strings.HasPrefix(deviceName, "sd") {
		return GetSCSIWwn(ctx, devicePath)
	} else {
		return "", fmt.Errorf("can't get device wwn, devicePath: %s", devicePath)
	}
}
