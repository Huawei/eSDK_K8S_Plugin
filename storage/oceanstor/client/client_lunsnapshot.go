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

const (
	lunSnapshotNotExist  int64 = 1077937880
	snapshotNotActivated int64 = 1077937891
)

// LunSnapshot defines interfaces for lun snapshot operations
type LunSnapshot interface {
	// GetLunSnapshotByName used for get lun snapshot by name
	GetLunSnapshotByName(ctx context.Context, name string) (map[string]interface{}, error)
	// DeleteLunSnapshot used for delete lun snapshot
	DeleteLunSnapshot(ctx context.Context, snapshotID string) error
	// CreateLunSnapshot used for create lun snapshot
	CreateLunSnapshot(ctx context.Context, name, lunID string) (map[string]interface{}, error)
	// ActivateLunSnapshot used for activate lun snapshot
	ActivateLunSnapshot(ctx context.Context, snapshotID string) error
	// DeactivateLunSnapshot used for stop lun snapshot
	DeactivateLunSnapshot(ctx context.Context, snapshotID string) error
}

// CreateLunSnapshot used for create lun snapshot
func (cli *BaseClient) CreateLunSnapshot(ctx context.Context, name, lunID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        name,
		"DESCRIPTION": description,
		"PARENTID":    lunID,
	}

	resp, err := cli.Post(ctx, "/snapshot", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create snapshot %s for lun %s error: %d", name, lunID, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// GetLunSnapshotByName used for get lun snapshot by name
func (cli *BaseClient) GetLunSnapshotByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/snapshot?filter=NAME::%s&range=[0-100]", name)

	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get snapshot by name %s error: %d", name, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Snapshot %s does not exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		return nil, nil
	}

	snapshot, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert snapshot to map failed, data: %v", respData[0])
	}
	return snapshot, nil
}

// DeleteLunSnapshot used for delete lun snapshot
func (cli *BaseClient) DeleteLunSnapshot(ctx context.Context, snapshotID string) error {
	url := fmt.Sprintf("/snapshot/%s", snapshotID)
	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == lunSnapshotNotExist {
		log.AddContext(ctx).Infof("Lun snapshot %s does not exist while deleting", snapshotID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

// ActivateLunSnapshot used for activate lun snapshot
func (cli *BaseClient) ActivateLunSnapshot(ctx context.Context, snapshotID string) error {
	data := map[string]interface{}{
		"SNAPSHOTLIST": []string{snapshotID},
	}

	resp, err := cli.Post(ctx, "/snapshot/activate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Activate snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

// DeactivateLunSnapshot used for stop lun snapshot
func (cli *BaseClient) DeactivateLunSnapshot(ctx context.Context, snapshotID string) error {
	data := map[string]interface{}{
		"ID": snapshotID,
	}

	resp, err := cli.Put(ctx, "/snapshot/stop", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == snapshotNotActivated {
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Deactivate snapshot %s error: %d", snapshotID, code)
	}

	return nil
}
