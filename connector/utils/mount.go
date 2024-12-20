/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

package utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	targetMountPathPermission = 0750
)

// MountParam is the parameters for mount
type MountParam struct {
	DashT string
	DashO string
}

// BindMountRawBlockDevice mounts the raw block device to target path
func BindMountRawBlockDevice(ctx context.Context, sourcePath, targetPath string, mountFlags []string) error {
	exists, err := connector.MountPathIsExist(ctx, targetPath)
	if err != nil {
		return err
	}

	mountParams := MountParam{DashO: strings.Join(append(mountFlags, "bind"), ",")}
	if exists {
		return Mount(ctx, sourcePath, targetPath, mountParams, false)
	}

	file, err := os.Lstat(targetPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to lstat target file %s: %w", targetPath, err)
		}

		if err = createMountTargetFile(targetPath); err != nil {
			return err
		}

		return Mount(ctx, sourcePath, targetPath, mountParams, false)
	}

	// Check if device file
	if file.Mode()&os.ModeDevice == os.ModeDevice {
		srcFile, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to stat source file %s: %w", sourcePath, err)
		}
		if os.SameFile(srcFile, file) {
			log.AddContext(ctx).Infof("Skipped bind mount because it is already exist on: %s", targetPath)
			return nil
		}

		return fmt.Errorf("bind mount already exist on: %s,and it is not the expected device: %s",
			targetPath, sourcePath)
	}

	// Before CSI V4.6.0, raw block volumes are used by creating symlink of devices,
	// so we need to check if the target path is a symlink and recreate it.
	if file.Mode()&os.ModeSymlink == os.ModeSymlink {
		if err = createMountTargetFile(targetPath); err != nil {
			return err
		}
	}

	log.AddContext(ctx).Infof("File %s is already exist but not mounted, skip creating file", targetPath)

	return Mount(ctx, sourcePath, targetPath, mountParams, false)
}

// MountToDir mounts source to target which is a directory.
func MountToDir(ctx context.Context, sourcePath, targetPath string, flags MountParam, checkSourcePath bool) error {
	err := preMount(sourcePath, targetPath, checkSourcePath)
	if err != nil {
		return err
	}

	return Mount(ctx, sourcePath, targetPath, flags, checkSourcePath)
}

// Mount mounts source to target with given flags.
func Mount(ctx context.Context, sourcePath, targetPath string, flags MountParam, checkSourcePath bool) error {
	var output string
	var err error

	mountMap, err := connector.ReadMountPoints(ctx)
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
		if value == sourcePath || path.Base(path.Dir(targetPath)) == path.Base(path.Dir(sourcePath)) ||
			ContainSourceDevice(ctx, sourcePath, value) {
			log.AddContext(ctx).Infof("Mount %s to %s is already exist", sourcePath, targetPath)
			return nil
		}

		return fmt.Errorf("the mount %s is already exist, but the source path is not %s, instead of %s",
			targetPath, sourcePath, value)
	}

	flags = appendXFSMountFlags(ctx, sourcePath, flags)

	if flags.DashT != "" {
		flags.DashT = fmt.Sprintf("-t %s", flags.DashT)
	}

	if flags.DashO != "" {
		flags.DashO = fmt.Sprintf("-o %s", flags.DashO)
	}

	output, err = utils.ExecShellCmd(ctx, "mount %s %s %s %s", sourcePath, targetPath, flags.DashT, flags.DashO)
	if err != nil {
		log.AddContext(ctx).Errorf("Mount %s to %s failed, error res: %s, error: %s",
			sourcePath, targetPath, output, err)
		return err
	}

	return nil
}

// Unmount unmounts the target path
func Unmount(ctx context.Context, targetPath string) error {
	mounted, err := connector.MountPathIsExist(ctx, targetPath)
	if err != nil {
		return err
	}
	if !mounted {
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

// ContainSourceDevice used to check whether target path referenced source device is equal to sourceDev
func ContainSourceDevice(ctx context.Context, targetPath, sourceDev string) bool {
	for _, value := range findSourceDevice(ctx, targetPath) {
		if value == sourceDev {
			return true
		}
	}
	return false
}

func createMountTargetFile(targetPath string) error {
	// Remove old mount target file then create new one
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove target file %s: %w", targetPath, err)
	}

	newFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, targetMountPathPermission)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", targetPath, err)
	}

	if err = newFile.Close(); err != nil {
		return fmt.Errorf("failed to close file %s: %w", targetPath, err)
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
		log.Infof("target path %s does not exist, create it", targetPath)
		if err := os.MkdirAll(targetPath, targetMountPathPermission); err != nil {
			return errors.New("can not create a target path")
		}
	}

	return nil
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

// findSourceDevice use findmnt command to find mountPath referenced source device
func findSourceDevice(ctx context.Context, targetPath string) []string {
	output, err := utils.ExecShellCmd(ctx, "findmnt -o source --noheadings --target %s", targetPath)
	if err != nil {
		return []string{}
	}

	return strings.Split(output, "\n")
}

func appendXFSMountFlags(ctx context.Context, sourcePath string, flags MountParam) MountParam {
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
		if flags.DashO == "" {
			flags.DashO = "nouuid"
		} else {
			flags.DashO = fmt.Sprintf("%s,nouuid", flags.DashO)
		}
	}

	log.AddContext(ctx).Infof("mount flags: [%v]", flags)
	return flags
}
