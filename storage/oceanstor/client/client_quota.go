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
	"context"
	"fmt"

	"huawei-csi-driver/utils"
)

const (
	QuotaTypeDir       int = 1
	QuotaTypeUser      int = 2
	QuotaTypeUserGroup int = 3

	SpaceUnitTypeGB int = 3

	ForceFlagTrue  bool = true
	ForceFlagFalse bool = false
)

type OceanStorQuota interface {
	// CreateQuota use for create quota for dTree or file system
	CreateQuota(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
	// UpdateQuota use for update a quota
	UpdateQuota(ctx context.Context, quotaID string, params map[string]interface{}) error
	// GetQuota use for get quota information
	GetQuota(ctx context.Context, quotaID, vStoreID string, spaceUnitType uint32) (map[string]interface{}, error)
	// BatchGetQuota use for get quota information
	BatchGetQuota(ctx context.Context, params map[string]interface{}) ([]interface{}, error)
	// DeleteQuota use for delete a quota
	DeleteQuota(ctx context.Context, quotaID, vStoreID string, forceFlag bool) error
}

func (cli *BaseClient) CreateQuota(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	resp, err := cli.Post(ctx, "/FS_QUOTA", params)
	if err != nil {
		return nil, err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return nil, fmt.Errorf("create quota failed, params: %+v error: %v", params, resp.Error["description"])
	}

	return cli.getResponseDataMap(ctx, resp.Data)
}

func (cli *BaseClient) UpdateQuota(ctx context.Context, quotaID string, params map[string]interface{}) error {
	resp, err := cli.Put(ctx, fmt.Sprintf("/FS_QUOTA/%v", quotaID), params)
	if err != nil {
		return err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return fmt.Errorf("update quota failed, params: %+v error: %v", params, resp.Error["description"])
	}

	return nil
}

func (cli *BaseClient) GetQuota(ctx context.Context, quotaID, vStoreID string, spaceUnitType uint32) (map[string]interface{}, error) {
	resp, err := cli.Get(ctx, fmt.Sprintf("/FS_QUOTA/%v", quotaID), map[string]interface{}{
		"SPACEUNITTYPE": spaceUnitType,
		"vstoreId":      vStoreID,
	})
	if err != nil {
		return nil, err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return nil, fmt.Errorf("get quota failed, quotaID: %v error: %s", quotaID, resp.Error["description"])
	}

	return cli.getResponseDataMap(ctx, resp.Data)
}

func (cli *BaseClient) BatchGetQuota(ctx context.Context, params map[string]interface{}) ([]interface{}, error) {
	url := fmt.Sprintf("/FS_QUOTA?PARENTTYPE=%v&PARENTID=%v&range=%v&vstoreId=%v&QUERYTYPE=%v&SPACEUNITTYPE=%v",
		params["PARENTTYPE"], params["PARENTID"], params["range"], params["vstoreId"],
		params["QUERYTYPE"], params["SPACEUNITTYPE"])
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return nil, fmt.Errorf("get quota failed, params: %v error: %s", params, resp.Error["description"])
	}

	return cli.getResponseDataList(ctx, resp.Data)
}

func (cli *BaseClient) DeleteQuota(ctx context.Context, quotaID, vStoreID string, forceFlag bool) error {
	resp, err := cli.Delete(ctx, fmt.Sprintf("/FS_QUOTA/%v", quotaID), map[string]interface{}{
		"forceFlag": forceFlag,
		"vstoreId":  vStoreID,
	})
	if err != nil {
		return err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return fmt.Errorf("delete quota failed, quotaID: %v error: %s", quotaID, resp.Error["description"])
	}

	return nil
}
