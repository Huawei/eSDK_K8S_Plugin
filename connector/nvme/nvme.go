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

package nvme

import (
	"context"
	"sync"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// FCNVMe implements the connector.VolumeConnector for FCNVMe protocol
type FCNVMe struct {
	mutex sync.Mutex
}

func init() {
	connector.RegisterConnector(connector.FCNVMeDriver, &FCNVMe{})
}

// ConnectVolume to mount the source to target path, the source path can be block or nfs
// Example:
//
//	mount /dev/sdb /<target-path>
//	mount <source-path> /<target-path>
func (fc *FCNVMe) ConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	log.AddContext(ctx).Infof("FC-NVMe Start to connect volume ==> connect info: %v", conn)
	tgtLunGuid, exist := conn["tgtLunGuid"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "there is no Lun GUID in connect info")
	}

	return connector.ConnectVolumeCommon(ctx, conn, tgtLunGuid, connector.FCNVMeDriver, tryConnectVolume)
}

// DisConnectVolume to unmount the target path
func (fc *FCNVMe) DisConnectVolume(ctx context.Context, tgtLunGuid string) error {
	log.AddContext(ctx).Infof("FC-NVMe Start to disconnect volume ==> Volume Guid info: %v", tgtLunGuid)
	return connector.DisConnectVolumeCommon(ctx, tgtLunGuid, connector.FCNVMeDriver, tryDisConnectVolume)
}
