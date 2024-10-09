/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package iscsi

import (
	"context"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// Connector implements the connector.VolumeConnector for Connector protocol
type Connector struct {
}

func init() {
	connector.RegisterConnector(connector.ISCSIDriver, &Connector{})
}

// ConnectVolume to mount the source to target path, the source path can be block or nfs
// Example:
//
//	mount /dev/sdb /<target-path>
//	mount <source-path> /<target-path>
func (isc *Connector) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("ISCSI Start to connect volume ==> connect info: %v",
		utils.MaskConnSensitiveInfo(conn))
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunWWN does not exist in connection properties")
	}
	return connector.ConnectVolumeCommon(ctx, conn, tgtLunWWN, connector.ISCSIDriver, tryConnectVolume)
}

// DisConnectVolume to unmount the target path
func (isc *Connector) DisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	log.AddContext(ctx).Infof("ISCSI Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	return connector.DisConnectVolumeCommon(ctx, tgtLunWWN, connector.ISCSIDriver, tryDisConnectVolume)
}
