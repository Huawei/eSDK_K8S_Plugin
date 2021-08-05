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

// Package command to manage the kubernetes resources, such as: secret, configMap...
package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"utils/log"

	k8sClient "cli/client"
	"golang.org/x/crypto/ssh/terminal"
)
const inputArgsLength = 2

// Update is to update the secret info for CSI
func Update() {
	if len(os.Args) >= inputArgsLength {
		storageNamespace = os.Args[1]
	} else {
		storageNamespace = HUAWEINamespace
	}

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
		if err := createSecret(); err != nil {
			recordErrorf("Create secret object error %v. See /var/log/huawei/huawei-csi-install for details.", err)
		}
	} else {
		if err := updateSecret(); err != nil {
			recordErrorf("Update secret object error %v. See /var/log/huawei/huawei-csi-install for details.", err)
		}
	}
}

func getExistSecret(secret CSISecret, backendName string) (map[string]string, error) {
	secrets, exist := secret.Secrets[backendName]
	if !exist {
		msg := fmt.Sprintf("The key %s is not in secret.", backendName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	backendSecret, ok := secrets.(map[string]interface{})
	if !ok {
		return nil, errors.New("converting the secret to map failed")
	}

	user, err := getSecret(backendSecret, "user")
	if err != nil {
		return nil, errors.New("get user from secret failed")
	}

	password, err := getSecret(backendSecret, "password")
	if err != nil {
		return nil, errors.New("get password from secret failed")
	}

	keyText, err := getSecret(backendSecret, "keyText")
	if err != nil {
		return nil, errors.New("get keyText from secret failed")
	}

	secretInfo := map[string]string{
		"user":     user,
		"password": password,
		"keyText":  keyText,
	}
	return secretInfo, nil
}

func getSecret(backendSecret map[string]interface{}, secretKey string) (string, error) {
	if secretValue, exist := backendSecret[secretKey].(string); exist {
		return secretValue, nil
	}
	recordErrorf("The key %s is not in secret.", secretKey)
	return "", errors.New("secret is not normal")
}

func getSecretMap(storageConfig CSIConfig) (map[string]string, error) {
	secretMap := make(map[string]string)
	var url interface{}
	var exist bool
	for index, config := range storageConfig.Backends {
		if url, exist = config["urls"].([]interface{}); !exist {
			url, exist = config["url"].(interface{})
			if !exist {
				recordErrorf("There is no backend urls info")
			}
		}
		backendName, ok := config["name"].(string)
		if !ok {
			recordErrorf("There is no backend name info")
		}

		recordInfof("The %d backend name is: %s\tbackend url is: %s\n",
			index+1, backendName, url)
		fmt.Println("Do you want to update it? Y/N")
		input, err := terminal.ReadPassword(0)
		if err != nil {
			recordErrorf("Input error: %v", err)
			return nil, err
		}

		var secretInfo map[string]string
		if strings.TrimSpace(strings.ToUpper(string(input))) != "Y" &&
			strings.TrimSpace(strings.ToUpper(string(input))) != "YES" {
			fmt.Println("The secret is no need to update.")
			secretInfo, err = getExistSecret(storageSecret, backendName)
		} else {
			secretInfo, err = generateSecret(backendName)
		}

		if err != nil {
			recordErrorf("get Secret info error: %v", err)
			return nil, err
		}
		secretBytes, err := json.Marshal(secretInfo)
		if err != nil {
			recordErrorf("Unmarshal secret info failed, error: %v", err)
		}
		secretMap[backendName] = string(secretBytes)
		fmt.Printf("\n")
	}
	return secretMap, nil
}

func updateSecret() error {
	// step 1. query the configMap to get the all backend names
	configMap, err := client.GetConfigMap(HUAWEICSIConfigMap)
	if err != nil {
		recordErrorf("failed to get configmap %s. Err: %v", HUAWEICSIConfigMap, err)
		return err
	}

	err = json.Unmarshal([]byte(configMap.Data["csi.json"]), &storageConfig)
	if err != nil {
		log.Errorf("Unmarshal csi.json %s of config file failed: %v", configMap.Data["csi.json"], err)
		return err
	}

	// step 2. query the secret object to get the origin secret info
	secret, err := client.GetSecret(HUAWEICSISecret)
	if err != nil {
		recordErrorf("failed to get secret %s. Err: %v", HUAWEICSISecret, err)
		return err
	}

	err = json.Unmarshal([]byte(secret.Data["secret.json"]), &storageSecret)
	if err != nil {
		log.Errorf("Unmarshal secret %s error: %v", HUAWEICSISecret, err)
		return err
	}
	fmt.Printf("**************************All Secret Info****************************\n"+
		"%+v\n"+
		"*********************************************************************\n",
		storageSecret)

	// step 3. get the update secret info
	recordInfof("**************************All Backend Info***************************\n")
	secretMap, err := getSecretMap(storageConfig)
	if err != nil {
		log.Errorf("get secret info failed, err: %v", err)
		return err
	}

	// step 4. construct the yaml of the secret
	newSecretYAML := k8sClient.GetSecretYAML(HUAWEICSISecret, storageNamespace, nil, secretMap)

	// step 5. create the secret
	err = client.CreateObjectByYAML(newSecretYAML)
	if err != nil {
		log.Errorf("could not update CSI Secret, err: %v", err)
		return err
	}
	recordInfof("********************Update CSI Secret Successful*********************\n")
	return nil
}
