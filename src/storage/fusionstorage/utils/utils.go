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

// Package utils to provide Pacific storage tools for csi
package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"utils"
	"utils/log"
)

const (
	softQuota = "softQuota"
	hardQuota = "hardQuota"
	delayDaysMinLimit = 0
	delayDaysMaxLimit = 4294967294
)

// IsStorageQuotaAvailable is to check the storageQuota is available or not
func IsStorageQuotaAvailable(storageQuotaConfig string) error {
	params, err := ExtractStorageQuotaParameters(storageQuotaConfig)
	if err != nil {
		return fmt.Errorf("StorageQuota parameter %s error: %v", storageQuotaConfig, err)
	}

	spaceQuota, exist := params["spaceQuota"].(string)
	if !exist {
		return fmt.Errorf("spaceQuota must configure for storageQuota")
	}

	if !checkQuotaTypeValid(spaceQuota) {
		return fmt.Errorf("spaceQuota just support softQuota or hardQuota, now is %s", spaceQuota)
	}

	delay, exist := params["gracePeriod"]
	if !exist {
		return nil
	}

	if spaceQuota == hardQuota {
		return fmt.Errorf("when spaceQuota is set to hardQuota, gracePeriod cannot be configured")
	}

	gracePeriod, err := utils.TransToIntStrict(delay)
	if err != nil {
		return fmt.Errorf("trans %s to int type error", delay)
	}

	if gracePeriod < delayDaysMinLimit || gracePeriod > delayDaysMaxLimit {
		msg := fmt.Sprintf("gracePeriod range is %d ~ %d", delayDaysMinLimit, delayDaysMaxLimit)
		log.Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

func checkQuotaTypeValid(spaceQuota string) bool {
	return spaceQuota == softQuota || spaceQuota == hardQuota
}

// ExtractStorageQuotaParameters is to extract the storageQuota info by use json Unmarshal
func ExtractStorageQuotaParameters(storageQuotaConfig string) (map[string]interface{}, error) {
	var params map[string]interface{}
	err := json.Unmarshal([]byte(storageQuotaConfig), &params)
	if err != nil {
		msg := fmt.Sprintf("Unmarshal storageQuota %s error: %v", storageQuotaConfig, err)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return params, nil
}
