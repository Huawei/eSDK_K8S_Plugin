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
	"regexp"
	"strconv"
	"strings"
	"sync"

	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/pwd"
)

func applySecret(secretExists bool) error {
	c := startPrintProgress("Getting backend configuration information.")
	backendList, err := getBackConfigStatusList(secretExists)
	stopPrintProgress(c)
	if err != nil {
		msg := "Failed to obtain backend configuration information."
		fmt.Println(msg)
		log.Errorf("%s %v", msg, err)
		return err
	}

	for {
		backend := selectOneBackend(backendList)
		if backend == nil {
			continue
		}

		if backend.Configured {
			ok, err := getInputBool(fmt.Sprintf("The backend %s has been configured. Update? [Y/N]:", backend.Name))
			if err != nil {
				log.Fatalf("failed to select whether to update the configured backend. %v", err)
			}
			if !ok {
				continue
			}
		}

		account, err := configOneBackend(*backend)
		if err != nil {
			log.Errorf("failed to configure the backend account. %v", err)
			continue
		}

		if account != nil {
			validAccountMap[backend.Name] = *account
			backend.Configured = true
		}
	}
}

func getBackConfigStatusList(secretExists bool) ([]backendConfigStatus, error) {
	var backends []backendConfigStatus
	var existAccountMap = make(map[string]backendAccount)
	var errChan = make(chan error, 2)

	var w sync.WaitGroup
	w.Add(1)
	go func() {
		defer w.Done()
		var err error
		backends, err = getBackendsFromConfigMap()
		if err != nil {
			errChan <- err
		}
	}()

	if !secretExists {
		w.Wait()
		select {
		case err := <-errChan:
			return nil, err
		default:
			return backends, nil
		}
	}

	w.Add(1)
	go func() {
		defer w.Done()
		var err error
		existAccountMap, err = getExistBackendAccount()
		if err != nil {
			errChan <- err
		}
	}()

	w.Wait()
	select {
	case err := <-errChan:
		return nil, err
	default:
		return updateStatusOfBackends(backends, existAccountMap)
	}
}

func getBackendsFromConfigMap() ([]backendConfigStatus, error) {
	configMap, err := client.GetConfigMap(HUAWEICSIConfigMap)
	if err != nil {
		return nil, fmt.Errorf("failed to get configmap %s. Err: %v", HUAWEICSIConfigMap, err)
	}

	var backendList struct {
		Backends []map[string]interface{} `json:"backends"`
	}
	err = json.Unmarshal([]byte(configMap.Data["csi.json"]), &backendList)
	if err != nil {
		return nil, fmt.Errorf("unmarshal config file %s error: %v", configMap.Data["csi.json"], err)
	}

	return parseBackends(backendList.Backends)
}

func selectOneBackend(backendList []backendConfigStatus) *backendConfigStatus {
	printBackendsStatus(backendList)

	number, err := getSelectedBackendNumber("Please enter the backend number to configure "+
		"(Enter 'exit' to exit):", len(backendList))
	if err != nil {
		log.Errorf("failed to get backend number entered by user. %v", err)
		return nil
	}

	return &backendList[number-1]
}

func configOneBackend(backend backendConfigStatus) (*backendAccount, error) {
	if backend.RetryTimes+1 > maxRetryTimes {
		fmt.Printf("\nThe account of %s has been locked.", backend.Name)
		return nil, fmt.Errorf("the account of %s has been locked", backend.Name)
	}

	backend.RetryTimes++
	account, err := getInputAccountInfo(backend)
	if err == nil {
		fmt.Printf("\nThe acount information of the backend %s has been configured successfully.", backend.Name)
		return account, nil
	}
	log.Errorf("Failed to execute the getInputAccountInfo function. %v", err)

	chances := maxRetryTimes - backend.RetryTimes
	if chances == 0 {
		return configOneBackend(backend)
	}

	tips := fmt.Sprintf("\nYou have %d chances to enter user and password, "+
		"Do you want to enter the account again? [Y/N]:", chances)
	ok, err := getInputBool(tips)
	if err != nil {
		return nil, fmt.Errorf("failed to execute getInputBool. %v", err)
	}

	if ok {
		return configOneBackend(backend)
	}

	fmt.Printf("Backend %s is unconfigured\n", backend.Name)
	return nil, nil
}

func getInputAccountInfo(backend backendConfigStatus) (*backendAccount, error) {
	fmt.Printf("**************************Current backend information***************************"+
		"\nName:%s\nUrls:%v\n", backend.Name, backend.Urls)

	var account = backendAccount{}
	var err error
	account.Username, err = getInputString("Please enter this backend user name:", true)
	if err != nil {
		return nil, errors.New("failed to obtain the user name")
	}

	account.Password, err = getInputString("Please enter this backend password:", false)
	if err != nil {
		return nil, errors.New("failed to obtain the password")
	}

	account.KeyText, err = generateKeyText()
	if err != nil {
		return nil, fmt.Errorf("generate random string error. %v", err)
	}

	c := startPrintProgress("\nVerifying user name and password. Please wait.")
	err = verifyingAccountValidity(backend, account)
	stopPrintProgress(c)
	if err != nil {
		return nil, err
	}

	return &account, nil
}

func getExistBackendAccount() (map[string]backendAccount, error) {
	secret, err := client.GetSecret(HUAWEICSISecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s. Err: %v", HUAWEICSISecret, err)
	}

	var csiSecret struct {
		Secrets map[string]backendAccount `json:"secrets"`
	}
	err = json.Unmarshal(secret.Data["secret.json"], &csiSecret)
	if err != nil {
		return nil, fmt.Errorf("unmarshal secret %s error: %v", HUAWEICSISecret, err)
	}

	return csiSecret.Secrets, nil
}

