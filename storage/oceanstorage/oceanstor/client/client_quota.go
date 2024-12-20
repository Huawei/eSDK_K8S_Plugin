/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

const (
	// QuotaTypeDir defines dir type quota
	QuotaTypeDir int = 1

	// QuotaTypeUser defines user type
	QuotaTypeUser int = 2

	// QuotaTypeUserGroup defines user group type
	QuotaTypeUserGroup int = 3

	// SpaceUnitTypeGB defines GB type of space unit
	SpaceUnitTypeGB int = 3

	// ForceFlagTrue defines force flag true
	ForceFlagTrue bool = true

	// ForceFlagFalse defines force flag false
	ForceFlagFalse bool = false
)

// OceanStorQuota defines interfaces for quota operations
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

// CreateQuota creates quota by params
func (cli *OceanstorClient) CreateQuota(ctx context.Context,
	params map[string]interface{}) (map[string]interface{}, error) {
	resp, err := cli.Post(ctx, "/FS_QUOTA", params)
	if err != nil {
		return nil, err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return nil, fmt.Errorf("create quota failed, params: %+v error: %v", params, resp.Error["description"])
	}

	return cli.getResponseDataMap(ctx, resp.Data)
}

// UpdateQuota updates quota by id
func (cli *OceanstorClient) UpdateQuota(ctx context.Context, quotaID string, params map[string]interface{}) error {
	resp, err := cli.Put(ctx, fmt.Sprintf("/FS_QUOTA/%v", quotaID), params)
	if err != nil {
		return err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return fmt.Errorf("update quota failed, params: %+v error: %v", params, resp.Error["description"])
	}

	return nil
}

// GetQuota gets quota info by id
func (cli *OceanstorClient) GetQuota(ctx context.Context,
	quotaID, vStoreID string, spaceUnitType uint32) (map[string]interface{}, error) {
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

// BatchGetQuota gets quotas filtered by params
func (cli *OceanstorClient) BatchGetQuota(ctx context.Context, params map[string]interface{}) ([]interface{}, error) {
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

// DeleteQuota deletes quota by id
func (cli *OceanstorClient) DeleteQuota(ctx context.Context, quotaID, vStoreID string, forceFlag bool) error {
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
