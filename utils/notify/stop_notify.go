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

// Package notify offers a wait and notify mechanism
package notify

import (
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var stopChan = make(chan struct{})

// Stop used to throw out the stop signal
func Stop(format string, args ...interface{}) {
	log.Errorf(format, args...)
	stopChan <- struct{}{}
	wait()
}

// GetStopChan used to get stop channel
func GetStopChan() chan struct{} {
	return stopChan
}

func wait() {
	// The purpose is to block business goroutine
	stopChan <- struct{}{}
}
