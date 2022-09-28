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

package iscsi

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"huawei-csi-driver/connector"
	connutils "huawei-csi-driver/connector/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type chapInfo struct {
	authUserName string
	authPassword string
	authMethod   string
}

type connectorInfo struct {
	tgtLunWWN   string
	tgtPortals  []string
	tgtIQNs     []string
	tgtHostLUNs []string

	tgtChapInfo        chapInfo
	volumeUseMultiPath bool
	multiPathType      string
}

type singleConnectorInfo struct {
	tgtPortal  string
	tgtIQN     string
	tgtHostLun string
}

type shareData struct {
	stopConnecting   bool
	numLogin         int64
	failedLogin      int64
	stoppedThreads   int64
	foundDevices     []string
	justAddedDevices []string
}

type scanRequest struct {
	sessionId            string
	tgtHostLun           string
	tgtLunWWN            string
	hostChannelTargetLun []string
	iSCSIShareData       *shareData
}

func parseISCSIInfo(ctx context.Context,
	connectionProperties map[string]interface{}) (connectorInfo, error) {
	var info connectorInfo
	var exist bool
	var err error

	info.tgtLunWWN, exist = connectionProperties["tgtLunWWN"].(string)
	if !exist {
		return info, errors.New("key tgtLunWWN does not exist in connectionProperties")
	}

	info.tgtPortals, exist = connectionProperties["tgtPortals"].([]string)
	if !exist {
		return info, errors.New("key tgtPortals does not exist in connectionProperties")
	}

	info.tgtIQNs, exist = connectionProperties["tgtIQNs"].([]string)
	if !exist {
		return info, errors.New("key tgtIQNs does not exist in connectionProperties")
	}

	info.tgtHostLUNs, exist = connectionProperties["tgtHostLUNs"].([]string)
	if !exist {
		return info, errors.New("key tgtHostLUNs does not exist in connectionProperties")
	}

	if len(info.tgtPortals) != len(info.tgtIQNs) || len(info.tgtPortals) != len(info.tgtHostLUNs) {
		return info, fmt.Errorf("the numbers of tgtPortals, targetIQNs and tgtHostLUNs are not equal. %d %d %d",
			len(info.tgtPortals), len(info.tgtIQNs), len(info.tgtHostLUNs))
	}

	info.tgtChapInfo.authUserName, exist = connectionProperties["authUserName"].(string)
	if !exist {
		log.AddContext(ctx).Infoln("key authUserName does not exist in connectionProperties")
	}

	info.tgtChapInfo.authPassword, exist = connectionProperties["authPassword"].(string)
	if !exist {
		log.AddContext(ctx).Infoln("key authPassword does not exist in connectionProperties")
	}

	info.tgtChapInfo.authMethod, exist = connectionProperties["authMethod"].(string)
	if !exist {
		log.AddContext(ctx).Infoln("key authMethod does not exist in connectionProperties")
	}

	info.volumeUseMultiPath, info.multiPathType, err = connutils.GetMultiPathInfo(connectionProperties)

	return info, err
}

func runISCSIAdmin(ctx context.Context,
	tgtPortal, targetIQN string,
	iSCSICommand string,
	checkExitCode []string) error {
	iSCSICmd := fmt.Sprintf("iscsiadm -m node -T %s -p %s %s", targetIQN, tgtPortal, iSCSICommand)
	output, err := utils.ExecShellCmdFilterLog(ctx, iSCSICmd)
	if err != nil {
		if err.Error() == "timeout" {
			return err
		}

		err2 := utils.CheckExistCode(err, checkExitCode)
		if err2 != nil {
			log.AddContext(ctx).Warningf("Run %s: output=%s, err=%v", utils.MaskSensitiveInfo(iSCSICmd),
				utils.MaskSensitiveInfo(output), err2)
			return err
		}
	}

	return nil
}

