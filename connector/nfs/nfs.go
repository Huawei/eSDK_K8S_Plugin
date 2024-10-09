/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils/log"
)

// Connector to define a local lock when connect or disconnect, in order to preventing mounting and unmounting confusion
type Connector struct {
}

const (
	formatWaitInternal       = 10
	halfTiSizeBytes    int64 = 549755813888
	oneTiSizeBytes     int64 = 1099511627776
	tenTiSizeBytes     int64 = 10995116277760
	hundredTiSizeBytes int64 = 109951162777600
	halfPiSizeBytes    int64 = 562949953421312
)

func init() {
	connector.RegisterConnector(connector.NFSDriver, &Connector{})
}

// ConnectVolume to mount the source to target path, the source path can be block or nfs
// Example:
//
//	mount /dev/sdb /<target-path>
//	mount <source-path> /<target-path>
func (nfs *Connector) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("Nfs Start to connect volume ==> connect info: %v", conn)
	return tryConnectVolume(ctx, conn)
}

// DisConnectVolume to unmount the target path
func (nfs *Connector) DisConnectVolume(ctx context.Context, targetPath string) error {
	log.AddContext(ctx).Infof("NFS Start to disconnect volume ==> target path is: %v", targetPath)
	return tryDisConnectVolume(ctx, targetPath)
}
