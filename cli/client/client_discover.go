/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

package client

import (
	"errors"
	"os/exec"
	"strings"

	"huawei-csi-driver/utils/log"
)

// DiscoverKubernetesCLI used to discover kubernetes CLI.
func DiscoverKubernetesCLI() (string, error) {
	for _, discoverFun := range LoadSupportedCLI() {
		if cli, err := discoverFun(); err == nil {
			return cli, nil
		}
	}
	return "", errors.New("Could not find any supported CLI")
}

// discoverKubeCLI used to discover kubectl CLI.
func discoverKubeCLI() (string, error) {
	_, err := exec.Command(CLIKubernetes, "version").CombinedOutput()
	if err == nil {
		return CLIKubernetes, nil
	}
	return "", errors.New("Could not find the Kubernetes CLI")
}

// discoverOpenShiftCLI used to discover oc CLI.
func discoverOpenShiftCLI() (string, error) {
	_, err := exec.Command(CLIOpenShift, "version").CombinedOutput()
	if err == nil && verifyOpenShiftAPIResources() {
		return CLIOpenShift, nil
	}
	return "", errors.New("Could not find the OpenShift CLI")
}

// verifyOpenShiftAPIResources used to verify api-resources.
func verifyOpenShiftAPIResources() bool {
	out, err := exec.Command("oc", "api-resources").CombinedOutput()
	if err != nil {
		return false
	}

	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		if strings.Contains(l, "config.openshift.io") {
			return true
		}
	}
	log.Warningln("Couldn't find OpenShift api-resources, hence not using oc tools for CLI")
	return false
}

// LoadSupportedCLI used to load all supported CLI, e.g. kubectl, oc
func LoadSupportedCLI() []func() (string, error) {
	return []func() (string, error){
		discoverKubeCLI,
		discoverOpenShiftCLI,
	}
}
