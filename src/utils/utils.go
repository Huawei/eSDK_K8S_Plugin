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

// Package utils to provide utils for CSI
package utils

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"utils/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DoradoV6Version = "V600"
	V5Version       = "V500"
	defaultTimeout  = 30
	longTimeout     = 60

	OceanStorDoradoV6 = "DoradoV6"
	OceanStorDorado   = "Dorado"
	OceanStorV3       = "V3"
	OceanStorV5       = "V5"
)

var maskObject = []string{"user", "password", "iqn", "tgt", "tgtname", "initiatorname"}

type VolumeMetrics struct {
	Available  *resource.Quantity
	Capacity   *resource.Quantity
	InodesUsed *resource.Quantity
	Inodes     *resource.Quantity
	InodesFree *resource.Quantity
	Used       *resource.Quantity
}

func PathExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	return true, nil
}

func MaskSensitiveInfo(info interface{}) string {
	message := fmt.Sprintf("%s", info)
	substitute := "***"

	for _, value := range maskObject {
		if strings.Contains(strings.ToLower(message), strings.ToLower(value)) {
			rePattern := fmt.Sprintf(`(?is)%s.*\s-p`, value)
			re := regexp.MustCompile(rePattern)
			message = re.ReplaceAllString(message, substitute)
		}
	}

	return message
}

func execShellCmdTimeout(fun func(string, bool, ...interface{}) (string, bool, error), format string, logFilter bool, args ...interface{}) (string, error) {
	var output string
	var err error
	var timeOut bool
	done := make(chan string)
	timeCh := make(chan bool)
	defer func() {
		close(done)
		close(timeCh)
	}()

	go func() {
		output, timeOut, err = fun(format, logFilter, args...)
		if !timeOut {
			done <- "run command done"
		} else {
			timeCh <- timeOut
		}
		if err != nil {
			log.Infoln("Run shell cmd failed.")
		}
	}()

	select {
	case <-done:
		return output, err
	case <-timeCh:
		return "", errors.New("timeout")
	}
}

// ExecShellCmd execs the command without filters the result log
func ExecShellCmd(format string, args ...interface{}) (string, error) {
	return execShellCmdTimeout(execShellCmd, format, false, args...)
}

// ExecShellCmdFilterLog execs the command and filters the result log
func ExecShellCmdFilterLog(format string, args ...interface{}) (string, error) {
	return execShellCmdTimeout(execShellCmd, format, true, args...)
}

func execShellCmd(format string, logFilter bool, args ...interface{}) (string, bool, error) {
	cmd := fmt.Sprintf(format, args...)
	log.Infof("Gonna run shell cmd \"%s\".", MaskSensitiveInfo(cmd))

	execCmd := []string{"-i/proc/1/ns/ipc", "-m/proc/1/ns/mnt", "-n/proc/1/ns/net", "/bin/sh", "-c", cmd}
	shCmd := exec.Command("nsenter", execCmd...)
	var timeOut bool
	if strings.Contains(cmd, "mkfs") || strings.Contains(cmd, "resize2fs") {
		time.AfterFunc(longTimeout*time.Second, func() {
			timeOut = true
		})
	} else {
		time.AfterFunc(defaultTimeout*time.Second, func() {
			_ = shCmd.Process.Kill()
			timeOut = true
		})
	}
	output, err := shCmd.CombinedOutput()
	if err != nil {
		log.Warningf("Run shell cmd \"%s\" output: %s, error: %v", MaskSensitiveInfo(cmd), MaskSensitiveInfo(output),
			MaskSensitiveInfo(err))
		return string(output), timeOut, err
	}

	if !logFilter {
		log.Infof("Shell cmd \"%s\" result:\n%s", MaskSensitiveInfo(cmd), MaskSensitiveInfo(output))
	}
	return string(output), timeOut, nil
}

func GetLunName(name string) string {
	if len(name) <= 31 {
		return name
	}

	return name[:31]
}

func GetSnapshotName(name string) string {
	if len(name) <= 31 {
		return name
	}

	return name[:31]
}

func GetFusionStorageLunName(name string) string {
	if len(name) <= 95 {
		return name
	}
	return name[:95]
}

func GetFusionStorageSnapshotName(name string) string {
	if len(name) <= 95 {
		return name
	}
	return name[:95]
}

func GetFileSystemName(name string) string {
	return strings.Replace(name, "-", "_", -1)
}

func GetFSSnapshotName(name string) string {
	return strings.Replace(name, "-", "_", -1)
}

func GetSharePath(name string) string {
	return "/" + strings.Replace(name, "-", "_", -1) + "/"
}

func GetFSSharePath(name string) string {
	return "/" + strings.Replace(name, "-", "_", -1) + "/"
}

func GetHostName() (string, error) {
	hostname, err := ExecShellCmd("hostname | xargs echo -n")
	if err != nil {
		return "", err
	}

	return hostname, nil
}

