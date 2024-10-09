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

// Package local to connect and disconnect local lun
package local

import (
	"context"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// Connector to define a local lock when connect or disconnect, in order to preventing connect and disconnect confusion
type Connector struct {
}

func init() {
	connector.RegisterConnector(connector.LocalDriver, &Connector{})
}

// ConnectVolume to connect local volume, such as /dev/disk/by-id/wwn-0x*
func (loc *Connector) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("Local connector Start to connect volume ==> connect info: %v", conn)
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunWWN does not exist in connection properties")
	}
	return connector.ConnectVolumeCommon(ctx, conn, tgtLunWWN, connector.LocalDriver, tryConnectVolume)
}

// DisConnectVolume to remove the local lun path
func (loc *Connector) DisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	log.AddContext(ctx).Infof("Local Connector Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	return connector.DisConnectVolumeCommon(ctx, tgtLunWWN, connector.LocalDriver, tryDisConnectVolume)
}