func updateStatusOfBackends(backends []backendConfigStatus,
	existAccountMap map[string]backendAccount) ([]backendConfigStatus, error) {
	var w sync.WaitGroup
	var m sync.Mutex
	var semaphore = make(chan struct{}, concurrentNumber)
	for i, oneBackend := range backends {
		oneAccount, ok := existAccountMap[oneBackend.Name]
		if !ok {
			continue
		}

		semaphore <- struct{}{}
		w.Add(1)
		go func(index int, backend backendConfigStatus, account backendAccount) {
			defer func() {
				<-semaphore
				w.Done()
			}()

			var err error
			account.Password, err = pwd.Decrypt(account.Password, account.KeyText)
			if err != nil {
				log.Errorf("decrypt storage %s error: %v", backend.Name, err)
				return
			}

			err = verifyingAccountValidity(backend, account)
			if err != nil {
				log.Errorf("failed while verifying account. %v", err)
			}

			if err == nil || !isUsernameOrPasswordIncorrect(err, backend.Storage) {
				backends[index].Configured = true
				m.Lock()
				validAccountMap[backend.Name] = account
				m.Unlock()
				return
			}
		}(i, oneBackend, oneAccount)
	}

	w.Wait()
	return backends, nil
}

func isUsernameOrPasswordIncorrect(err error, storage string) bool {
	switch strings.ToLower(storage) {
	case fusionstorageSan, fusionstorageNas:
		return parseResponseErrorCode(err, `errorCode:([\d.e+]+)\s`) == fusionstorageAccountIncorrect
	case oceanstorSan, oceanstorNas:
		return parseResponseErrorCode(err, `code:([\d.e+]+)\s`) == oceanstorAccountIncorrect
	default:
		return false
	}
}

func parseResponseErrorCode(err error, expStr string) int64 {
	reg := regexp.MustCompile(expStr)
	sub := reg.FindStringSubmatch(err.Error())
	if len(sub) == 0 {
		log.Errorf("failed to parse the response error code. %", err)
		return 0
	}

	code, err := strconv.ParseFloat(sub[1], 64)
	if err != nil {
		log.Errorf("failed to parse the response error code. code:%s err:%v", sub[1], err)
		return 0
	}
	return int64(code)
}

func parseBackends(backends []map[string]interface{}) ([]backendConfigStatus, error) {
	var configStatusList []backendConfigStatus
	for _, backend := range backends {
		var config backendConfigStatus
		var err error

		config.Name, err = parseBackendName(backend)
		if err != nil {
			return nil, err
		}

		config.Storage, err = parseBackendStorage(backend, config.Name)
		if err != nil {
			return nil, err
		}

		config.Urls, err = parseBackendUrls(backend, config.Name)
		if err != nil {
			return nil, err
		}

		var ok bool
		config.VStoreName, ok = backend["vstoreName"].(string)
		if !ok {
			log.Infof("key vstoreName does not exist in backend %s", config.Name)
		}

		configStatusList = append(configStatusList, config)
	}

	return configStatusList, nil
}

func parseBackendName(backend map[string]interface{}) (string, error) {
	backendName, ok := backend["name"].(string)
	if !ok {
		return "", errors.New("the configured configmap is incorrect")
	}

	match, err := regexp.MatchString(`^[\w-]+$`, backendName)
	if err != nil || !match {
		return "", fmt.Errorf("backend name %v is invalid, "+
			"support upper&lower characters, numeric and [-_]", backendName)
	}

	_, exist := backendNameSet[backendName]
	if exist {
		return "", fmt.Errorf("backend name %s is duplicated", backendName)
	}
	backendNameSet[backendName] = struct{}{}

	return backendName, nil
}

func parseBackendStorage(backend map[string]interface{}, backendName string) (string, error) {
	storage, ok := backend["storage"].(string)
	if !ok {
		return "", fmt.Errorf("storage type must be configured for backend %s", backendName)
	}

	if !(storage == fusionstorageSan || storage == fusionstorageNas ||
		storage == oceanstorSan || storage == oceanstorNas) {
		return "", fmt.Errorf("the backend %s of %s type is unsupported", backendName, storage)
	}
	return storage, nil
}

func parseBackendUrls(backend map[string]interface{}, backendName string) ([]string, error) {
	var urlList []string
	urls, ok := backend["urls"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("urls must be configured for backend %s", backendName)
	}

	for _, url := range urls {
		str, ok := url.(string)
		if !ok {
			return nil, fmt.Errorf("the urls parameter in the backend %s is incorrect", backendName)
		}
		urlList = append(urlList, str)
	}

	if len(urlList) == 0 {
		return nil, fmt.Errorf("the urls parameter in the backend %s is empty", backendName)
	}
	return urlList, nil
}

func printBackendsStatus(statusList []backendConfigStatus) {
	const line = "\n------------------------------------------------------------------------------------"
	const space = 3
	var maxLength = len("BackendName")
	for _, backend := range statusList {
		if maxLength < len(backend.Name) {
			maxLength = len(backend.Name)
		}
	}

	nameColumnWidth := strconv.Itoa(maxLength + space)
	headerFormat := "\n%-8s%-12s%-" + nameColumnWidth + "s%s"
	rowFormat := "\n%-8d%-12t%-" + nameColumnWidth + "s%s"

	var builder strings.Builder
	builder.WriteString(line)
	builder.WriteString(fmt.Sprintf(headerFormat, "Number", "Configured", "BackendName", "Urls"))
	for i, backend := range statusList {
		builder.WriteString(fmt.Sprintf(rowFormat, i+1, backend.Configured, backend.Name, backend.Urls))
	}
	builder.WriteString(line)

	fmt.Println(builder.String())
}
