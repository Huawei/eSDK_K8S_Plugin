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

	"huawei-csi-driver/utils/log"
)

const (
	clonePairNotExist int64 = 1073798147
)

type Clone interface {
	// DeleteClonePair used for delete clone pair
	DeleteClonePair(ctx context.Context, clonePairID string) error
	// GetClonePairInfo used for get clone pair info
	GetClonePairInfo(ctx context.Context, clonePairID string) (map[string]interface{}, error)
	// CreateClonePair used for create clone pair
	CreateClonePair(ctx context.Context, srcLunID, dstLunID string, cloneSpeed int) (map[string]interface{}, error)
	// SyncClonePair used for synchronize clone pair
	SyncClonePair(ctx context.Context, clonePairID string) error
	// StopCloneFSSplit used for stop clone split
	StopCloneFSSplit(ctx context.Context, fsID string) error
	// SplitCloneFS used to split clone
	SplitCloneFS(ctx context.Context, fsID, vStoreId string, splitSpeed int, isDeleteParentSnapshot bool) error
	// CloneFileSystem used for clone file system
	CloneFileSystem(ctx context.Context, name string, allocType int, parentID, parentSnapshotID string) (
		map[string]interface{}, error)
}

// DeleteClonePair used for delete clone pair
func (cli *BaseClient) DeleteClonePair(ctx context.Context, clonePairID string) error {
	data := map[string]interface{}{
		"ID":             clonePairID,
		"isDeleteDstLun": false,
	}

	resp, err := cli.Delete(ctx, "/clonepair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == clonePairNotExist {
		log.AddContext(ctx).Infof("ClonePair %s does not exist while deleting", clonePairID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete ClonePair %s error: %d", clonePairID, code)
	}

	return nil
}

// GetClonePairInfo used for get clone pair info
func (cli *BaseClient) GetClonePairInfo(ctx context.Context, clonePairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/clonepair?filter=ID::%s", clonePairID)

	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ClonePair info %s error: %d", clonePairID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("clonePair %s does not exist", clonePairID)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("clonePair %s does not exist", clonePairID)
		return nil, nil
	}

	clonePair, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("convert respData[0] to map[string]interface{} failed")
	}
	return clonePair, nil
}

// CreateClonePair used for create clone pair
func (cli *BaseClient) CreateClonePair(ctx context.Context,
	srcLunID, dstLunID string,
	cloneSpeed int) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"copyRate":          cloneSpeed,
		"sourceID":          srcLunID,
		"targetID":          dstLunID,
		"isNeedSynchronize": "0",
	}

	resp, err := cli.Post(ctx, "/clonepair/relation", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create ClonePair from %s to %s, error: %d", srcLunID, dstLunID, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to map[string]interface{} failed")
	}
	return respData, nil
}

// SyncClonePair used for synchronize clone pair
func (cli *BaseClient) SyncClonePair(ctx context.Context, clonePairID string) error {
	data := map[string]interface{}{
		"ID":         clonePairID,
		"copyAction": 0,
	}

	resp, err := cli.Put(ctx, "/clonepair/synchronize", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync ClonePair %s error: %d", clonePairID, code)
	}

	return nil
}

// StopCloneFSSplit used for stop clone split
func (cli *BaseClient) StopCloneFSSplit(ctx context.Context, fsID string) error {
	data := map[string]interface{}{
		"ID":          fsID,
		"SPLITENABLE": false,
	}

	resp, err := cli.Put(ctx, "/filesystem_split_switch", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop FS %s splitting error: %d", fsID, code)
	}

	return nil
}

// SplitCloneFS used to split clone
func (cli *BaseClient) SplitCloneFS(ctx context.Context,
	fsID, vStoreId string,
	splitSpeed int,
	isDeleteParentSnapshot bool) error {
	data := map[string]interface{}{
		"ID":                     fsID,
		"SPLITENABLE":            true,
		"SPLITSPEED":             splitSpeed,
		"ISDELETEPARENTSNAPSHOT": isDeleteParentSnapshot,
	}

	resp, err := cli.Put(ctx, SplitCloneFileSystem, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Split FS %s error: %d", fsID, code)
	}

	return nil
}

// CloneFileSystem used for clone file system
func (cli *BaseClient) CloneFileSystem(ctx context.Context, name string, allocType int, parentID,
	parentSnapshotID string) (map[string]interface{}, error) {

	data := map[string]interface{}{
		"NAME":               name,
		"ALLOCTYPE":          allocType,
		"DESCRIPTION":        description,
		"PARENTFILESYSTEMID": parentID,
	}

	if parentSnapshotID != "" {
		data["PARENTSNAPSHOTID"] = parentSnapshotID
	}

	resp, err := cli.Post(ctx, "/filesystem", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Clone FS from %s error: %d", parentID, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to map[string]interface{} failed")
	}
	return respData, nil
}
