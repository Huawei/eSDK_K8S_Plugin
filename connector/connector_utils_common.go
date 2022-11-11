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
	"strings"

	"huawei-csi-driver/connector/utils/lock"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// GetPhysicalDevices to get physical devices
var GetPhysicalDevices = func(ctx context.Context, device string, deviceType int) ([]string, error) {
	switch deviceType {
	case NotUseMultipath:
		return []string{device}, nil
	case UseDMMultipath:
		return GetPhyDevicesFromDM(device)
	default:
		return nil, utils.Errorf(ctx, "Invalid device type %d.", deviceType)
	}
}

// GetNVMePhysicalDevices to get NVMe physical devices
var GetNVMePhysicalDevices = func(ctx context.Context, device string, deviceType int) ([]string, error) {
	switch deviceType {
	case NotUseMultipath:
		return []string{device}, nil
	default:
		return nil, utils.Errorf(ctx, "Invalid device type %d.", deviceType)
	}
}

// DisConnectVolumeCommon used for disconnect volume for all protocol
func DisConnectVolumeCommon(ctx context.Context,
	tgtLunWWN, protocol string,
	f func(context.Context, string) error) error {
	err := lock.SyncLock(ctx, tgtLunWWN, DisConnect)
	if err != nil {
		return utils.Errorf(ctx, "get %s disconnect sync lock for LUN %s error: %v", protocol, tgtLunWWN, err)
	}

	defer func() {
		err = lock.SyncUnlock(ctx, tgtLunWWN, DisConnect)
		if err != nil {
			log.AddContext(ctx).Errorf("release %s disconnect sync Unlock for LUN %s error: %v",
				protocol, tgtLunWWN, err)
		}
	}()

	return f(ctx, tgtLunWWN)
}

// CheckHostConnectivity used to check host connectivity
func CheckHostConnectivity(ctx context.Context, portal string) bool {
	const addrLength = 2
	addr := strings.Split(portal, ":")
	if len(addr) != addrLength {
		log.AddContext(ctx).Errorf("the portal format is incorrect. %s", portal)
		return false
	}

	_, err := utils.ExecShellCmd(ctx, PingCommand, addr[0])
	return err == nil
}

// ConnectVolumeCommon used for connect volume for all protocol
func ConnectVolumeCommon(ctx context.Context,
	conn map[string]interface{},
	tgtLunWWN, protocol string,
	f func(context.Context, map[string]interface{}) (string, error)) (string, error) {
	err := lock.SyncLock(ctx, tgtLunWWN, Connect)
	if err != nil {
		return "", utils.Errorf(ctx, "get [%s] connect sync lock for LUN [%s] failed, error: %v",
			protocol, tgtLunWWN, err)
	}

	defer func() {
		err = lock.SyncUnlock(ctx, tgtLunWWN, Connect)
		if err != nil {
			log.AddContext(ctx).Errorf("release [%s] connect sync lock for LUN [%s] failed, error: %v",
				protocol, tgtLunWWN, err)
		}
	}()

	return f(ctx, conn)
}
