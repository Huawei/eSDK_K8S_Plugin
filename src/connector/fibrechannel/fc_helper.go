package fibrechannel

import (
	"connector"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"utils"
	"utils/log"
)

type target struct {
	tgtWWN     string
	tgtHostLun string
}

type rawDevice struct {
	platform string
	pciNum   string
	wwn      string
	lun      string
}

type deviceInfo struct {
	tries          int
	hostDevice     string
	realDeviceName string
}

type connectorInfo struct {
	tgtWWNs     []string
	tgtHostLUNs []string
	tgtTargets  []target
	volumeUseMultiPath bool
}

const (
	deviceScanAttemptsDefault int = 3
)

func scanHost() {
	output, err := utils.ExecShellCmd("for host in $(ls /sys/class/fc_host/); " +
		"do echo \"- - -\" > /sys/class/scsi_host/${host}/scan; done")
	if err != nil {
		log.Warningf("rescan fc_host error: %s", output)
	}
}

func parseFCInfo(connectionProperties map[string]interface{}) (*connectorInfo, error) {
	tgtWWNs, WWNsExist := connectionProperties["tgtWWNs"].([]string)
	if !WWNsExist {
		msg := "there are no target WWNs in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	tgtHostLUNs, hostLunIdExist := connectionProperties["tgtHostLUNs"].([]string)
	if !hostLunIdExist {
		msg := "there are no target hostLun in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	volumeUseMultiPath, useMultiPathExist := connectionProperties["volumeUseMultiPath"].(bool)
	if !useMultiPathExist {
		msg := "there are no multiPath switch in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if tgtWWNs == nil || len(tgtWWNs) != len(tgtHostLUNs) {
		msg := "the numbers of tgtWWNs and tgtHostLUNs are not equal"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	var con connectorInfo
	con.tgtWWNs = tgtWWNs
	con.tgtHostLUNs = tgtHostLUNs
	con.volumeUseMultiPath = volumeUseMultiPath
	return &con, nil
}

func constructFCInfo(conn *connectorInfo) {
	for index := range conn.tgtWWNs {
		conn.tgtTargets = append(conn.tgtTargets, target{conn.tgtWWNs[index],
			conn.tgtHostLUNs[index]})
	}
}

func tryConnectVolume(connMap map[string]interface{}) (string, error) {
	conn, err := parseFCInfo(connMap)
	if err != nil {
		return "", err
	}

	constructFCInfo(conn)
	hbas, err := getFcHBAsInfo()
	if err != nil {
		return "", err
	}

	hostDevice := getPossibleVolumePath(conn.tgtTargets, hbas)
	devInfo, err := waitDeviceDiscovery(hbas, hostDevice, conn.tgtTargets, conn.volumeUseMultiPath)
	if err != nil {
		return "", err
	}

	if devInfo.realDeviceName == "" {
		return "", errors.New("NoFibreChannelVolumeDeviceFound")
	}

	log.Infof("Found Fibre Channel volume %v (after %d rescans.)", devInfo, devInfo.tries + 1)
	deviceWwn, err := connector.GetSCSIWwn(devInfo.hostDevice)
	if err != nil {
		return "", err
	}

	if !conn.volumeUseMultiPath {
		err := connector.WaitDeviceRW(deviceWwn, devInfo.realDeviceName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("/dev/%s", devInfo.realDeviceName), nil
	}

	mPath := connector.FindMultiDevicePath(deviceWwn)
	if mPath != "" {
		err := connector.WaitDeviceRW(deviceWwn, mPath)
		if err != nil {
			return "", err
		}
		return mPath, nil
	}

	return "", errors.New("NoFibreChannelVolumeDeviceFound")
}

func getFcHBAs() ([]map[string]string, error) {
	if !supportFC() {
		return nil, errors.New("no Fibre Channel support detected on system")
	}

	output, err := utils.ExecShellCmd("systool -c fc_host -v")
	if err != nil && strings.Contains(err.Error(), "command not found") {
		return nil, err
	}

	var hbas []map[string]string
	hba := make(map[string]string)
	lastLine := "defaultLine"
	lines := strings.Split(output, "\n")[2:]
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" && lastLine == "" {
			if len(hba) > 0 {
				hbas = append(hbas, hba)
				hba = make(map[string]string)
			}
		} else {
			val := strings.Split(line, "=")
			if len(val) == 2 {
				key := strings.Replace(strings.TrimSpace(val[0]), " ", "", -1)
				value := strings.Replace(strings.TrimSpace(val[1]), "\"", "", -1)
				hba[key] = value
			}
		}
		lastLine = line
	}

	return hbas, nil
}

func getFcHBAsInfo() ([]map[string]string, error) {
	hbas, err := getFcHBAs()
	if err != nil {
		return nil, err
	}

	var hbaInfos []map[string]string
	for _, hba := range hbas {
		wwpn := strings.Replace(hba["port_name"], "0x", "", -1)
		wwnn := strings.Replace(hba["node_name"], "0x", "", -1)
		hbaInfo := map[string]string{
			"port_name":   wwpn,
			"node_name":   wwnn,
			"host_device": hba["ClassDevice"],
			"device_path": hba["ClassDevicepath"],
		}
		hbaInfos = append(hbaInfos, hbaInfo)
	}
	return hbaInfos, nil
}

func supportFC() bool {
	fcHostSysFSPath := "/sys/class/fc_host"
	if exist, _ := utils.PathExist(fcHostSysFSPath); !exist {
		return false
	}

	return true
}

func getPossibleVolumePath(targets []target, hbas []map[string]string) []string {
	possibleDevices := getPossibleDeices(hbas, targets)
	return getHostDevices(possibleDevices)
}

func getPossibleDeices(hbas []map[string]string, targets []target) []rawDevice {
	var rawDevices []rawDevice
	for _, hba := range hbas {
		platform, pciNum := getPciNumber(hba)
		if pciNum != "" {
			for _, target := range targets {
				targetWWN := fmt.Sprintf("0x%s", strings.ToLower(target.tgtWWN))
				rawDev := rawDevice{platform, pciNum, targetWWN, target.tgtHostLun}
				rawDevices = append(rawDevices, rawDev)
			}
		}
	}

	return rawDevices
}

func getPci(devPath []string) (string, string) {
	var platform string
	platformSupport := len(devPath) > 3 && devPath[3] == "platform"
	for index, value := range devPath {
		if platformSupport && strings.HasPrefix(value, "pci") {
			platform = fmt.Sprintf("platform-%s", devPath[index-1])
		}
		if strings.HasPrefix(value, "net") || strings.HasPrefix(value, "host") {
			return platform, devPath[index-1]
		}
	}
	return "", ""
}

func getPciNumber(hba map[string]string) (string, string) {
	if hba == nil {
		return "", ""
	}

	if _, exist := hba["device_path"]; exist {
		devPath := strings.Split(hba["device_path"], "/")
		platform, device := getPci(devPath)
		if device != "" {
			return platform, device
		}
	}

	return "", ""
}

func formatLunId(lunId string) string {
	intLunId, _ := strconv.Atoi(lunId)
	if intLunId < 256 {
		return lunId
	} else {
		return fmt.Sprintf("0x%04x%04x00000000", intLunId&0xffff, intLunId>>16&0xffff)
	}
}

func getHostDevices(possibleDevices []rawDevice) []string {
	var hostDevices []string
	var platform string
	for _, value := range possibleDevices {
		if value.platform != "" {
			platform = value.platform + "-"
		} else {
			platform = ""
		}

		hostDevice := fmt.Sprintf("/dev/disk/by-path/%spci-%s-fc-%s-lun-%s", platform, value.pciNum, value.wwn, formatLunId(value.lun))
		hostDevices = append(hostDevices, hostDevice)
	}
	return hostDevices
}

func checkValidDevice(dev string) bool {
	cmd := fmt.Sprintf("dd if=%s of=/dev/null count=1", dev)
	output, err := utils.ExecShellCmd(cmd)
	if err != nil {
		log.Errorf("Failed to access the device on the path %s: %v", dev, err)
		return false
	}

	if output == "" {
		return false
	}

	if strings.Contains(output, "0+0 records in") {
		log.Errorf("the size of %s may be zero, it is abnormal device.", dev)
		return false
	}

	return true
}

func waitDeviceDiscovery(hbas []map[string]string, hostDevices []string, targets []target, volumeUseMultiPath bool) (
	deviceInfo, error) {
	var info deviceInfo
	err := utils.WaitUntil(func() (bool, error) {
		for _, dev := range hostDevices {
			if exist, _ := utils.PathExist(dev); exist && checkValidDevice(dev) {
				info.hostDevice = dev
				if realPath, err := os.Readlink(dev); err == nil {
					info.realDeviceName = filepath.Base(realPath)
				}
				return true, nil
			}
		}

		if info.tries >= deviceScanAttemptsDefault {
			log.Errorln("Fibre Channel volume device not found.")
			return false, errors.New("NoFibreChannelVolumeDeviceFound")
		}

		rescanHosts(hbas, targets, volumeUseMultiPath)
		info.tries += 1
		return false, nil
	}, time.Second*60, time.Second*2)
	return info, err
}

func getHBAChannelSCSITargetLun(hba map[string]string, targets []target) ([][]string, []string) {
	hostDevice := hba["host_device"]
	if hostDevice != "" && len(hostDevice) > 4 {
		hostDevice = hostDevice[4:]
	}

	path := fmt.Sprintf("/sys/class/fc_transport/target%s:", hostDevice)

	var channelTargetLun [][]string
	var lunNotFound []string
	for _, tar := range targets {
		cmd := fmt.Sprintf("grep -Gil \"%s\" %s*/port_name", tar.tgtWWN, path)
		output, err := utils.ExecShellCmd(cmd)
		if err != nil {
			lunNotFound = append(lunNotFound, tar.tgtHostLun)
			continue
		}

		lines := strings.Split(output, "\n")
		var tempCtl [][]string
		for _, line := range lines {
			if strings.HasPrefix(line, path) {
				ctl := append(strings.Split(strings.Split(line, "/")[4], ":")[1:], tar.tgtHostLun)
				tempCtl = append(tempCtl, ctl)
			}
		}

		channelTargetLun = append(channelTargetLun, tempCtl...)
	}

	return channelTargetLun, lunNotFound
}

func rescanHosts(hbas []map[string]string, targets []target, volumeUseMultiPath bool) {
	var process []interface{}
	var skipped []interface{}
	for _, hba := range hbas {
		ctls, lunWildCards := getHBAChannelSCSITargetLun(hba, targets)
		if ctls != nil {
			process = append(process, []interface{}{hba, ctls})
		} else if process == nil {
			var lunInfo []interface{}
			for _, lun := range lunWildCards {
				lunInfo = append(lunInfo, []string{"-", "-", lun})
			}
			skipped = append(skipped, []interface{}{hba, lunInfo})
		}
	}

	if process == nil {
		process = skipped
	}

	for _, p := range process {
		pro := p.([]interface{})
		hba := pro[0].(map[string]string)
		ctls := pro[1].([][]string)
		for _, c := range ctls {
			scanFC(c, hba["host_device"])
			if !volumeUseMultiPath {
				break
			}
		}
		if !volumeUseMultiPath {
			break
		}
	}
}

func scanFC(channelTargetLun []string, hostDevice string) {
	scanCommand := fmt.Sprintf("echo \"%s %s %s\" > /sys/class/scsi_host/%s/scan",
		channelTargetLun[0], channelTargetLun[1], channelTargetLun[2], hostDevice)
	output, err := utils.ExecShellCmd(scanCommand)
	if err != nil {
		log.Warningf("rescan FC host error: %s", output)
	}
}

func tryDisConnectVolume(tgtLunWWN string) error {
	device, err := connector.GetDevice(nil, tgtLunWWN)
	if err != nil {
		log.Errorf("Get device of WWN %s error: %v", tgtLunWWN, err)
		return err
	}

	multiPathName, err := connector.RemoveDevice(device)
	if err != nil {
		log.Errorf("Remove device %s error: %v", device, err)
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
