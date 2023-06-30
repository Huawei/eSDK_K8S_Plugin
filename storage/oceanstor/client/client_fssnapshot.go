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
	"strconv"

	"huawei-csi-driver/utils/log"
)

const (
	fsSnapshotNotExist       int64 = 1073754118
	snapshotParentNotExistV3 int64 = 1073754117
	snapshotParentNotExistV6 int64 = 1073754136
)

type FSSnapshot interface {
	// DeleteFSSnapshot used for delete file system snapshot by id
	DeleteFSSnapshot(ctx context.Context, snapshotID string) error
	// CreateFSSnapshot used for create file system snapshot
	CreateFSSnapshot(ctx context.Context, name, parentID string) (map[string]interface{}, error)
	// GetFSSnapshotByName used for get file system snapshot by snapshot name
	GetFSSnapshotByName(ctx context.Context, parentID, snapshotName string) (map[string]interface{}, error)
	// GetFSSnapshotCountByParentId used for get file system snapshot count by parent id
	GetFSSnapshotCountByParentId(ctx context.Context, ParentId string) (int, error)
}

// DeleteFSSnapshot used for delete file system snapshot by id
func (cli *BaseClient) DeleteFSSnapshot(ctx context.Context, snapshotID string) error {
	url := fmt.Sprintf("/FSSNAPSHOT/%s", snapshotID)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == fsSnapshotNotExist {
		log.AddContext(ctx).Infof("FS Snapshot %s does not exist while deleting", snapshotID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete FS snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

// GetFSSnapshotByName used for get file system snapshot by snapshot name
func (cli *BaseClient) GetFSSnapshotByName(ctx context.Context, parentID, snapshotName string) (map[string]interface{},
	error) {

	url := fmt.Sprintf("/FSSNAPSHOT?PARENTID=%s&filter=NAME::%s", parentID, snapshotName)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		if code == snapshotParentNotExistV3 || code == snapshotParentNotExistV6 {
			log.AddContext(ctx).Infof("The parent filesystem %s of snapshot %s does not exist",
				parentID, snapshotName)
			return nil, nil
		}

		return nil, fmt.Errorf("failed to Get filesystem snapshot %s, error is %d", snapshotName, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Filesystem snapshot %s does not exist", snapshotName)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		return nil, nil
	}

	snapshot := respData[0].(map[string]interface{})
	return snapshot, nil
}

// GetFSSnapshotCountByParentId used for get file system snapshot count by parent id
func (cli *BaseClient) GetFSSnapshotCountByParentId(ctx context.Context, ParentId string) (int, error) {
	url := fmt.Sprintf("/FSSNAPSHOT/count?PARENTID=%s", ParentId)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("failed to Get snapshot count of filesystem %s, error is %d", ParentId, code)
		return 0, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	countStr := respData["COUNT"].(string)
	count, _ := strconv.Atoi(countStr)
	return count, nil
}

// CreateFSSnapshot used for create file system snapshot
func (cli *BaseClient) CreateFSSnapshot(ctx context.Context, name, parentID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        name,
		"DESCRIPTION": description,
		"PARENTID":    parentID,
		"PARENTTYPE":  "40",
	}

	resp, err := cli.Post(ctx, "/FSSNAPSHOT", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create snapshot %s for FS %s error: %d", name, parentID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}
