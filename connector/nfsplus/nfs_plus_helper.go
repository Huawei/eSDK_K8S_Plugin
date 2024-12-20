/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

// Package nfsplus to mount or unmount filesystem
package nfsplus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	connUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	nfsPlusMountCommand = "mount %s %s %s %s"
	mountPathPermission = 0750
)

type connectorInfo struct {
	sourcePath string
	targetPath string
	portals    []string
	localAdds  string
	remoteAdds string
	mntFlags   mountParam
}

type mountParam struct {
	dashT string
	dashO string
}

func tryConnectVolume(ctx context.Context, connMap map[string]interface{}) (string, error) {
	connInfo, err := parseNFSPlusInfo(ctx, connMap)
	if err != nil {
		log.AddContext(ctx).Errorf("parse nfs plus info failed, connMap: %+v err: %v", connMap, err)
		return "", err
	}

	err = mountNFSPlus(ctx, connInfo)
	if err != nil {
		log.AddContext(ctx).Errorf("mount  plus info failed, connMap: %+v err: %v", connInfo, err)
		return "", err
	}

	return "", nil
}

func parseNFSPlusInfo(ctx context.Context, connectionProperties map[string]interface{}) (*connectorInfo, error) {
	var con connectorInfo
	sourcePath, srcPathExist := connectionProperties["sourcePath"].(string)
	if !srcPathExist || sourcePath == "" {
		return nil, pkgUtils.Errorln(ctx, "there are no source path in the connection info")
	}

	targetPath, tgtPathExist := connectionProperties["targetPath"].(string)
	if !tgtPathExist || targetPath == "" {
		return nil, pkgUtils.Errorln(ctx, "there are no target path in the connection info")
	}

	portals, portalsExist := connectionProperties["portals"].([]string)
	if !portalsExist || len(portals) == 0 {
		return nil, pkgUtils.Errorln(ctx, "there are no portals in the connection info")
	}

	// format mount flags : remoteAdds
	con.remoteAdds = strings.Join(portals, "~")
	// format mount flags : mountFlag
	mountFlags, _ := connectionProperties["mountFlags"].(string)
	mountFlagsArr := make([]string, 0)
	mountFlagsArr = append(mountFlagsArr, fmt.Sprintf("remoteaddrs=%s", con.remoteAdds))
	mountFlagsArr = append(mountFlagsArr, mountFlags)

	con.sourcePath = sourcePath
	con.targetPath = targetPath
	con.mntFlags = mountParam{dashO: strings.TrimSpace(strings.Join(mountFlagsArr, ",")), dashT: plugin.ProtocolNfs}

	log.AddContext(ctx).Infof("parseNFSPlusInfo success, data: %+v", con)
	return &con, nil
}

func checkMountPath(ctx context.Context, targetPath string) error {
	if _, err := os.Stat(targetPath); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(targetPath, mountPathPermission); err != nil {
			return pkgUtils.Errorln(ctx, "can not create a target path")
		}
	}

	return nil
}

func mountNFSPlus(ctx context.Context, conn *connectorInfo) error {
	var output string
	var err error
	err = checkMountPath(ctx, conn.targetPath)
	if err != nil {
		return pkgUtils.Errorf(ctx, "check mount path failed, err: %v", err)
	}

	mountMap, err := connector.ReadMountPoints(ctx)
	if err != nil {
		return pkgUtils.Errorf(ctx, "get mount point map failed, err: %v", err)
	}

	value, exist := mountMap[conn.targetPath]
	if exist {
		// check the filesystem by comparing the sourcePath and mountPath
		if value == conn.sourcePath || path.Base(path.Dir(conn.targetPath)) == path.Base(path.Dir(conn.sourcePath)) ||
			connUtils.ContainSourceDevice(ctx, conn.sourcePath, value) {
			log.AddContext(ctx).Infof("Mount %s to %s is already exist", conn.sourcePath, conn.targetPath)
			return nil
		}

		return pkgUtils.Errorf(ctx, "The mount %s is already exist, source: %s realSource: %s",
			conn.targetPath, conn.sourcePath, value)
	}

	if conn.mntFlags.dashT != "" {
		conn.mntFlags.dashT = fmt.Sprintf("-t %s", conn.mntFlags.dashT)
	}
	if conn.mntFlags.dashO != "" {
		conn.mntFlags.dashO = fmt.Sprintf("-o %s", conn.mntFlags.dashO)
	}

	output, err = utils.ExecShellCmd(ctx, fmt.Sprintf(nfsPlusMountCommand, conn.mntFlags.dashT, conn.mntFlags.dashO,
		conn.sourcePath, conn.targetPath))
	if err != nil {
		log.AddContext(ctx).Errorf("Mount %s to %s failed, error res: %s, error: %s",
			conn.sourcePath, conn.targetPath, output, err)
		return err
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
