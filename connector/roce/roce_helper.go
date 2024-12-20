/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

package roce

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	connutils "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/concurrent"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

type connectorInfo struct {
	tgtPortals         []string
	tgtLunGUID         string
	volumeUseMultiPath bool
	multiPathType      string
}

type shareData struct {
	stopConnecting   atomic.Bool
	numLogin         atomic.Int64
	failedLogin      atomic.Int64
	stoppedThreads   atomic.Int64
	foundDevices     concurrent.Slice[string]
	justAddedDevices concurrent.Slice[string]
}

const (
	connectTimeOut     = 15
	subNqnSegmentCount = 2
)

func parseRoCEInfo(ctx context.Context, connectionProperties map[string]interface{}) (connectorInfo, error) {
	var con connectorInfo
	var err error

	tgtPortals, exist := connectionProperties["tgtPortals"].([]string)
	if !exist {
		return con, utils.Errorln(ctx, "key tgtPortals does not exist in connectionProperties")
	}

	var availablePortals []string
	for _, portal := range tgtPortals {
		_, err = utils.ExecShellCmd(ctx, connector.PingCommand, portal)
		if err != nil {
			log.AddContext(ctx).Errorf("failed to check the host connectivity. %s", portal)
			continue
		}
		availablePortals = append(availablePortals, portal)
	}

	if len(availablePortals) == 0 {
		return con, utils.Errorf(ctx, "No portal available. tgtPortals:%v", tgtPortals)
	}
	con.tgtPortals = availablePortals

	con.tgtLunGUID, exist = connectionProperties["tgtLunGuid"].(string)
	if !exist {
		return con, utils.Errorln(ctx, "key tgtLunGuid does not exist in connectionProperties")
	}

	con.volumeUseMultiPath, con.multiPathType, err = connutils.GetMultiPathInfo(connectionProperties)

	return con, err
}

func getTargetNQN(ctx context.Context, tgtPortal string) (string, error) {
	output, err := utils.ExecShellCmdFilterLog(ctx, "nvme discover -t rdma -a %s", tgtPortal)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot discover nvme target %s, reason: %v", tgtPortal, output)
		return "", err
	}

	var tgtNqn string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "subnqn") {
			splits := strings.SplitN(line, ":", subNqnSegmentCount)
			if len(splits) == subNqnSegmentCount && splits[0] == "subnqn" {
				tgtNqn = strings.Trim(splits[1], " ")
				break
			}
		}
	}

	if tgtNqn == "" {
		return "", errors.New("cannot find nvme target NQN")
	}
	return tgtNqn, nil
}

func connectRoCEPortal(ctx context.Context,
	existSessions map[string]bool,
	tgtPortal, targetNQN string) error {
	if value, exist := existSessions[tgtPortal]; exist && value {
		log.AddContext(ctx).Infof("RoCE target %s has already login, no need login again", tgtPortal)
		return nil
	}

	checkExitCode := []string{"exit status 0", "exit status 70"}
	iSCSICmd := fmt.Sprintf("nvme connect -t rdma -a %s -n %s", tgtPortal, targetNQN)
	output, err := utils.ExecShellCmdFilterLog(ctx, iSCSICmd)
	if strings.Contains(output, "Input/output error") {
		log.AddContext(ctx).Infof("RoCE target %s has already login, no need login again", tgtPortal)
		return nil
	}

	if err != nil {
		if err.Error() == "timeout" {
			return err
		}

		err2 := utils.IgnoreExistCode(err, checkExitCode)
		if err2 != nil {
			log.AddContext(ctx).Warningf("Run %s: output=%s, err=%v", utils.MaskSensitiveInfo(iSCSICmd),
				utils.MaskSensitiveInfo(output), err2)
			return err
		}
	}
	return nil
}

