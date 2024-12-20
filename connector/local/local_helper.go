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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const waitDevOnlineTimeInterval = 2 * time.Second

func waitDevOnline(ctx context.Context, tgtLunWWN string) string {
	devPath := fmt.Sprintf("/dev/disk/by-id/wwn-0x%s", tgtLunWWN)
	for i := 0; i < 30; i++ {
		output, _ := utils.ExecShellCmd(ctx, "ls -l %s", devPath)
		if strings.Contains(output, "No such file or directory") {
			time.Sleep(waitDevOnlineTimeInterval)
		} else if strings.Contains(output, devPath) {
			return devPath
		}
	}
	log.AddContext(ctx).Warningf("Wait dev %s online timeout", devPath)
	return ""
}

func tryConnectVolume(ctx context.Context, conn map[string]interface{}) (string, error) {
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		return "", utils.Errorln(ctx, "key tgtLunWWN does not exist in connectionProperties")
	}

	devPath := waitDevOnline(ctx, tgtLunWWN)
	if devPath == "" {
		return "", nil
	}

	err := connector.VerifySingleDevice(ctx, devPath, tgtLunWWN,
		connector.VolumeDeviceNotFound, tryDisConnectVolume)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func tryDisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	return connector.DisConnectVolume(ctx, tgtLunWWN, tryToDisConnectVolume)
}

func tryToDisConnectVolume(ctx context.Context, tgtLunWWN string) error {
	virtualDevice, devType, err := connector.GetVirtualDevice(ctx, tgtLunWWN)
	if err != nil {
		log.AddContext(ctx).Errorf("Get device of WWN %s error: %v", tgtLunWWN, err)
		return err
	}

	if virtualDevice == "" {
		log.AddContext(ctx).Infof("The device of WWN %s does not exist on host", tgtLunWWN)
		return errors.New("FindNoDevice")
	}

	phyDevices, err := connector.GetPhysicalDevices(ctx, virtualDevice, devType)
	if err != nil {
		return err
	}

	_, err = connector.RemoveAllDevice(ctx, virtualDevice, phyDevices, devType)
	return err
}
