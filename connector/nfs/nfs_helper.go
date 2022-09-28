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

// Package nfs to mount or unmount filesystem
package nfs

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type connectorInfo struct {
	srcType    string
	sourcePath string
	targetPath string
	fsType     string
	mntFlags   mountParam
	accessMode csi.VolumeCapability_AccessMode_Mode
}

type mountParam struct {
	dashT string
	dashO string
}

func parseNFSInfo(ctx context.Context,
	connectionProperties map[string]interface{}) (*connectorInfo, error) {
	var con connectorInfo
	srcType, typeExist := connectionProperties["srcType"].(string)
	if !typeExist || srcType == "" {
		msg := "there are no srcType in the connection info"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	sourcePath, srcPathExist := connectionProperties["sourcePath"].(string)
	if !srcPathExist || sourcePath == "" {
		msg := "there are no source path in the connection info"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	targetPath, tgtPathExist := connectionProperties["targetPath"].(string)
	if !tgtPathExist || targetPath == "" {
		msg := "there are no target path in the connection info"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	fsType, _ := connectionProperties["fsType"].(string)
	if fsType == "" {
		fsType = "ext4"
	}

	accessMode, _ := connectionProperties["accessMode"].(csi.VolumeCapability_AccessMode_Mode)
	mntDashO, _ := connectionProperties["mountFlags"].(string)
	protocol, _ := connectionProperties["protocol"].(string)
	var mntDashT string
	if protocol == "dpc" {
		mntDashT = "dpc"
	}

	con.srcType = srcType
	con.sourcePath = sourcePath
	con.targetPath = targetPath
	con.fsType = fsType
	con.accessMode = accessMode
	con.mntFlags = mountParam{dashO: strings.TrimSpace(mntDashO), dashT: mntDashT}

	return &con, nil
}

func tryConnectVolume(ctx context.Context, connMap map[string]interface{}) (string, error) {
	conn, err := parseNFSInfo(ctx, connMap)
	if err != nil {
		return "", err
	}

	switch conn.srcType {
	case "block":
		_, err = connector.ReadDevice(ctx, conn.sourcePath)
		if err != nil {
			return "", err
		}

		err = mountDisk(ctx, conn.sourcePath, conn.targetPath, conn.fsType, conn.mntFlags, conn.accessMode)
		if err != nil {
			return "", err
		}
	case "fs":
		err = mountFS(ctx, conn.sourcePath, conn.targetPath, conn.mntFlags)
		if err != nil {
			return "", err
		}
	default:
		return "", errors.New("not support source type")
	}
	return "", nil
}

func preMount(sourcePath, targetPath string, checkSourcePath bool) error {
	if checkSourcePath {
		if _, err := os.Stat(sourcePath); err != nil && os.IsNotExist(err) {
			return errors.New("source path does not exist")
		}
	}

	if _, err := os.Stat(targetPath); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(targetPath, 0750); err != nil {
			return errors.New("can not create a target path")
		}
	}

	return nil
}

func mountFS(ctx context.Context, sourcePath, targetPath string, flags mountParam) error {
	return mountUnix(ctx, sourcePath, targetPath, flags, false)
}

var readFile = ioutil.ReadFile

func readMountPoints(ctx context.Context) (map[string]string, error) {
	data, err := readFile("/proc/mounts")
	if err != nil {
		log.AddContext(ctx).Errorf("Read the mount file error: %v", err)
		return nil, err
	}

	mountMap := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			splitValue := strings.Split(line, " ")
			if len(splitValue) >= 2 && splitValue[0] != "#" {
				mountMap[splitValue[1]] = splitValue[0]
			}
		}
	}
	return mountMap, nil
}