func runISCSIBare(ctx context.Context, iSCSICommand string, checkExitCode []string) (string, error) {
	iSCSICmd := fmt.Sprintf("iscsiadm %s", iSCSICommand)
	output, err := utils.ExecShellCmdFilterLog(ctx, iSCSICmd)
	if err != nil {
		if err.Error() == "timeout" {
			return "", err
		}

		err2 := utils.CheckExistCode(err, checkExitCode)
		if err2 != nil {
			log.AddContext(ctx).Errorf("Run bare %s: output=%s, err=%v",
				utils.MaskSensitiveInfo(iSCSICmd),
				utils.MaskSensitiveInfo(output), err2)
			return "", err
		}
	}

	return output, nil
}

func updateISCSIAdminWithExitCode(ctx context.Context,
	tgtPortal, targetIQN string,
	iscsiCMD string,
	checkExitCode []string) error {
	return runISCSIAdmin(ctx, tgtPortal, targetIQN, iscsiCMD, checkExitCode)
}

func iscsiCMD(propertyKey, propertyValue string) string {
	return fmt.Sprintf("--op update -n %s -v %s", propertyKey, propertyValue)
}

func updateISCSIAdmin(ctx context.Context,
	tgtPortal, targetIQN string,
	propertyKey, propertyValue string) error {
	iSCSICmd := fmt.Sprintf("--op update -n %s -v %s", propertyKey, propertyValue)
	return runISCSIAdmin(ctx, tgtPortal, targetIQN, iSCSICmd, nil)
}

func updateChapInfo(ctx context.Context, tgtPortal, targetIQN string, tgtChapInfo chapInfo) error {
	if tgtChapInfo.authMethod != "" {
		err := updateISCSIAdmin(ctx, tgtPortal, targetIQN,
			"node.session.auth.authmethod", tgtChapInfo.authMethod)
		if err != nil {
			log.AddContext(ctx).Errorf("Update node session auth method %s error, reason: %v",
				tgtChapInfo.authMethod, err)
			return err
		}

		err = updateISCSIAdmin(ctx, tgtPortal, targetIQN,
			"node.session.auth.username", tgtChapInfo.authUserName)
		if err != nil {
			log.AddContext(ctx).Errorf("Update node session auth username %s error, reason: %v",
				tgtChapInfo.authUserName, err)
			return err
		}

		err = updateISCSIAdmin(ctx, tgtPortal, targetIQN,
			"node.session.auth.password", tgtChapInfo.authPassword)
		if err != nil {
			log.AddContext(ctx).Errorf("Update node session auth password %s error, reason: %v",
				utils.MaskSensitiveInfo(tgtChapInfo.authPassword), err)
			return err
		}
	}
	return nil
}

func getAllISCSISession(ctx context.Context) [][]string {
	checkExitCode := []string{"exit status 0", "exit status 21", "exit status 255"}
	allSessions, err := runISCSIBare(ctx, "-m session", checkExitCode)
	if err != nil {
		log.AddContext(ctx).Warningf("Get all iSCSI session error , reason: %v", err)
		return nil
	}

	var iSCSIInfo [][]string
	for _, iscsi := range strings.Split(allSessions, "\n") {
		if iscsi != "" {
			splitInfo := strings.Split(iscsi, " ")
			if len(splitInfo) < 4 {
				log.AddContext(ctx).Warningf("iscsi session %s error", splitInfo)
				continue
			}

			sid := splitInfo[1][1 : len(splitInfo[1])-1]
			tgtInfo := strings.Split(splitInfo[2], ",")
			if len(tgtInfo) < 2 {
				continue
			}
			portal, tpgt := tgtInfo[0], tgtInfo[1]
			iSCSIInfo = append(iSCSIInfo, []string{splitInfo[0], sid, portal, tpgt, splitInfo[3]})
		}
	}

	return iSCSIInfo
}

