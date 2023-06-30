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
	"time"

	"huawei-csi-driver/utils/log"
)

const (
	smartQosAlreadyExist int64 = 1077948993
)

type Qos interface {
	// GetQosByName used for get qos by name
	GetQosByName(ctx context.Context, name, vStoreID string) (map[string]interface{}, error)
	// GetQosByID used for get qos by id
	GetQosByID(ctx context.Context, qosID, vStoreID string) (map[string]interface{}, error)
	// DeleteQos used for delete qos
	DeleteQos(ctx context.Context, qosID, vStoreID string) error
	// CreateQos used for create qos
	CreateQos(ctx context.Context, name, objID, objType, vStoreID string, params map[string]int) (map[string]interface{}, error)
	// UpdateQos used for update qos
	UpdateQos(ctx context.Context, qosID, vStoreID string, params map[string]interface{}) error
	// ActivateQos used for active qos
	ActivateQos(ctx context.Context, qosID, vStoreID string) error
	// DeactivateQos used for deactivate qos
	DeactivateQos(ctx context.Context, qosID, vStoreID string) error
}

// CreateQos used for create qos
func (cli *BaseClient) CreateQos(ctx context.Context, name, objID, objType, vStoreID string, params map[string]int) (
	map[string]interface{}, error) {

	utcTime, err := cli.getSystemUTCTime(ctx)
	if err != nil {
		return nil, err
	}

	days := time.Unix(utcTime, 0).Format("2006-01-02")
	utcZeroTime, err := time.ParseInLocation("2006-01-02", days, time.UTC)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"NAME":              name,
		"SCHEDULEPOLICY":    1,
		"SCHEDULESTARTTIME": utcZeroTime.Unix(),
		"STARTTIME":         "00:00",
		"DURATION":          86400,
	}

	if objType == "fs" {
		data["FSLIST"] = []string{objID}
	} else {
		data["LUNLIST"] = []string{objID}
	}

	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	for k, v := range params {
		data[k] = v
	}

	resp, err := cli.Post(ctx, "/ioclass", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == smartQosAlreadyExist {
		log.AddContext(ctx).Warningf("The QoS %s is already exist.", name)
		return cli.GetQosByName(ctx, name, vStoreID)
	} else if code != 0 {
		return nil, fmt.Errorf("Create qos %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

// ActivateQos used for active qos
func (cli *BaseClient) ActivateQos(ctx context.Context, qosID, vStoreID string) error {
	data := map[string]interface{}{
		"ID":           qosID,
		"ENABLESTATUS": "true",
	}

	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Put(ctx, "/ioclass/active", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Activate qos %s error: %d", qosID, code)
	}

	return nil
}

// DeactivateQos used for deactivate qos
func (cli *BaseClient) DeactivateQos(ctx context.Context, qosID, vStoreID string) error {
	data := map[string]interface{}{
		"ID":           qosID,
		"ENABLESTATUS": "false",
	}

	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Put(ctx, "/ioclass/active", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Deactivate qos %s error: %d", qosID, code)
	}

	return nil
}

// DeleteQos used for delete qos
func (cli *BaseClient) DeleteQos(ctx context.Context, qosID, vStoreID string) error {
	url := fmt.Sprintf("/ioclass/%s", qosID)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Delete(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Delete qos %s error: %d", qosID, code)
	}

	return nil
}

// GetQosByName used for get qos by name
func (cli *BaseClient) GetQosByName(ctx context.Context, name, vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/ioclass?filter=NAME::%s", name)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Get(ctx, url, data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get qos by name %s error: %d", name, code)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		return nil, nil
	}

	qos := respData[0].(map[string]interface{})
	return qos, nil
}

// GetQosByID used for get qos by id
func (cli *BaseClient) GetQosByID(ctx context.Context, qosID, vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/ioclass/%s", qosID)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}
	resp, err := cli.Get(ctx, url, data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get qos by ID %s error: %d", qosID, code)
	}

	qos := resp.Data.(map[string]interface{})
	return qos, nil
}

// UpdateQos used for update qos
func (cli *BaseClient) UpdateQos(ctx context.Context, qosID, vStoreID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/ioclass/%s", qosID)
	if vStoreID != "" {
		params["vstoreId"] = vStoreID
	}
	resp, err := cli.Put(ctx, url, params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Update qos %s to %v error: %d", qosID, params, code)
	}

	return nil
}
