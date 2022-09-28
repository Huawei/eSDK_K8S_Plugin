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

// Package iscsi provide the way to connect/disconnect volume within iSCSI protocol
package iscsi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

func getSessionIds(ctx context.Context, devices []string, deviceType int) ([]string, error) {
	var devSessionIds []string
	for _, dev := range devices {
		sessionID, err := getSessionID(ctx, dev, deviceType)
		if err != nil {
			return nil, err
		}

		if sessionID == "" {
			log.AddContext(ctx).Infof("can not get the session info for device %s", dev)
			continue
		}

		devSessionIds = append(devSessionIds, sessionID)
	}
	return devSessionIds, nil
}

func getSessionID(ctx context.Context, device string, deviceType int) (string, error) {
	if deviceType == connector.NotUseMultipath || deviceType == connector.UseDMMultipath {
		return getSessionIDByDevice(device)
	} else if deviceType == connector.UseUltraPath || deviceType == connector.UseUltraPathNVMe {
		return getSessionIDByHCTL(ctx, device)
	}
	return "", errors.New("unSupport device Type")
}

func getSessionIDByDevice(devPath string) (string, error) {
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

func getSessionIDByHCTL(ctx context.Context, devHCTL string) (string, error) {
	hostChannelTargetLun := strings.Split(devHCTL, ":")
	if len(hostChannelTargetLun) != lengthOfHCTL {
		return "", utils.Errorf(ctx, "device %s is not host:Channel:Target:Lun", devHCTL)
	}

	path := fmt.Sprintf("/sys/class/scsi_host/host%s/device/session*", hostChannelTargetLun[0])
	sessions, err := filepath.Glob(path)
	if err != nil {
		return "", err
	}

	if sessions == nil {
		return "", utils.Errorf(ctx, "There is no session info in the path %s", path)
	}

	_, session := filepath.Split(sessions[0])
	return strings.Split(session, "session")[1], nil
}