func connectISCSIPortal(ctx context.Context,
	tgtPortal, targetIQN string,
	tgtChapInfo chapInfo) (string, bool) {
	checkExitCode := []string{"exit status 0", "exit status 21", "exit status 255"}
	// If the host already discovery the target, we do not need to run --op new.
	// Therefore, we check to see if the target exists, and if we get 255(Not Found), should run --op new.
	// It will return 21 for No records Found after version 2.0-871
	err := runISCSIAdmin(ctx, tgtPortal, targetIQN, "", checkExitCode)
	if err != nil {
		if err.Error() == "timeout" {
			return "", false
		}

		err := runISCSIAdmin(ctx, tgtPortal, targetIQN,
			"--interface default --op new", nil)
		if err != nil {
			log.AddContext(ctx).Errorf("Create new portal %s error , reason: %v", tgtPortal, err)
			return "", false
		}
	}

	var manualScan bool
	err = updateISCSIAdmin(ctx, tgtPortal, targetIQN, "node.session.scan", "manual")
	if err != nil {
		log.AddContext(ctx).Warningf("Update node session scan mode to manual error, reason: %v",
			tgtPortal, err)
	}
	manualScan = err == nil

	err = updateChapInfo(ctx, tgtPortal, targetIQN, tgtChapInfo)
	if err != nil {
		log.AddContext(ctx).Errorf("Update chap %s error, reason: %v",
			utils.MaskSensitiveInfo(tgtChapInfo), err)
		return "", false
	}

	for i := 0; i < 60; i++ {
		sessions := getAllISCSISession(ctx)
		for _, s := range sessions {
			if s[0] == "tcp:" && strings.ToLower(tgtPortal) == strings.ToLower(s[2]) && targetIQN == s[4] {
				log.AddContext(ctx).Infof("Login iSCSI session success. Session: %s, manualScan: %s",
					s[1], manualScan)
				return s[1], manualScan
			}
		}

		checkExitCode := []string{"exit status 0", "exit status 15", "exit status 255"}
		err := runISCSIAdmin(ctx, tgtPortal, targetIQN, "--login", checkExitCode)
		if err != nil {
			log.AddContext(ctx).Warningf("Login iSCSI session %s error, reason: %v", tgtPortal, err)
			return "", false
		}

		err = updateISCSIAdmin(ctx, tgtPortal, targetIQN, "node.startup", "automatic")
		if err != nil {
			log.AddContext(ctx).Warningf("Update node startUp error, reason: %v", err)
			return "", false
		}

		time.Sleep(time.Second * 2)
	}
	return "", false
}

func getHostChannelTargetLun(session, tgtLun string) []string {
	var hostChannelTargetLun []string
	var host, channel, target string
	globPath := "/sys/class/iscsi_host/host*/device/session" + session
	paths, err := filepath.Glob(globPath + "/target*")
	if err != nil {
		return nil
	}

	if paths != nil {
		_, file := filepath.Split(paths[0])
		splitPath := strings.Split(file, ":")
		channel = splitPath[1]
		target = splitPath[2]
	} else {
		target = "-"
		channel = "-"
		paths, err = filepath.Glob(globPath)
		if err != nil || paths == nil {
			return nil
		}
	}

	index := strings.Index(paths[0][26:], "/")
	host = paths[0][26:][:index]
	hostChannelTargetLun = append(hostChannelTargetLun, host, channel, target, tgtLun)
	return hostChannelTargetLun
}

func scanISCSI(ctx context.Context, hostChannelTargetLun []string) {
	channelTargetLun := fmt.Sprintf("%s %s %s", hostChannelTargetLun[1], hostChannelTargetLun[2],
		hostChannelTargetLun[3])
	scanCommand := fmt.Sprintf("echo \"%s\" > /sys/class/scsi_host/host%s/scan",
		channelTargetLun, hostChannelTargetLun[0])
	output, err := utils.ExecShellCmd(ctx, scanCommand)
	if err != nil {
		log.AddContext(ctx).Warningf("rescan iSCSI host error: %s", output)
	}
}

