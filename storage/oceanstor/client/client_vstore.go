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

	"huawei-csi-driver/utils/log"
)

type VStore interface {
	// GetvStoreName used for get vstore name in *BaseClient
	GetvStoreName() string
	// GetvStoreByName used for get vstore info by vstore name
	GetvStoreByName(ctx context.Context, name string) (map[string]interface{}, error)
	// GetvStorePairByID used for get vstore pair by pair id
	GetvStorePairByID(ctx context.Context, pairID string) (map[string]interface{}, error)
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

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("vstore %s does not exist", name)
		return nil, nil
	}

	vstore := respData[0].(map[string]interface{})
	return vstore, nil
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

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("vstore pair %s does not exist", pairID)
		return nil, nil
	}

	pair := respData[0].(map[string]interface{})
	return pair, nil
}
