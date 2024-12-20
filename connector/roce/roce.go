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

package roce

import (
	"context"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Connector implements the connector.VolumeConnector for Connector protocol
type Connector struct {
}

const (
	intNumTwo   = 2
	intNumThree = 3
)

func init() {
	connector.RegisterConnector(connector.RoCEDriver, &Connector{})
}

// ConnectVolume to mount the source to target path, the source path can be block or nfs
// Example:
//
//	mount /dev/sdb /<target-path>
//	mount <source-path> /<target-path>
func (roce *Connector) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("RoCE Start to connect volume ==> connect info: %v", conn)
	tgtLunGUID, exist := conn["tgtLunGuid"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunGuid does not exist in connection properties")
	}
	return connector.ConnectVolumeCommon(ctx, conn, tgtLunGUID, connector.RoCEDriver, tryConnectVolume)
}

// DisConnectVolume to unmount the target path
func (roce *Connector) DisConnectVolume(ctx context.Context, tgtLunGuid string) error {
	log.AddContext(ctx).Infof("RoCE Start to disconnect volume ==> Volume Guid info: %v", tgtLunGuid)
	return connector.DisConnectVolumeCommon(ctx, tgtLunGuid, connector.RoCEDriver, tryDisConnectVolume)
}
