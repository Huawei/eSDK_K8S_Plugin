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

type ISCSI struct {
}

func init() {
	connector.RegisterConnector(connector.ISCSIDriver, &ISCSI{})
}

func (isc *ISCSI) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("ISCSI Start to connect volume ==> connect info: %v", conn)
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunWWN does not exist in connection properties")
	}
	return connector.ConnectVolumeCommon(ctx, conn, tgtLunWWN, connector.ISCSIDriver, tryConnectVolume)
}

func (isc *ISCSI) DisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	log.AddContext(ctx).Infof("ISCSI Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	return connector.DisConnectVolumeCommon(ctx, tgtLunWWN, connector.ISCSIDriver, tryDisConnectVolume)
}
