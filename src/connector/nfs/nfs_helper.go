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

// Package nfs to mount or unmount filesystem
package nfs

import (
	"connector"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"utils"
	"utils/log"
)

type connectorInfo struct {
	srcType     string
	sourcePath  string
	targetPath  string
	fsType      string
	mntFlags    string
}

func parseNFSInfo(connectionProperties map[string]interface{}) (*connectorInfo, error) {
	var con connectorInfo
	srcType, typeExist := connectionProperties["srcType"].(string)
	if !typeExist || srcType == "" {
		msg := "there are no srcType in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	sourcePath, srcPathExist := connectionProperties["sourcePath"].(string)
	if !srcPathExist || sourcePath == "" {
		msg := "there are no source path in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	targetPath, tgtPathExist := connectionProperties["targetPath"].(string)
	if !tgtPathExist || targetPath == "" {
		msg := "there are no target path in the connection info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	fsType, _ := connectionProperties["fsType"].(string)
	if fsType == "" {
		fsType = "ext4"
	}

	mntFlags, _ := connectionProperties["mountFlags"].(string)

	con.srcType = srcType
	con.sourcePath = sourcePath
	con.targetPath = targetPath
	con.fsType = fsType
	con.mntFlags = mntFlags
	return &con, nil
}

func tryConnectVolume(connMap map[string]interface{}) error {
	conn, err := parseNFSInfo(connMap)
	if err != nil {
		return err
	}

	switch conn.srcType {
	case "block":
		_, err = connector.ReadDevice(conn.sourcePath)
		if err != nil {
			return err
		}

		err = mountDisk(conn.sourcePath, conn.targetPath, conn.fsType, conn.mntFlags)
		if err != nil {
			return err
		}
	case "fs":
		err = mountFS(conn.sourcePath, conn.targetPath, conn.mntFlags)
		if err != nil {
			return err
		}
	default:
		return errors.New("volume device not found")
	}
	return nil
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

func mountFS(sourcePath, targetPath, flags string) error {
	return mountUnix(sourcePath, targetPath, flags, false)
}

func readMountPoints() (map[string]string, error) {
	data, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		log.Errorf("Read the mount file error: %v", err)
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

func mountUnix(sourcePath, targetPath, flags string, checkSourcePath bool) error {
	var output string
	var err error
	err = preMount(sourcePath, targetPath, checkSourcePath)
	if err != nil {
		return err
	}

	mountMap, err := readMountPoints()
	value, exist := mountMap[targetPath]
	if exist && value != sourcePath {
		msg := fmt.Sprintf("The mount %s is already exist, but the source path is not %s",
			targetPath, sourcePath)
		log.Errorln(msg)
		return errors.New(msg)
	}

	if flags != "" {
		output, err = utils.ExecShellCmd("mount %s %s -o %s", sourcePath, targetPath, flags)
	} else {
		output, err = utils.ExecShellCmd("mount %s %s", sourcePath, targetPath)
	}

	if err != nil {
		log.Errorf("Mount %s to %s error: %s", sourcePath, targetPath, output)
		return err
	}

	return nil
}

func getFSType(sourcePath string) (string, error) {
	// the errorCode 2 means an unFormatted filesystem and the unavailable filesystem. So ensure the device is
	// available before calling command blkid
	if exist, err := utils.PathExist(sourcePath); !exist {
		return "", fmt.Errorf("find the device %s failed before get filesystem info, error: %v", sourcePath, err)
	}

	output, err := utils.ExecShellCmd("blkid -o udev %s", sourcePath)
	if err != nil {
		if errCode, ok := err.(*exec.ExitError); ok && errCode.ExitCode() == 2 {
			log.Infof("Query fs of %s error: %s", sourcePath, output)
			if formatted, err := connector.IsDeviceFormatted(sourcePath); err != nil {
				return "", fmt.Errorf("check device %s formatted failed, error: %v", sourcePath, err)
			} else if formatted {
				return "", fmt.Errorf("the device %s is formatted, error: %v", sourcePath, err)
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

func formatDisk(sourcePath, fsType string) error {
	output, err := utils.ExecShellCmd("mkfs -t %s -F %s", fsType, sourcePath)
	if err != nil {
		log.Errorf("Couldn't mkfs %s to %s: %s", sourcePath, fsType, output)
		return err
	}
	return nil
}

func mountDisk(sourcePath, targetPath, fsType, flags string) error {
	var err error
	existFsType, err := getFSType(sourcePath)
	if err != nil {
		return err
	}

	if existFsType == "" {
		err = formatDisk(sourcePath, fsType)
		if err != nil {
			return err
		}

		err = mountUnix(sourcePath, targetPath, flags, true)
		if err != nil {
			return err
		}
	} else {
		err = mountUnix(sourcePath, targetPath, flags, true)
		if err != nil {
			return err
		}

		err = connector.ResizeMountPath(targetPath)
		if err != nil {
			log.Errorf("Resize mount path %s err %s", targetPath, err)
			return err
		}
	}
	return nil
}

func unmountUnix(targetPath string) error {
	output, err := utils.ExecShellCmd("umount %s", targetPath)
	if err != nil && !strings.Contains(output, "not mounted") {
		log.Errorf("Unmount %s error: %s", targetPath, output)
		return err
	}

	return nil
}

func tryDisConnectVolume(targetPath string) error {
	return unmountUnix(targetPath)
}
