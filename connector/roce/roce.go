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

package roce

import (
	"context"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type RoCE struct {
}

const (
	intNumTwo   = 2
	intNumThree = 3
)

func init() {
	connector.RegisterConnector(connector.RoCEDriver, &RoCE{})
}

func (roce *RoCE) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("RoCE Start to connect volume ==> connect info: %v", conn)
	tgtLunGUID, exist := conn["tgtLunGuid"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunGuid does not exist in connection properties")
	}
	return connector.ConnectVolumeCommon(ctx, conn, tgtLunGUID, connector.RoCEDriver, tryConnectVolume)
}

func (roce *RoCE) DisConnectVolume(ctx context.Context, tgtLunGuid string) error {
	log.AddContext(ctx).Infof("RoCE Start to disconnect volume ==> Volume Guid info: %v", tgtLunGuid)
	return connector.DisConnectVolumeCommon(ctx, tgtLunGuid, connector.RoCEDriver, tryDisConnectVolume)
}
