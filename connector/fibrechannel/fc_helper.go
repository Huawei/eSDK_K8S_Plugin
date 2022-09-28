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

package fibrechannel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"huawei-csi-driver/connector"
	connutils "huawei-csi-driver/connector/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
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
	tgtLunWWN          string
	tgtWWNs            []string
	tgtHostLUNs        []string
	tgtTargets         []target
	volumeUseMultiPath bool
	multiPathType      string
	pathCount          int
}

const (
	deviceScanAttemptsDefault int = 3
	intNumTwo                 int = 2
)

var expectPathCount sync.Map

func getDevicePathNumber(lunWWN string) int {
	v, ok := expectPathCount.Load(lunWWN)
	if !ok {
		return 1
	}

	number, ok := v.(int)
	if !ok {
		return 1
	}

	return number
}

func parseFCInfo(ctx context.Context, connectionProperties map[string]interface{}) (*connectorInfo, error) {
	var info = new(connectorInfo)
	var exist bool

	info.tgtLunWWN, exist = connectionProperties["tgtLunWWN"].(string)
	if !exist {
		return info, utils.Errorln(ctx, "key tgtLunWWN does not exist in connectionProperties")
	}

	info.tgtWWNs, exist = connectionProperties["tgtWWNs"].([]string)
	if !exist {
		return info, utils.Errorln(ctx, "key tgtWWNs does not exist in connectionProperties")
	}

	info.tgtHostLUNs, exist = connectionProperties["tgtHostLUNs"].([]string)
	if !exist {
		return info, utils.Errorln(ctx, "key tgtHostLUNs does not exist in connectionProperties")
	}

	var err error
	info.volumeUseMultiPath, info.multiPathType, err = connutils.GetMultiPathInfo(connectionProperties)
	if err != nil {
		return info, utils.Errorf(ctx, "failed to execute GetMultiPathInfo. %v", err)
	}

	if len(info.tgtWWNs) != len(info.tgtHostLUNs) {
		return info, utils.Errorf(ctx, "the numbers of tgtWWNs and tgtHostLUNs are not equal. %d %d",
			len(info.tgtWWNs), len(info.tgtHostLUNs))
	}

	return info, nil
}

func constructFCInfo(conn *connectorInfo) {
	for index := range conn.tgtWWNs {
		conn.tgtTargets = append(conn.tgtTargets, target{conn.tgtWWNs[index],
			conn.tgtHostLUNs[index]})
	}
}

func tryConnectVolume(ctx context.Context, connMap map[string]interface{}) (string, error) {
	conn, err := parseFCInfo(ctx, connMap)
	if err != nil {
		return "", utils.Errorf(ctx, "failed to execute parseFCInfo. %v", err)
	}

	constructFCInfo(conn)
	hbas, err := getFcHBAsInfo(ctx)
	if err != nil {
		return "", utils.Errorf(ctx, "failed to execute getFcHBAsInfo. %v", err)
	}

	hostDevice := getPossibleVolumePath(ctx, conn.tgtTargets, hbas)
	if len(hostDevice) == 0 {
		return "", utils.Errorln(ctx, "can not find any Fibre Channel devices, "+
			"Please check the host's fiber network.")
	}

	devInfo, err := scanDevice(ctx, hbas, hostDevice, conn)
	if err != nil {
		return "", utils.Errorf(ctx, "failed to execute waitDeviceDiscovery. %v", err)
	}

	if devInfo.realDeviceName == "" {
		log.AddContext(ctx).Warningln("No FibreChannel volume device found")
		return "", errors.New(connector.VolumeNotFound)
	}

	return checkPathAvailable(ctx, *conn, devInfo)
}