func compareMountPath(ctx context.Context, sourcePath, mountSourcePath string) error {
	// the mount source path is like: /dev/mapper/mpath<x> or /dev/sd<x>
	// but the source path is like: /dev/dm-<n> or /dev/sd<n>.
	// The relationship of these two path as follows:
	//     lrwxrwxrwx 1 root root 7 Aug 1 17:13 /dev/mapper/mpathc -> ../dm-3
	_, err := connector.ReadDevice(ctx, mountSourcePath)
	if err != nil {
		return err
	}

	mountRealPath, err := filepath.EvalSymlinks(mountSourcePath)
	if err != nil {
		return err
	}

	sourceRealPath, err := filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return err
	}

	if sourceRealPath != mountRealPath {
		msg := fmt.Sprintf("The source path is %s, the real path is %s",
			sourcePath, mountRealPath)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

func appendXFSMountFlags(ctx context.Context, sourcePath string, flags mountParam) mountParam {
	// Only disk devices need to be determined whether the system is XFS.
	if !strings.Contains(sourcePath, "/dev/") {
		return flags
	}
	fsType, err := connector.GetFsTypeByDevPath(ctx, sourcePath)
	if err != nil {
		log.AddContext(ctx).Warningf("Get device: [%s] FsType failed. error: %v", sourcePath, err)
		return flags
	}
	if fsType == "xfs" {
		// xfs volumes are always mounted with '-o nouuid' to allow clones to be mounted to the same node as the source
		if flags.dashO == "" {
			flags.dashO = "nouuid"
		} else {
			flags.dashO = fmt.Sprintf("%s,nouuid", flags.dashO)
		}
	}

	log.AddContext(ctx).Infof("mount flags: [%v]", flags)
	return flags
}

func mountUnix(ctx context.Context, sourcePath, targetPath string, flags mountParam, checkSourcePath bool) error {
	var output string
	var err error
	err = preMount(sourcePath, targetPath, checkSourcePath)
	if err != nil {
		return err
	}

	mountMap, err := readMountPoints(ctx)
	value, exist := mountMap[targetPath]
	if exist {
		// checkSourcePath means the source is block type, need to check the realpath
		if checkSourcePath {
			err := compareMountPath(ctx, sourcePath, value)
			if err != nil {
				return err
			}
			log.AddContext(ctx).Infof("%s is already mount to %s", sourcePath, targetPath)
			return nil
		}

		// if the checkSourcePath is false, check the filesystem by comparing the sourcePath and mountPath
		if value == sourcePath || path.Base(path.Dir(targetPath)) == path.Base(path.Dir(sourcePath)) {
			log.AddContext(ctx).Infof("Mount %s to %s is already exist", sourcePath, targetPath)
			return nil
		}

		return utils.Errorf(ctx, "The mount %s is already exist, but the source path is not %s, instead of %s",
			targetPath, sourcePath, value)
	}

	flags = appendXFSMountFlags(ctx, sourcePath, flags)

	if flags.dashT != "" {
		flags.dashT = fmt.Sprintf("-t %s", flags.dashT)
	}

	if flags.dashO != "" {
		flags.dashO = fmt.Sprintf("-o %s", flags.dashO)
	}

	output, err = utils.ExecShellCmd(ctx, "mount %s %s %s %s", sourcePath, targetPath, flags.dashT, flags.dashO)
	if err != nil {
		log.AddContext(ctx).Errorf("Mount %s to %s error: %s", sourcePath, targetPath, output)
		return err
	}

	return nil
}

func getFSType(ctx context.Context, sourcePath string) (string, error) {
	// the errorCode 2 means an unFormatted filesystem and the unavailable filesystem. So ensure the device is
	// available before calling command blkid
	if exist, err := utils.PathExist(sourcePath); !exist {
		return "", fmt.Errorf("find the device %s failed before get filesystem info, error: %v", sourcePath, err)
	}

	output, err := utils.ExecShellCmd(ctx, "blkid -o udev %s", sourcePath)
	if err != nil {
		if errCode, ok := err.(*exec.ExitError); ok && errCode.ExitCode() == 2 {
			log.AddContext(ctx).Infof("Query fs of %s, output: %s, error: %s", sourcePath, output, err)
			if formatted, err := connector.IsDeviceFormatted(ctx, sourcePath); err != nil {
				return "", fmt.Errorf("check device %s formatted failed, error: %v", sourcePath, err)
			} else if formatted {
				return "", fmt.Errorf("1. Maybe the device %s is formatted; 2. Maybe the device is a "+
					"raw block volume, please check. error: %v", sourcePath, err)
			}

			return "", nil
		}
		return "", err
	}

	for _, out := range strings.Split(output, "\n") {
		fsInfo := strings.Split(out, "=")
		if len(fsInfo) == 2 && fsInfo[0] == "ID_FS_TYPE" {
			return fsInfo[1], nil
		}
	}

	return "", errors.New("get fsType failed")
}

func formatDisk(ctx context.Context, sourcePath, fsType, diskSizeType string) error {
	var cmd string
	if "xfs" == fsType {
		cmd = fmt.Sprintf("mkfs -t %s -f %s", fsType, sourcePath)
	} else {
		// Handle ext types
		switch diskSizeType {
		case "default":
			cmd = fmt.Sprintf("mkfs -t %s -F %s", fsType, sourcePath)
		case "big":
			cmd = fmt.Sprintf("mkfs -t %s -T big -F %s", fsType, sourcePath)
		case "huge":
			cmd = fmt.Sprintf("mkfs -t %s -T huge -F %s", fsType, sourcePath)
		case "large":
			cmd = fmt.Sprintf("mkfs -t %s -T largefile -F %s", fsType, sourcePath)
		case "veryLarge":
			cmd = fmt.Sprintf("mkfs -t %s -T largefile4 -F %s", fsType, sourcePath)
		}
	}
	output, err := utils.ExecShellCmd(ctx, cmd)
	if err != nil {
		if strings.Contains(output, "in use by the system") {
			log.AddContext(ctx).Infof("The disk %s is in formatting, wait for 10 second", sourcePath)
			time.Sleep(time.Second * formatWaitInternal)
			return errors.New("the disk is in formatting, please wait")
		}
		log.AddContext(ctx).Errorf("Couldn't mkfs %s to %s: %s", sourcePath, fsType, output)
		return err
	}
	return nil
}

func getDiskSizeType(ctx context.Context, sourcePath string) (string, error) {
	size, err := connector.GetDeviceSize(ctx, sourcePath)
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to get size from %s, error is %s", sourcePath, err)
		return "", err
	}

	log.AddContext(ctx).Infof("Get disk %s's size: %d", sourcePath, size)
	if size <= halfTiSizeBytes {
		return "default", nil
	} else if size > halfTiSizeBytes && size <= oneTiSizeBytes {
		return "big", nil
	} else if size > oneTiSizeBytes && size <= tenTiSizeBytes {
		return "huge", nil
	} else if size > tenTiSizeBytes && size <= hundredTiSizeBytes {
		return "large", nil
	} else if size > hundredTiSizeBytes && size <= halfPiSizeBytes {
		return "veryLarge", nil
	}

	// if the size bigger than 512TiB, mark it is a large disk, more info: /etc/mke2fs.conf
	return "", errors.New("the disk size does not support")
}