func getDeviceByHCTL(session string, hostChannelTargetLun []string) string {
	copyHCTL := make([]string, 4, 4)
	copy(copyHCTL, hostChannelTargetLun)
	for index, value := range copyHCTL {
		if value == "-" {
			copyHCTL[index] = "*"
		}
	}

	host, channel, target, lun := copyHCTL[0], copyHCTL[1], copyHCTL[2], copyHCTL[3]
	path := fmt.Sprintf("/sys/class/scsi_host/host%s/device/session%s/target%s:%s:%s/%s:%s:%s:%s/block/*",
		host, session, host, channel, target, host, channel, target, lun)

	devices, err := filepath.Glob(path)
	if err != nil {
		return ""
	}

	var device string
	if devices != nil {
		sort.Strings(devices)
		_, device = filepath.Split(devices[0])
	}

	return device
}

type deviceScan struct {
	numRescans     int
	secondNextScan int
}

func (s *deviceScan) scan(ctx context.Context,
	req scanRequest) string {
	var device string
	doScans := true
	for doScans {
		if len(req.hostChannelTargetLun) == 0 {
			req.hostChannelTargetLun = getHostChannelTargetLun(req.sessionId, req.tgtHostLun)
		}

		if len(req.hostChannelTargetLun) != 0 {
			if s.secondNextScan <= 0 {
				s.numRescans++
				scanISCSI(ctx, req.hostChannelTargetLun)
				s.secondNextScan = int(math.Pow(float64(s.numRescans+2), 2.0))
			}

			device = getDeviceByHCTL(req.sessionId, req.hostChannelTargetLun)
		}

		if device != "" {
			device = connector.ClearUnavailableDevice(ctx, device, req.tgtLunWWN)
		}

		doScans = s.numRescans <= deviceScanAttemptsDefault && !(device != "" || req.iSCSIShareData.stopConnecting)
		if doScans {
			time.Sleep(time.Second)
			s.secondNextScan--
		}
	}
	return device
}

func connectVol(ctx context.Context,
	tgt singleConnectorInfo,
	conn connectorInfo,
	iSCSIShareData *shareData) {
	var device string

	session, manualScan := connectISCSIPortal(ctx, tgt.tgtPortal, tgt.tgtIQN, conn.tgtChapInfo)
	if session != "" {
		var numRescans, secondNextScan int
		var hostChannelTargetLun []string
		if manualScan {
			numRescans = -1
			secondNextScan = 0
		} else {
			numRescans = 0
			secondNextScan = 4
		}

		iSCSIShareData.numLogin += 1
		dScan := deviceScan{
			numRescans:     numRescans,
			secondNextScan: secondNextScan,
		}
		device = dScan.scan(ctx, scanRequest{sessionId: session, tgtHostLun: tgt.tgtHostLun,
			tgtLunWWN: conn.tgtLunWWN, hostChannelTargetLun: hostChannelTargetLun, iSCSIShareData: iSCSIShareData})
		log.AddContext(ctx).Infof("Found device %s", device)

		if device == "" {
			log.AddContext(ctx).Debugf("LUN %s on iSCSI portal %s not found on sysfs after logging in.",
				tgt.tgtHostLun, tgt.tgtPortal)
		} else {
			iSCSIShareData.foundDevices = append(iSCSIShareData.foundDevices, device)
			iSCSIShareData.justAddedDevices = append(iSCSIShareData.justAddedDevices, device)
		}
	} else {
		log.AddContext(ctx).Warningf("build iSCSI session %s error", tgt.tgtPortal)
		iSCSIShareData.failedLogin += 1
	}

	iSCSIShareData.stoppedThreads += 1
	return
}

