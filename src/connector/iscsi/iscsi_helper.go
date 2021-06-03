package iscsi

import (
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/src/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
)

type connectorInfo struct {
	tgtPortals []string
	tgtLunWWNs []string
}

type shareData struct {
	stopConnecting bool
	numLogin       int64
	failedLogin    int64
	stoppedThreads int64
	foundDevices   []string
	findDeviceMap  map[string]string
}

func getISCSIInfo(connectionProperties map[string]interface{}) (*connectorInfo, error) {
	var con connectorInfo

	tgtPortals, portalExist := connectionProperties["tgtPortals"].([]string)
	if !portalExist {
		msg := "there are no target portals in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	tgtLunWWNs, wwnExist := connectionProperties["tgtLunWWNs"].([]string)
	if !wwnExist {
		msg := "there are no target lun wwns in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if tgtLunWWNs == nil || len(tgtPortals) != len(tgtLunWWNs) {
		msg := "the num of tgtPortals and num of tgtLunWWN is not equal"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	con.tgtPortals = tgtPortals
	con.tgtLunWWNs = tgtLunWWNs
	return &con, nil
}

func buildISCSISession(allSessions, tgtPortal string) error {
	if strings.Contains(allSessions, tgtPortal) {
		log.Infof("iscsi target %s has already login, no need login again", tgtPortal)
		return nil
	}

	output, err := utils.ExecShellCmd("iscsiadm -m discovery -t sendtargets -p %s", tgtPortal)
	if err != nil {
		log.Errorf("Cannot discover iscsi target %s, reason: %v", tgtPortal, output)
		return err
	}

	output, err = utils.ExecShellCmd("iscsiadm -m node -p %s --login", tgtPortal)
	if err != nil {
		log.Errorf("Cannot login iscsi target %s, reason: %v", tgtPortal, output)
		return err
	}

	return nil
}

func scanHost() {
	output, err := utils.ExecShellCmd("for host in $(ls /sys/class/iscsi_host/); " +
		"do echo \"- - -\" > /sys/class/scsi_host/${host}/scan; done")
	if err != nil {
		log.Warningf("rescan iscsi_host error: %s", output)
	}
}

func singleConnectVolume(allSessions, tgtPortal, tgtLunWWN string, iscsiShareData *shareData) {
	var device string

	err := buildISCSISession(allSessions, tgtPortal)
	if err != nil {
		log.Errorf("build iscsi session %s error, reason: %v", tgtPortal, err)
		iscsiShareData.failedLogin += 1
	} else {
		iscsiShareData.numLogin += 1
		for i := 1; i < 4; i++ {
			scanHost()
			device, err = connector.GetDevice(iscsiShareData.findDeviceMap, tgtLunWWN)
			if err != nil {
				log.Errorf("Get device of LUN WWN %s error: %v", tgtLunWWN, err)
				break
			}
			if device != "" {
				break
			}

			if !iscsiShareData.stopConnecting {
				time.Sleep(time.Second * time.Duration(math.Pow(2, float64(i))))
			} else {
				break
			}
		}

		if device != "" {
			iscsiShareData.foundDevices = append(iscsiShareData.foundDevices, device)
			if iscsiShareData.findDeviceMap == nil {
				iscsiShareData.findDeviceMap = map[string]string{
					device: device,
				}
			} else {
				iscsiShareData.findDeviceMap[device] = device
			}
		}
	}

	iscsiShareData.stoppedThreads += 1
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

func addMultiParams(iscsiShareData *shareData, conn *connectorInfo) bool {
	wwnAdded, err := addMultiWWN(conn.tgtLunWWNs[0])
	if err != nil {
		log.Warningf("Add multiPath wwn failed, error: %s", err)
	}

	for _, d := range iscsiShareData.foundDevices {
		path := fmt.Sprintf("/dev/%s", d)
		err := addMultiPath(path)
		if err != nil {
			log.Warningf("Add multiPath path failed, error: %s", err)
		}
	}
	return wwnAdded
}

func findEachMultiPath(lenIndex int, iscsiShareData *shareData, conn *connectorInfo) (string, bool) {
	var mPath string
	mPath, err := findMultiPath(conn.tgtLunWWNs[0])
	if err != nil {
		log.Warningf("Can not find dm path, error: %s", err)
	}

	if (int64(lenIndex) == iscsiShareData.stoppedThreads && iscsiShareData.foundDevices == nil) || (
		mPath != "" && int64(lenIndex) == iscsiShareData.numLogin+iscsiShareData.failedLogin) {
		return mPath, true
	}

	return mPath, false
}

func findTgtMultiPath(lenIndex int, iscsiShareData *shareData, conn *connectorInfo) string {
	var wwnAdded bool
	var lastTryOn int64

	err := utils.WaitUntil(func() (bool, error) {
		mPath, finished := findEachMultiPath(lenIndex, iscsiShareData, conn)
		if finished {
			return true, nil
		}

		if mPath == "" && !wwnAdded && iscsiShareData.foundDevices != nil {
			wwnAdded = addMultiParams(iscsiShareData, conn)
		}

		if mPath != "" {
			return true, nil
		}

		if lastTryOn == 0 && iscsiShareData.foundDevices != nil && int64(lenIndex) == iscsiShareData.stoppedThreads {
			log.Infoln("All connection threads finished, giving 15 seconds for dm to appear.")
			lastTryOn = time.Now().Unix() + 15
		} else if lastTryOn != 0 && lastTryOn < time.Now().Unix() {
			return true, nil
		}

		return false, nil
	}, time.Second*120, time.Second*5)

	if err != nil {
		return ""
	}

	mPath, err := findMultiPath(conn.tgtLunWWNs[0])
	if err != nil {
		return ""
	}

	return mPath
}

func tryConnectVolume(connMap map[string]interface{}) (string, error) {
	conn, err := getISCSIInfo(connMap)
	if err != nil {
		return "", err
	}

	allSessions, _ := utils.ExecShellCmd("iscsiadm -m session")

	var mPath string
	var wait sync.WaitGroup
	var iscsiShareData = new(shareData)
	lenIndex := len(conn.tgtPortals)
	for index := 0; index < lenIndex; index++ {
		tgtPortal := conn.tgtPortals[index]
		tgtLunWWN := conn.tgtLunWWNs[index]

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

			singleConnectVolume(allSessions, tgtPortal, tgtLunWWN, iscsiShareData)
		}()
	}

	if lenIndex > 1 && mPath == "" {
		mPath = findTgtMultiPath(lenIndex, iscsiShareData, conn)
	}

	iscsiShareData.stopConnecting = true
	wait.Wait()

	if mPath != "" {
		mPath = fmt.Sprintf("/dev/%s", mPath)
		log.Infof("Found the dm path %s", mPath)
		return mPath, nil
	} else {
		log.Infoln("no dm was created, connection to volume is probably bad and will perform poorly")
	}

	if iscsiShareData.foundDevices != nil {
		dev := fmt.Sprintf("/dev/%s", iscsiShareData.foundDevices[0])
		log.Infof("Found the dev %s", iscsiShareData.foundDevices[0])
		return dev, nil
	}

	msg := fmt.Sprintf("volume device not found, lun is %s", conn.tgtLunWWNs[0])
	log.Errorln(msg)
	return "", errors.New(msg)
}
