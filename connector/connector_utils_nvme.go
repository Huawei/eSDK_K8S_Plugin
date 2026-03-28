/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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
	"bufio"
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	// nvmeDeviceRegex is a regular expression used to match NVMe device names.
	// Example matches: "nvme0n1", "nvme1n2".
	nvmeDeviceRegex = regexp.MustCompile(`^nvme(\d+)n(\d+)$`)

	// nvmeSubDeviceRegex is a regular expression used to match NVMe sub-device names.
	// Example matches: "nvme0n1", "nvme1c2n3".
	nvmeSubDeviceRegex = regexp.MustCompile(`^nvme\d+(c\d+)?n\d+$`)
)

// DoScanNVMeDevice used to scan device by command nvme ns-rescan
var DoScanNVMeDevice = func(ctx context.Context, devicePort string) error {
	output, err := utils.ExecShellCmd(ctx, "nvme ns-rescan /dev/%s", devicePort)
	if err != nil {
		log.AddContext(ctx).Errorf("Scan nvme port failed. output:%s, error:%v", output, err)
		return err
	}
	return nil
}

// GetSubSysInfo used to get subsys info by command nvme list-subsys
var GetSubSysInfo = func(ctx context.Context) (map[string]interface{}, error) {
	err := checkNVMeVersion(ctx)
	if err != nil {
		return nil, utils.Errorf(ctx, "Failed to check the NVMe version. err:%v", err)
	}

	output, err := utils.ExecShellCmdFilterLog(ctx, "nvme list-subsys -o json")
	if err != nil {
		log.AddContext(ctx).Errorf("Get exist nvme connect info failed, output:%s, error:%v", output, err)
		return nil, errors.New("get nvme connect port failed")
	}

	if len(output) == 0 {
		return nil, errors.New("get nvme connect info with empty return")
	}

	if output[0] == '{' {
		output = fmt.Sprintf("[%s]", output)
	}

	var nvmeConnectInfo []map[string]interface{}
	if err = json.Unmarshal([]byte(output), &nvmeConnectInfo); err != nil {
		return nil, errors.New("unmarshal nvme connect info failed")
	}

	if len(nvmeConnectInfo) == 0 {
		log.AddContext(ctx).Warningf("nvme connect info is empty, origin: %s", output)
		return map[string]interface{}{}, nil
	}

	return nvmeConnectInfo[0], nil
}

func checkNVMeVersion(ctx context.Context) error {
	output, err := utils.ExecShellCmd(ctx, "nvme version")
	if err != nil {
		return fmt.Errorf("failed to query the NVMe version. err: %v output: %s", err, output)
	}

	pattern := regexp.MustCompile(`version\s+([\d]).([\d]+)[\d.]*`)
	version := pattern.FindStringSubmatch(output)
	const versionLength = 3
	if len(version) != versionLength {
		return fmt.Errorf("the format of the data returned by the NVME command is incorrect. output:%s", output)
	}

	const majorPosition = 1
	major, err := strconv.Atoi(version[majorPosition])
	if err != nil {
		return fmt.Errorf("failed to parse the NVMe major version number. err: %v version: %v", err, version)
	}

	const minorPosition = 2
	minor, err := strconv.Atoi(version[minorPosition])
	if err != nil {
		return fmt.Errorf("failed to parse the NVMe minor version number. err: %v version: %v", err, version)
	}

	const minimumMajor = 1
	const minimumMinor = 9
	if major <= minimumMajor && minor < minimumMinor {
		return fmt.Errorf("the current NVMe CLI version is not supported. Please upgrade it. %v", version)
	}

	return nil
}

// GetNVMeDevice used to get device name by channel
func GetNVMeDevice(ctx context.Context, devicePort string, tgtLunGUID string) (string, error) {
	nvmePortPath := path.Join("/sys/devices/virtual/nvme-fabrics/ctl/", devicePort)
	exist, err := utils.PathExist(nvmePortPath)
	if !exist {
		return "", utils.Errorf(ctx, "NVMe device path %s is not exist.", nvmePortPath)
	}

	if err != nil {
		log.AddContext(ctx).Errorf("get NVMe device path failed, error:%v", err)
		return "", err
	}

	output, err := utils.ExecShellCmd(ctx, fmt.Sprintf("ls %s | grep nvme", nvmePortPath))
	if err != nil {
		log.AddContext(ctx).Errorf("get nvme device failed, error:%v", err)
		return "", err
	}

	outputLines := strings.Split(output, "\n")
	for _, dev := range outputLines {
		if nvmeSubDeviceRegex.MatchString(dev) {
			uuid, err := getNVMeWWN(ctx, devicePort, dev)
			if err != nil {
				log.AddContext(ctx).Warningf("Get nvme device uuid failed, dev: %s, error: %v", dev, err)
				continue
			}
			if strings.Contains(uuid, tgtLunGUID) {
				return dev, nil
			}
		}
	}

	return "", utils.Errorf(ctx, "Find device of lun:%s failed.", tgtLunGUID)
}