func constructISCSIInfo(ctx context.Context, conn connectorInfo) []singleConnectorInfo {
	var iSCSIInfoList []singleConnectorInfo
	for index, portal := range conn.tgtPortals {
		ok := connector.CheckHostConnectivity(ctx, portal)
		if !ok {
			log.AddContext(ctx).Errorf("failed to check the host connectivity. %s", portal)
			continue
		}

		var iSCSIInfo singleConnectorInfo
		iSCSIInfo.tgtPortal = portal
		iSCSIInfo.tgtIQN = conn.tgtIQNs[index]
		iSCSIInfo.tgtHostLun = conn.tgtHostLUNs[index]
		iSCSIInfoList = append(iSCSIInfoList, iSCSIInfo)
	}

	return iSCSIInfoList
}

func tryConnectVolume(ctx context.Context, connMap map[string]interface{}) (string, error) {
	conn, err := parseISCSIInfo(ctx, connMap)
	if err != nil {
		return "", err
	}

	constructInfos := constructISCSIInfo(ctx, conn)
	lenIndex := len(constructInfos)
	if !conn.volumeUseMultiPath {
		lenIndex = 1
	}

	var wait sync.WaitGroup
	iSCSIShareData := connectVolume(ctx, &wait, constructInfos[:lenIndex], conn)
	diskName, err := findDevice(ctx, conn, iSCSIShareData, lenIndex)
	if err != nil {
		log.AddContext(ctx).Errorf("failed to find a disk. %v", err)
	}
	iSCSIShareData.stopConnecting = true
	wait.Wait()

	return checkDeviceAvailable(ctx, conn, iSCSIShareData, diskName, int(iSCSIShareData.numLogin))
}

func catchConnectError(ctx context.Context) {
	if r := recover(); r != nil {
		log.AddContext(ctx).Errorf("runtime error caught in loop routine: %v", r)
		log.AddContext(ctx).Errorf("%s", debug.Stack())
		log.Flush()
	}
}

func connectVolume(ctx context.Context, wait *sync.WaitGroup, constructInfos []singleConnectorInfo,
	conn connectorInfo) *shareData {
	var iSCSIShareData = new(shareData)
	wait.Add(len(constructInfos))
	for _, tgtInfo := range constructInfos {

		go func(tgt singleConnectorInfo) {
			defer catchConnectError(ctx)
			connectVol(ctx, tgt, conn, iSCSIShareData)
			wait.Done()
		}(tgtInfo)
	}

	return iSCSIShareData
}

func findDevice(ctx context.Context,
	conn connectorInfo,
	iSCSIShareData *shareData,
	lenIndex int) (string, error) {
	if !conn.volumeUseMultiPath {
		scanSingle(iSCSIShareData)
		return "", nil
	}

	var diskName string
	var err error
	switch conn.multiPathType {
	case connector.DMMultiPath:
		diskName, _ = findDiskOfDM(ctx, lenIndex, conn.tgtLunWWN, iSCSIShareData)
	case connector.HWUltraPath:
		diskName = findDiskOfUltraPath(ctx, lenIndex, iSCSIShareData, connector.UltraPathCommand, conn.tgtLunWWN)
	case connector.HWUltraPathNVMe:
		diskName = findDiskOfUltraPath(ctx, lenIndex, iSCSIShareData, connector.UltraPathNVMeCommand, conn.tgtLunWWN)
	default:
		err = utils.Errorf(ctx, "%s. %s", connector.UnsupportedMultiPathType, conn.multiPathType)
	}

	return diskName, err
}

