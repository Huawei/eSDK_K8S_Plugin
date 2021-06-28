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
	"errors"
	"fmt"
	"time"

	"sync"
	"utils/log"
)

// Local to define a local lock when connect or disconnect, in order to preventing connect and disconnect confusion
type Local struct {
	mutex sync.Mutex
}

const waitInternal = 2

func init() {
	connector.RegisterConnector(connector.LocalDriver, &Local{})
}

// ConnectVolume to connect local volume, such as /dev/disk/by-id/wwn-0x*
func (loc *Local) ConnectVolume(conn map[string]interface{}) (string, error) {
	loc.mutex.Lock()
	defer loc.mutex.Unlock()
	log.Infof("Local Start to connect volume ==> connect info: %v", conn)
	tgtLunWWN, exist := conn["tgtLunWWN"].(string)
	if !exist {
		msg := "there are no target lun WWN in the connection info"
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	for i := 0; i < 3; i++ {
		device, err := tryConnectVolume(tgtLunWWN)
		if err == nil {
			return device, nil
		}

		time.Sleep(time.Second * waitInternal)
	}

	return "", nil
}

// DisConnectVolume to remove the local lun path
func (loc *Local) DisConnectVolume(tgtLunWWN string) error {
	loc.mutex.Lock()
	defer loc.mutex.Unlock()
	log.Infof("Local Start to disconnect volume ==> volume wwn is: %v", tgtLunWWN)
	for i := 0; i < 3; i++ {
		err := tryDisConnectVolume(tgtLunWWN)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second * waitInternal)
	}

	msg := fmt.Sprintf("failed to delete volume %s.", tgtLunWWN)
	log.Errorln(msg)
	return errors.New(msg)
}
