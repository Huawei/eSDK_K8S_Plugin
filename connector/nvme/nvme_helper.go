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

package nvme

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	connutils "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/concurrent"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/iputils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

type connectorInfo struct {
	tgtPortals         []string
	tgtLunGUID         string
	volumeUseMultiPath bool
	multiPathType      string
	transport          string
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
	connectTimeOut      = 15
	nvmeScanningTimeout = 15 * time.Second
	watchDeviceInterval = 100 * time.Millisecond
	subNqnSegmentCount  = 2
	pathInfoLength      = 2
	transportOfTcp      = "tcp"
	transportOfRdma     = "rdma"
	tcpPort             = "4420"
)

var transportMap = map[string]string{
	constants.ProtocolRoce:     transportOfRdma,
	constants.ProtocolRoceNVMe: transportOfRdma,
	constants.ProtocolTCPNVMe:  transportOfTcp,
}

func parseNVMeInfo(ctx context.Context, connectionProperties map[string]interface{}) (connectorInfo, error) {
	var con connectorInfo
	var err error

	tgtPortals, exist := connectionProperties["tgtPortals"].([]string)
	if !exist {
		return con, utils.Errorln(ctx, "key tgtPortals does not exist in connectionProperties")
	}

	var availablePortals []string
	for _, portal := range tgtPortals {
		wrapper := iputils.NewIPDomainWrapper(portal)
		if wrapper == nil {
			log.AddContext(ctx).Errorf("Portal [%s] is invalid", portal)
			continue
		}

		_, err = utils.ExecShellCmd(ctx, wrapper.GetPingCommand(), portal)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to check the host connectivity. %s", portal)
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

	protocol, ok := utils.GetValue[string](connectionProperties, "protocol")
	if !ok {
		return con, utils.Errorln(ctx, "protocol is invalid in connectionProperties")
	}
	con.transport, ok = transportMap[protocol]
	if !ok {
		return con, utils.Errorf(ctx, "protocol [%s] is not supported for nvme connector", protocol)
	}

	con.volumeUseMultiPath, con.multiPathType, err = connutils.GetMultiPathInfo(connectionProperties)
	return con, err
}

func getTargetNQN(ctx context.Context, tgtPortal, transport string) (string, error) {
	command := fmt.Sprintf("nvme discover -t %s -a %s", transport, tgtPortal)
	if transport == transportOfTcp {
		command = fmt.Sprintf("%s -s %s", command, tcpPort)
	}

	output, err := utils.ExecShellCmdFilterLog(ctx, command)
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

func connectPortal(ctx context.Context, request connectVolRequest, targetNQN string) error {
	if value, exist := request.existSessions[request.portal]; exist && value {
		log.AddContext(ctx).Infof("NVMe target %s has already login, no need login again", request.portal)
		return nil
	}

	checkExitCode := []string{"exit status 0", "exit status 70"}
	command := fmt.Sprintf("nvme connect -t %s -a %s -n %s", request.transport, request.portal, targetNQN)
	if request.transport == transportOfTcp {
		command = fmt.Sprintf("%s -s %s", command, tcpPort)
	}
	output, err := utils.ExecShellCmdFilterLog(ctx, command)
	if strings.Contains(output, "Input/output error") {
		log.AddContext(ctx).Infof("NVMe target %s has already login, no need login again", request.portal)
		return nil
	}

	if err != nil {
		if err.Error() == "timeout" {
			return err
		}

		ignoredErr := utils.IgnoreExistCode(err, checkExitCode)
		if ignoredErr != nil {
			log.AddContext(ctx).Warningf("Run %s: output=%s, err=%v", utils.MaskSensitiveInfo(command),
				utils.MaskSensitiveInfo(output), ignoredErr)
			return err
		}
	}
	return nil
}

type connectVolRequest struct {
	existSessions map[string]bool
	portal        string
	lunGUID       string
	transport     string
}

func connectVol(ctx context.Context, request connectVolRequest, nvmeShareData *shareData) {
	targetNQN, err := getTargetNQN(ctx, request.portal, request.transport)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot discover nvme target %s, reason: %v", request.portal, err)
		nvmeShareData.failedLogin.Add(1)
		nvmeShareData.stoppedThreads.Add(1)
		return
	}

	if app.GetGlobalConfig().EnableRoCEConnect {
		err = connectPortal(ctx, request, targetNQN)
		if err != nil {
			log.AddContext(ctx).Errorf("Connect NVMe portal %s error, reason: %v", request.portal, err)
			nvmeShareData.failedLogin.Add(1)
			nvmeShareData.stoppedThreads.Add(1)
			return
		}
	}

	nvmeShareData.numLogin.Add(1)
	var device string
	for i := 1; i < 4; i++ {
		nvmeConnectInfo, err := connector.GetSubSysInfo(ctx)
		if err != nil {
			log.AddContext(ctx).Errorf("Get nvme info error: %v", err)
			break
		}

		device, err = scanNVMeDevice(ctx, nvmeConnectInfo, targetNQN, request.portal, request.lunGUID)
		if err != nil && err.Error() != "FindNoDevice" {
			log.AddContext(ctx).Errorf("Get device of guid %s error: %v", request.lunGUID, err)
			break
		}
		if device != "" || nvmeShareData.stopConnecting.Load() {
			break
		}

		time.Sleep(time.Second * time.Duration(math.Pow(sleepInternal, float64(i))))
	}

	if device == "" {
		log.AddContext(ctx).Debugf("LUN %s on NVMe portal %s not found on sysfs after logging in.",
			request.lunGUID, request.portal)
	}

	if device != "" {
		nvmeShareData.foundDevices.Append(device)
		nvmeShareData.justAddedDevices.Append(device)
	}

	nvmeShareData.stoppedThreads.Add(1)
}

func findSinglePath(ctx context.Context, nvmeShareData *shareData) {
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
		log.AddContext(ctx).Warningln("There are noSubsystems in the nvmeConnectInfo")
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
		portal, nvmePath := getSubPathInfo(ctx, p)
		if portal != "" && nvmePath != "" {
			existPortals[portal] = true
		}
	}

	log.AddContext(ctx).Infof("Exist Portals %v", existPortals)
	return existPortals, nil
}

