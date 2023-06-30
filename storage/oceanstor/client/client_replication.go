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
	"strconv"

	"huawei-csi-driver/utils/log"
)

const (
	replicationNotExist int64 = 1077937923
)

type Replication interface {
	// GetReplicationPairByResID used for get replication
	GetReplicationPairByResID(ctx context.Context, resID string, resType int) ([]map[string]interface{}, error)
	// GetReplicationPairByID used for get replication pair by pair id
	GetReplicationPairByID(ctx context.Context, pairID string) (map[string]interface{}, error)
	// GetReplicationvStorePairCount used for get replication vstore pair count
	GetReplicationvStorePairCount(ctx context.Context) (int64, error)
	// GetReplicationvStorePairRange used for get replication vstore pair range
	GetReplicationvStorePairRange(ctx context.Context, startRange, endRange int64) ([]interface{}, error)
	// GetReplicationvStorePairByvStore used for get replication vstore pair by vstore id
	GetReplicationvStorePairByvStore(ctx context.Context, vStoreID string) (map[string]interface{}, error)
	// DeleteReplicationPair used for delete replication pair by pair id
	DeleteReplicationPair(ctx context.Context, pairID string) error
	// CreateReplicationPair used for create replication pair
	CreateReplicationPair(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error)
	// SyncReplicationPair used for synchronize replication pair
	SyncReplicationPair(ctx context.Context, pairID string) error
	// SplitReplicationPair used for split replication pair by pair id
	SplitReplicationPair(ctx context.Context, pairID string) error
}

// CreateReplicationPair used for create replication pair
func (cli *BaseClient) CreateReplicationPair(ctx context.Context, data map[string]interface{}) (
	map[string]interface{}, error) {

	resp, err := cli.Post(ctx, "/REPLICATIONPAIR", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create replication %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

// SplitReplicationPair used for split replication pair by pair id
func (cli *BaseClient) SplitReplicationPair(ctx context.Context, pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.Put(ctx, "/REPLICATIONPAIR/split", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Split replication pair %s error: %d", pairID, code)
	}

	return nil
}

// SyncReplicationPair used for synchronize replication pair
func (cli *BaseClient) SyncReplicationPair(ctx context.Context, pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.Put(ctx, "/REPLICATIONPAIR/sync", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync replication pair %s error: %d", pairID, code)
	}

	return nil
}

// DeleteReplicationPair used for delete replication pair by pair id
func (cli *BaseClient) DeleteReplicationPair(ctx context.Context, pairID string) error {
	url := fmt.Sprintf("/REPLICATIONPAIR/%s", pairID)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == replicationNotExist {
		log.AddContext(ctx).Infof("Replication pair %s does not exist while deleting", pairID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete replication pair %s error: %d", pairID, code)
	}

	return nil
}

// GetReplicationPairByResID used for get replication
func (cli *BaseClient) GetReplicationPairByResID(ctx context.Context, resID string, resType int) (
	[]map[string]interface{}, error) {

	url := fmt.Sprintf("/REPLICATIONPAIR/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", resType, resID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication pairs resource %s associated error: %d", resID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Replication pairs resource %s associated does not exist", resID)
		return nil, nil
	}

	var pairs []map[string]interface{}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		pairs = append(pairs, i.(map[string]interface{}))
	}

	return pairs, nil
}

// GetReplicationPairByID used for get replication pair by pair id
func (cli *BaseClient) GetReplicationPairByID(ctx context.Context, pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/REPLICATIONPAIR/%s", pairID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication pair %s error: %d", pairID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

// GetReplicationvStorePairCount used for get replication vstore pair count
func (cli *BaseClient) GetReplicationvStorePairCount(ctx context.Context) (int64, error) {
	resp, err := cli.Get(ctx, "/replication_vstorepair/count", nil)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return 0, fmt.Errorf("Get replication vstore pair count error: %d", code)
	}

	respData := resp.Data.(map[string]interface{})
	countStr := respData["COUNT"].(string)
	count, _ := strconv.ParseInt(countStr, 10, 64)

	return count, nil
}

// GetReplicationvStorePairRange used for get replication vstore pair range
func (cli *BaseClient) GetReplicationvStorePairRange(ctx context.Context, startRange, endRange int64) (
	[]interface{}, error) {

	url := fmt.Sprintf("/replication_vstorepair?range=[%d-%d]", startRange, endRange)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication vstore pairs error: %d", code)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

// GetReplicationvStorePairByvStore used for get replication vstore pair by vstore id
func (cli *BaseClient) GetReplicationvStorePairByvStore(ctx context.Context,
	vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/replication_vstorepair/associate?ASSOCIATEOBJTYPE=16442&ASSOCIATEOBJID=%s", vStoreID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication vstore pair by vstore %s error: %d", vStoreID, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("Replication vstore pair of vstore %s does not exist", vStoreID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("Replication vstore pair of vstore %s does not exist", vStoreID)
		return nil, nil
	}

	pair := respData[0].(map[string]interface{})
	return pair, nil
}
