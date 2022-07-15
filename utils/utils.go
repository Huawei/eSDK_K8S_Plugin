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
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"huawei-csi-driver/utils/log"

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

var (
	createSymLock sync.Mutex
	removeSymLock sync.Mutex
)

const (
	iSCSIProtocol  string = "iscsi"
	scsiProtocol   string = "scsi"
	fcProtocol     string = "fc"
	roceProtocol   string = "roce"
	fcNVMeProtocol string = "fc-nvme"
	nfsProtocol    string = "nfs"

	dmMultipathService string = "multipathd.service"
	nxupService        string = "nxup.service"
	upudevService      string = "upudev.service"
	upPlusService      string = "upService_plus.service"

	dmMultiPath     string = "DM-multipath"
	hwUltraPath     string = "HW-UltraPath"
	hwUltraPathNVMe string = "HW-UltraPath-NVMe"

	oceantorSan      string = "oceanstor-san"
	oceantorNas      string = "oceanstor-nas"
	fusionstorageSan string = "fusionstorage-san"
	fusionstorageNas string = "fusionstorage-nas"
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

var PathExist = func(path string) (bool, error) {
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

func execShellCmdTimeout(ctx context.Context,
	fun func(context.Context, string, bool, ...interface{}) (string, bool, error),
	format string,
	logFilter bool,
	args ...interface{}) (string, error) {
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
		output, timeOut, err = fun(ctx, format, logFilter, args...)
		if !timeOut {
			done <- "run command done"
		} else {
			timeCh <- timeOut
		}
		if err != nil {
			log.AddContext(ctx).Infoln("Run shell cmd failed.")
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
var ExecShellCmd = func(ctx context.Context, format string, args ...interface{}) (string, error) {
	return execShellCmdTimeout(ctx, execShellCmd, format, false, args...)
}

// ExecShellCmdFilterLog execs the command and filters the result log
func ExecShellCmdFilterLog(ctx context.Context, format string, args ...interface{}) (string, error) {
	return execShellCmdTimeout(ctx, execShellCmd, format, true, args...)
}

func execShellCmd(ctx context.Context, format string, logFilter bool, args ...interface{}) (string, bool, error) {
	cmd := fmt.Sprintf(format, args...)
	log.AddContext(ctx).Infof("Gonna run shell cmd \"%s\".", MaskSensitiveInfo(cmd))

	execCmd := []string{"-i/proc/1/ns/ipc", "-m/proc/1/ns/mnt", "-n/proc/1/ns/net", "-u/proc/1/ns/uts", "/bin/sh",
		"-c", cmd}
	shCmd := exec.Command("nsenter", execCmd...)
	var timeOut bool
	if strings.Contains(cmd, "mkfs") || strings.Contains(cmd, "resize2fs") ||
		strings.Contains(cmd, "xfs_growfs") {
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
		log.AddContext(ctx).Warningf("Run shell cmd \"%s\" output: %s, error: %v", MaskSensitiveInfo(cmd),
			MaskSensitiveInfo(output),
			MaskSensitiveInfo(err))
		return string(output), timeOut, err
	}

	if !logFilter {
		log.AddContext(ctx).Infof("Shell cmd \"%s\" result:\n%s", MaskSensitiveInfo(cmd),
			MaskSensitiveInfo(output))
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

func GetHostName(ctx context.Context) (string, error) {
	hostname, err := ExecShellCmd(ctx, "hostname | xargs echo -n")
	if err != nil {
		return "", err
	}

	return hostname, nil
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

func StrToBool(ctx context.Context, str string) bool {
	b, err := strconv.ParseBool(str)
	if err != nil {
		log.AddContext(ctx).Warningf("Parse bool string %s error, return false")
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
func GetProductVersion(systemInfo map[string]interface{}) (string, error) {
	productVersion, ok := systemInfo["PRODUCTVERSION"].(string)
	if !ok {
		return "", errors.New("there is no PRODUCTVERSION field in system info")
	}

	if strings.HasPrefix(productVersion, DoradoV6Version) {
		return OceanStorDoradoV6, nil
	} else if strings.HasPrefix(productVersion, V5Version) {
		return OceanStorV5, nil
	}

	productMode, ok := systemInfo["PRODUCTMODE"].(string)
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

func GetAlua(ctx context.Context, alua map[string]interface{}, host string) map[string]interface{} {
	if alua == nil {
		return nil
	}

	for k, v := range alua {
		if k == "*" {
			continue
		}

		match, err := regexp.MatchString(k, host)
		if err != nil {
			log.AddContext(ctx).Errorf("Regexp match error: %v", err)
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

func GetLunUniqueId(ctx context.Context, protocol string, lun map[string]interface{}) (string, error) {
	if protocol == "roce" || protocol == "fc-nvme" {
		tgtLunGuid, exist := lun["NGUID"].(string)
		if !exist {
			msg := fmt.Sprintf("The Lun info %s does not contain key NGUID", lun)
			log.AddContext(ctx).Errorln(msg)
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
func TransToIntStrict(ctx context.Context, val interface{}) (int, error) {
	floatVal, ok := val.(float64)
	if !ok {
		return 0, Errorf(ctx, "Value type invalid: {%v}, expect integer variable.", val)
	}

	if floatVal-math.Trunc(floatVal) != 0 {
		return 0, Errorf(ctx, "Value type invalid: {%v}, expect integer variable.", floatVal)
	}

	return int(floatVal), nil
}

func getBackendProtocols(ctx context.Context, backendConfigs []map[string]interface{}) []string {
	var protocols []string
	for _, config := range backendConfigs {
		parameters, exist := config["parameters"].(map[string]interface{})
		if !exist {
			log.AddContext(ctx).Errorf("parameters must be configured in backend %v", config)
			continue
		}

		protocol, exist := parameters["protocol"].(string)
		if !exist {
			log.AddContext(ctx).Errorf("protocol must be configured in parameters %v", parameters)
			continue
		}

		if !IsContain(protocol, protocols) {
			protocols = append(protocols, protocol)
		}
	}
	return protocols
}

func getBackendStorages(ctx context.Context, backendConfigs []map[string]interface{}) []string {
	var storages []string
	for _, config := range backendConfigs {
		storage, exist := config["storage"].(string)
		if !exist {
			log.AddContext(ctx).Errorf("storage must be configured in backend config %v", config)
			continue
		}

		storages = append(storages, storage)
	}
	return storages
}

// IsContain used to determine whether stringList contains target.
func IsContain(target string, stringList []string) bool {
	for _, val := range stringList {
		if val == target {
			return true
		}
	}
	return false
}

// GetForbiddenMultipath used to get forbidden multipath service by configuration and backend info.
func GetForbiddenMultipath(ctx context.Context,
	multipathConfig map[string]interface{},
	backendConfigs []map[string]interface{}) []string {
	var forbiddenServices []string

	// If volumeUseMultiPath=true, there is no need to forbid any multipath service.
	if multipathConfig["volumeUseMultiPath"].(bool) {
		return forbiddenServices
	}

	// If there is no SAN storage, there is need to forbid any multipath service.
	storages := getBackendStorages(ctx, backendConfigs)
	if IsContain(oceantorSan, storages) || IsContain(fusionstorageSan, storages) {
		forbiddenServices = []string{dmMultipathService, nxupService, upudevService, upPlusService}
	}

	log.AddContext(ctx).Infof("Forbidden multipath service:%v", forbiddenServices)
	return forbiddenServices
}

// GetRequiredMultipath used to get required multipath service by configuration and backend info.
func GetRequiredMultipath(ctx context.Context,
	multipathConfig map[string]interface{},
	backendConfigs []map[string]interface{}) ([]string, error) {
	serviceMap := map[string][]string{dmMultiPath: {dmMultipathService}, hwUltraPath: {nxupService},
		hwUltraPathNVMe: {upudevService, upPlusService}}
	var requiredServices []string

	if !multipathConfig["volumeUseMultiPath"].(bool) {
		log.AddContext(ctx).Infoln("No multipath service is required.")
		return requiredServices, nil
	}

	scsiMultipathType, exist := multipathConfig["SCSIMultipathType"].(string)
	if !exist {
		scsiMultipathType = dmMultiPath
	}
	nvmeMultipathType, exist := multipathConfig["NVMeMultipathType"].(string)
	if !exist {
		nvmeMultipathType = hwUltraPathNVMe
	}

	// Ultrapath* not support fusionstorage-san
	storages := getBackendStorages(ctx, backendConfigs)
	if (!IsContain(oceantorSan, storages) && IsContain(fusionstorageSan, storages)) &&
		scsiMultipathType != dmMultiPath {
		return nil, Errorf(ctx, "Configured storages are %v, should not configure multipath type %s",
			storages, scsiMultipathType)
	}

	protocols := getBackendProtocols(ctx, backendConfigs)
	for _, protocol := range protocols {
		var relatedServices []string
		if protocol == iSCSIProtocol || protocol == fcProtocol {
			relatedServices, exist = serviceMap[scsiMultipathType]
			if !exist {
				log.AddContext(ctx).Errorf("scsi-multipath-type: %s is incorrectly configured.", scsiMultipathType)
			}
		} else if protocol == roceProtocol || protocol == fcNVMeProtocol {
			relatedServices, exist = serviceMap[nvmeMultipathType]
			if !exist {
				log.AddContext(ctx).Errorf("nvme-multipath-type: %s is incorrectly configured.", nvmeMultipathType)
			}
		}

		for _, service := range relatedServices {
			if !IsContain(service, requiredServices) {
				requiredServices = append(requiredServices, service)
			}
		}
	}

	log.AddContext(ctx).Infof("Required multipath service:%v", requiredServices)
	return requiredServices, nil
}

// Errorln used to create and print error messages.
func Errorln(ctx context.Context, msg string) error {
	log.AddContext(ctx).Errorln(msg)
	return errors.New(msg)
}

// Errorf used to create and print formatted error messages.
func Errorf(ctx context.Context, format string, a ...interface{}) error {
	msg := fmt.Sprintf(format, a...)
	log.AddContext(ctx).Errorln(msg)
	return errors.New(msg)
}

// GetValueByRegexp used to get value by regular expression
func GetValueByRegexp(sourceString string, patternString string, valueIndex int) string {
	for _, line := range strings.Split(sourceString, "\n") {
		pattern := regexp.MustCompile(patternString)
		ret := pattern.FindAllStringSubmatch(line, -1)
		if ret != nil {
			return ret[0][valueIndex]
		}
	}

	return ""
}

// IsPathSymlink checks weather this targetPath is symlink
func IsPathSymlink(targetPath string) (symlink bool, err error) {
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return (info.Mode()&os.ModeSymlink == os.ModeSymlink), nil
}

// CreateSymlink between source and target
func CreateSymlink(ctx context.Context, source string, target string) error {
	// First check if File exists in the staging area, then remove the mount
	// and then create a symlink to the devpath Serialize symlink request

	createSymLock.Lock()
	defer createSymLock.Unlock()

	log.AddContext(ctx).Infof("Create symlink called for [%s] to [%s]", source, target)

	finfo, err := os.Lstat(target)
	if err != nil && os.IsNotExist(err) {
		log.AddContext(ctx).Infof("Mountpoint [%v] does not exist", target)
	} else {
		if finfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			log.AddContext(ctx).Infof("Path symlink already exists")
			return nil
		}
		// As the file exists but no symlink created,
		// then throw error and it should be handled by user
		log.AddContext(ctx).Errorf("Already target path exists [%v].", target)
		return err
	}
	log.AddContext(ctx).Infof("Creating symlink for [%s] to [%s]", source, target)
	err = os.Symlink(source, target)
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to create a link for [%v] to [%v]", source, target)
		return err
	}
	return nil
}

// RemoveSymlink from target path
func RemoveSymlink(ctx context.Context, target string) error {
	log.AddContext(ctx).Infof("Remove symlink called for [%s]", target)

	removeSymLock.Lock()
	defer removeSymLock.Unlock()

	// If the file is symlink delete it
	symLink, err := IsPathSymlink(target)
	if symLink {
		clierr := os.Remove(target)
		if clierr != nil {
			log.AddContext(ctx).Errorf("Failed to delete the target [%v]", target)
			return clierr
		}
		log.AddContext(ctx).Infof("Successfully deleted the target [%v]", target)
		return nil
	}
	return err
}

// RemoveDir delete directory from filePath
func RemoveDir(filePath, dir string) {
	if _, err := os.Lstat(filePath); err != nil && os.IsNotExist(err) {
		log.Errorf("File: %s not exist! Error: %v", filePath, err)
	}

	if err := os.RemoveAll(dir); err != nil {
		log.Errorf("Directory: %s delete failed: %s", dir, err)
	}
}

// RecoverPanic used to recover panic
func RecoverPanic(ctx context.Context) {
	if r := recover(); r != nil {
		log.AddContext(ctx).Errorf("Panic message: [%s]\nPanic stack: [%s]", r, debug.Stack())
	}
}
