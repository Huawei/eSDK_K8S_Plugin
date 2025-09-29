/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.
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

// Package base provide base operations for oceanstor base storage
package base

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/api"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	smartQosAlreadyExist int64 = 1077948993
	queryCountPerBatch         = 100
)

// Qos defines interfaces for qos operations
type Qos interface {
	// GetQosByName used for get qos by name
	GetQosByName(ctx context.Context, name, vStoreID string) (map[string]interface{}, error)
	// GetQosByID used for get qos by id
	GetQosByID(ctx context.Context, qosID, vStoreID string) (map[string]interface{}, error)
	// DeleteQos used for delete qos
	DeleteQos(ctx context.Context, qosID, vStoreID string) error
	// CreateQos used for create qos
	CreateQos(ctx context.Context, args CreateQoSArgs) (map[string]any, error)
	// UpdateQos used for update qos
	UpdateQos(ctx context.Context, qosID, vStoreID string, params map[string]interface{}) error
	// ActivateQos used for active qos
	ActivateQos(ctx context.Context, qosID, vStoreID string) error
	// DeactivateQos used for deactivate qos
	DeactivateQos(ctx context.Context, qosID, vStoreID string) error
	// GetAllQos used for get all qos
	GetAllQos(ctx context.Context) ([]map[string]interface{}, error)
	// GetSystemUTCTime used to get system UTC time
	GetSystemUTCTime(ctx context.Context) (int64, error)
}

// QosClient defines client implements the Qos interface
type QosClient struct {
	RestClientInterface
}

// CreateQoSArgs is the arguments to create QoS
type CreateQoSArgs struct {
	Name     string
	ObjID    string
	ObjType  string
	VStoreID string
	Params   map[string]int
}

// CreateQos used for create qos
func (cli *QosClient) CreateQos(ctx context.Context, args CreateQoSArgs) (map[string]any, error) {

	utcTime, err := cli.GetSystemUTCTime(ctx)
	if err != nil {
		return nil, err
	}

	days := time.Unix(utcTime, 0).Format("2006-01-02")
	utcZeroTime, err := time.ParseInLocation("2006-01-02", days, time.UTC)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"NAME":              args.Name,
		"SCHEDULEPOLICY":    1,
		"SCHEDULESTARTTIME": utcZeroTime.Unix(),
		"STARTTIME":         "00:00",
		"DURATION":          86400,
	}

	if args.ObjType == "fs" {
		data["FSLIST"] = []string{args.ObjID}
	} else {
		data["LUNLIST"] = []string{args.ObjID}
	}

	if args.VStoreID != "" {
		data["vstoreId"] = args.VStoreID
	}

	for k, v := range args.Params {
		data[k] = v
	}

	resp, err := cli.Post(ctx, "/ioclass", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == smartQosAlreadyExist {
		log.AddContext(ctx).Warningf("The QoS %s is already exist.", args.Name)
		return cli.GetQosByName(ctx, args.Name, args.VStoreID)
	} else if code != 0 {
		return nil, fmt.Errorf("Create qos %v error: %d", data, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	return respData, nil
}

// ActivateQos used for active qos
func (cli *QosClient) ActivateQos(ctx context.Context, qosID, vStoreID string) error {
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
func (cli *QosClient) DeactivateQos(ctx context.Context, qosID, vStoreID string) error {
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
func (cli *QosClient) DeleteQos(ctx context.Context, qosID, vStoreID string) error {
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
func (cli *QosClient) GetQosByName(ctx context.Context, name, vStoreID string) (map[string]interface{}, error) {
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

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		return nil, nil
	}

	qos, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert qos to map failed, data: %v", respData[0])
	}
	return qos, nil
}

// GetQosByID used for get qos by id
func (cli *QosClient) GetQosByID(ctx context.Context, qosID, vStoreID string) (map[string]interface{}, error) {
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

	qos, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert qos to map failed, data: %v", resp.Data)
	}
	return qos, nil
}

// UpdateQos used for update qos
func (cli *QosClient) UpdateQos(ctx context.Context, qosID, vStoreID string, params map[string]interface{}) error {
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

// GetAllQos used for get all qos
func (cli *QosClient) GetAllQos(ctx context.Context) ([]map[string]interface{}, error) {
	return GetBatchObjs(ctx, cli.RestClientInterface, api.GetAllQos)
}

// GetSystemUTCTime used to get system UTC time
func (cli *QosClient) GetSystemUTCTime(ctx context.Context) (int64, error) {
	resp, err := cli.Get(ctx, "/system_utc_time", nil)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return 0, pkgUtils.Errorf(ctx, "get system UTC time error: %d", code)
	}

	if resp.Data == nil {
		return 0, pkgUtils.Errorf(ctx, "can not get the system UTC time")
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return 0, pkgUtils.Errorf(ctx, "the response data is not a map[string]interface{}")
	}

	utcTime, ok := respData["CMO_SYS_UTC_TIME"].(string)
	if !ok {
		return 0, pkgUtils.Errorf(ctx, "The CMO_SYS_UTC_TIME is not in respData %v", respData)
	}

	resTime, err := strconv.ParseInt(utcTime, constants.DefaultIntBase, constants.DefaultIntBitSize)
	if err != nil {
		return 0, err
	}
	return resTime, nil
}