func tryConnectVolume(ctx context.Context, connMap map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("Enter function:tryConnectVolume, param:%v", connMap)
	conn, err := parseNVMeInfo(ctx, connMap)
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

		go func(portal, lunGUID, transport string) {
			defer func() {
				wait.Done()
				if r := recover(); r != nil {
					log.AddContext(ctx).Errorf("Runtime error caught in loop routine: %v", r)
					log.AddContext(ctx).Errorf("%s", debug.Stack())
				}

				log.Flush()
			}()

			connectVol(ctx, connectVolRequest{
				existSessions: existSessions,
				portal:        portal,
				lunGUID:       lunGUID,
				transport:     transport,
			}, nvmeShareData)
		}(tgtPortal, conn.tgtLunGUID, conn.transport)
	}

	mPath, err = findDevice(ctx, conn, nvmeShareData)
	if err != nil {
		return "", err
	}

	nvmeShareData.stopConnecting.Store(true)
	wait.Wait()

	return verifyDevice(ctx, conn, nvmeShareData, mPath)
}

func findDevice(ctx context.Context, conn connectorInfo, nvmeShareData *shareData) (string, error) {
	if !conn.volumeUseMultiPath {
		findSinglePath(ctx, nvmeShareData)
		return "", nil
	}

	switch conn.multiPathType {
	case connector.HWUltraPathNVMe:
		return findDiskOfUltraPath(ctx, conn, nvmeShareData), nil
	case connector.NVMeNative:
		if !connector.IsNVMeMultipathEnabled(ctx) {
			return "", errors.New("NVMe-Native multipath is not enabled")
		}
		return findDiskOfNativePath(ctx, conn, nvmeShareData), nil
	default:
		return "", fmt.Errorf("multipath type %s is not supported for nvme connector", conn.multiPathType)
	}
}

