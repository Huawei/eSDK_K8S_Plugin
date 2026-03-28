/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

// Package nvme provide the way to connect/disconnect volume within NVMe over Connector protocol
package nvme

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func getSessionPorts(ctx context.Context, devices []string, deviceType int) ([]string, error) {
	switch deviceType {
	case connector.UseUltraPathNVMe:
		return getSessionPortsByUltraPath(ctx, devices)
	case connector.NotUseMultipath:
		// not use multipath or use NVMe-Native
		return getSessionPortsByNative(ctx, devices)
	default:
		return nil, fmt.Errorf("device type %s is not supported", connector.MultipathTypeToString(deviceType))
	}
}

func getSessionPortsByNative(ctx context.Context, devices []string) ([]string, error) {
	if len(devices) != 1 {
		return nil, fmt.Errorf("only one NVMe device should be provided, got %v", devices)
	}

	sysOrPathNo, deviceId, err := connector.GetNVMeDiskNumber(devices[0])
	if err != nil {
		return nil, err
	}

	subPath, err := filepath.Glob(fmt.Sprintf("/sys/class/nvme-fabrics/ctl/*/nvme%s*n%s", sysOrPathNo, deviceId))
	if err != nil {
		return nil, fmt.Errorf("get NVMe device path failed, err: %w", err)
	}

	var devSessionPorts []string
	for _, path := range subPath {
		dir := filepath.Dir(path)
		sessionPort := filepath.Base(dir)
		devSessionPorts = append(devSessionPorts, sessionPort)
	}

	return devSessionPorts, nil
}

func getSessionPortsByUltraPath(ctx context.Context, devices []string) ([]string, error) {
	var devSessionPorts []string
	for _, dev := range devices {
		sessionPort, err := getSessionPortBySubDevice(ctx, dev)
		if err != nil {
			return nil, err
		}

		if sessionPort == "" {
			log.AddContext(ctx).Infof("can not get the session info for device %s", dev)
			continue
		}

		devSessionPorts = append(devSessionPorts, sessionPort)
	}
	return devSessionPorts, nil
}

func getSessionPortBySubDevice(ctx context.Context, devPath string) (string, error) {
	splitS := strings.Split(devPath, "n")
	if len(splitS) != intNumThree {
		return "", utils.Errorf(ctx, "device %s is not valid", devPath)
	}

	return fmt.Sprintf("n%s", splitS[1]), nil
}

func disconnectSessions(ctx context.Context, sessionPorts []string) error {
	for _, nvmePort := range sessionPorts {
		cmd := fmt.Sprintf("ls /sys/devices/virtual/nvme-fabrics/ctl/%s/ |grep nvme |wc -l |awk "+
			"'{if($1>1) print 1; else print 0}'", nvmePort)
		output, err := utils.ExecShellCmd(ctx, cmd)
		if err != nil {
			return utils.Errorf(ctx, "Disconnect Connector target path %s failed, err: %v", nvmePort, err)
		}

		outputSplit := strings.Split(output, "\n")
		if len(outputSplit) != 0 && outputSplit[0] == "0" {
			disconnectRoCEController(ctx, nvmePort)
		}
	}
	return nil
}

func disconnectRoCEController(ctx context.Context, devPath string) {
	output, err := utils.ExecShellCmd(ctx, "nvme disconnect -d %s", devPath)
	if err != nil || output != "" {
		log.AddContext(ctx).Warningf("Disconnect controller %s error %v", devPath, err)
	}
}