func mountDisk(ctx context.Context, sourcePath, targetPath, fsType string, flags mountParam,
	accessMode csi.VolumeCapability_AccessMode_Mode) error {
	var err error
	existFsType, err := getFSType(ctx, sourcePath)
	if err != nil {
		return err
	}

	if existFsType == "" {
		// check this disk is in formatting
		inFormatting, err := connector.IsInFormatting(ctx, sourcePath, fsType)
		if err != nil {
			return err
		}

		if inFormatting {
			log.AddContext(ctx).Infof("Device %s is in formatting, no need format again. Wait 10 seconds", sourcePath)
			time.Sleep(time.Second * formatWaitInternal)
			return errors.New("the disk is in formatting, please wait")
		}

		diskSizeType, err := getDiskSizeType(ctx, sourcePath)
		if err != nil {
			return err
		}

		err = formatDisk(ctx, sourcePath, fsType, diskSizeType)
		if err != nil {
			return err
		}

		err = mountUnix(ctx, sourcePath, targetPath, flags, true)
		if err != nil {
			return err
		}
	} else {
		err = mountUnix(ctx, sourcePath, targetPath, flags, true)
		if err != nil {
			return err
		}

		if accessMode == csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER {
			log.AddContext(ctx).Infoln("PVC accessMode is ReadWriteMany, not support to expend filesystem")
			return nil
		}

		if accessMode == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
			log.AddContext(ctx).Infoln("PVC accessMode is ReadOnlyMany, no need to expend filesystem")
			return nil
		}

		err = connector.ResizeMountPath(ctx, targetPath)
		if err != nil {
			log.AddContext(ctx).Errorf("Resize mount path %s err %s", targetPath, err)
			return err
		}
	}
	return nil
}

func unmountUnix(ctx context.Context, targetPath string) error {
	_, err := os.Stat(targetPath)
	if err != nil && os.IsNotExist(err) {
		return nil
	}

	output, err := utils.ExecShellCmd(ctx, "umount %s", targetPath)
	if err != nil && !(strings.Contains(output, "not mounted") ||
		strings.Contains(output, "not found")) {
		log.AddContext(ctx).Errorf("Unmount %s error: %s", targetPath, output)
		return err
	}

	return nil
}

func removeTargetPath(targetPath string) error {
	_, err := os.Stat(targetPath)
	if err != nil && os.IsNotExist(err) {
		return nil
	}

	if err != nil && !os.IsNotExist(err) {
		msg := fmt.Sprintf("get target path %s state error %v", targetPath, err)
		log.Errorln(msg)
		return errors.New(msg)
	}

	if err := os.RemoveAll(targetPath); err != nil {
		msg := fmt.Sprintf("remove target path %s error %v", targetPath, err)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func tryDisConnectVolume(ctx context.Context, targetPath string) error {
	err := unmountUnix(ctx, targetPath)
	if err != nil {
		return err
	}

	return removeTargetPath(targetPath)
}