func scanDevice(ctx context.Context,
	hbas []map[string]string,
	hostDevices []string,
	conn *connectorInfo) (deviceInfo, error) {
	if !conn.volumeUseMultiPath {
		return waitDeviceDiscovery(ctx, hbas, hostDevices, conn)
	}

	switch conn.multiPathType {
	case connector.DMMultiPath:
		return waitDeviceDiscovery(ctx, hbas, hostDevices, conn)
	case connector.HWUltraPath, connector.HWUltraPathNVMe:
		return waitUltraPathDeviceDiscovery(ctx, hbas, conn)
	default:
		return deviceInfo{}, utils.Errorf(ctx, "%s: %s", connector.UnsupportedMultiPathType, conn.multiPathType)
	}
}

func checkPathAvailable(ctx context.Context, conn connectorInfo, devInfo deviceInfo) (string, error) {
	log.AddContext(ctx).Infof("Found Fibre Channel volume %v (after %d rescans.)", devInfo, devInfo.tries+1)

	if !conn.volumeUseMultiPath {
		return checkSinglePathAvailable(ctx, devInfo.realDeviceName, conn.tgtLunWWN)
	}

	switch conn.multiPathType {
	case connector.DMMultiPath:
		return connector.VerifyDeviceAvailableOfDM(ctx, conn.tgtLunWWN, conn.pathCount, []string{devInfo.realDeviceName}, tryDisConnectVolume)
	case connector.HWUltraPath:
		return connector.GetDiskPathAndCheckStatus(ctx, connector.UltraPathCommand, conn.tgtLunWWN)
	case connector.HWUltraPathNVMe:
		return connector.GetDiskPathAndCheckStatus(ctx, connector.UltraPathNVMeCommand, conn.tgtLunWWN)
	default:
		log.AddContext(ctx).Errorf("%s. %s", connector.UnsupportedMultiPathType, conn.multiPathType)
		return "", errors.New(connector.UnsupportedMultiPathType)
	}
}

func checkSinglePathAvailable(ctx context.Context, realDeviceName, tgtLunWWN string) (string, error) {
	device := fmt.Sprintf("/dev/%s", realDeviceName)
	err := connector.VerifySingleDevice(ctx, device, tgtLunWWN,
		connector.VolumeDeviceNotFound, tryDisConnectVolume)
	if err != nil {
		return "", utils.Errorf(ctx, "failed to execute connector.VerifySingleDevice. %v", err)
	}
	return device, nil
}

func getHostInfo(ctx context.Context, host, portAttr string) (string, error) {
	output, err := utils.ExecShellCmd(ctx, "cat /sys/class/fc_host/%s/%s", host, portAttr)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host %s FC initiator Attr %s output: %s", host, portAttr, output)
		return "", err
	}

	return output, nil
}

func getHostAttrName(ctx context.Context, host, portAttr string) (string, error) {
	nodeName, err := getHostInfo(ctx, host, portAttr)
	if err != nil {
		return "", err
	}

	lines := strings.Split(nodeName, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "0x") {
			continue
		}
		attrWwn := line[2:]
		return attrWwn, nil
	}

	msg := fmt.Sprintf("Can not find the %s of host %s", portAttr, host)
	log.AddContext(ctx).Errorln(msg)
	return "", errors.New(msg)
}

func isPortOnline(ctx context.Context, host string) (bool, error) {
	output, err := utils.ExecShellCmd(ctx, "cat /sys/class/fc_host/%s/port_state", host)
	if err != nil {
		return false, err
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		return line == "Online", nil
	}

	return false, errors.New("check port state error")
}

func getClassDevicePath(ctx context.Context, host string) (string, error) {
	hostPath := fmt.Sprintf("/sys/class/fc_host/%s", host)
	classDevicePath, err := filepath.EvalSymlinks(hostPath)
	if err != nil || classDevicePath == "" {
		msg := fmt.Sprintf("Get host %s class device path failed.", host)
		log.AddContext(ctx).Errorln(msg)
		return "", errors.New(msg)
	}

	return classDevicePath, nil
}

