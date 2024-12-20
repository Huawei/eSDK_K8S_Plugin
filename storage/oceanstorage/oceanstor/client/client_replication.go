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

	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	replicationNotExist int64 = 1077937923
)

// Replication defines interfaces for replication operations
type Replication interface {
	// GetReplicationPairByResID used for get replication
	GetReplicationPairByResID(ctx context.Context, resID string, resType int) ([]map[string]interface{}, error)
	// GetReplicationPairByID used for get replication pair by pair id
	GetReplicationPairByID(ctx context.Context, pairID string) (map[string]interface{}, error)
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
func (cli *OceanstorClient) CreateReplicationPair(ctx context.Context, data map[string]interface{}) (
	map[string]interface{}, error) {

	resp, err := cli.Post(ctx, "/REPLICATIONPAIR", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create replication %v error: %d", data, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// SplitReplicationPair used for split replication pair by pair id
func (cli *OceanstorClient) SplitReplicationPair(ctx context.Context, pairID string) error {
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
func (cli *OceanstorClient) SyncReplicationPair(ctx context.Context, pairID string) error {
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
func (cli *OceanstorClient) DeleteReplicationPair(ctx context.Context, pairID string) error {
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
func (cli *OceanstorClient) GetReplicationPairByResID(ctx context.Context, resID string, resType int) (
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

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	for _, i := range respData {
		pairs = append(pairs, i.(map[string]interface{}))
	}

	return pairs, nil
}

// GetReplicationPairByID used for get replication pair by pair id
func (cli *OceanstorClient) GetReplicationPairByID(ctx context.Context, pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/REPLICATIONPAIR/%s", pairID)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication pair %s error: %d", pairID, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// GetReplicationvStorePairByvStore used for get replication vstore pair by vstore id
func (cli *OceanstorClient) GetReplicationvStorePairByvStore(ctx context.Context,
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

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("Replication vstore pair of vstore %s does not exist", vStoreID)
		return nil, nil
	}

	pair, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert pair to map failed, data: %v", respData[0])
	}
	return pair, nil
}