func GetPathTail(device string) string {
	strs := strings.Split(device, "/")
	if len(strs) > 0 {
		return strs[len(strs)-1]
	}
	return ""
}

func GetBackendAndVolume(volumeId string) (string, string) {
	var backend, volume string

	splits := strings.SplitN(volumeId, "-", 2)
	if len(splits) == 2 {
		backend, volume = splits[0], splits[1]
	} else {
		backend, volume = "", splits[0]
	}

	log.Infof("Backend %s, volume %s", backend, volume)
	return backend, volume
}

func SplitVolumeId(volumeId string) (string, string) {
	splits := strings.SplitN(volumeId, ".", 2)
	if len(splits) == 2 {
		return splits[0], splits[1]
	}
	return splits[0], ""
}

func SplitSnapshotId(snapshotId string) (string, string, string) {
	splits := strings.SplitN(snapshotId, ".", 3)
	if len(splits) == 3 {
		return splits[0], splits[1], splits[2]
	}
	return splits[0], "", ""
}

func MergeMap(args ...map[string]interface{}) map[string]interface{} {
	newMap := make(map[string]interface{})
	for _, arg := range args {
		for k, v := range arg {
			newMap[k] = v
		}
	}

	return newMap
}

func WaitUntil(f func() (bool, error), timeout time.Duration, interval time.Duration) error {
	done := make(chan error)
	defer close(done)

	go func() {
		timeout := time.After(timeout)

		for {
			condition, err := f()
			if err != nil {
				done <- err
				return
			}

			if condition {
				done <- nil
				return
			}

			select {
			case <-timeout:
				done <- fmt.Errorf("Wait timeout")
				return
			default:
				time.Sleep(interval)
			}
		}
	}()

	select {
	case err := <-done:
		return err
	}
}

func RandomInt(n int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(n)
}

func CopyMap(srcMap interface{}) map[string]interface{} {
	copied := make(map[string]interface{})

	if m, ok := srcMap.(map[string]string); ok {
		for k, v := range m {
			copied[k] = v
		}
	} else if m, ok := srcMap.(map[string]interface{}); ok {
		for k, v := range m {
			copied[k] = v
		}
	}

	return copied
}

func StrToBool(str string) bool {
	b, err := strconv.ParseBool(str)
	if err != nil {
		log.Warningf("Parse bool string %s error, return false")
		return false
	}

	return b
}

func ReflectCall(obj interface{}, method string, args ...interface{}) []reflect.Value {
	in := make([]reflect.Value, len(args))
	for i, v := range args {
		in[i] = reflect.ValueOf(v)
	}

	if v := reflect.ValueOf(obj).MethodByName(method); v.IsValid() {
		return v.Call(in)
	}

	return nil
}

// GetProductVersion is to get the oceanStorage version by get info from the system
func GetProductVersion(SystemInfo map[string]interface{}) (string, error) {
	productVersion, ok := SystemInfo["PRODUCTVERSION"].(string)
	if !ok {
		return "", errors.New("there is no PRODUCTVERSION field in system info")
	}

	if strings.HasPrefix(productVersion, DoradoV6Version) {
		return OceanStorDoradoV6, nil
	} else if strings.HasPrefix(productVersion, V5Version) {
		return OceanStorV5, nil
	}

	productMode, ok := SystemInfo["PRODUCTMODE"].(string)
	if !ok {
		log.Warningln("There is no PRODUCTMODE field in system info")
	}

	if match, _ := regexp.MatchString(`8[0-9][0-9]`, productMode); match {
		return OceanStorDorado, nil
	}

	return OceanStorV3, nil
}

func IsSupportFeature(features map[string]int, feature string) bool {
	var support bool

	status, exist := features[feature]
	if exist {
		support = status == 1 || status == 2
	}

	return support
}

func TransVolumeCapacity(size int64, unit int64) int64 {
	newSize := RoundUpSize(size, unit)
	return newSize
}

func RoundUpSize(volumeSizeBytes int64, allocationUnitBytes int64) int64 {
	roundedUp := volumeSizeBytes / allocationUnitBytes
	if volumeSizeBytes%allocationUnitBytes > 0 {
		roundedUp++
	}
	return roundedUp
}

func GetAlua(alua map[string]interface{}, host string) map[string]interface{} {
	if alua == nil {
		return nil
	}

	for k, v := range alua {
		if k == "*" {
			continue
		}

		match, err := regexp.MatchString(k, host)
		if err != nil {
			log.Errorf("Regexp match error: %v", err)
		} else if match {
			return v.(map[string]interface{})
		}
	}

	return alua["*"].(map[string]interface{})
}

