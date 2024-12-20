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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	snapshotNotExist int64 = 50150006
)

// CreateSnapshot creates volume snapshot
func (cli *RestClient) CreateSnapshot(ctx context.Context, snapshotName, volName string) error {
	data := map[string]interface{}{
		"volName":      volName,
		"snapshotName": snapshotName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/snapshot/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Create snapshot %s of volume %s error: %d", snapshotName, volName, result)
	}

	return nil
}

// DeleteSnapshot deletes volume snapshot
func (cli *RestClient) DeleteSnapshot(ctx context.Context, snapshotName string) error {
	data := map[string]interface{}{
		"snapshotName": snapshotName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/snapshot/delete", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Delete snapshot %s error: %d", snapshotName, result)
	}

	return nil
}

// GetSnapshotByName get snapshot by name
func (cli *RestClient) GetSnapshotByName(ctx context.Context, snapshotName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/snapshot/queryByName?snapshotName=%s", snapshotName)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(float64)
		if int64(errorCode) == snapshotNotExist {
			log.AddContext(ctx).Warningf("Snapshot of name %s doesn't exist", snapshotName)
			return nil, nil
		}

		return nil, fmt.Errorf("get snapshot by name %s error: %d", snapshotName, result)
	}

	snapshot, ok := resp["snapshot"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	return snapshot, nil
}

// CreateVolumeFromSnapshot creates volume from snapshot
func (cli *RestClient) CreateVolumeFromSnapshot(ctx context.Context,
	volName string,
	volSize int64,
	snapshotName string) error {
	data := map[string]interface{}{
		"volName": volName,
		"volSize": volSize,
		"src":     snapshotName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/snapshot/volume/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Create volume %s from snapshot %s error: %d", volName, snapshotName, result)
	}

	return nil
}
