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

// Package nvme provide the way to connect/disconnect volume within FC NVMe protocol
package nvme

import (
	"context"
	"path"
	"strings"
	"time"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

var flushTimeInterval = 3 * time.Second

// PortWWNPair contains initiator wwn and target wwn
type PortWWNPair struct {
	InitiatorPortWWN string
	TargetPortWWN    string
}

type connectorInfo struct {
	tgtLunGUID         string
	volumeUseMultiPath bool
	multiPathType      string
	portWWNList        []PortWWNPair
}

func tryDisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	virtualDevice, devType, err := connector.GetVirtualDevice(ctx, tgtLunWWN)
	if err != nil {
		log.AddContext(ctx).Errorf("Get device of WWN %s error: %v", tgtLunWWN, err)
		return err
	}

	if virtualDevice == "" {
		log.AddContext(ctx).Infof("The device of WWN %s does not exist on host", tgtLunWWN)
		return nil
	}

	phyDevices, err := connector.GetNVMePhysicalDevices(ctx, virtualDevice, devType)
	if err != nil {
		return err
	}

	multiPathName, err := connector.RemoveAllDevice(ctx, virtualDevice, phyDevices, devType)
	if err != nil {
		return err
	}

	if multiPathName != "" {
		time.Sleep(flushTimeInterval)
		err = connector.FlushDMDevice(ctx, virtualDevice)
		if err != nil {
			return err
		}
	}

	return nil
}

func tryConnectVolume(ctx context.Context, connMap map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("Enter function:tryConnectVolume, param:%v", connMap)

	conn, err := parseFCNVMeInfo(ctx, connMap)
	if err != nil {
		return "", err
	}

	channels, err := getAllChannel(ctx, conn)
	if err != nil {
		return "", err
	}

	err = scanDevice(ctx, channels)
	if err != nil {
		return "", err
	}

	virtualDevice, err := getVirtualDevice(ctx, conn, channels)
	if err != nil {
		return "", err
	}

	return path.Join("/dev/", virtualDevice), nil
}

func getVirtualDevice(ctx context.Context, conn connectorInfo, channels []string) (string, error) {
	var virtualDevice string
	var err error
	if conn.volumeUseMultiPath {
		virtualDevice, err = getVirtualDeviceUseMultipath(ctx, conn)
	} else {
		virtualDevice, err = connector.GetNVMeDevice(ctx, channels[0], conn.tgtLunGUID)
	}

	if err != nil || virtualDevice == "" {
		return "", utils.Errorf(ctx, "Get virtual device failed. device:%s, error:%v", virtualDevice, err)
	}
	return virtualDevice, nil
}

func getVirtualDeviceUseMultipath(ctx context.Context, conn connectorInfo) (string, error) {
	var virtualDevice string
	var err error
	for i := 0; i < 5; i++ {
		virtualDevice, err = connector.GetDevNameByLunWWN(ctx, connector.UltraPathNVMeCommand, conn.tgtLunGUID)
		if err != nil && virtualDevice != "" {
			log.AddContext(ctx).Errorf("Get virtual device failed. error:%v", err)
			return "", err
		}
		if virtualDevice != "" {
			abnormalDev, err := connector.IsUpNVMeResidualPath(ctx, virtualDevice, conn.tgtLunGUID)
			if err != nil {
				log.AddContext(ctx).Warningf("Verify fc-nvme device:%s failed. error:%v", virtualDevice, err)
				continue
			}
			if abnormalDev {
				log.AddContext(ctx).Warningf("Verify fc-nvme device:%s failed.", virtualDevice)
				continue
			}
			return virtualDevice, nil
		}

		time.Sleep(time.Second)
	}
	log.AddContext(ctx).Warningln("Get virtual device failed.")
	return virtualDevice, nil
}

