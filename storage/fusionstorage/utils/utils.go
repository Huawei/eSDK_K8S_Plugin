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

// Package utils to provide Pacific storage tools for csi
package utils

import (
	"context"
	"encoding/json"
	"fmt"

	"huawei-csi-driver/utils"
)

const (
	softQuota = "softQuota"
	hardQuota = "hardQuota"

	delayDaysMinLimit = 0
	delayDaysMaxLimit = 4294967294
)

// IsStorageQuotaAvailable is to check the storageQuota is available or not
func IsStorageQuotaAvailable(ctx context.Context, storageQuotaConfig string) error {
	params, err := ExtractStorageQuotaParameters(ctx, storageQuotaConfig)
	if err != nil {
		return utils.Errorf(ctx, "StorageQuota parameter %s error: %v", storageQuotaConfig, err)
	}

	spaceQuota, exist := params["spaceQuota"].(string)
	if !exist {
		return utils.Errorf(ctx, "spaceQuota must configure for storageQuota")
	}

	if !checkQuotaTypeValid(spaceQuota) {
		return utils.Errorf(ctx, "spaceQuota just support softQuota or hardQuota, now is %s", spaceQuota)
	}

	delay, exist := params["gracePeriod"]
	if !exist {
		return nil
	}

	if spaceQuota == hardQuota {
		return utils.Errorf(ctx, "when spaceQuota is set to hardQuota, gracePeriod cannot be configured")
	}

	gracePeriod, err := utils.TransToIntStrict(ctx, delay)
	if err != nil {
		return utils.Errorf(ctx, "trans %s to int type error", delay)
	}

	if gracePeriod < delayDaysMinLimit || gracePeriod > delayDaysMaxLimit {
		return utils.Errorf(ctx, "gracePeriod range is %d ~ %d", delayDaysMinLimit, delayDaysMaxLimit)
	}

	return nil
}

func checkQuotaTypeValid(spaceQuota string) bool {
	return spaceQuota == softQuota || spaceQuota == hardQuota
}

// ExtractStorageQuotaParameters is to extract the storageQuota info by use json Unmarshal
func ExtractStorageQuotaParameters(ctx context.Context, storageQuotaConfig string) (map[string]interface{}, error) {
	var params map[string]interface{}
	err := json.Unmarshal([]byte(storageQuotaConfig), &params)
	if err != nil {
		return nil, utils.Errorf(ctx, "Unmarshal storageQuota %s error: %v", storageQuotaConfig, err)
	}

	return params, nil
}

// CheckErrorCode used to check Response
func CheckErrorCode(response map[string]interface{}) error {
	// Response example
	// Response: {
	//   "data": {
	// 	    "id": 14
	//    },
	//   "result": {
	// 	    "code": 0,
	//      "description": ""
	//    }
	// }

	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("convert result [%v] to map[string]interface{} failed", response["result"])
	}

	code, ok := result["code"].(float64)
	if !ok {
		return fmt.Errorf("convert errCode [%v] to float64 failed", result["code"])
	}

	if int(code) != 0 {
		return fmt.Errorf("invoking failed., code: [%d], description: [%v]", int(code), result["description"])
	}

	return nil
}
