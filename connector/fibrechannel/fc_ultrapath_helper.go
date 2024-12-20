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

// Package fibrechannel provide the way to connect/disconnect volume within FC protocol
package fibrechannel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func getVirtualLunPath(ctx context.Context, lunWWN string) (string, error) {
	if lunWWN == "" {
		return lunWWN, utils.Errorln(ctx, "The lun wwn is empty.")
	}
	return fmt.Sprintf("/dev/disk/by-id/wwn-0x%s", lunWWN), nil
}

func checkConnectSuccess(ctx context.Context, lunWWN, devicePath string) bool {
	exist, err := utils.PathExist(devicePath)
	if err != nil {
		log.AddContext(ctx).Infof("get device path [%s] failed, err: %v.", devicePath, err)
		return false
	}
	if !exist {
		log.AddContext(ctx).Infof("The device %s is not exist.", devicePath)
		return false
	}

	_, err = connector.ReadDevice(ctx, devicePath)
	if err != nil {
		log.AddContext(ctx).Infof("The device %s is not readable.", devicePath)
		return false
	}

	available, err := connector.IsDeviceAvailable(ctx, devicePath, lunWWN)
	if err != nil || !available {
		log.AddContext(ctx).Infof("The device %s is not available.", devicePath)
		return false
	}

	return true
}

func waitUltraPathDeviceDiscovery(ctx context.Context,
	hbas []map[string]string,
	conn *connectorInfo) (
	deviceInfo, error) {
	var info deviceInfo
	devicePath, err := getVirtualLunPath(ctx, conn.tgtLunWWN)
	if err != nil {
		return deviceInfo{}, err
	}

	err = utils.WaitUntil(func() (bool, error) {
		if info.tries >= deviceScanAttemptsDefault {
			log.AddContext(ctx).Errorln("Fibre Channel volume device not found.")
			return false, errors.New(connector.VolumeNotFound)
		}

		if checkConnectSuccess(ctx, conn.tgtLunWWN, devicePath) {
			info.hostDevice = devicePath
			realPath, err := os.Readlink(devicePath)
			if err == nil {
				info.realDeviceName = filepath.Base(realPath)
			} else {
				log.AddContext(ctx).Warningf("Can not get the real link of device %s", devicePath)
			}
			return true, nil
		}

		rescanHosts(ctx, hbas, conn)
		info.tries++
		return false, nil
	}, time.Minute, time.Second)
	return info, err
}