func getAllFcHosts(ctx context.Context) ([]string, error) {
	output, err := utils.ExecShellCmd(ctx, "ls /sys/class/fc_host/")
	if err != nil {
		return nil, err
	}

	var hosts []string
	hostLines := strings.Fields(output)
	for _, h := range hostLines {
		host := strings.TrimSpace(h)
		hosts = append(hosts, host)
	}

	return hosts, nil
}

func getAvailableFcHBAsInfo(ctx context.Context) ([]map[string]string, error) {
	allFcHosts, err := getAllFcHosts(ctx)
	if err != nil {
		return nil, err
	}
	if allFcHosts == nil {
		return nil, errors.New("there is no fc host")
	}

	var hbas []map[string]string
	for _, h := range allFcHosts {
		hbaInfo, err := getFcHbaInfo(ctx, h)
		if err != nil {
			log.AddContext(ctx).Warningf("Get Fc HBA info error %v", err)
			continue
		}
		hbas = append(hbas, hbaInfo)
	}
	log.AddContext(ctx).Infof("Get available hbas are %v", hbas)
	return hbas, nil
}

func getFcHbaInfo(ctx context.Context, host string) (map[string]string, error) {
	online, err := isPortOnline(ctx, host)
	if err != nil || !online {
		return nil, errors.New("the port state is not available")
	}

	portName, err := getHostAttrName(ctx, host, "port_name")
	if err != nil {
		return nil, errors.New("the port name is not available")
	}

	nodeName, err := getHostAttrName(ctx, host, "node_name")
	if err != nil {
		return nil, errors.New("the node name is not available")
	}

	classDevicePath, err := getClassDevicePath(ctx, host)
	if err != nil {
		return nil, errors.New("the device path is not available")
	}

	hba := map[string]string{
		"port_name":   portName,
		"node_name":   nodeName,
		"host_device": host,
		"device_path": classDevicePath,
	}
	return hba, nil
}

func getFcHBAsInfo(ctx context.Context) ([]map[string]string, error) {
	if !supportFC() {
		return nil, errors.New("no Fibre Channel support detected on system")
	}

	hbas, err := getAvailableFcHBAsInfo(ctx)
	if err != nil || hbas == nil {
		return nil, errors.New("there is no available port")
	}

	return hbas, nil
}

func supportFC() bool {
	fcHostSysFSPath := "/sys/class/fc_host"
	if exist, _ := utils.PathExist(fcHostSysFSPath); !exist {
		return false
	}

	return true
}

