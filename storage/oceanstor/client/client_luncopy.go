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
	lunCopyNotExist int64 = 1077950183
)

// LunCopy defines interfaces for lun copy operations
type LunCopy interface {
	// GetLunCopyByID used for get lun copy by id
	GetLunCopyByID(ctx context.Context, lunCopyID string) (map[string]interface{}, error)
	// GetLunCopyByName used for get lun copy by name
	GetLunCopyByName(ctx context.Context, name string) (map[string]interface{}, error)
	// DeleteLunCopy used for delete lun copy by id
	DeleteLunCopy(ctx context.Context, lunCopyID string) error
	// CreateLunCopy used for create lun copy
	CreateLunCopy(ctx context.Context, name, srcLunID, dstLunID string, copySpeed int) (map[string]interface{}, error)
	// StartLunCopy used for start lun copy
	StartLunCopy(ctx context.Context, lunCopyID string) error
	// StopLunCopy used for stop lun copy
	StopLunCopy(ctx context.Context, lunCopyID string) error
}

// CreateLunCopy used for create lun copy
func (cli *BaseClient) CreateLunCopy(ctx context.Context, name, srcLunID, dstLunID string, copySpeed int) (
	map[string]interface{}, error) {

	data := map[string]interface{}{
		"NAME":      name,
		"COPYSPEED": copySpeed,
		"SOURCELUN": fmt.Sprintf("INVALID;%s;INVALID;INVALID;INVALID", srcLunID),
		"TARGETLUN": fmt.Sprintf("INVALID;%s;INVALID;INVALID;INVALID", dstLunID),
	}

	resp, err := cli.Post(ctx, "/LUNCOPY", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create luncopy from %s to %s error: %d", srcLunID, dstLunID, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// GetLunCopyByID used for get lun copy by id
func (cli *BaseClient) GetLunCopyByID(ctx context.Context, lunCopyID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/LUNCOPY/%s", lunCopyID)

	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get luncopy %s error: %d", lunCopyID, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// GetLunCopyByName used for get lun copy by name
func (cli *BaseClient) GetLunCopyByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/LUNCOPY?filter=NAME::%s", name)

	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get luncopy by name %s error: %d", name, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Luncopy %s does not exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Luncopy %s does not exist", name)
		return nil, nil
	}

	lunCopy, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert lunCopy to map failed, data: %v", respData[0])
	}
	return lunCopy, nil
}

// StartLunCopy used for start lun copy
func (cli *BaseClient) StartLunCopy(ctx context.Context, lunCopyID string) error {
	data := map[string]interface{}{
		"ID": lunCopyID,
	}

	resp, err := cli.Put(ctx, "/LUNCOPY/start", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Start luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}

// StopLunCopy used for stop lun copy
func (cli *BaseClient) StopLunCopy(ctx context.Context, lunCopyID string) error {
	data := map[string]interface{}{
		"ID": lunCopyID,
	}

	resp, err := cli.Put(ctx, "/LUNCOPY/stop", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}

// DeleteLunCopy used for delete lun copy by id
func (cli *BaseClient) DeleteLunCopy(ctx context.Context, lunCopyID string) error {
	url := fmt.Sprintf("/LUNCOPY/%s", lunCopyID)

	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == lunCopyNotExist {
		log.AddContext(ctx).Infof("Luncopy %s does not exist while deleting", lunCopyID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}