func connectVol(ctx context.Context,
	existSessions map[string]bool,
	tgtPortal, tgtLunGUID string,
	nvmeShareData *shareData) {
	log.AddContext(ctx).Infof("Enter function:connectVol, portal:%s, LunGUID:%s", tgtPortal, tgtLunGUID)
	targetNQN, err := getTargetNQN(ctx, tgtPortal)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot discover nvme target %s, reason: %v", tgtPortal, err)
		nvmeShareData.failedLogin.Add(1)
		nvmeShareData.stoppedThreads.Add(1)
		return
	}

	err = connectRoCEPortal(ctx, existSessions, tgtPortal, targetNQN)
	if err != nil {
		log.AddContext(ctx).Errorf("connect roce portal %s error, reason: %v", tgtPortal, err)
		nvmeShareData.failedLogin.Add(1)
		nvmeShareData.stoppedThreads.Add(1)
		return
	}

	nvmeShareData.numLogin.Add(1)
	var device string
	for i := 1; i < 4; i++ {
		nvmeConnectInfo, err := connector.GetSubSysInfo(ctx)
		if err != nil {
			log.AddContext(ctx).Errorf("Get nvme info error: %v", err)
			break
		}

		device, err = scanRoCEDevice(ctx, nvmeConnectInfo, targetNQN, tgtPortal, tgtLunGUID)
		if err != nil && err.Error() != "FindNoDevice" {
			log.AddContext(ctx).Errorf("Get device of guid %s error: %v", tgtLunGUID, err)
			break
		}
		if device != "" || nvmeShareData.stopConnecting.Load() {
			break
		}

		time.Sleep(time.Second * time.Duration(math.Pow(sleepInternal, float64(i))))
	}

	if device == "" {
		log.AddContext(ctx).Debugf("LUN %s on RoCE portal %s not found on sysfs after logging in.",
			tgtLunGUID, tgtPortal)
	}

	if device != "" {
		nvmeShareData.foundDevices.Append(device)
		nvmeShareData.justAddedDevices.Append(device)
	}

	nvmeShareData.stoppedThreads.Add(1)
	return
}

func scanSingle(ctx context.Context, nvmeShareData *shareData) {
	log.AddContext(ctx).Infoln("Enter function:scanSingle")
	for i := 0; i < 15; i++ {
		if nvmeShareData.foundDevices.Len() != 0 {
			break
		}
		time.Sleep(time.Second * intNumTwo)
	}
}

func getExistSessions(ctx context.Context) (map[string]bool, error) {
	nvmeConnectInfo, err := connector.GetSubSysInfo(ctx)
	if err != nil {
		return nil, err
	}
	log.AddContext(ctx).Infof("All SubSysInfo %v", nvmeConnectInfo)

	subSystems, ok := nvmeConnectInfo["Subsystems"].([]interface{})
	if !ok {
		log.AddContext(ctx).Warningln("there are noSubsystems in the nvmeConnectInfo")
		return nil, nil
	}

	var allSubPaths []interface{}
	for _, s := range subSystems {
		subSystem, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		subPaths, ok := subSystem["Paths"].([]interface{})
		if !ok {
			continue
		}
		allSubPaths = append(allSubPaths, subPaths...)
	}

	existPortals := make(map[string]bool)
	for _, p := range allSubPaths {
		portal, path := getSubPathInfo(ctx, p)
		if portal != "" && path != "" {
			existPortals[portal] = true
		}
	}

	log.AddContext(ctx).Infof("Exist Portals %v", existPortals)
	return existPortals, nil
}