func getNVMeWWN(ctx context.Context, devicePort, device string) (string, error) {
	uuidFile := path.Join("/sys/devices/virtual/nvme-fabrics/ctl/", devicePort, device, "wwid")
	data, err := ioutil.ReadFile(uuidFile)
	if err != nil {
		return "", utils.Errorf(ctx, "Read NVMe uuid file:%s failed. error:%v", uuidFile, err)
	}

	if data != nil {
		return string(data), nil
	}

	return "", errors.New("uuid is not exist")
}

// IsNVMeMultipathEnabled checks whether the NVMe-Native multipath is enabled in the system
func IsNVMeMultipathEnabled(ctx context.Context) bool {
	data, err := os.ReadFile(nvmeMultipathConfigPath)
	if err != nil {
		log.AddContext(ctx).Errorf("Read NVMe multipath failed, err: %v", err)
		return false
	}

	return strings.TrimSpace(string(data)) == nvmeMultipathEnabledValue
}

// GetNVMeDiskByGuid gets the disk name of NVMe-Native multipath by guid
func GetNVMeDiskByGuid(ctx context.Context, guid string) (string, error) {
	symlinkPath := fmt.Sprintf("/dev/disk/by-id/nvme-eui.%s", guid)
	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		return "", fmt.Errorf("symbolic link does not exist: %s", symlinkPath)
	}

	devicePath, err := os.Readlink(symlinkPath)
	if err != nil {
		return "", fmt.Errorf("failed to read symbolic link %s: %w", symlinkPath, err)
	}

	return filepath.Base(devicePath), nil
}

// IsNVMeSubPathExist check whether the nvme sub path is existed
func IsNVMeSubPathExist(ctx context.Context, subsystem, nvmePath, device string) bool {
	// If subsystem is nvme-subsys0, nvmePath is nvme1, and device is nvme0n2,
	// the subpath is /sys/class/nvme-subsystem/nvme-subsys0/nvme1/nvme0c1n2
	subsystemNo := strings.TrimPrefix(subsystem, subsystemPrefix)
	pathNo := strings.TrimPrefix(nvmePath, nvmePathPrefix)
	deviceNo := strings.TrimPrefix(device, fmt.Sprintf("nvme%sn", subsystemNo))
	subpath := fmt.Sprintf("/sys/class/nvme-subsystem/%s/%s/nvme%sc%sn%s",
		subsystem, nvmePath, subsystemNo, pathNo, deviceNo)
	_, err := os.Stat(subpath)
	if os.IsNotExist(err) {
		log.AddContext(ctx).Warningf("NVMe subpath %s is not existed", subpath)
		return false
	}

	if err != nil {
		log.AddContext(ctx).Warningf("Check NVMe subpath %s stat failed, reason: %v", subpath, err)
		return true
	}

	return true
}

// CheckIsTakeOverByNVMeNative check whether the device is takeover by NVMe-Native multipath
func CheckIsTakeOverByNVMeNative(ctx context.Context, device string) error {
	output, err := utils.ExecShellCmd(ctx, "nvme list | grep %s", device)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return nil
		}
	}

	return fmt.Errorf("device %s is not takeover by NVMe-Native multipath", device)
}

// GetNVMeDiskNumber extracts the number of an NVMe device.
// In non-multipath scenarios, it returns the nvme path number and device ID.
// In NVMe-Native multipath scenarios, it returns the nvme subsys number and device ID.
func GetNVMeDiskNumber(diskName string) (string, string, error) {
	matches := nvmeDeviceRegex.FindStringSubmatch(diskName)
	if len(matches) < nvmeDeviceMatchNum {
		return "", "", fmt.Errorf("invalid NVMe disk name format: %s", diskName)
	}

	return matches[1], matches[2], nil
}
