package iscsi

import (
	"connector"
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
	"utils"
	"utils/log"
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
	tgtChapInfo chapInfo
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

const (
	deviceScanAttemptsDefault int = 3
)

func parseISCSIInfo(connectionProperties map[string]interface{}) (*connectorInfo, error) {
	tgtLunWWN, LunWWNExist := connectionProperties["tgtLunWWN"].(string)
	if !LunWWNExist {
		msg := "there is no target Lun WWN in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	tgtPortals, portalExist := connectionProperties["tgtPortals"].([]string)
	if !portalExist {
		msg := "there are no target portals in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	tgtIQNs, iqnExist := connectionProperties["tgtIQNs"].([]string)
	if !iqnExist {
		msg := "there are no target IQNs in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	tgtHostLUNs, hostLunIdExist := connectionProperties["tgtHostLUNs"].([]string)
	if !hostLunIdExist {
		msg := "there are no target hostLun in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if (tgtIQNs == nil || tgtHostLUNs == nil) || (
		len(tgtPortals) != len(tgtIQNs) || len(tgtPortals) != len(tgtHostLUNs)) {
		msg := "the numbers of tgtPortals, targetIQNs and tgtHostLUNs are not equal"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	authUserName, _ := connectionProperties["authUserName"].(string)
	authPassword, _ := connectionProperties["authPassword"].(string)
	authMethod, _ := connectionProperties["authMethod"].(string)

	var con connectorInfo
	con.tgtLunWWN = tgtLunWWN
	con.tgtPortals = tgtPortals
	con.tgtIQNs = tgtIQNs
	con.tgtHostLUNs = tgtHostLUNs
	con.tgtChapInfo.authUserName = authUserName
	con.tgtChapInfo.authPassword = authPassword
	con.tgtChapInfo.authMethod = authMethod
	return &con, nil
}

func runISCSIAdmin(tgtPortal, targetIQN string, iSCSICommand string, checkExitCode []string) error {
	iSCSICmd := fmt.Sprintf("iscsiadm -m node -T %s -p %s %s", targetIQN, tgtPortal, iSCSICommand)
	output, err := utils.ExecShellCmdFilterLog(iSCSICmd)
	if err != nil {
		if err.Error() == "timeout" {
			return err
		}

		err2 := utils.CheckExistCode(err, checkExitCode)
		if err2 != nil {
			log.Warningf("Run %s: output=%s, err=%v", utils.MaskSensitiveInfo(iSCSICmd),
				utils.MaskSensitiveInfo(output), err2)
			return err
		}
	}

	return nil
}

func runISCSIBare(iSCSICommand string, checkExitCode []string) (string, error) {
	iSCSICmd := fmt.Sprintf("iscsiadm %s", iSCSICommand)
	output, err := utils.ExecShellCmdFilterLog(iSCSICmd)
	if err != nil {
		if err.Error() == "timeout" {
			return "", err
		}

		err2 := utils.CheckExistCode(err, checkExitCode)
		if err2 != nil {
			log.Errorf("Run bare %s: output=%s, err=%v", utils.MaskSensitiveInfo(iSCSICmd),
				utils.MaskSensitiveInfo(output), err2)
			return "", err
		}
	}

	return output, nil
}

func updateISCSIAdminWithExitCode(tgtPortal, targetIQN string, propertyKey, propertyValue string,
	checkExitCode []string) error {
	iSCSICmd := fmt.Sprintf("--op update -n %s -v %s", propertyKey, propertyValue)
	return runISCSIAdmin(tgtPortal, targetIQN, iSCSICmd, checkExitCode)
}

func updateISCSIAdmin(tgtPortal, targetIQN string, propertyKey, propertyValue string) error {
	iSCSICmd := fmt.Sprintf("--op update -n %s -v %s", propertyKey, propertyValue)
	return runISCSIAdmin(tgtPortal, targetIQN, iSCSICmd, nil)
}

func updateChapInfo(tgtPortal, targetIQN string, tgtChapInfo chapInfo) error {
	if tgtChapInfo.authMethod != "" {
		err := updateISCSIAdmin(tgtPortal, targetIQN, "node.session.auth.authmethod", tgtChapInfo.authMethod)
		if err != nil {
			log.Errorf("Update node session auth method %s error, reason: %v", tgtChapInfo.authMethod, err)
			return err
		}

		err = updateISCSIAdmin(tgtPortal, targetIQN, "node.session.auth.username", tgtChapInfo.authUserName)
		if err != nil {
			log.Errorf("Update node session auth username %s error, reason: %v", tgtChapInfo.authUserName, err)
			return err
		}

		err = updateISCSIAdmin(tgtPortal, targetIQN, "node.session.auth.password", tgtChapInfo.authPassword)
		if err != nil {
			log.Errorf("Update node session auth password %s error, reason: %v",
				utils.MaskSensitiveInfo(tgtChapInfo.authPassword), err)
			return err
		}
	}
	return nil
}

func getAllISCSISession() [][]string {
	checkExitCode := []string{"exit status 0", "exit status 21", "exit status 255"}
	allSessions, err := runISCSIBare("-m session", checkExitCode)
	if err != nil {
		log.Warningf("Get all iSCSI session error , reason: %v", err)
		return nil
	}

	var iSCSIInfo [][]string
	for _, iscsi := range strings.Split(allSessions, "\n") {
		if iscsi != "" {
			splitInfo := strings.Split(iscsi, " ")
			if len(splitInfo) < 4 {
				log.Warningf("iscsi session %s error", splitInfo)
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

func connectISCSIPortal(tgtPortal, targetIQN string, tgtChapInfo chapInfo) (string, bool) {
	checkExitCode := []string{"exit status 0", "exit status 21", "exit status 255"}
	err := runISCSIAdmin(tgtPortal, targetIQN, "", checkExitCode)
	if err != nil {
		if err.Error() == "timeout" {
			return "", false
		}

		err := runISCSIAdmin(tgtPortal, targetIQN, "--interface default --op new", nil)
		if err != nil {
			log.Errorf("Create new portal %s error , reason: %v", tgtPortal, err)
			return "", false
		}
	}

	var manualScan bool
	err = updateISCSIAdmin(tgtPortal, targetIQN, "node.session.scan", "manual")
	if err != nil {
		manualScan = true
	}

	err = updateChapInfo(tgtPortal, targetIQN, tgtChapInfo)
	if err != nil {
		log.Errorf("Update chap %s error, reason: %v", utils.MaskSensitiveInfo(tgtChapInfo), err)
		return "", false
	}

	for i := 0; i < 60; i++ {
		sessions := getAllISCSISession()
		for _, s := range sessions {
			if s[0] == "tcp:" && strings.ToLower(tgtPortal) == strings.ToLower(s[2]) && targetIQN == s[4] {
				return s[1], manualScan
			}
		}

		checkExitCode := []string{"exit status 0", "exit status 15", "exit status 255"}
		err := runISCSIAdmin(tgtPortal, targetIQN, "--login", checkExitCode)
		if err != nil {
			log.Warningf("Login iSCSI session %s error, reason: %v", tgtPortal, err)
			return "", false
		}

		err = updateISCSIAdmin(tgtPortal, targetIQN, "node.startup", "automatic")
		if err != nil {
			log.Warningf("Update node startUp error, reason: %v", err)
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

func scanISCSI(hostChannelTargetLun []string) {
	channelTargetLun := fmt.Sprintf("%s %s %s", hostChannelTargetLun[1], hostChannelTargetLun[2],
		hostChannelTargetLun[3])
	scanCommand := fmt.Sprintf("echo \"%s\" > /sys/class/scsi_host/host%s/scan",
		channelTargetLun, hostChannelTargetLun[0])
	output, err := utils.ExecShellCmd(scanCommand)
	if err != nil {
		log.Warningf("rescan iSCSI host error: %s", output)
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

func scan(session, tgtLun string, hostChannelTargetLun []string, numRescans, secondNextScan int,
	doScans bool, iSCSIShareData *shareData) (string, int, int, bool) {
	var device string
	if len(hostChannelTargetLun) == 0 {
		hostChannelTargetLun = getHostChannelTargetLun(session, tgtLun)
	}

	if len(hostChannelTargetLun) != 0 {
		if secondNextScan <= 0 {
			numRescans += 1
			scanISCSI(hostChannelTargetLun)
			secondNextScan = int(math.Pow(float64(numRescans+2), 2.0))
		}

		device = getDeviceByHCTL(session, hostChannelTargetLun)
	}

	doScans = numRescans <= deviceScanAttemptsDefault && !(device != "" || iSCSIShareData.stopConnecting)
	if doScans {
		time.Sleep(time.Second)
		secondNextScan -= 1
	}
	return device, numRescans, secondNextScan, doScans
}

func connectVol(tgtPortal, targetIQN, tgtLun string, tgtChapInfo chapInfo, iSCSIShareData *shareData) {
	var device string

	session, manualScan := connectISCSIPortal(tgtPortal, targetIQN, tgtChapInfo)
	if session != "" {
		var numRescans, secondNextScan int
		var hostChannelTargetLun []string
		doScans := true
		if manualScan {
			numRescans = -1
			secondNextScan = 0
		} else {
			numRescans = 0
			secondNextScan = 4
		}

		iSCSIShareData.numLogin += 1
		for doScans {
			device, numRescans, secondNextScan, doScans = scan(session, tgtLun, hostChannelTargetLun, numRescans,
				secondNextScan, doScans, iSCSIShareData)
			log.Infof("found device %s", device)
		}

		if device == "" {
			log.Debugf("LUN %s on iSCSI portal %s not found on sysfs after logging in.", tgtLun, tgtPortal)
		}

		if device != "" {
			iSCSIShareData.foundDevices = append(iSCSIShareData.foundDevices, device)
			iSCSIShareData.justAddedDevices = append(iSCSIShareData.justAddedDevices, device)
		}
	} else {
		log.Warningf("build iSCSI session %s error", tgtPortal)
		iSCSIShareData.failedLogin += 1
	}

	iSCSIShareData.stoppedThreads += 1
	return
}

func constructISCSIInfo(conn *connectorInfo) []singleConnectorInfo {
	var iSCSIInfoList []singleConnectorInfo
	for index, portal := range conn.tgtPortals {
		var iSCSIInfo singleConnectorInfo
		iSCSIInfo.tgtPortal = portal
		iSCSIInfo.tgtIQN = conn.tgtIQNs[index]
		iSCSIInfo.tgtHostLun = conn.tgtHostLUNs[index]
		iSCSIInfoList = append(iSCSIInfoList, iSCSIInfo)
	}

	return iSCSIInfoList
}

func tryConnectVolume(connMap map[string]interface{}) (string, error) {
	conn, err := parseISCSIInfo(connMap)
	if err != nil {
		return "", err
	}

	constructInfos := constructISCSIInfo(conn)
	var mPath string
	var wait sync.WaitGroup
	var iSCSIShareData = new(shareData)
	lenIndex := len(conn.tgtPortals)
	if !connMap["volumeUseMultiPath"].(bool) {
		lenIndex = 1
	}
	for index := 0; index < lenIndex; index++ {
		tgtInfo := constructInfos[index]
		wait.Add(1)

		go func(tgt singleConnectorInfo) {
			defer func() {
				wait.Done()
				if r := recover(); r != nil {
					log.Errorf("Runtime error caught in loop routine: %v", r)
					log.Errorf("%s", debug.Stack())
				}

				log.Flush()
			}()

			connectVol(tgt.tgtPortal, tgt.tgtIQN, tgt.tgtHostLun, conn.tgtChapInfo, iSCSIShareData)
		}(tgtInfo)
	}

	if connMap["volumeUseMultiPath"].(bool) {
		mPath, _ = scanMultiPath(lenIndex, iSCSIShareData)
	} else {
		scanSingle(iSCSIShareData)
	}

	iSCSIShareData.stopConnecting = true
	wait.Wait()

	if iSCSIShareData.foundDevices == nil {
		return "", errors.New("volume device not found")
	}

	if !connMap["volumeUseMultiPath"].(bool) {
		device := fmt.Sprintf("/dev/%s", iSCSIShareData.foundDevices[0])
		err := connector.VerifySingleDevice(device, conn.tgtLunWWN,
			"volume device not found", false, tryDisConnectVolume)
		if err != nil {
			return "", err
		}
		return device, nil
	}

	// mPath: dm-<id>
	if mPath != "" {
		dev, err := connector.VerifyMultiPathDevice(mPath, conn.tgtLunWWN,
			"volume device not found", false, tryDisConnectVolume)
		if err != nil {
			return "", err
		}
		return dev, nil
	}

	log.Errorln("no dm was created")
	return "", errors.New("volume device not found")
}

func scanSingle(iSCSIShareData *shareData) {
	for i := 0; i < 15; i++ {
		if len(iSCSIShareData.foundDevices) != 0 {
			break
		}
		time.Sleep(time.Second * 2)
	}
}

func getSYSfsWwn(foundDevices []string, mPath string) (string, error) {
	if mPath != "" {
		dmFile := fmt.Sprintf("/sys/block/%s/dm/uuid", mPath)
		data, err := ioutil.ReadFile(dmFile)
		if err != nil {
			msg := fmt.Sprintf("Read dm file %s error: %v", dmFile, err)
			log.Errorln(msg)
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
			log.Errorln(msg)
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
	log.Errorln(msg)
	return "", errors.New(msg)
}

func findSYSfsMultiPath(foundDevices []string) string {
	for _, device := range foundDevices {
		dmPath := fmt.Sprintf("/sys/block/%s/holders/dm-*", device)

		paths, err := filepath.Glob(dmPath)
		if err != nil {
			continue
		}
		if paths != nil {
			splitPath := strings.Split(paths[0], "/")
			return splitPath[len(splitPath)-1]
		}
	}
	return ""
}

func addMultiWWN(tgtLunWWN string) (bool, error) {
	output, err := utils.ExecShellCmd("multipath -a %s", tgtLunWWN)
	if err != nil {
		if strings.TrimSpace(output) != fmt.Sprintf("wwid \"%s\" added", tgtLunWWN) {
			return false, nil
		}

		msg := "run cmd multipath -a error"
		log.Errorln(msg)
		return false, errors.New(msg)
	}

	return true, nil
}

func addMultiPath(devPath string) error {
	output, err := utils.ExecShellCmd("multipath add path %s", devPath)
	if err != nil {
		msg := "run cmd multipath add path error"
		log.Errorln(msg)
		return errors.New(msg)
	}

	if strings.TrimSpace(output) != "ok" {
		log.Warningln("run cmd multiPath add path, output is not ok")
	}
	return nil
}

func tryScanMultiDevice(mPath string, iSCSIShareData *shareData) string {
	for mPath == "" && len(iSCSIShareData.justAddedDevices) != 0 {
		devicePath := "/dev/" + iSCSIShareData.justAddedDevices[0]
		iSCSIShareData.justAddedDevices = iSCSIShareData.justAddedDevices[1:]
		err := addMultiPath(devicePath)
		if err != nil {
			log.Warningf("Add multiPath path failed, error: %s", err)
		}

		mPath = findSYSfsMultiPath(iSCSIShareData.foundDevices)
	}
	return mPath
}

func scanMultiDevice(mPath, wwn string, iSCSIShareData *shareData, wwnAdded bool) (string, bool) {
	var err error
	if mPath == "" && len(iSCSIShareData.foundDevices) != 0 {
		mPath = findSYSfsMultiPath(iSCSIShareData.foundDevices)
		if wwn != "" && !(mPath != "" || wwnAdded) {
			wwnAdded, err = addMultiWWN(wwn)
			if err != nil {
				log.Warningf("Add multiPath wwn failed, error: %s", err)
			}

			mPath = tryScanMultiDevice(mPath, iSCSIShareData)
		}
	}

	return mPath, wwnAdded
}

func scanMultiPath(lenIndex int, iSCSIShareData *shareData) (string, string) {
	var wwnAdded bool
	var lastTryOn int64
	var mPath, wwn string
	var err error
	for !((int64(lenIndex) == iSCSIShareData.stoppedThreads && len(iSCSIShareData.foundDevices) == 0) ||
		(mPath != "" && int64(lenIndex) == iSCSIShareData.numLogin+iSCSIShareData.failedLogin)) {
		if wwn == "" && len(iSCSIShareData.foundDevices) != 0 {
			wwn, err = getSYSfsWwn(iSCSIShareData.foundDevices, mPath)
			if err != nil {
				continue
			}
		}

		mPath, wwnAdded = scanMultiDevice(mPath, wwn, iSCSIShareData, wwnAdded)
		if lastTryOn == 0 && len(iSCSIShareData.foundDevices) != 0 && int64(
			lenIndex) == iSCSIShareData.stoppedThreads {
			log.Infoln("All connection threads finished, giving 15 seconds for dm to appear.")
			lastTryOn = time.Now().Unix() + 15
		} else if lastTryOn != 0 && lastTryOn < time.Now().Unix() {
			break
		}

		time.Sleep(time.Second)
	}
	return mPath, wwn
}

func getISCSISession(devSessionIds []string) []singleConnectorInfo {
	var devConnectorInfos []singleConnectorInfo
	sessions := getAllISCSISession()
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

func disconnectFromISCSIPortal(tgtPortal, targetIQN string) {
	checkExitCode := []string{"exit status 0", "exit status 15", "exit status 255"}
	err := updateISCSIAdminWithExitCode(tgtPortal, targetIQN, "node.startup", "manual",
		checkExitCode)
	if err != nil {
		log.Warningf("Update node startUp error, reason: %v", err)
	}

	err = runISCSIAdmin(tgtPortal, targetIQN, "--logout", checkExitCode)
	if err != nil {
		log.Warningf("Logout iSCSI node error, reason: %v", err)
	}

	err = runISCSIAdmin(tgtPortal, targetIQN, "--op delete", checkExitCode)
	if err != nil {
		log.Warningf("Delete iSCSI node error, reason: %v", err)
	}
}

func disconnectSessions(devConnectorInfos []singleConnectorInfo) error {
	for _, connectorInfo := range devConnectorInfos {
		tgtPortal := connectorInfo.tgtPortal
		tgtIQN := connectorInfo.tgtIQN
		cmd := fmt.Sprintf("ls /dev/disk/by-path/ |grep -w %s |grep -w %s |wc -l |awk '{if($1>0) print 1; " +
			"else print 0}'", tgtPortal, utils.MaskSensitiveInfo(tgtIQN))
		output, err := utils.ExecShellCmd(cmd)
		if err != nil {
			log.Infof("Disconnect iSCSI target %s failed, err: %v", tgtPortal, err)
			return err
		}
		outputSplit := strings.Split(output, "\n")
		if len(outputSplit) != 0 && outputSplit[0] == "0" {
			disconnectFromISCSIPortal(tgtPortal, tgtIQN)
		}
	}
	return nil
}

func tryDisConnectVolume(tgtLunWWN string, checkDeviceAvailable bool) error {
	return connector.DisConnectVolume(tgtLunWWN, checkDeviceAvailable, tryToDisConnectVolume)
}

func tryToDisConnectVolume(tgtLunWWN string, checkDeviceAvailable bool) error {
	device, err := connector.GetDevice(nil, tgtLunWWN, checkDeviceAvailable)
	if err != nil {
		log.Warningf("Get device of WWN %s error: %v", tgtLunWWN, err)
		return err
	}

	devSessionIds, multiPathName, err := connector.RemoveDeviceConnection(device)
	if err != nil {
		log.Errorf("Remove device %s error: %v", device, err)
		return err
	}
	devConnectorInfos := getISCSISession(devSessionIds)

	err = disconnectSessions(devConnectorInfos)
	if err != nil {
		log.Errorf("Disconnect portals %s error: %v", utils.MaskSensitiveInfo(devConnectorInfos), err)
		return err
	}

	if multiPathName != "" {
		err = connector.FlushDMDevice(device)
		if err != nil {
			return err
		}
	}

	return nil
}
