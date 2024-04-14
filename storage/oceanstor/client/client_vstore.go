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
	"fmt"

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

// VStore defines interfaces for vstore operations
type VStore interface {
	// GetvStoreName used for get vstore name in *BaseClient
	GetvStoreName() string
	// GetvStoreByName used for get vstore info by vstore name
	GetvStoreByName(ctx context.Context, name string) (map[string]interface{}, error)
	// GetvStorePairByID used for get vstore pair by pair id
	GetvStorePairByID(ctx context.Context, pairID string) (map[string]interface{}, error)
	// GetVStorePairs used for vStore pairs
	GetVStorePairs(ctx context.Context) ([]interface{}, error)
}

// GetvStoreName used for get vstore name in *BaseClient
func (cli *BaseClient) GetvStoreName() string {
	return cli.VStoreName
}

// GetvStoreByName used for get vstore info by vstore name
func (cli *BaseClient) GetvStoreByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/vstore?filter=NAME::%s", name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get vstore %s error: %d", name, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("vstore %s does not exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("vstore %s does not exist", name)
		return nil, nil
	}

	vstore, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData[0] to map failed, data: %v", respData[0])
	}
	return vstore, nil
}

// GetVStorePairs used for get vStore pairs
func (cli *BaseClient) GetVStorePairs(ctx context.Context) ([]interface{}, error) {
	resp, err := cli.Get(ctx, "/vstore_pair?RETYPE=1", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get vstore pair by reType error: %d", code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Debugln("vstore pairs with reType does not exist")
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}

	return respData, nil
}

// GetvStorePairByID used for get vstore pair by pair id
func (cli *BaseClient) GetvStorePairByID(ctx context.Context, pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/vstore_pair?filter=ID::%s", pairID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get vstore pair by ID %s error: %d", pairID, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("vstore pair %s does not exist", pairID)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("vstore pair %s does not exist", pairID)
		return nil, nil
	}

	pair, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData[0] to map failed, data: %v", respData[0])
	}
	return pair, nil
}
