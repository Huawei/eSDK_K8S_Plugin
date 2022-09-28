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

// Package connector provide methods of interacting with the host
package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"regexp"
	"strconv"
	"strings"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
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

	var nvmeConnectInfo map[string]interface{}
	if err = json.Unmarshal([]byte(output), &nvmeConnectInfo); err != nil {
		return nil, errors.New("unmarshal nvme connect info failed")
	}

	return nvmeConnectInfo, nil
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
	if exist, _ := utils.PathExist(nvmePortPath); !exist {
		return "", utils.Errorf(ctx, "NVMe device path %s is not exist.", nvmePortPath)
	}

	output, err := utils.ExecShellCmd(ctx, fmt.Sprintf("ls %s |grep nvme", nvmePortPath))
	if err != nil {
		log.AddContext(ctx).Errorf("get nvme device failed, error:%v", err)
		return "", err
	}

	outputLines := strings.Split(output, "\n")
	for _, dev := range outputLines {
		match, err := regexp.MatchString(`nvme[0-9]+n[0-9]+`, dev)
		if err != nil {
			log.AddContext(ctx).Warningf("Match string failed. dev:%s, error:%v", dev, err)
			continue
		}
		if match {
			uuid, err := getNVMeWWN(ctx, devicePort, dev)
			if err != nil {
				log.AddContext(ctx).Warningf("Get nvme device uuid failed, error:%v", err)
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