func getAllChannel(ctx context.Context, conn connectorInfo) ([]string, error) {
	nvmeConnectInfo, err := connector.GetSubSysInfo(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Get nvme info error: %v", err)
		return nil, err
	}

	subSystems, ok := nvmeConnectInfo["Subsystems"].([]interface{})
	if !ok {
		return nil, utils.Errorln(ctx, "there are noSubsystems in the nvmeConnectInfo")
	}

	subSystemMap := getSubSystemsMapData(ctx, subSystems)
	allChannel := getChannels(ctx, conn, subSystemMap)
	if len(allChannel) == 0 {
		return nil, utils.Errorln(ctx, "Find channels failed.")
	}

	log.AddContext(ctx).Infof("Get channels:%v", allChannel)
	return allChannel, nil
}

func scanDevice(ctx context.Context, channels []string) error {
	for _, channel := range channels {
		err := connector.DoScanNVMeDevice(ctx, channel)
		if err != nil {
			log.AddContext(ctx).Errorf("scan nvme port:%s failed. error:%v", channel, err)
			return err
		}
	}
	return nil
}

func getSubSystemsMapData(ctx context.Context, subSystems []interface{}) map[string]string {
	subSystemMap := make(map[string]string)
	for _, s := range subSystems {
		subSystem, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		allSubPaths, ok := subSystem["Paths"].([]interface{})
		if !ok {
			continue
		}

		for _, s := range allSubPaths {
			subPath, ok := s.(map[string]interface{})
			if !ok {
				continue
			}

			transport, exist := subPath["Transport"].(string)
			if !exist || transport != "fc" {
				log.AddContext(ctx).Warningf("Transport does not exist in path:%v or Transport value is not fc.",
					subPath)
				continue
			}
			state, exist := subPath["State"].(string)
			if !exist || state != "live" {
				log.AddContext(ctx).Warningf("State does not exist in path:%v or state is not live.", subPath)
				continue
			}
			channelName, exist := subPath["Name"].(string)
			if !exist {
				log.AddContext(ctx).Warningf("Name does not exist in path:%v.", subPath)
				continue
			}
			address, exist := subPath["Address"].(string)
			if !exist {
				log.AddContext(ctx).Warningf("Name does not exist in path:%v.", subPath)
				continue
			}
			subSystemMap[channelName] = address
		}
	}

	return subSystemMap
}

func getChannels(ctx context.Context, conn connectorInfo, subSystemMap map[string]string) []string {
	var ret []string
	for _, portWWN := range conn.portWWNList {
		channel := getChannel(ctx, subSystemMap, portWWN.InitiatorPortWWN, portWWN.TargetPortWWN)
		if channel != "" {
			ret = append(ret, channel)
		}
	}

	return ret
}

func getChannel(ctx context.Context, subSystemMap map[string]string, storageWWPN, hostWWPN string) string {
	for channelName, address := range subSystemMap {
		if strings.Contains(address, storageWWPN) && strings.Contains(address, hostWWPN) {
			return channelName
		}
	}
	log.AddContext(ctx).Warningf("Get channel name failed by storageWWPN:%s hostWWPN:%s from %v",
		storageWWPN, hostWWPN, subSystemMap)
	return ""
}

func parseFCNVMeInfo(ctx context.Context, connectionProperties map[string]interface{}) (connectorInfo, error) {
	var con connectorInfo
	var exist bool
	var err error

	con.tgtLunGUID, exist = connectionProperties["tgtLunGuid"].(string)
	if !exist {
		return con, utils.Errorln(ctx, "key tgtLunGuid does not exist in connectionProperties")
	}

	con.volumeUseMultiPath, exist = connectionProperties["volumeUseMultiPath"].(bool)
	if !exist {
		return con, utils.Errorln(ctx, "there is no multiPath switch in the connection info")
	}

	con.multiPathType, exist = connectionProperties["multiPathType"].(string)
	if !exist {
		return con, utils.Errorln(ctx, "the connection information does not contain multiPathType")
	}

	con.portWWNList, exist = connectionProperties["portWWNList"].([]PortWWNPair)
	if !exist {
		return con, utils.Errorln(ctx, "key tgtWWNs does not exist in connectionProperties")
	}

	return con, err
}