func tryConnectVolume(ctx context.Context, connMap map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("Enter function:tryConnectVolume, param:%v", connMap)
	conn, err := parseRoCEInfo(ctx, connMap)
	if err != nil {
		return "", err
	}

	existSessions, err := getExistSessions(ctx)
	if err != nil {
		return "", err
	}

	var mPath string
	var wait sync.WaitGroup
	var nvmeShareData = new(shareData)
	lenIndex := len(conn.tgtPortals)
	if !conn.volumeUseMultiPath {
		lenIndex = 1
	}
	for index := 0; index < lenIndex; index++ {
		tgtPortal := conn.tgtPortals[index]
		wait.Add(1)

		go func(portal, lunGUID string) {
			defer func() {
				wait.Done()
				if r := recover(); r != nil {
					log.AddContext(ctx).Errorf("Runtime error caught in loop routine: %v", r)
					log.AddContext(ctx).Errorf("%s", debug.Stack())
				}

				log.Flush()
			}()

			connectVol(ctx, existSessions, portal, lunGUID, nvmeShareData)
		}(tgtPortal, conn.tgtLunGUID)
	}

	mPath = scanDevice(ctx, conn, nvmeShareData)

	nvmeShareData.stopConnecting.Store(true)
	wait.Wait()

	return verifyDevice(ctx, conn, nvmeShareData, mPath)
}

func scanDevice(ctx context.Context, conn connectorInfo, nvmeShareData *shareData) string {
	var mPath string
	if conn.volumeUseMultiPath {
		mPath = scanUpNVMeMultiPath(ctx, conn, nvmeShareData)
	} else {
		scanSingle(ctx, nvmeShareData)
	}

	return mPath
}

func scanUpNVMeMultiPath(ctx context.Context, conn connectorInfo, nvmeShareData *shareData) string {
	log.AddContext(ctx).Infof("Enter function:scanUpNVMeMultiPath. connectorInfo:%#v", conn)
	var device string
	var err error
	var timeout int64
	allThread := int64(len(conn.tgtPortals))
	for isThreadNotStoppedOrFoundDevices(allThread, nvmeShareData) &&
		isThreadNotFinishedOrDeviceNotObtained(device, allThread, nvmeShareData) {
		if timeout == 0 && nvmeShareData.foundDevices.Len() != 0 && nvmeShareData.stoppedThreads.Load() == allThread {
			log.AddContext(ctx).Infof("All connection threads finished, "+
				"giving %d seconds for ultrapath* to appear.", connectTimeOut)
			timeout = time.Now().Unix() + connectTimeOut
		} else if timeout != 0 && time.Now().Unix() > timeout {
			log.AddContext(ctx).Infof("scanUpNVMeMultiPath time out. device:%s", device)
			break
		}

		device, err = connector.GetDevNameByLunWWN(ctx, connector.UltraPathNVMeCommand, conn.tgtLunGUID)
		if err != nil {
			log.AddContext(ctx).Warningf("get disk name by wwn failed. error:%v", err)
		}
		time.Sleep(time.Second)
	}

	return device
}

func isThreadNotStoppedOrFoundDevices(allThread int64, nvmeShareData *shareData) bool {
	return nvmeShareData.stoppedThreads.Load() != allThread || nvmeShareData.foundDevices.Len() != 0
}

func isThreadNotFinishedOrDeviceNotObtained(device string, allThread int64, nvmeShareData *shareData) bool {
	return nvmeShareData.numLogin.Load()+nvmeShareData.failedLogin.Load() != allThread || device == ""
}

func verifyDevice(ctx context.Context,
	conn connectorInfo,
	nvmeShareData *shareData,
	mPath string) (string, error) {
	log.AddContext(ctx).Infof("Enter function:verifyDevice, mPath:%s", mPath)
	if nvmeShareData.foundDevices.Values() == nil {
		return "", utils.Errorf(ctx, connector.VolumeDeviceNotFound)
	}

	if !conn.volumeUseMultiPath {
		device := fmt.Sprintf("/dev/%s", nvmeShareData.foundDevices.Get(0))
		err := connector.VerifySingleDevice(ctx, device, conn.tgtLunGUID,
			connector.VolumeDeviceNotFound, tryDisConnectVolume)
		if err != nil {
			log.AddContext(ctx).Errorf("Verify single device:%s failed. error:%v", mPath, err)
			return "", err
		}

		return device, nil
	}

	// mPath: ultrapath*
	if mPath != "" {
		abnormalDev, err := connector.IsUpNVMeResidualPath(ctx, mPath, conn.tgtLunGUID)
		if err != nil {
			log.AddContext(ctx).Errorf("Verify multipath device:%s failed. error:%v", mPath, err)
			return "", err
		}
		if abnormalDev {
			return "", utils.Errorf(ctx, "Verify multipath device:%s failed.", mPath)
		}

		return fmt.Sprintf("/dev/%s", mPath), nil
	}

	log.AddContext(ctx).Errorln("no device was created")
	return "", errors.New(connector.VolumeDeviceNotFound)
}