func checkDeviceAvailable(ctx context.Context,
	conn connectorInfo,
	iSCSIShareData *shareData,
	diskName string, expectPathNumber int) (string, error) {
	if !conn.volumeUseMultiPath {
		return checkSinglePathAvailable(ctx, iSCSIShareData, conn.tgtLunWWN)
	}

	if diskName == "" {
		err := connector.RemoveDevices(ctx, iSCSIShareData.foundDevices)
		if err != nil {
			log.AddContext(ctx).Warningf("Remove devices %v error: %v",
				iSCSIShareData.foundDevices, err)
		}
		return "", utils.Errorln(ctx, connector.VolumeNotFound)
	}

	switch conn.multiPathType {
	case connector.DMMultiPath:
		return connector.VerifyDeviceAvailableOfDM(ctx, conn.tgtLunWWN,
			expectPathNumber, iSCSIShareData.foundDevices, tryDisConnectVolume)
	case connector.HWUltraPath:
		return connector.VerifyDeviceAvailableOfUltraPath(ctx, connector.UltraPathCommand, diskName)
	case connector.HWUltraPathNVMe:
		return connector.VerifyDeviceAvailableOfUltraPath(ctx, connector.UltraPathNVMeCommand, diskName)
	default:
		return "", utils.Errorf(ctx, "%s. %s", connector.UnsupportedMultiPathType, conn.multiPathType)
	}
}

func checkSinglePathAvailable(ctx context.Context, iSCSIShareData *shareData, tgtLunWWN string) (string, error) {
	if len(iSCSIShareData.foundDevices) == 0 {
		return "", errors.New(connector.VolumeNotFound)
	}

	device := fmt.Sprintf("/dev/%s", iSCSIShareData.foundDevices[0])
	err := connector.VerifySingleDevice(ctx, device, tgtLunWWN,
		connector.VolumeNotFound, tryDisConnectVolume)
	if err != nil {
		return "", err
	}
	return device, nil
}

func scanSingle(iSCSIShareData *shareData) {
	for i := 0; i < 15; i++ {
		if len(iSCSIShareData.foundDevices) != 0 {
			break
		}
		time.Sleep(time.Second * 2)
	}
}

func getSYSfsWwn(ctx context.Context, foundDevices []string, mPath string) (string, error) {
	if mPath != "" {
		dmFile := fmt.Sprintf("/sys/block/%s/dm/uuid", mPath)
		data, err := ioutil.ReadFile(dmFile)
		if err != nil {
			msg := fmt.Sprintf("Read dm file %s error: %v", dmFile, err)
			log.AddContext(ctx).Errorln(msg)
			return "", errors.New(msg)
		}

		if wwid := data[6:]; wwid != nil {
			return string(wwid), nil
		}
	}

	wwnTypes := map[string]string{
		"t10.": "1", "eui.": "2", "naa.": "3",
	}
	for _, device := range foundDevices {
		deviceFile := fmt.Sprintf("/sys/block/%s/device/wwid", device)
		data, err := ioutil.ReadFile(deviceFile)
		if err != nil {
			msg := fmt.Sprintf("Read device file %s error: %v", deviceFile, err)
			log.AddContext(ctx).Errorln(msg)
			continue
		}

		wwnType, exist := wwnTypes[string(data[:4])]
		if !exist {
			wwnType = "8"
		}

		wwid := wwnType + string(data[4:])
		return wwid, nil
	}

	msg := fmt.Sprintf("Cannot find device %s wwid", foundDevices)
	log.AddContext(ctx).Errorln(msg)
	return "", nil
}

func addMultiWWN(ctx context.Context, tgtLunWWN string) (bool, error) {
	output, err := utils.ExecShellCmd(ctx, "multipath -a %s", tgtLunWWN)
	if err != nil {
		if strings.TrimSpace(output) != fmt.Sprintf("wwid \"%s\" added", tgtLunWWN) {
			return false, nil
		}

		msg := "run cmd multipath -a error"
		log.AddContext(ctx).Errorln(msg)
		return false, errors.New(msg)
	}

	return true, nil
}

