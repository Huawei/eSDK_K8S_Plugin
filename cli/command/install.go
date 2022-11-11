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

package command

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	logging "github.com/sirupsen/logrus"

	k8sClient "huawei-csi-driver/cli/client"
	"huawei-csi-driver/cli/utils"
	"huawei-csi-driver/utils/log"
)

const (
	KubernetesCSIVersionMin = "v1.12.0"

	HUAWEICSIConfigMap = "huawei-csi-configmap"
	HUAWEICSISecret    = "huawei-csi-secret"
	HUAWEINamespace    = "kube-system"
)

var (
	client k8sClient.Interface
)

func Init() {
	initInstallerLogging()
	processInstallationArguments()
	installSecret()
}

func installSecret() {
	exist, err := client.CheckConfigMapExists(HUAWEICSIConfigMap)
	if err != nil {
		log.Fatalf("Could not find csi config map. Error: %v", err)
	} else if !exist {
		log.Fatalf("The configMap %s does not exist. Please config configMap first.", HUAWEICSIConfigMap)
	}

	exist, err = client.CheckSecretExists(HUAWEICSISecret)
	if err != nil {
		log.Fatalf("Could not find csi secret. Error: %v", err)
	} else if exist {
		ok, err := getInputBool("The secret info is exist. Do you force to update it? [Y/N]:")
		if err != nil {
			log.Fatalf("Input error: %v", err)
		}

		if !ok {
			fmt.Println("The secret already exists and is not updated.")
			os.Exit(0)
		}
	}

	if err = applySecret(exist); err != nil {
		log.Fatalf("Create secret object error %v. See /var/log/huawei/huawei-csi-install for details.", err)
	}
}

func initClient() (k8sClient.Interface, error) {
	client, err := k8sClient.NewCliClient(*secretNamespace, 180*time.Second)
	if err != nil {
		return nil, fmt.Errorf("Could not new a Kubernetes client, err: %v", err)
	}

	return client, nil
}

func initInstallerLogging() {
	// Installer logs to stdout only
	logging.SetOutput(os.Stdout)
	logging.SetFormatter(&logging.TextFormatter{DisableTimestamp: true})

	err := log.InitLogging("huawei-csi-install")
	if err != nil {
		logging.WithField("error", err).Fatal("Failed to initialize logging.")
	}

	logging.WithField("logLevel", logging.GetLevel().String()).Debug("Initialized logging.")
}

func processInstallationArguments() {
	var err error
	if client, err = initClient(); err != nil {
		recordErrorf("could not initialize Kubernetes client; %v", err)
	}

	minOptionalCSIVersion := utils.MustParseSemantic(KubernetesCSIVersionMin)
	if client.ServerVersion().LessThan(minOptionalCSIVersion) {
		recordErrorf("CSI OceanStor requires Kubernetes %s or later.",
			minOptionalCSIVersion.ShortString())
	}
}

func generateKeyText() (string, error) {
	cmd := "head -c32 /dev/urandom | base64"
	shCmd := exec.Command("/bin/sh", "-c", cmd)
	output, err := shCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Generate random string error: %v", err)
		return "", err
	}
	output = output[:32]
	return string(output), nil
}
