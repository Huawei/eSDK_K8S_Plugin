/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.
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
	"context"
	"errors"
	"fmt"

	"huawei-csi-driver/utils/log"
)

const (
	quotaNotExist int64 = 37767685
)

// CreateQuota creates quota by params
func (cli *RestClient) CreateQuota(ctx context.Context, params map[string]interface{}) error {
	resp, err := cli.post(ctx, "/api/v2/file_service/fs_quota", params)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Failed to create quota %v, error: %d", params, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

// UpdateQuota updates quota by params
func (cli *RestClient) UpdateQuota(ctx context.Context, params map[string]interface{}) error {
	resp, err := cli.put(ctx, "/api/v2/file_service/fs_quota", params)
	if err != nil {
		return err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Failed to Update quota %v, error: %d", params, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

// GetQuotaByFileSystemById query quota info by file system id
func (cli *RestClient) GetQuotaByFileSystemById(ctx context.Context, fsID string) (map[string]interface{}, error) {
	url := "/api/v2/file_service/fs_quota?parent_type=40&parent_id=" +
		fsID + "&range=%7B%22offset%22%3A0%2C%22limit%22%3A100%7D"
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		return nil, fmt.Errorf("get quota by filesystem id %s error: %d", fsID, errorCode)
	}

	fsQuotas, exist := resp["data"].([]interface{})
	if !exist || len(fsQuotas) <= 0 {
		return nil, nil
	}

	for _, q := range fsQuotas {
		quota, ok := q.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The fsQuota %v's format is not map[string]interface{}", q)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}
		return quota, nil
	}
	return nil, nil
}

// DeleteQuota deletes quota by id
func (cli *RestClient) DeleteQuota(ctx context.Context, quotaID string) error {
	url := fmt.Sprintf("/api/v2/file_service/fs_quota/%s", quotaID)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		if errorCode == quotaNotExist {
			log.AddContext(ctx).Warningf("Quota %s doesn't exist while deleting.", quotaID)
			return nil
		}
		return fmt.Errorf("delete quota %s error: %d", quotaID, errorCode)
	}

	return nil
}