func getSubSysPaths(ctx context.Context,
	nvmeConnectInfo map[string]interface{},
	targetNqn string) []interface{} {
	subSystems, ok := nvmeConnectInfo["Subsystems"].([]interface{})
	if !ok {
		msg := "there are noSubsystems in the nvmeConnectInfo"
		log.AddContext(ctx).Errorln(msg)
		return nil
	}

	var allSubPaths []interface{}
	for _, s := range subSystems {
		subSystem, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		if strings.Contains(subSystem["NQN"].(string), targetNqn) {
			allSubPaths, ok = subSystem["Paths"].([]interface{})
			if !ok {
				continue
			}
			break
		}
	}

	return allSubPaths
}

func getSubPathInfo(ctx context.Context, p interface{}) (string, string) {
	path, ok := p.(map[string]interface{})
	if !ok {
		return "", ""
	}

	transport, exist := path["Transport"].(string)
	if !exist || transport != "rdma" {
		log.AddContext(ctx).Warningf("Transport does not exist in path %v or Transport value is not rdma.",
			path)
		return "", ""
	}

	state, exist := path["State"].(string)
	if !exist || state != "live" {
		log.AddContext(ctx).Warningf("The state of path %v is not live.", path)
		return "", ""
	}

	address, exist := path["Address"].(string)
	if !exist {
		log.AddContext(ctx).Warningf("Address does not exist in path %v.", path)
		return "", ""
	}

	splitAddress := strings.Split(address, " ")
	for _, addr := range splitAddress {
		splitPortal := strings.Split(addr, "=")
		if len(splitPortal) != intNumTwo {
			continue
		}

		if splitPortal[0] == "traddr" {
			return splitPortal[1], path["Name"].(string)
		}
	}

	log.AddContext(ctx).Warningf("Didn't find portal in path %v", path)
	return "", ""
}

func getSubSysPort(ctx context.Context, subPaths []interface{}, tgtPortal string) string {
	for _, p := range subPaths {
		portal, path := getSubPathInfo(ctx, p)
		if portal != "" && portal == tgtPortal {
			return path
		}
	}
	return ""
}

func scanRoCEDevice(ctx context.Context,
	nvmeConnectInfo map[string]interface{},
	targetNqn, tgtPortal, tgtLunGUID string) (string, error) {
	subPaths := getSubSysPaths(ctx, nvmeConnectInfo, targetNqn)
	devicePort := getSubSysPort(ctx, subPaths, tgtPortal)

	if devicePort == "" {
		msg := fmt.Sprintf("Cannot get nvme device port of portal %s", tgtPortal)
		log.AddContext(ctx).Warningln(msg)
		return "", errors.New(msg)
	}

	err := connector.DoScanNVMeDevice(ctx, devicePort)
	if err != nil {
		return "", err
	}

	return connector.GetNVMeDevice(ctx, devicePort, tgtLunGUID)
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

	sessionPorts, err := getSessionPorts(ctx, phyDevices, devType)
	if err != nil {
		return err
	}

	multiPathName, err := connector.RemoveAllDevice(ctx, virtualDevice, phyDevices, devType)
	if err != nil {
		return err
	}

	err = disconnectSessions(ctx, sessionPorts)
	if err != nil {
		log.AddContext(ctx).Errorf("Disconnect portals %s error: %v",
			utils.MaskSensitiveInfo(sessionPorts), err)
		return err
	}

	if multiPathName != "" {
		time.Sleep(time.Second * intNumThree)
		err = connector.FlushDMDevice(ctx, virtualDevice)
		if err != nil {
			return err
		}
	}

	return nil
}