func addMultiPath(ctx context.Context, devPath string) error {
	output, err := utils.ExecShellCmd(ctx, "multipath add path %s", devPath)
	if err != nil {
		msg := "run cmd multipath add path error"
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	if strings.TrimSpace(output) != "ok" {
		log.AddContext(ctx).Warningln("run cmd multiPath add path, output is not ok")
	}
	return nil
}

func tryScanMultiDevice(ctx context.Context, mPath string, iSCSIShareData *shareData) string {
	for mPath == "" && len(iSCSIShareData.justAddedDevices) != 0 {
		devicePath := "/dev/" + iSCSIShareData.justAddedDevices[0]
		iSCSIShareData.justAddedDevices = iSCSIShareData.justAddedDevices[1:]
		err := addMultiPath(ctx, devicePath)
		if err != nil {
			log.AddContext(ctx).Warningf("Add multiPath path failed, error: %s", err)
		}

		var isClear bool
		mPath, isClear = connector.FindAvailableMultiPath(ctx, iSCSIShareData.foundDevices)
		if isClear {
			iSCSIShareData.foundDevices = nil
			iSCSIShareData.justAddedDevices = nil
		}
	}
	return mPath
}

func scanMultiDevice(ctx context.Context,
	mPath, wwn string,
	iSCSIShareData *shareData,
	wwnAdded bool) (string, bool) {
	var err error
	if mPath == "" && len(iSCSIShareData.foundDevices) != 0 {
		var isClear bool
		mPath, isClear = connector.FindAvailableMultiPath(ctx, iSCSIShareData.foundDevices)
		if isClear {
			iSCSIShareData.foundDevices = nil
			iSCSIShareData.justAddedDevices = nil
		}

		if wwn != "" && !(mPath != "" || wwnAdded) {
			wwnAdded, err = addMultiWWN(ctx, wwn)
			if err != nil {
				log.AddContext(ctx).Warningf("Add multiPath wwn failed, error: %s", err)
			}

			mPath = tryScanMultiDevice(ctx, mPath, iSCSIShareData)
		}
	}

	return mPath, wwnAdded
}

func findDiskOfUltraPath(ctx context.Context, lenIndex int, iSCSIShareData *shareData, upType, lunWWN string) string {
	var diskName string
	var err error
	for !((int64(lenIndex) == iSCSIShareData.stoppedThreads && len(iSCSIShareData.foundDevices) == 0) ||
		(diskName != "" && int64(lenIndex) == iSCSIShareData.numLogin+iSCSIShareData.failedLogin)) {

		diskName, err = connector.GetDiskNameByWWN(ctx, upType, lunWWN)
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}
	return diskName
}

func findDiskOfDM(ctx context.Context, lenIndex int, LunWWN string, iSCSIShareData *shareData) (string, string) {
	var wwnAdded bool
	var lastTryOn int64
	var mPath, wwn string
	var err error
	for !((int64(lenIndex) == iSCSIShareData.stoppedThreads && len(iSCSIShareData.foundDevices) == 0) ||
		(mPath != "" && int64(lenIndex) == iSCSIShareData.numLogin+iSCSIShareData.failedLogin)) {
		if wwn == "" && len(iSCSIShareData.foundDevices) != 0 {
			wwn, err = getSYSfsWwn(ctx, iSCSIShareData.foundDevices, mPath)
			if err != nil {
				break
			}

			if wwn == "" {
				wwn = LunWWN
			}
		}

		mPath, wwnAdded = scanMultiDevice(ctx, mPath, wwn, iSCSIShareData, wwnAdded)
		if lastTryOn == 0 && len(iSCSIShareData.foundDevices) != 0 && int64(
			lenIndex) == iSCSIShareData.stoppedThreads {
			log.AddContext(ctx).Infoln("All connection threads finished, giving 15 seconds for dm to appear.")
			lastTryOn = time.Now().Unix() + 15
		} else if lastTryOn != 0 && lastTryOn < time.Now().Unix() {
			break
		}

		time.Sleep(time.Second)
	}
	return mPath, wwn
}

func getISCSISession(ctx context.Context, devSessionIds []string) []singleConnectorInfo {
	var devConnectorInfos []singleConnectorInfo
	sessions := getAllISCSISession(ctx)
	for _, devSessionId := range devSessionIds {
		var devConnectorInfo singleConnectorInfo
		for _, s := range sessions {
			if devSessionId == s[1] {
				devConnectorInfo.tgtPortal = s[2]
				devConnectorInfo.tgtIQN = s[4]
				devConnectorInfos = append(devConnectorInfos, devConnectorInfo)
				break
			}
		}
	}
	return devConnectorInfos
}

func disconnectFromISCSIPortal(ctx context.Context, tgtPortal, targetIQN string) {
	checkExitCode := []string{"exit status 0", "exit status 15", "exit status 255"}
	err := updateISCSIAdminWithExitCode(ctx, tgtPortal, targetIQN,
		iscsiCMD("node.startup", "manual"),
		checkExitCode)
	if err != nil {
		log.AddContext(ctx).Warningf("Update node startUp error, reason: %v", err)
	}

	err = runISCSIAdmin(ctx, tgtPortal, targetIQN, "--logout", checkExitCode)
	if err != nil {
		log.AddContext(ctx).Warningf("Logout iSCSI node error, reason: %v", err)
	}

	err = runISCSIAdmin(ctx, tgtPortal, targetIQN, "--op delete", checkExitCode)
	if err != nil {
		log.AddContext(ctx).Warningf("Delete iSCSI node error, reason: %v", err)
	}
}

func disconnectSessions(ctx context.Context, devConnectorInfos []singleConnectorInfo) error {
	for _, connectorInfo := range devConnectorInfos {
		tgtPortal := connectorInfo.tgtPortal
		tgtIQN := connectorInfo.tgtIQN
		cmd := fmt.Sprintf("ls /dev/disk/by-path/ |grep -w %s |grep -w %s |wc -l |awk '{if($1>0) print 1; "+
			"else print 0}'", tgtPortal, utils.MaskSensitiveInfo(tgtIQN))
		output, err := utils.ExecShellCmd(ctx, cmd)
		if err != nil {
			log.AddContext(ctx).Infof("Disconnect iSCSI target %s failed, err: %v", tgtPortal, err)
			return err
		}
		outputSplit := strings.Split(output, "\n")
		if len(outputSplit) != 0 && outputSplit[0] == "0" {
			disconnectFromISCSIPortal(ctx, tgtPortal, tgtIQN)
		}
	}
	return nil
}

func tryDisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	return connector.DisConnectVolume(ctx, tgtLunWWN, tryToDisConnectVolume)
}

func tryToDisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	virtualDevice, devType, err := connector.GetVirtualDevice(ctx, tgtLunWWN)
	if err != nil {
		log.AddContext(ctx).Errorf("Get device of WWN %s error: %v", tgtLunWWN, err)
		return err
	}

	if virtualDevice == "" {
		log.AddContext(ctx).Infof("The device of WWN %s does not exist on host", tgtLunWWN)
		return errors.New("FindNoDevice")
	}

	phyDevices, err := connector.GetPhysicalDevices(ctx, virtualDevice, devType)
	if err != nil {
		return err
	}

	sessionIds, err := getSessionIds(ctx, phyDevices, devType)
	if err != nil {
		return err
	}

	multiPathName, err := connector.RemoveAllDevice(ctx, virtualDevice, phyDevices, devType)
	if err != nil {
		return err
	}

	devConnectorInfos := getISCSISession(ctx, sessionIds)
	err = disconnectSessions(ctx, devConnectorInfos)
	if err != nil {
		log.AddContext(ctx).Errorf("Disconnect portals %s error: %v",
			utils.MaskSensitiveInfo(devConnectorInfos), err)
		return err
	}

	if multiPathName != "" {
		err = connector.FlushDMDevice(ctx, virtualDevice)
		if err != nil {
			return err
		}
	}

	return nil
}
