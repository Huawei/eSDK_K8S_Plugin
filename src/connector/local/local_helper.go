/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package local to connect and disconnect local lun
package local

import (
	"connector"
	"fmt"
	"strings"
	"time"

	"utils"
	"utils/log"
)

func waitDevOnline(tgtLunWWN string) string {
	devPath := fmt.Sprintf("/dev/disk/by-id/wwn-0x%s", tgtLunWWN)
	for i := 0; i < 30; i++ {
		output, _ := utils.ExecShellCmd("ls -l %s", devPath)
		if strings.Contains(output, "No such file or directory") {
			time.Sleep(time.Second * waitInternal)
			continue
		} else if strings.Contains(output, devPath) {
			return devPath
		}
	}
	log.Warningf("Wait dev %s online timeout", devPath)
	return ""
}

func tryConnectVolume(tgtLunWWN string) (string, error) {
	devPath := waitDevOnline(tgtLunWWN)
	if devPath == "" {
		return "", nil
	}

	err := connector.WaitDeviceRW(tgtLunWWN, devPath)
	if err != nil {
		return "", err
	}

	return devPath, nil
}

func tryDisConnectVolume(tgtLunWWN string) error {
	device, err := connector.GetDevice(nil, tgtLunWWN)
	if err != nil {
		log.Errorf("Get device of WWN %s error: %v", tgtLunWWN, err)
		return err
	}

	_, err = connector.RemoveDevice(device)
	if err != nil {
		log.Errorf("Remove device %s error: %v", device, err)
		return err
	}

	return nil
}
