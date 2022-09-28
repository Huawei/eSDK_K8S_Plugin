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

// Package command to manage the kubernetes resources, such as: secret, configMap...
package command

import (
	"fmt"

	"huawei-csi-driver/utils/log"
)

// Update is to update the secret info for CSI
func Update() {
	initInstallerLogging()
	processInstallationArguments()
	update()
}

func recordErrorf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
	log.Fatalf(format, v...)
}

func recordInfof(format string, v ...interface{}) {
	fmt.Printf(format, v...)
	log.Infof(format, v...)
}

func update() {
	exist, err := client.CheckConfigMapExists(HUAWEICSIConfigMap)
	if err != nil {
		recordErrorf("Could not find csi config map. Error: %v", err)
	} else if !exist {
		recordErrorf("The configMap %s does not exist. Please config configMap first.", HUAWEICSIConfigMap)
	}

	exist, err = client.CheckSecretExists(HUAWEICSISecret)
	if err != nil {
		recordErrorf("Could not find csi secret. Error: %v", err)
	} else if !exist {
		recordInfof("The secret %s does not exist. Now to create a new secret object.\n", HUAWEICSISecret)
		if err := applySecret(exist); err != nil {
			recordErrorf("Create secret object error %v. See /var/log/huawei/huawei-csi-install for details.", err)
		}
	} else {
		if err := applySecret(exist); err != nil {
			recordErrorf("Update secret object error %v. See /var/log/huawei/huawei-csi-install for details.", err)
		}
	}
}

func updateSecret() error {
	return applySecret(true)
}
