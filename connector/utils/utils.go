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

// Package utils provides common utils for connector
package utils

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	notInstall string = "NOT_INSTALL"
)

// VerifyMultipathService is used to verify the installation and running status of the multipath service.
func VerifyMultipathService(requiredServices []string, forbiddenServices []string) error {
	err := verifyServicesStates(requiredServices, true)
	if err != nil {
		log.Infof("Verify required multipath service failed. error:%v", err)
		return err
	}

	err = verifyServicesStates(forbiddenServices, false)
	if err != nil {
		log.Infof("Verify forbidden multipath service failed. error:%v", err)
		return err
	}

	return nil
}

func verifyServicesStates(services []string, needRunning bool) error {
	expectState := "active"
	if !needRunning {
		expectState = "inactive"
	}

	for _, service := range services {
		state, _, err := getServiceStates(service)
		if err != nil {
			log.Errorf("Get service %s status failed, error info: %v", service, err)
			return err
		}

		// forbidden scenario, verification passed if service not installed.
		if !needRunning && strings.Contains(state, notInstall) {
			continue
		}

		if strings.TrimSpace(state) != expectState {
			log.Errorf("Service status verified failed, service %s actual state: %s, expect state: %s.",
				service, state, expectState)
			return errors.New("service status verified failed")
		}
	}

	return nil
}

func getServiceStates(service string) (string, string, error) {
	var state string
	var subState string

	queryCmd := fmt.Sprintf("systemctl status %s | grep Active", service)
	output, err := utils.ExecShellCmd(context.Background(), queryCmd)
	if err != nil {
		if err.Error() == "exit status 1" && strings.Contains(strings.ToLower(output), "could not be found") {
			return notInstall, notInstall, nil
		}

		return state, subState, err
	}

	if output == "" {
		log.Warningf(`The output of command "%s" is empty.`, queryCmd)
		return state, subState, errors.New("query service status empty")
	}

	for _, line := range strings.Split(output, "\n") {
		pattern, err := regexp.Compile(`^[\s]*Active: ([\w]+) \(([\w]+)\)`)
		if err != nil {
			return state, subState, err
		}
		ret := pattern.FindAllStringSubmatch(line, -1)
		if ret != nil {
			// If the regular expression is met, there is no doubt that the data can be obtained.
			state = ret[0][1]
			subState = ret[0][2]
			break
		}
	}

	return state, subState, nil
}

// GetMultiPathInfo is used to obtain MultiPath configuration information.
func GetMultiPathInfo(connectionProperties map[string]interface{}) (bool, string, error) {
	volumeUseMultiPath, exist := connectionProperties["volumeUseMultiPath"].(bool)
	if !exist {
		return false, "", errors.New("there is no multiPath switch in the connection info")
	}

	multiPathType, exist := connectionProperties["multiPathType"].(string)
	if !exist {
		return volumeUseMultiPath, "", errors.New("the connection information does not contain multiPathType")
	}
	return volumeUseMultiPath, multiPathType, nil
}
