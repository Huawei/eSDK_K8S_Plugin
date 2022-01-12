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

// Package command provides the method of configuring the backend
package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	k8sClient "cli/client"
	"utils"
	"utils/log"
	"utils/pwd"

	fusionstorageClient "storage/fusionstorage/client"
	oceanstorClient "storage/oceanstor/client"
)

const (
	maxRetryTimes    = 3
	concurrentNumber = 10
	duration         = 500 * time.Millisecond
	progressChar     = "..."
	exitCommand      = "exit"

	fusionstorageSan = "fusionstorage-san"
	fusionstorageNas = "fusionstorage-nas"
	oceanstorSan     = "oceanstor-san"
	oceanstorNas     = "oceanstor-nas"

	fusionstorageAccountIncorrect int64 = 1077949005
	oceanstorAccountIncorrect     int64 = 1077949061
)

var (
	validAccountMap = make(map[string]backendAccount)
	backendNameSet  = make(map[string]struct{})
)

type backendConfigStatus struct {
	Storage    string
	Urls       []string
	Name       string
	VStoreName string
	Configured bool
	RetryTimes uint8
}

type backendAccount struct {
	Username string `json:"user"`
	Password string `json:"password"`
	KeyText  string `json:"keyText"`
}

func safeExit() {
	c := startPrintProgress("Saving configuration. Please wait")
	defer stopPrintProgress(c)

	const saveErr = "\nFailed to save the configuration. Please check whether the configured configmap is correct."
	secretMap, err := getBackendSecretMap(validAccountMap)
	if err != nil {
		fmt.Println(saveErr)
		log.Errorf("Error saving configuration. %v", err)
		return
	}

	newSecretYAML := k8sClient.GetSecretYAML(HUAWEICSISecret, storageNamespace, nil, secretMap)
	err = client.CreateObjectByYAML(newSecretYAML)
	if err != nil {
		fmt.Println(saveErr)
		log.Errorf("could not update CSI Secret, err: %v", err)
		return
	}

	fmt.Println("\nThe configuration is saved successfully.")
	log.Infoln("********************Save CSI Secret Successful*********************")
	os.Exit(0)
}

func getBackendSecretMap(nameToAccountMap map[string]backendAccount) (map[string]string, error) {
	secretMap := make(map[string]string)
	for backendName, account := range nameToAccountMap {
		encrypted, err := pwd.Encrypt(account.Password, account.KeyText)
		if err != nil {
			return nil, fmt.Errorf("encrypt storage %s error: %v", backendName, err)
		}

		account.Password = encrypted
		secretBytes, err := json.Marshal(account)
		if err != nil {
			return nil, fmt.Errorf("marshal secret info failed, error: %v", err)
		}
		secretMap[backendName] = string(secretBytes)
	}
	return secretMap, nil
}

func checkBackendConnectivity(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil || u.Host == "" || u.Port() == "" {
		return fmt.Errorf("the format of the backend URL is incorrect. %s", urlStr)
	}

	addr := u.Host[:len(u.Host)-len(u.Port())-1]
	_, err = utils.ExecShellCmd("ping -c 3 -i 0.001 -w 1 %s", addr)
	if err != nil {
		return fmt.Errorf("the network between the host and the backend is disconnected. %s", addr)
	}
	return nil
}

func getSelectedBackendNumber(tips string, maxValue int) (int, error) {
	input, err := getInputString(tips, true)
	if err != nil {
		return 0, err
	}

	if strings.ToLower(input) == exitCommand {
		safeExit()
		os.Exit(0)
		return 0, nil
	}

	number, err := strconv.Atoi(input)
	if err == nil && number <= maxValue {
		return number, nil
	}

	fmt.Printf("Input invalid. The valid backend number is [1-%d].\n", maxValue)
	return getSelectedBackendNumber(tips, maxValue)
}

func getInputBool(tips string) (bool, error) {
	input, err := getInputString(tips, true)

	if err != nil {
		return false, err
	}

	str := strings.ToLower(input)
	if str == "yes" || str == "y" {
		return true, nil
	} else if str == "no" || str == "n" {
		return false, nil
	} else {
		return getInputBool(tips)
	}
}

func getInputString(tips string, isVisible bool) (string, error) {
	fmt.Print(tips)

	var sh string
	if isVisible {
		sh = "stty erase '^H' -isig -ixon && read str && echo $str"
	} else {
		sh = "stty erase '^H' -isig -ixon && read -s pwd && echo $pwd"
	}

	cmd := exec.Command("sh", "-c", sh)
	cmd.Stdin = os.Stdin
	bs, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	str := strings.TrimSpace(string(bs))
	if str == "" {
		return getInputString(tips, isVisible)
	}
	return str, nil
}

func startPrintProgress(tips string) chan<- struct{} {
	fmt.Print(tips)
	var c = make(chan struct{})
	go func() {
		for {
			select {
			case <-c:
				return
			default:
				fmt.Print(progressChar)
				time.Sleep(duration)
			}
		}
	}()
	return c
}

func stopPrintProgress(c chan<- struct{}) {
	c <- struct{}{}
	close(c)
}

func verifyingAccountValidity(backend backendConfigStatus, account backendAccount) error {
	for _, urlStr := range backend.Urls {
		err := checkBackendConnectivity(urlStr)
		if err != nil {
			return err
		}
	}

	switch strings.ToLower(backend.Storage) {
	case fusionstorageSan, fusionstorageNas:
		return checkFusionStorageAccount(backend.Urls[0], account)
	case oceanstorSan, oceanstorNas:
		return checkOceanStorAccount(backend.Urls, backend.VStoreName, account)
	default:
		msg := fmt.Sprintf("The backend of %s type is unsupported", backend.Storage)
		fmt.Println(msg)
		return errors.New(msg)
	}
}

func checkFusionStorageAccount(url string, account backendAccount) error {
	cli := fusionstorageClient.NewClient(url, account.Username, account.Password, "")
	err := cli.Login()
	if err != nil {
		log.Errorf("failed to log in to fusionstorage. %v", err)
		return err
	}

	cli.Logout()
	return nil
}

func checkOceanStorAccount(urls []string, vStoreName string, account backendAccount) error {
	cli := oceanstorClient.NewClient(urls, account.Username, account.Password, vStoreName, "")
	err := cli.Login()
	if err != nil {
		log.Errorf("failed to log in to oceanstor. %v", err)
		return err
	}

	cli.Logout()
	return nil
}
