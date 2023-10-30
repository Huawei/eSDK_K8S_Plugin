/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

// Package connector is a package that used to operate host devices
package connector

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	nxupLunMapFile            = "/proc/nxup_lun_map_a"
	nxupLunMapFileWithUpgrade = "/proc/nxup_lun_map_b"
	deviceDelete              = "deleted"
)

type ultrapathDeviceInfo struct {
	deviceId   string
	deviceName string
	upHctl     string
	hctls      []string
}

type ultrapathDeviceTuple struct {
	name string
	id   string
}

// CleanDeviceByLunId clean device by Lun id
func CleanDeviceByLunId(ctx context.Context, lunId string, targets []string) error {
	log.AddContext(ctx).Infof("start checking for device with the same lunId, current lunId: %s", lunId)
	devices, err := ReadNxupMap(ctx, lunId)
	if err != nil {
		log.AddContext(ctx).Infof("read nxup map failed, lunId:%s, err: %v", lunId, err)
		return err
	}
	if len(devices) == 0 {
		log.AddContext(ctx).Infof("no devices with the same lunId were found in nxup, skipping")
		return nil
	}

	arrays, err := FindAllArrays(ctx)
	if err != nil {
		log.AddContext(ctx).Infof("find all arrays failed, err: %v", err)
		return err
	}
	if len(arrays) == 0 {
		log.AddContext(ctx).Infof("no arrays found, skipping")
		return nil
	}

	arrayId, err := FindArrayByTarget(ctx, targets, arrays)
	if err != nil {
		log.AddContext(ctx).Infof("find array by iqn failed, arrayId: %s, err: %v", arrayId, err)
		return err
	}

	var needCleanDevice ultrapathDeviceInfo
	for _, device := range devices {
		if needClean(ctx, arrayId, device) {
			log.AddContext(ctx).Infof("found need clean device, device: %s, "+
				"deviceId: %s, arrayId: %s", device.deviceName, device.deviceId, arrayId)
			needCleanDevice = device
			break
		} else {
			log.AddContext(ctx).Infof("device[%s] is normal in array[%s]", device.deviceId, arrayId)
		}
	}

	if needCleanDevice.deviceName == "" || len(needCleanDevice.hctls) == 0 {
		log.AddContext(ctx).Infof("not found abnormal device,arrayId: %s", arrayId)
		return nil
	}

	_, err = RemoveAllDevice(ctx, needCleanDevice.deviceName, needCleanDevice.hctls, UseUltraPath)
	if err != nil {
		log.AddContext(ctx).Errorf("remove device failed, error:%v", err)
	}

	if needCleanDevice.deviceName == deviceDelete {
		log.AddContext(ctx).Infoln("device name deleted, waiting 5s")
		time.Sleep(5 * time.Second)
	}

	return nil
}

func needClean(ctx context.Context, arrayId string, device ultrapathDeviceInfo) bool {
	if !deviceIsInArray(ctx, device.deviceId, arrayId) {
		return false
	}

	if device.deviceName == deviceDelete {
		return true
	}

	if !deviceIsNormal(ctx, device.deviceId) {
		return true
	}
	return false
}

// ReadNxupFile read nxup file
func ReadNxupFile(ctx context.Context) (string, error) {
	var content []byte
	var err error
	for _, filePath := range []string{nxupLunMapFile, nxupLunMapFileWithUpgrade} {
		content, err = ioutil.ReadFile(filePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			log.AddContext(ctx).Infof("read up device map failed, fileName: %s, error: %v", filePath, err)
			return "", err
		}
		break
	}

	if len(content) == 0 {
		return "", errors.New("unable to find files nxup_lun_map_a and nxup_lun_map_b in the /proc directory")
	}

	return string(content), nil
}

// ReadNxupMap read nxup map
func ReadNxupMap(ctx context.Context, lunId string) ([]ultrapathDeviceInfo, error) {
	content, err := ReadNxupFile(ctx)
	if err != nil {
		return nil, err
	}

	deviceHctlMap := make(map[string][]string)
	deviceTupleMap := make(map[string]ultrapathDeviceTuple)
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			parseDevice(line, lunId, deviceHctlMap, deviceTupleMap)
		}
	}

	var devices []ultrapathDeviceInfo
	for upHctl, hctls := range deviceHctlMap {
		if deviceTuple, ok := deviceTupleMap[upHctl]; ok {
			devices = append(devices, ultrapathDeviceInfo{
				deviceId:   deviceTuple.id,
				deviceName: deviceTuple.name,
				hctls:      hctls,
			})
		}
	}
	return devices, nil
}

func parseDevice(line string, lunId string, deviceHctlMap map[string][]string,
	deviceTupleMap map[string]ultrapathDeviceTuple) {
	if deviceTupleMap == nil {
		return
	}
	splitValue := strings.Split(line, "=")
	if len(splitValue) > 5 {
		hctl := strings.Split(splitValue[3], ":")
		if len(hctl) == 4 && hctl[3] == lunId {
			addToMap(deviceHctlMap, splitValue[2], splitValue[3])
			deviceTupleMap[splitValue[2]] = ultrapathDeviceTuple{name: splitValue[4], id: splitValue[1]}
		}
	}
}

func addToMap(source map[string][]string, key, value string) {
	if source == nil {
		return
	}
	values, ok := source[key]
	if !ok {
		values = []string{}
	}
	values = append(values, value)
	source[key] = values
}

// FindAllArrays find all arrays
func FindAllArrays(ctx context.Context) ([]string, error) {
	output, err := utils.ExecShellCmd(ctx, "upadmin show array")
	if err != nil {
		return nil, err
	}

	var arrays []string
	for _, line := range strings.Split(output, "\n") {
		if id := findFirstNumber(line); id != "" {
			arrays = append(arrays, id)
		}
	}
	return arrays, nil
}

// FindArrayByTarget find array by iqn
func FindArrayByTarget(ctx context.Context, targets, arrayIds []string) (string, error) {
	for _, target := range targets {
		for _, id := range arrayIds {
			output, err := utils.ExecShellCmd(ctx, "upadmin show path array_id=%s | grep %s", id, target)
			if err == nil && len(output) != 0 {
				return id, nil
			}
		}
	}

	return "", errors.New("unable to find any array containing target")
}

func deviceIsNormal(ctx context.Context, deviceId string) bool {
	output, err := utils.ExecShellCmd(ctx,
		"upadmin show vlun id=%s | grep -w Status", deviceId)
	if err != nil {
		log.AddContext(ctx).Infof("run show vlun with status failed, error:%v", err)
		return true
	}
	return strings.Contains(output, "Normal")
}

// FindAllArrays find all arrays
func deviceIsInArray(ctx context.Context, deviceId, arrayId string) bool {
	output, err := utils.ExecShellCmd(ctx, "upadmin show vlun array_id=%s", arrayId)
	if err != nil {
		log.AddContext(ctx).Infof("run show vlun with array failed, error:%v", err)
		return false
	}

	for _, line := range strings.Split(output, "\n") {
		if findFirstNumber(line) == deviceId {
			return true
		}
	}
	return false
}

func findFirstNumber(line string) string {
	pattern := regexp.MustCompile("^\\s*(\\d+)\\s+")
	for _, line := range strings.Split(line, "\n") {
		ret := pattern.FindAllStringSubmatch(line, -1)
		if len(ret) != 0 && len(ret[0]) > 1 {
			return ret[0][1]
		}
	}
	return ""
}
