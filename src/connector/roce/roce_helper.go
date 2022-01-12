package roce

import (
	"connector"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"utils"
	"utils/log"
)

type connectorInfo struct {
	tgtPortals []string
	tgtLunGUID string
}

type shareData struct {
	stopConnecting bool
	numLogin       int64
	failedLogin    int64
	stoppedThreads int64
	foundDevices   []string
	justAddedDevices []string
}
const sleepInternal = 2

func getNVMeInfo(connectionProperties map[string]interface{}) (*connectorInfo, error) {
	var con connectorInfo

	tgtPortals, portalExist := connectionProperties["tgtPortals"].([]string)
	if !portalExist {
		msg := "there are no target portals in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	tgtLunGUID, lunGuidExist := connectionProperties["tgtLunGuid"].(string)
	if !lunGuidExist {
		msg := "there are no target lun guid in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	con.tgtPortals = tgtPortals
	con.tgtLunGUID = tgtLunGUID
	return &con, nil
}

func getTargetNQN(tgtPortal string) (string, error) {
	output, err := utils.ExecShellCmdFilterLog("nvme discover -t rdma -a %s", tgtPortal)
	if err != nil {
		log.Errorf("Cannot discover nvme target %s, reason: %v", tgtPortal, output)
		return "", err
	}

	var tgtNqn string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "subnqn") {
			splits := strings.SplitN(line, ":", 2)
			if len(splits) == 2 && splits[0] == "subnqn" {
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

func connectRoCEPortal(existSessions map[string]bool, tgtPortal, targetNQN string) error {
	if value, exist := existSessions[tgtPortal]; exist && value {
		log.Infof("RoCE target %s has already login, no need login again", tgtPortal)
		return nil
	}

	checkExitCode := []string{"exit status 0", "exit status 70"}
	iSCSICmd := fmt.Sprintf("nvme connect -t rdma -a %s -n %s", tgtPortal, targetNQN)
	output, err := utils.ExecShellCmdFilterLog(iSCSICmd)
	if strings.Contains(output, "Input/output error") {
		log.Infof("RoCE target %s has already login, no need login again", tgtPortal)
		return nil
	}

	if err != nil {
		if err.Error() == "timeout" {
			return err
		}

		err2 := utils.IgnoreExistCode(err, checkExitCode)
		if err2 != nil {
			log.Warningf("Run %s: output=%s, err=%v", utils.MaskSensitiveInfo(iSCSICmd),
				utils.MaskSensitiveInfo(output), err2)
			return err
		}
	}
	return nil
}

func connectVol(existSessions map[string]bool, tgtPortal, tgtLunGUID string, nvmeShareData *shareData) {
	targetNQN, err := getTargetNQN(tgtPortal)
	if err != nil {
		log.Errorf("Cannot discover nvme target %s, reason: %v", tgtPortal, err)
		return
	}

	err = connectRoCEPortal(existSessions, tgtPortal, targetNQN)
	if err != nil {
		log.Errorf("connect roce portal %s error, reason: %v", tgtPortal, err)
		nvmeShareData.failedLogin += 1
		nvmeShareData.stoppedThreads += 1
		return
	}

	nvmeShareData.numLogin += 1
	var device string
	for i := 1; i < 4; i++ {
		nvmeConnectInfo, err := getSubSysInfo()
		if err != nil {
			log.Errorf("Get nvme info error: %v", err)
			break
		}

		device, err = scanRoCEDevice(nvmeConnectInfo, targetNQN, tgtPortal, tgtLunGUID)
		if err != nil && err.Error() != "FindNoDevice" {
			log.Errorf("Get device of guid %s error: %v", tgtLunGUID, err)
			break
		}
		if device != "" || nvmeShareData.stopConnecting {
			break
		}

		time.Sleep(time.Second * time.Duration(math.Pow(sleepInternal, float64(i))))
	}

	if device == "" {
		log.Debugf("LUN %s on RoCE portal %s not found on sysfs after logging in.", tgtLunGUID, tgtPortal)
	}

	if device != "" {
		nvmeShareData.foundDevices = append(nvmeShareData.foundDevices, device)
		nvmeShareData.justAddedDevices = append(nvmeShareData.justAddedDevices, device)
	}
	nvmeShareData.stoppedThreads += 1
	return
}

func scanMultiPath(lenIndex int, LunGUID string, nvmeShareData *shareData) (string, string) {
	var wwnAdded bool
	var lastTryOn int64
	var mPath, wwn string
	var err error
	for !((int64(lenIndex) == nvmeShareData.stoppedThreads && len(nvmeShareData.foundDevices) == 0) ||
		(mPath != "" && int64(lenIndex) == nvmeShareData.numLogin+nvmeShareData.failedLogin)) {
		if wwn == "" && len(nvmeShareData.foundDevices) != 0 {
			wwn, err = getSYSfsWwn(nvmeShareData.foundDevices, mPath)
			if err != nil {
				break
			}

			if wwn == "" {
				wwn = LunGUID
			}
		}

		mPath, wwnAdded = scanMultiDevice(mPath, wwn, nvmeShareData, wwnAdded)
		if lastTryOn == 0 && len(nvmeShareData.foundDevices) != 0 && int64(
			lenIndex) == nvmeShareData.stoppedThreads {
			log.Infoln("All connection threads finished, giving 15 seconds for dm to appear.")
			lastTryOn = time.Now().Unix() + 15
		} else if lastTryOn != 0 && lastTryOn < time.Now().Unix() {
			break
		}

		time.Sleep(time.Second)
	}
	return mPath, wwn
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

	for _, device := range foundDevices {
		deviceFile := fmt.Sprintf("/sys/block/%s/wwid", device)
		data, err := ioutil.ReadFile(deviceFile)
		if err != nil {
			msg := fmt.Sprintf("Read device file %s error: %v", deviceFile, err)
			log.Errorln(msg)
			continue
		}

		return string(data), nil
	}

	msg := fmt.Sprintf("Cannot find device %s wwid", foundDevices)
	log.Errorln(msg)
	return "", nil
}

func scanMultiDevice(mPath, wwn string, nvmeShareData *shareData, wwnAdded bool) (string, bool) {
	var err error
	if mPath == "" && len(nvmeShareData.foundDevices) != 0 {
		mPath = connector.FindAvailableMultiPath(nvmeShareData.foundDevices)
		if wwn != "" && !(mPath != "" || wwnAdded) {
			wwnAdded, err = addMultiWWN(wwn)
			if err != nil {
				log.Warningf("Add multiPath wwn failed, error: %s", err)
			}

			mPath = tryScanMultiDevice(mPath, nvmeShareData)
		}
	}

	return mPath, wwnAdded
}

func addMultiWWN(deviceWWN string) (bool, error) {
	output, err := utils.ExecShellCmd("multipath -a %s", deviceWWN)
	if err != nil {
		if strings.TrimSpace(output) != fmt.Sprintf("wwid \"%s\" added", deviceWWN) {
			return false, nil
		}

		msg := "run cmd multipath -a error"
		log.Errorln(msg)
		return false, errors.New(msg)
	}

	return true, nil
}

func tryScanMultiDevice(mPath string, nvmeShareData *shareData) string {
	for mPath == "" && len(nvmeShareData.justAddedDevices) != 0 {
		devicePath := "/dev/" + nvmeShareData.justAddedDevices[0]
		nvmeShareData.justAddedDevices = nvmeShareData.justAddedDevices[1:]
		err := addMultiPath(devicePath)
		if err != nil {
			log.Warningf("Add multiPath path failed, error: %s", err)
		}

		mPath = connector.FindAvailableMultiPath(nvmeShareData.foundDevices)
	}
	return mPath
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

func scanSingle(nvmeShareData *shareData) {
	for i := 0; i < 15; i++ {
		if len(nvmeShareData.foundDevices) != 0 {
			break
		}
		time.Sleep(time.Second * intNumTwo)
	}
}

func getExistSessions() (map[string]bool, error) {
	nvmeConnectInfo, err := getSubSysInfo()
	if err != nil {
		return nil, err
	}
	log.Infof("All SubSysInfo %v", nvmeConnectInfo)

	subSystems, ok := nvmeConnectInfo["Subsystems"].([]interface{})
	if !ok {
		msg := "there are noSubsystems in the nvmeConnectInfo"
		log.Errorln(msg)
		return nil, errors.New(msg)
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
		portal, path := getSubPathInfo(p)
		if portal != "" && path != "" {
			existPortals[portal] = true
		}
	}

	log.Infof("Exist Portals %v", existPortals)
	return existPortals, nil
}

func tryConnectVolume(connMap map[string]interface{}) (string, error) {
	conn, err := getNVMeInfo(connMap)
	if err != nil {
		return "", err
	}

	existSessions, err := getExistSessions()
	if err != nil {
		return "", err
	}

	var mPath string
	var wait sync.WaitGroup
	var nvmeShareData = new(shareData)
	lenIndex := len(conn.tgtPortals)
	if !connMap["volumeUseMultiPath"].(bool) {
		lenIndex = 1
	}
	for index := 0; index < lenIndex; index++ {
		tgtPortal := conn.tgtPortals[index]
		wait.Add(1)

		go func(portal, lunGUID string) {
			defer func() {
				wait.Done()
				if r := recover(); r != nil {
					log.Errorf("Runtime error caught in loop routine: %v", r)
					log.Errorf("%s", debug.Stack())
				}

				log.Flush()
			}()

			connectVol(existSessions, portal, lunGUID, nvmeShareData)
		}(tgtPortal, conn.tgtLunGUID)
	}

	if connMap["volumeUseMultiPath"].(bool) {
		mPath, _ = scanMultiPath(lenIndex, conn.tgtLunGUID, nvmeShareData)
	} else {
		scanSingle(nvmeShareData)
	}

	nvmeShareData.stopConnecting = true
	wait.Wait()

	return findDevice(nvmeShareData, connMap["volumeUseMultiPath"].(bool), mPath, conn.tgtLunGUID)
}

func findDevice(nvmeShareData *shareData, volumeUseMultiPath bool, mPath, tgtLunGUID string) (string, error) {
	if nvmeShareData.foundDevices == nil {
		return "", errors.New("volume device not found")
	}

	if !volumeUseMultiPath {
		device := fmt.Sprintf("/dev/%s", nvmeShareData.foundDevices[0])
		err := connector.VerifySingleDevice(device, tgtLunGUID,
			"volume device not found", false, tryDisConnectVolume)
		if err != nil {
			return "", err
		}
		return device, nil
	}

	// mPath: dm-<id>
	if mPath != "" {
		dev, err := connector.VerifyMultiPathDevice(mPath, tgtLunGUID,
			"volume device not found", false, tryDisConnectVolume)
		if err != nil {
			return "", err
		}
		return dev, nil
	}

	log.Errorln("no device was created")
	return "", errors.New("volume device not found")
}

func getSubSysInfo() (map[string]interface{}, error) {
	output, err := utils.ExecShellCmdFilterLog("nvme list-subsys -o json")
	if err != nil {
		log.Errorf("get exist nvme connect info %s,  error: %s", output, err)
		return nil, errors.New("get nvme connect port failed")
	}

	var nvmeConnectInfo map[string]interface{}
	if err = json.Unmarshal([]byte(output), &nvmeConnectInfo); err != nil {
		return nil, errors.New("unmarshal nvme connect info failed")
	}

	return nvmeConnectInfo, nil
}

func getSubSysPaths(nvmeConnectInfo map[string]interface{}, targetNqn string) []interface{} {
	subSystems, ok := nvmeConnectInfo["Subsystems"].([]interface{})
	if !ok {
		msg := "there are noSubsystems in the nvmeConnectInfo"
		log.Errorln(msg)
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

func getSubPathInfo(p interface{}) (string, string) {
	path, ok := p.(map[string]interface{})
	if !ok {
		return "", ""
	}

	transport, exist := path["Transport"].(string)
	if !exist || transport != "rdma" {
		log.Warningf("Transport does not exist in path %v or Transport value is not rdma.", path)
		return "", ""
	}

	state, exist := path["State"].(string)
	if !exist || state != "live" {
		log.Warningf("The state of path %v is not live.", path)
		return "", ""
	}

	address, exist := path["Address"].(string)
	if !exist {
		log.Warningf("Address does not exist in path %v.", path)
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

	log.Warningf("Didn't find portal in path %v", path)
	return "", ""
}

func getSubSysPort(subPaths []interface{}, tgtPortal string) string {
	for _, p := range subPaths {
		portal, path := getSubPathInfo(p)
		if portal != "" && portal == tgtPortal {
			return path
		}
	}
	return ""
}

func scanNVMeDevice(devicePort string) error {
	output, err := utils.ExecShellCmd("nvme ns-rescan /dev/%s", devicePort)
	if err != nil {
		log.Errorf("scan nvme port error: %s", output)
		return err
	}
	return nil
}

func getNVMeDevice(devicePort string, tgtLunGUID string) (string, error) {
	nvmePortPath := fmt.Sprintf("/sys/devices/virtual/nvme-fabrics/ctl/%s/", devicePort)
	if exist, _ := utils.PathExist(nvmePortPath); !exist {
		msg := fmt.Sprintf("NVMe device path %s is not exist.", nvmePortPath)
		log.Errorf(msg)
		return "", errors.New(msg)
	}

	cmd := fmt.Sprintf("ls %s |grep nvme", nvmePortPath)
	output, err := utils.ExecShellCmd(cmd)
	if err != nil {
		log.Errorf("get nvme device failed, error: %s", err)
		return "", err
	}

	outputLines := strings.Split(output, "\n")
	for _, dev := range outputLines {
		if match, _ := regexp.MatchString(`nvme[0-9]+n[0-9]+`, dev); match {
			uuid, err := getNVMeWWN(devicePort, dev)
			if err != nil {
				log.Warningf("get nvme device uuid failed, error: %s", err)
				continue
			}
			if strings.Contains(uuid, tgtLunGUID) {
				return dev, nil
			}
		}
	}

	msg := fmt.Sprintf("can not find device of lun %s", tgtLunGUID)
	log.Errorln(msg)
	return "", errors.New(msg)
}

func getNVMeWWN(devicePort, device string) (string, error) {
	uuidFile := fmt.Sprintf("/sys/devices/virtual/nvme-fabrics/ctl/%s/%s/wwid", devicePort, device)
	data, err := ioutil.ReadFile(uuidFile)
	if err != nil {
		msg := fmt.Sprintf("Read NVMe uuid file %s error: %v", uuidFile, err)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	if data != nil {
		return string(data), nil
	}

	return "", errors.New("uuid is not exist")
}

func scanRoCEDevice(nvmeConnectInfo map[string]interface{}, targetNqn, tgtPortal, tgtLunGUID string) (string, error) {
	subPaths := getSubSysPaths(nvmeConnectInfo, targetNqn)
	devicePort := getSubSysPort(subPaths, tgtPortal)

	if devicePort == "" {
		msg := fmt.Sprintf("Cannot get nvme device port of portal %s", tgtPortal)
		log.Warningln(msg)
		return "", errors.New(msg)
	}

	err := scanNVMeDevice(devicePort)
	if err != nil {
		return "", err
	}

	return getNVMeDevice(devicePort, tgtLunGUID)
}

func tryDisConnectVolume(tgtLunWWN string, checkDeviceAvailable bool) error {
	device, err := connector.GetDevice(nil, tgtLunWWN, checkDeviceAvailable)
	if err != nil {
		log.Warningf("Get device of WWN %s error: %v", tgtLunWWN, err)
		return err
	}

	devices, multiPathName, err := connector.RemoveRoCEDevice(device)
	if err != nil {
		log.Errorf("Remove device %s error: %v", device, err)
		return err
	}

	err = disconnectSessions(devices)
	if err != nil {
		log.Warningf("Disconnect RoCE controller %s error: %v", devices, err)
		return err
	}

	if multiPathName != "" {
		time.Sleep(time.Second * intNumThree)
		err = connector.FlushDMDevice(device)
		if err != nil {
			return err
		}
	}

	return nil
}

func disconnectSessions(devPaths []string) error {
	for _, dev := range devPaths {
		splitS := strings.Split(dev, "n")
		if len(splitS) != intNumThree {
			continue
		}

		nvmePort := fmt.Sprintf("n%s", splitS[1])
		cmd := fmt.Sprintf("ls /sys/devices/virtual/nvme-fabrics/ctl/%s/ |grep nvme |wc -l |awk " +
			"'{if($1>1) print 1; else print 0}'", nvmePort)
		output, err := utils.ExecShellCmd(cmd)
		if err != nil {
			log.Infof("Disconnect RoCE target path %s failed, err: %v", dev, err)
			return err
		}
		outputSplit := strings.Split(output, "\n")
		if len(outputSplit) != 0 && outputSplit[0] == "0" {
			disconnectRoCEController(nvmePort)
		}
	}
	return nil
}

func disconnectRoCEController(devPath string) {
	output, err := utils.ExecShellCmd("nvme disconnect -d %s", devPath)
	if err != nil || output != "" {
		log.Errorf("Disconnect controller %s error %v", devPath, err)
	}
}