func findDiskOfUltraPath(ctx context.Context, conn connectorInfo, nvmeShareData *shareData) string {
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
			log.AddContext(ctx).Infof("findDiskOfUltraPath time out. device:%s", device)
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

func findDiskOfNativePath(ctx context.Context, conn connectorInfo, nvmeShareData *shareData) string {
	var device string
	allThread := int64(len(conn.tgtPortals))
	err := utils.WaitUntil(func() (bool, error) {
		// break while all thread down but no device found, or find device with all portals connected.
		if allThreadDownButNoDeviceFound(allThread, nvmeShareData) ||
			findDeviceWithAllPortalsConnected(device, allThread, nvmeShareData) {
			return true, nil
		}

		var err error
		device, err = connector.GetNVMeDiskByGuid(ctx, conn.tgtLunGUID)
		if err != nil {
			log.AddContext(ctx).Warningf("Get NVMe device by guid failed, err: %v", err)
			return false, nil
		}

		return true, nil
	}, nvmeScanningTimeout, time.Second)

	if err != nil {
		log.AddContext(ctx).Errorf("Failed to find disk of NVMe-Native multipath, err: %v", err)
	}

	return device
}

func allThreadDownButNoDeviceFound(allThread int64, nvmeShareData *shareData) bool {
	return allThread == nvmeShareData.stoppedThreads.Load() && nvmeShareData.foundDevices.Len() == 0
}

func findDeviceWithAllPortalsConnected(device string, allThread int64, nvmeShareData *shareData) bool {
	return device != "" && allThread == nvmeShareData.numLogin.Load()+nvmeShareData.failedLogin.Load()
}

func verifyDevice(ctx context.Context, conn connectorInfo, nvmeShareData *shareData, disk string) (string, error) {
	if !conn.volumeUseMultiPath {
		return checkSinglePathAvailable(ctx, nvmeShareData, conn.tgtLunGUID)
	}

	if disk == "" {
		return "", utils.Errorln(ctx, connector.VolumeNotFound)
	}

	switch conn.multiPathType {
	case connector.HWUltraPathNVMe:
		return checkUltraPathAvailable(ctx, disk, conn.tgtLunGUID)
	case connector.NVMeNative:
		return checkNativePathAvailable(ctx, disk, conn, int(nvmeShareData.numLogin.Load()))
	default:
		return "", fmt.Errorf("multiPathType %s is not supported for nvme connector", conn.multiPathType)
	}
}

func checkSinglePathAvailable(ctx context.Context, nvmeShareData *shareData, guid string) (string, error) {
	if nvmeShareData.foundDevices.Len() == 0 {
		return "", errors.New(connector.VolumeNotFound)
	}

	device := fmt.Sprintf("/dev/%s", nvmeShareData.foundDevices.Get(0))
	err := connector.VerifySingleDevice(ctx, device, guid, connector.VolumeNotFound, tryDisConnectVolume)
	if err != nil {
		return "", err
	}

	return device, nil
}

func checkUltraPathAvailable(ctx context.Context, disk, guid string) (string, error) {
	abnormalDev, err := connector.IsUpNVMeResidualPath(ctx, disk, guid)
	if err != nil {
		return "", fmt.Errorf("verify Ultra-NVMe multipath device: %s failed. err: %w", disk, err)
	}
	if abnormalDev {
		return "", utils.Errorf(ctx, "Verify multipath device:%s failed.", disk)
	}

	return path.Join("/dev", disk), nil
}

func checkNativePathAvailable(ctx context.Context, disk string, conn connectorInfo, expectPathNum int) (string, error) {
	if app.GetGlobalConfig().AllPathOnline {
		err := waitAllPathOnline(ctx, disk, conn.tgtPortals, expectPathNum)
		if err != nil {
			return "", err
		}
	}

	err := connector.CheckIsTakeOverByNVMeNative(ctx, disk)
	if err != nil {
		return "", err
	}

	return path.Join("/dev", disk), nil
}

func getSubSysPaths(ctx context.Context, nvmeConnectInfo map[string]interface{}, targetNqn string) []interface{} {
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
	nvmePath, ok := p.(map[string]interface{})
	if !ok {
		return "", ""
	}

	transport, exist := nvmePath["Transport"].(string)
	if !exist || transport != transportOfRdma && transport != transportOfTcp {
		log.AddContext(ctx).Warningf("Transport does not exist in path %v or Transport value is not invalid", nvmePath)
		return "", ""
	}

	state, exist := nvmePath["State"].(string)
	if !exist || state != "live" {
		log.AddContext(ctx).Warningf("The state of path %v is not live.", nvmePath)
		return "", ""
	}

	address, exist := nvmePath["Address"].(string)
	if !exist {
		log.AddContext(ctx).Warningf("Address does not exist in path %v.", nvmePath)
		return "", ""
	}

	splitAddress := strings.FieldsFunc(address, func(r rune) bool {
		// nvme client v2,x separator is ','
		// nvme client v1,x separator is ' '
		return r == ',' || r == ' '
	})

	for _, addr := range splitAddress {
		splitPortal := strings.Split(addr, "=")
		if len(splitPortal) != intNumTwo {
			continue
		}

		if splitPortal[0] == "traddr" {
			pathName, _ := utils.GetValue[string](nvmePath, "Name")
			return splitPortal[1], pathName
		}
	}

	log.AddContext(ctx).Warningf("Didn't find portal in path %v", nvmePath)
	return "", ""
}

func getSubSysPort(ctx context.Context, subPaths []interface{}, tgtPortal string) string {
	for _, p := range subPaths {
		portal, nvmePath := getSubPathInfo(ctx, p)
		if portal != "" && portal == tgtPortal {
			return nvmePath
		}
	}
	return ""
}

func scanNVMeDevice(ctx context.Context,
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

func waitAllPathOnline(ctx context.Context, device string, portals []string, expectNum int) error {
	return utils.WaitUntil(func() (bool, error) {
		log.AddContext(ctx).Infof("Start to watch subpath, device:%s, portals:%v, expectNum:%d",
			device, portals, expectNum)
		nvmeConnectInfo, err := connector.GetSubSysInfo(ctx)
		if err != nil {
			return false, err
		}

		pathMap := getPathInfos(ctx, nvmeConnectInfo)
		pathNum := 0
		for _, portal := range portals {
			pathInfo, ok := pathMap[portal]
			if !ok {
				continue
			}

			if connector.IsNVMeSubPathExist(ctx, pathInfo[0], pathInfo[1], device) {
				pathNum++
			}
		}

		return pathNum >= expectNum, nil
	}, time.Duration(app.GetGlobalConfig().ScanVolumeTimeout)*time.Second, watchDeviceInterval)
}

func getPathInfos(ctx context.Context, nvmeConnectInfo map[string]interface{}) map[string][pathInfoLength]string {
	subSystems, ok := nvmeConnectInfo["Subsystems"].([]interface{})
	if !ok {
		log.AddContext(ctx).Errorln("There are noSubsystems in the nvmeConnectInfo")
		return map[string][pathInfoLength]string{}
	}

	result := make(map[string][pathInfoLength]string)
	for _, s := range subSystems {
		subSystem, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		subSystemName, ok := utils.GetValue[string](subSystem, "Name")
		if !ok {
			continue
		}

		subPaths, ok := utils.GetValue[[]interface{}](subSystem, "Paths")
		if !ok {
			continue
		}

		for _, p := range subPaths {
			portal, pathName := getSubPathInfo(ctx, p)
			pathInfo := [pathInfoLength]string{subSystemName, pathName}
			result[portal] = pathInfo
		}
	}

	return result
}
