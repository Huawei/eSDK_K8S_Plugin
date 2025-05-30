/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2025. All rights reserved.
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

const (
	// ParentTypeFS defines fs type of parent
	ParentTypeFS int = 40

	// ParentTypeDTree defines DTree type of parent
	ParentTypeDTree int = 16445

	// SecurityStyleUnix defines unix style of security
	SecurityStyleUnix int = 3

	dtreeNotExist = 1077955336
)

// DTree defines interfaces for DTree operations
type DTree interface {
	// CreateDTree use for create a dTree
	CreateDTree(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
	// GetDTreeByName use for get dTree information
	GetDTreeByName(ctx context.Context, parentID, parentName, vStoreID, name string) (map[string]interface{}, error)
	// DeleteDTreeByID use for delete a dTree
	DeleteDTreeByID(ctx context.Context, vStoreID, dTreeID string) error
	// DeleteDTreeByName use for delete a dTree by name
	DeleteDTreeByName(ctx context.Context, parentName, dTreeName, vStoreID string) error
}

// CreateDTree use for create a dTree
func (cli *OceanstorClient) CreateDTree(ctx context.Context,
	params map[string]interface{}) (map[string]interface{}, error) {
	resp, err := cli.Post(ctx, "/QUOTATREE", params)
	if err != nil {
		return nil, err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return nil, fmt.Errorf("create dtree failed,data: %+v error: %s", params, resp.Error["description"])
	}

	return cli.getResponseDataMap(ctx, resp.Data)
}

// GetDTreeByName use for get dTree information
func (cli *OceanstorClient) GetDTreeByName(ctx context.Context,
	parentID, parentName, vStoreID, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/QUOTATREE?PARENTNAME=%s&NAME=%s&vstoreId=%s", parentName, name, vStoreID)

	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	if err = resp.AssertErrorWithTolerations(ctx, base.ResponseToleration{
		Code:   dtreeNotExist,
		Reason: fmt.Sprintf("dtree %q of %q not exist", name, parentName),
	}); err != nil {
		return nil, fmt.Errorf("failed to get dtree %q: %w", name, err)
	}

	return cli.getResponseDataMap(ctx, resp.Data)
}

// DeleteDTreeByID use for delete a dTree
func (cli *OceanstorClient) DeleteDTreeByID(ctx context.Context, vStoreID, dTreeID string) error {
	url := fmt.Sprintf("/QUOTATREE")
	resp, err := cli.Delete(ctx, url, map[string]interface{}{
		"ID":       dTreeID,
		"vstoreId": vStoreID,
	})
	if err != nil {
		return err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return errors.New(fmt.Sprintf("%s", resp.Error["description"]))
	}

	return nil
}

// DeleteDTreeByName use for delete a dTree
func (cli *OceanstorClient) DeleteDTreeByName(ctx context.Context, parentName, dTreeName, vStoreID string) error {
	url := fmt.Sprintf("/QUOTATREE")
	resp, err := cli.Delete(ctx, url, map[string]interface{}{
		"PARENTNAME": parentName,
		"vstoreId":   vStoreID,
		"NAME":       dTreeName,
	})
	if err != nil {
		return err
	}

	if utils.ResCodeExist(resp.Error["code"]) {
		return errors.New(fmt.Sprintf("%s", resp.Error["description"]))
	}

	return nil
}
