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
	"fmt"
	"os/exec"
	"strings"

	"huawei-csi-driver/utils/log"
)

// DiscoverKubernetesCLI used to discover kubernetes CLI.
func DiscoverKubernetesCLI(detailPath string) (string, error) {
	var supportedClis []string
	for _, discoverFun := range LoadSupportedCLI() {
		cli, err := discoverFun()
		if err == nil {
			return cli, nil
		}
		supportedClis = append(supportedClis, cli)
	}
	return "", errors.New(noneSupportedCliErrorMsg(supportedClis, detailPath))
}

// noneSupportedCliErrorMsg used to generate error messages when could not find any supported CLI
func noneSupportedCliErrorMsg(supportedClis []string, detailPath string) string {
	prompt := "Could not find any supported CLI"
	promptFmt := "%s, e.g. %s, details see " + detailPath
	length := len(supportedClis)
	switch length {
	case 0:
		return prompt
	case 1:
		return fmt.Sprintf(promptFmt, prompt, supportedClis[0])
	case 2:
		return fmt.Sprintf(promptFmt, prompt, strings.Join(supportedClis, " or "))
	default:
		clis := strings.Join(supportedClis[0:length-1], ", ")
		lastone := supportedClis[length-1]
		return fmt.Sprintf(promptFmt, prompt, clis+" or "+lastone)
	}
}

// discoverKubeCLI used to discover kubectl CLI.
func discoverKubeCLI() (string, error) {
	output, err := exec.Command(CLIKubernetes, "version").CombinedOutput()
	if err == nil {
		return CLIKubernetes, nil
	}
	log.Errorf("run '%s version' failed, error: %v, output: %s", CLIKubernetes, err, output)
	return CLIKubernetes, errors.New("Could not find the Kubernetes CLI: " + CLIKubernetes)
}

// discoverOpenShiftCLI used to discover oc CLI.
func discoverOpenShiftCLI() (string, error) {
	output, err := exec.Command(CLIOpenShift, "version").CombinedOutput()
	if err == nil && verifyOpenShiftAPIResources() {
		return CLIOpenShift, nil
	}
	log.Errorf("run '%s version' failed, error: %v, output: %s", CLIOpenShift, err, output)
	return CLIOpenShift, errors.New("Could not find the OpenShift CLI: " + CLIOpenShift)
}

// verifyOpenShiftAPIResources used to verify api-resources.
func verifyOpenShiftAPIResources() bool {
	out, err := exec.Command("oc", "api-resources").CombinedOutput()
	if err != nil {
		log.Errorf("run 'oc api-resource' command failed, error: %v", err)
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