func fsInfo(path string) (int64, int64, int64, int64, int64, int64, error) {
	statfs := &unix.Statfs_t{}
	err := unix.Statfs(path, statfs)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	capacity := int64(statfs.Blocks) * int64(statfs.Bsize)
	available := int64(statfs.Bavail) * int64(statfs.Bsize)
	used := (int64(statfs.Blocks) - int64(statfs.Bfree)) * int64(statfs.Bsize)

	inodes := int64(statfs.Files)
	inodesFree := int64(statfs.Ffree)
	inodesUsed := inodes - inodesFree
	return inodes, inodesFree, inodesUsed, available, capacity, used, nil
}

func GetVolumeMetrics(path string) (*VolumeMetrics, error) {
	volumeMetrics := &VolumeMetrics{}

	inodes, inodesFree, inodesUsed, available, capacity, usage, err := fsInfo(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get FsInfo, error %v", err)
	}
	volumeMetrics.Inodes = resource.NewQuantity(inodes, resource.BinarySI)
	volumeMetrics.InodesFree = resource.NewQuantity(inodesFree, resource.BinarySI)
	volumeMetrics.InodesUsed = resource.NewQuantity(inodesUsed, resource.BinarySI)
	volumeMetrics.Available = resource.NewQuantity(available, resource.BinarySI)
	volumeMetrics.Capacity = resource.NewQuantity(capacity, resource.BinarySI)
	volumeMetrics.Used = resource.NewQuantity(usage, resource.BinarySI)

	return volumeMetrics, nil
}

func GetLunUniqueId(protocol string, lun map[string]interface{}) (string, error) {
	if protocol == "roce" || protocol == "fc-nvme" {
		tgtLunGuid, exist := lun["NGUID"].(string)
		if !exist {
			msg := fmt.Sprintf("The Lun info %s does not contain key NGUID", lun)
			log.Errorln(msg)
			return "", errors.New(msg)
		}
		return tgtLunGuid, nil
	} else {
		return lun["WWN"].(string), nil
	}
}

func GetAccessModeType(accessMode csi.VolumeCapability_AccessMode_Mode) string {
	switch accessMode {
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
		return "ReadWrite"
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY:
		return "ReadOnly"
	case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
		return "ReadOnly"
	case csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER:
		return "ReadWrite"
	case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER:
		return "ReadWrite"
	default:
		return ""
	}
}

// CheckExistCode if the error code exist in ExitCode, return err
func CheckExistCode(err error, checkExitCode []string) error {
	for _, v := range checkExitCode {
		if err.Error() == v {
			return err
		}
	}

	return nil
}

// IgnoreExistCode if the error code exist in ExitCode, return nil
func IgnoreExistCode(err error, checkExitCode []string) error {
	for _, v := range checkExitCode {
		if err.Error() == v {
			return nil
		}
	}

	return err
}

func TestMultiPathService() error {
	output, err := ExecShellCmd("systemctl status multipathd.service")
	if err != nil {
		return err
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Active") {
			activeLines := strings.Split(strings.TrimSpace(line), ":")
			if len(activeLines) < 2 {
				continue
			}
			if activeLines[0] == "Active" && strings.Contains(activeLines[1], "active (running)") {
				return nil
			}
		}
	}

	return errors.New("multipathd service not running")
}

// NeedMultiPath to check whether the multipathing service is required based on the storage configuration.
func NeedMultiPath(backendConfigs []map[string]interface{}) bool {
	var needMultiPath bool
	for _, config := range backendConfigs {
		parameters, exist := config["parameters"].(map[string]interface{})
		if !exist {
			log.Errorf("parameters must be configured in backend %v", config)
			continue
		}

		protocol, exist := parameters["protocol"].(string)
		if !exist {
			log.Errorf("protocol must be configured in parameters %v", config)
			continue
		}

		if strings.ToLower(protocol) == "iscsi" || strings.ToLower(protocol) == "fc" ||
			strings.ToLower(protocol) == "roce" {
			needMultiPath = true
			break
		}
	}

	return needMultiPath
}

// IsCapacityAvailable indicates whether the volume size is an integer multiple of 512.
func IsCapacityAvailable(volumeSizeBytes int64, allocationUnitBytes int64) bool {
	return volumeSizeBytes%allocationUnitBytes == 0
}

// TransToInt is to trans different type to int type.
func TransToInt(v interface{}) (int, error) {
	switch v.(type) {
	case string:
		return strconv.Atoi(v.(string))
	case int:
		return v.(int), nil
	case float64:
		return int(v.(float64)), nil
	default:
		return 0, errors.New("unSupport type")
	}
}

// TransToIntStrict only trans int type.
func TransToIntStrict(val interface{}) (int, error) {
	floatVal, ok := val.(float64)
	if !ok {
		log.Errorf("Value type invalid: {%v}, expect integer variable.", val)
		return 0, errors.New("value type invalid")
	}

	if floatVal-math.Trunc(floatVal) != 0 {
		log.Errorf("Value type invalid: {%v}, expect integer variable.", floatVal)
		return 0, errors.New("value type invalid")
	}

	return int(floatVal), nil
}
