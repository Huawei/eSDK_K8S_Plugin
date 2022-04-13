/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package connector provide methods of interacting with the host
package connector

import (
	"bufio"
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	diskStatusNormal      = "Normal"
	diskStatusFault       = "Fault"
	diskStatusDegraded    = "Degraded"
	diskStatusUnavailable = "Unavailable"

	exitStatus1 = "exit status 1"

	defaultSuspensionTime = "60"
)

func runUpCommand(ctx context.Context, upType, format string, args ...interface{}) (string, error) {
	var upCmd string
	if upType == UltraPathCommand {
		upCmd = "upadmin"
	} else if upType == UltraPathNVMeCommand {
		upCmd = "upadmin_plus"
	} else {
		return "", utils.Errorln(ctx, "wrong ultraPath type")
	}

	msg := fmt.Sprintf(format, args...)

	return utils.ExecShellCmd(ctx, "%s %s", upCmd, msg)
}

// GetUltraPathInfoByLunWWN to get the ultraPath info by using the lun wwn
func GetUltraPathInfoByLunWWN(ctx context.Context, upType, targetLunWWN string) (string, error) {
	return runUpCommand(ctx, upType, "show vlun | grep -w %s", targetLunWWN)
}

// GetUltraPathInfoByDevName to get the ultraPath info by using the device Name
func GetUltraPathInfoByDevName(ctx context.Context, upType, devName string) (string, error) {
	return runUpCommand(ctx, upType, "show vlun | grep -w %s", devName)
}

// GetUltraPathDetailsByvLunID to get the ultraPath detail info by using vLun Id
func GetUltraPathDetailsByvLunID(ctx context.Context, upType, vLunID string) (string, error) {
	output, err := runUpCommand(ctx, upType, "show vlun id=%s", vLunID)
	if err != nil {
		return "", err
	}

	if output == "" || strings.Contains(output, "does not exist") {
		return "", utils.Errorf(ctx, "The vLun Id %s does not exist.", vLunID)
	}

	return output, nil
}

// GetUltraPathDetailsByPath to get the ultraPath detail info by using device path
func GetUltraPathDetailsByPath(ctx context.Context, upType, device string) (string, error) {
	output, err := runUpCommand(ctx, upType, "show vlun disk=%s", device)
	if err != nil {
		return "", err
	}

	if output == "" || strings.Contains(output, "does not exist") {
		return "", utils.Errorf(ctx, "The device path %s does not exist.", device)
	}

	return output, nil
}

// GetDiskPathAndCheckStatus to get the device path and status based on the LUN WWN.
func GetDiskPathAndCheckStatus(ctx context.Context, upType, lunWWN string) (string, error) {
	diskName, err := GetDiskNameByWWN(ctx, upType, lunWWN)
	if err != nil {
		return "", utils.Errorf(ctx, "volume device not found. error:%v", err)
	}
	diskPath := path.Join("/dev", diskName)

	status, err := getDiskStatusByName(ctx, upType, diskName)
	if err != nil {
		return diskPath, utils.Errorf(ctx, "failed to execute getDiskStatusOfUltraPath. error:%v", err)
	}

	if status != diskStatusNormal {
		if status != diskStatusDegraded {
			return diskPath, utils.Errorf(ctx, "the LUN status is abnormal. status=%s", status)
		}
		log.AddContext(ctx).Warningf("the LUN status is %s.", diskStatusDegraded)
	}

	return diskPath, nil
}

// GetDiskNameByWWN to get the device name based on the LUN WWN
func GetDiskNameByWWN(ctx context.Context, upType, lunWWN string) (string, error) {
	output, err := runUpCommand(ctx, upType, "show vlun | grep %s", lunWWN)
	if err != nil {
		return "", utils.Errorf(ctx, "failed to execute runUpCommand. error:%v output:%s", err, output)
	}

	arr := strings.Fields(output)
	const deviceColumn = 1
	if len(arr) < deviceColumn+1 {
		return "", utils.Errorf(ctx, "failed to find the LUN in UltraPath. lunwwn=%s output:%s",
			lunWWN, output)
	}

	return arr[deviceColumn], nil
}

