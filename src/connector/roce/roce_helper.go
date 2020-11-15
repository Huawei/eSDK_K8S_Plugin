package roce

import (
	"connector"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"utils"
	"utils/log"
)

type connectorInfo struct {
	tgtPortals []string
	tgtLunGuids []string
}

type shareData struct {
	stopConnecting bool
	numLogin       int64
	failedLogin    int64
	stoppedThreads int64
	foundDevices   []string
	findDeviceMap  map[string]string
}

func getNVMeInfo(connectionProperties map[string]interface{}) (*connectorInfo, error) {
	var con connectorInfo

	tgtPortals, portalExist := connectionProperties["tgtPortals"].([]string)
	if !portalExist {
		msg := "there are no target portals in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	tgtLunGuids, lunGuidExist := connectionProperties["tgtLunGuids"].([]string)
	if !lunGuidExist {
		msg := "there are no target lun guid in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if tgtLunGuids == nil || len(tgtPortals) != len(tgtLunGuids) {
		msg := "the num of tgtPortals and num of tgtLunGuids is not equal"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	con.tgtPortals = tgtPortals
	con.tgtLunGuids = tgtLunGuids
	return &con, nil
}

func buildNVMeSession(allSessions, tgtPortal string) (string, error) {
	output, err := utils.ExecShellCmd("nvme discover -t rdma -a %s", tgtPortal)
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

	if strings.Contains(allSessions, tgtPortal) {
		log.Infof("RoCE target %s has already login, no need login again", tgtPortal)
		return tgtNqn, nil
	} else {
		output, err = utils.ExecShellCmd("nvme connect -t rdma -a %s -n %s", tgtPortal, tgtNqn)
		if err != nil {
			log.Errorf("Cannot login nvme target %s, reason: %v", tgtPortal, output)
			return "", err
		}
	}

	return tgtNqn, nil
}

func singleConnectVolume(allSessions, tgtPortal, tgtLunGuid string, nvmeShareData *shareData) {
	var device string

	tgtNqn, err := buildNVMeSession(allSessions, tgtPortal)
	if err != nil {
		log.Errorf("build nvme session %s error, reason: %v", tgtPortal, err)
		nvmeShareData.failedLogin += 1
	} else {
		nvmeShareData.numLogin += 1
		connectInfo := map[string]interface{} {
			"protocol": "iscsi",
			"targetNqn": tgtNqn,
		}
		for i := 1; i < 4; i++ {
			connector.ScanNVMe(connectInfo)
			device, err = connector.GetDevice(nvmeShareData.findDeviceMap, tgtLunGuid)
			if err != nil {
				log.Errorf("Get device of guid %s error: %v", tgtLunGuid, err)
				break
			}
			if device != "" {
				break
			}

			if !nvmeShareData.stopConnecting {
				time.Sleep(time.Second * time.Duration(math.Pow(2, float64(i))))
			} else {
				break
			}
		}

		if device != "" {
			nvmeShareData.foundDevices = append(nvmeShareData.foundDevices, device)
			if nvmeShareData.findDeviceMap == nil {
				nvmeShareData.findDeviceMap = map[string]string{
					device: device,
				}
			} else {
				nvmeShareData.findDeviceMap[device] = device
			}
		}
	}

	nvmeShareData.stoppedThreads += 1
	return
}

func findMultiPath(tgtLunWWN string) (string, error) {
	output, err := utils.ExecShellCmd("multipath -l | grep %s", tgtLunWWN)
	if err != nil {
		if strings.Contains(output, "command not found") {
			msg := fmt.Sprintf("run cmd multipath -l error, error: %s", output)
			log.Errorln(msg)
			return "", errors.New(msg)
		}

		return "", err
	}

	var mPath string
	if output != "" {
		multiLines := strings.Split(output, " ")
		for _, line := range multiLines {
			if strings.HasPrefix(line, "dm") {
				mPath = line
				break
			}
		}
	}

	return mPath, nil
}

func findTgtMultiPath(lenIndex int, nvmeShareData *shareData, conn *connectorInfo) string {
	var mPath string
	var lastTryOn int64
	for {
		if (int64(lenIndex) == nvmeShareData.stoppedThreads && nvmeShareData.foundDevices == nil) || (
			mPath != "" && int64(lenIndex) == nvmeShareData.numLogin+nvmeShareData.failedLogin) {
			break
		}

		mPath, err := findMultiPath(conn.tgtLunGuids[0])
		if err != nil {
			log.Warningf("Can not find dm path, error: %s", err)
		}

		if mPath != "" {
			return mPath
		}

		if lastTryOn == 0 && nvmeShareData.foundDevices != nil && int64(lenIndex) == nvmeShareData.stoppedThreads {
			log.Infoln("All connection threads finished, giving 15 seconds for dm to appear.")
			lastTryOn = time.Now().Unix() + 15
		} else if lastTryOn != 0 && lastTryOn < time.Now().Unix() {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return ""
}

func tryConnectVolume(connMap map[string]interface{}) (string, error) {
	conn, err := getNVMeInfo(connMap)
	if err != nil {
		return "", err
	}

	allSessions, err := utils.ExecShellCmd("nvme list-subsys")
	if err != nil {
		return "", err
	}

	var mPath string
	var wait sync.WaitGroup
	var nvmeShareData = new(shareData)
	lenIndex := len(conn.tgtPortals)
	for index := 0; index < lenIndex; index++ {
		tgtPortal := conn.tgtPortals[index]
		tgtLunGuid := conn.tgtLunGuids[index]

		wait.Add(1)

		go func() {
			defer func() {
				wait.Done()
				if r := recover(); r != nil {
					log.Errorf("Runtime error caught in loop routine: %v", r)
					log.Errorf("%s", debug.Stack())
				}

				log.Flush()
			}()

			singleConnectVolume(allSessions, tgtPortal, tgtLunGuid, nvmeShareData)
		}()
	}

	if lenIndex > 1 && mPath == "" {
		mPath = findTgtMultiPath(lenIndex, nvmeShareData, conn)
	}

	nvmeShareData.stopConnecting = true
	wait.Wait()

	if mPath != "" {
		mPath = fmt.Sprintf("/dev/%s", mPath)
		log.Infof("Found the dm path %s", mPath)
		return mPath, nil
	} else {
		log.Infoln("no dm was created, connection to volume is probably bad and will perform poorly")
	}

	if nvmeShareData.foundDevices != nil {
		dev := fmt.Sprintf("/dev/%s", nvmeShareData.foundDevices[0])
		log.Infof("find the dev %s", nvmeShareData.foundDevices[0])
		return dev, nil
	}

	msg := fmt.Sprintf("volume device not found, lun is %s", conn.tgtLunGuids[0])
	log.Errorln(msg)
	return "", errors.New(msg)
}
