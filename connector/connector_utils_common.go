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
	"context"
	"net"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils/lock"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/iputils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// GetPhysicalDevices to get physical devices
var GetPhysicalDevices = func(ctx context.Context, device string, deviceType int) ([]string, error) {
	switch deviceType {
	case NotUseMultipath:
		return []string{device}, nil
	case UseDMMultipath:
		return GetPhyDevicesFromDM(device)
	case UseUltraPath:
		return GetDeviceFromUltraPath(ctx, device)
	case UseUltraPathNVMe:
		return GetDeviceFromUltraPathNVMe(ctx, device)
	default:
		return nil, utils.Errorf(ctx, "Invalid device type %d.", deviceType)
	}
}

// GetNVMePhysicalDevices to get NVMe physical devices
var GetNVMePhysicalDevices = func(ctx context.Context, device string, deviceType int) ([]string, error) {
	switch deviceType {
	case NotUseMultipath:
		return []string{device}, nil
	case UseUltraPathNVMe:
		return GetNVMeDeviceFromUltraPathNVMe(ctx, device)
	default:
		return nil, utils.Errorf(ctx, "Invalid device type %d.", deviceType)
	}
}

func getDeviceFromUltraPathCommon(ctx context.Context, upType, upDevice string) ([]string, error) {
	vLunID, err := GetVLunIDByDevName(ctx, upType, upDevice)
	if err != nil {
		return nil, err
	}

	return GetPhyDev(ctx, upType, vLunID, deviceTypeSCSI)
}

// GetDeviceFromUltraPath used to get physical device from ultrapath
func GetDeviceFromUltraPath(ctx context.Context, upDevice string) ([]string, error) {
	return getDeviceFromUltraPathCommon(ctx, UltraPathCommand, upDevice)
}

// GetDeviceFromUltraPathNVMe used to get physical device from ultrapath-nvme
func GetDeviceFromUltraPathNVMe(ctx context.Context, upDevice string) ([]string, error) {
	return getDeviceFromUltraPathCommon(ctx, UltraPathNVMeCommand, upDevice)
}

func getNVMeDeviceFromUltraPath(ctx context.Context, upType, upDevice string) ([]string, error) {
	vLunID, err := GetVLunIDByDevName(ctx, upType, upDevice)
	if err != nil {
		return nil, err
	}

	return GetPhyDev(ctx, upType, vLunID, deviceTypeNVMe)
}

// GetNVMeDeviceFromUltraPathNVMe used to get physical nvme device from ultrapath-nvme
func GetNVMeDeviceFromUltraPathNVMe(ctx context.Context, upDevice string) ([]string, error) {
	return getNVMeDeviceFromUltraPath(ctx, UltraPathNVMeCommand, upDevice)
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
	hostAddr, _, err := net.SplitHostPort(portal)
	if err != nil {
		log.AddContext(ctx).Errorf("failed to split host port from portal [%s]", portal)
		return false
	}

	ipWrapper := iputils.NewIPWrapper(hostAddr)
	if ipWrapper == nil {
		log.AddContext(ctx).Errorf("hostAddr [%s] is not a valid ip address", hostAddr)
		return false
	}

	_, err = utils.ExecShellCmd(ctx, ipWrapper.GetPingCommand(), hostAddr)
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