func getPossibleVolumePath(ctx context.Context, targets []target, hbas []map[string]string) []string {
	possibleDevices := getPossibleDeices(hbas, targets)
	return getHostDevices(ctx, possibleDevices)
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

func getHostDevices(ctx context.Context, possibleDevices []rawDevice) []string {
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
	log.AddContext(ctx).Infof("Get host devices are %v", hostDevices)
	return hostDevices
}

func checkValidDevice(ctx context.Context, dev string) bool {
	_, err := connector.ReadDevice(ctx, dev)
	if err != nil {
		return false
	}

	return true
}

func waitDeviceDiscovery(ctx context.Context,
	hbas []map[string]string,
	hostDevices []string,
	conn *connectorInfo) (
	deviceInfo, error) {
	var info deviceInfo
	err := utils.WaitUntil(func() (bool, error) {
		rescanHosts(ctx, hbas, conn)
		for _, dev := range hostDevices {
			if exist, _ := utils.PathExist(dev); exist && checkValidDevice(ctx, dev) {
				info.hostDevice = dev
				if realPath, err := os.Readlink(dev); err == nil {
					info.realDeviceName = filepath.Base(realPath)
				}
				return true, nil
			}
		}

		if info.tries >= deviceScanAttemptsDefault {
			log.AddContext(ctx).Errorln("Fibre Channel volume device not found.")
			return false, errors.New(connector.VolumeNotFound)
		}

		info.tries += 1
		return false, nil
	}, time.Second*60, time.Second*2)
	return info, err
}

func getHBAChannelSCSITargetLun(ctx context.Context, hba map[string]string, targets []target) ([][]string, []string) {
	hostDevice := hba["host_device"]
	if hostDevice != "" && len(hostDevice) > 4 {
		hostDevice = hostDevice[4:]
	}

	path := fmt.Sprintf("/sys/class/fc_transport/target%s:", hostDevice)

	var channelTargetLun [][]string
	var lunNotFound []string
	for _, tar := range targets {
		cmd := fmt.Sprintf("grep -Gil \"%s\" %s*/port_name", tar.tgtWWN, path)
		output, err := utils.ExecShellCmd(ctx, cmd)
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

func rescanHosts(ctx context.Context, hbas []map[string]string, conn *connectorInfo) {
	var process []interface{}
	var skipped []interface{}
	for _, hba := range hbas {
		ctls, lunWildCards := getHBAChannelSCSITargetLun(ctx, hba, conn.tgtTargets)
		if ctls != nil {
			process = append(process, []interface{}{hba, ctls})
		} else if process == nil {
			var lunInfo [][]string
			for _, lun := range lunWildCards {
				lunInfo = append(lunInfo, []string{"-", "-", lun})
			}
			skipped = append(skipped, []interface{}{hba, lunInfo})
		}
	}

	if process == nil {
		process = skipped
	}

	var pathCount int
	defer func() {
		conn.pathCount = pathCount
	}()
	for _, p := range process {
		pro, ok := p.([]interface{})
		if !ok {
			log.AddContext(ctx).Errorf("the %v is not interface", p)
			return
		}

		if len(pro) != intNumTwo {
			log.AddContext(ctx).Errorf("the length of %s not equal 2", pro)
			return
		}

		hba := pro[0].(map[string]string)
		if !ok {
			log.AddContext(ctx).Errorf("the %v is not map[string]string", pro[0])
			return
		}

		ctls := pro[1].([][]string)
		if !ok {
			log.AddContext(ctx).Errorf("the %v is not [][]string", pro[1])
			return
		}

		for _, c := range ctls {
			scanFC(ctx, c, hba["host_device"])
			pathCount++
			if !conn.volumeUseMultiPath {
				break
			}
		}
		if !conn.volumeUseMultiPath {
			break
		}
	}
}

func scanFC(ctx context.Context, channelTargetLun []string, hostDevice string) {
	scanCommand := fmt.Sprintf("echo \"%s %s %s\" > /sys/class/scsi_host/%s/scan",
		channelTargetLun[0], channelTargetLun[1], channelTargetLun[2], hostDevice)
	output, err := utils.ExecShellCmd(ctx, scanCommand)
	if err != nil {
		log.AddContext(ctx).Warningf("rescan FC host error: %s", output)
	}
}

func tryDisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	return connector.DisConnectVolume(ctx, tgtLunWWN, tryToDisConnectVolume)
}

func tryToDisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	virtualDevice, devType, err := connector.GetVirtualDevice(ctx, tgtLunWWN)
	if err != nil {
		log.AddContext(ctx).Errorf("Get device of WWN [%s] failed, error: %v.", tgtLunWWN, err)
		return err
	}

	if virtualDevice == "" {
		log.AddContext(ctx).Infof("The device of WWN [%s] does not exist on host.", tgtLunWWN)
		return errors.New("FindNoDevice")
	}

	phyDevices, err := connector.GetPhysicalDevices(ctx, virtualDevice, devType)
	if err != nil {
		return err
	}

	multiPathName, err := connector.RemoveAllDevice(ctx, virtualDevice, phyDevices, devType)
	if err != nil {
		log.AddContext(ctx).Errorf("Remove physical devices [%s], virtual device [%v], error: %v",
			phyDevices, virtualDevice, err)
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