func getDiskStatusByName(ctx context.Context, upType, diskName string) (string, error) {
	output, err := runUpCommand(ctx, upType, "show vlun disk=%s", diskName)
	if err != nil {
		return "", utils.Errorf(ctx, "failed to execute runUpCommand. error:%v output:%s", err, output)
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	reg := regexp.MustCompile(`^Status[\s]+:[\s]+([\w]+)`)
	for scanner.Scan() {
		line := scanner.Text()
		status := reg.FindAllStringSubmatch(line, -1)
		if status != nil {
			return status[0][1], nil
		}
	}

	return diskStatusUnavailable, utils.Errorf(ctx,
		"failed to query the LUN status. diskName=%s output: %s", diskName, output)
}

// VerifyDeviceAvailableOfUltraPath used to check whether the UltraPath device is available
func VerifyDeviceAvailableOfUltraPath(ctx context.Context, upType, diskName string) (string, error) {
	status, err := getDiskStatusByName(ctx, upType, diskName)
	if err != nil {
		return "", err
	}

	if status != diskStatusNormal {
		if status != diskStatusDegraded {
			return "", utils.Errorf(ctx, "the LUN status is abnormal. status=%s", status)
		}
		log.AddContext(ctx).Warningf("the LUN status is %s.", diskStatusDegraded)
	}

	return path.Join("/dev", diskName), nil
}

// GetLunWWNByDevName used to get device lun wwn by device name
func GetLunWWNByDevName(ctx context.Context, upType, dev string) (string, error) {
	output, err := runUpCommand(ctx, upType, "show vlun disk=%s | grep WWN", dev)
	if err != nil {
		return "", err
	}

	ret := utils.GetValueByRegexp(output, `^LUN WWN[\s]+:[\s]*([\S]+)$`, 1)
	if ret == "" {
		log.AddContext(ctx).Warningf("Get lunWWN by device:%s failed, output:%s", dev, output)
		return "", nil
	}
	return ret, nil
}

// GetDevNameByLunWWN used to get device name by lun wwn
var GetDevNameByLunWWN = func(ctx context.Context, upType, lunWWN string) (string, error) {
	output, err := runUpCommand(ctx, upType, "show vlun | grep -w %s", lunWWN)
	if err != nil {
		return "", err
	}

	ret := utils.GetValueByRegexp(output, `^[\s]*[\d]+[\s]*([\S]+)[\s]+`, 1)
	if ret == "" {
		log.AddContext(ctx).Warningf("Get device by lunWWN:%s failed, output:%s", lunWWN, output)
		return "", nil
	}
	return ret, nil
}

func isTakeOverByUltraPath(ctx context.Context, multipathType, lunWWN string) (bool, error) {
	output, err := GetUltraPathInfoByLunWWN(ctx, multipathType, lunWWN)
	if err != nil {
		// If the grep result is empty, the return value is 1.
		if err.Error() == exitStatus1 && output == "" {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// SetUltraPathIOSuspensionTime used to set IO suspension time
func SetUltraPathIOSuspensionTime(ctx context.Context, upType, vlunID, time string) error {
	_, err := runUpCommand(ctx, upType, "set iosuspensiontime=%s vlun_id=%s", time, vlunID)
	if err != nil {
		log.AddContext(ctx).Errorf("Set UltraPath IO suspension time failed. error:%v", err)
		return err
	}

	return nil
}

// GetUltraPathIOSuspensionTime used to get IO suspension time
func GetUltraPathIOSuspensionTime(ctx context.Context, upType, vlunID string) (string, error) {
	output, err := runUpCommand(ctx, upType, `show upconfig vlun_id=%s | grep "Io Suspension Time"`, vlunID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get UltraPath IO suspension time failed. error:%v", err)
		return "", err
	}
	if output == "" {
		log.AddContext(ctx).Infof("UltraPath IO suspension time not configured, use default value:%s",
			defaultSuspensionTime)
		return defaultSuspensionTime, nil
	}

	ret := utils.GetValueByRegexp(output, `^Io Suspension Time : ([\d]+)`, 1)
	if ret == "" {
		log.AddContext(ctx).Infof("UltraPath IO suspension time not configured, use default value:%s",
			defaultSuspensionTime)
		return defaultSuspensionTime, nil
	}

	return ret, nil
}

func removeUltraPathDeviceCommon(ctx context.Context, virtualDevice string, phyDevices []string) (string, error) {
	devPath := fmt.Sprintf("/dev/%s", virtualDevice)
	err := flushDeviceIO(ctx, devPath)
	if err != nil {
		log.AddContext(ctx).Errorf("Flush %s error: %v", devPath, err)
		return "", err
	}

	// clear the phy device
	for _, phyDevice := range phyDevices {
		if !strings.HasPrefix(phyDevice, "nvme") {
			if err = deletePhysicalDevice(ctx, phyDevice); err != nil {
				log.AddContext(ctx).Warningf("delete physical device %s failed, error is %v", phyDevice, err)
			}
		}
	}
	return "", nil
}

// RemoveUltraPathDevice to remove the ultrapath device through virtual device and physical device
func RemoveUltraPathDevice(ctx context.Context, virtualDevice string, phyDevices []string) error {
	_, err := removeUltraPathDeviceCommon(ctx, virtualDevice, phyDevices)
	if err != nil {
		return err
	}

	err = deleteVirtualDevice(ctx, virtualDevice)
	if err != nil {
		return err
	}

	return nil
}

func setIOSuspensionTimeByPath(ctx context.Context, upDevice string) error {
	vLunID, err := GetVLunIDByDevName(ctx, UltraPathNVMeCommand, upDevice)
	if err != nil {
		return err
	}

	return SetUltraPathIOSuspensionTime(ctx, UltraPathNVMeCommand, vLunID, "0")
}

// RemoveUltraPathNVMeDevice to remove the ultrapath device through virtual device and physical device
func RemoveUltraPathNVMeDevice(ctx context.Context, virtualDevice string, phyDevices []string) error {
	err := setIOSuspensionTimeByPath(ctx, virtualDevice)
	if err != nil {
		return err
	}

	_, err = removeUltraPathDeviceCommon(ctx, virtualDevice, phyDevices)
	return err
}
