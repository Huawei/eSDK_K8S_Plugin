/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.
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

func (cli *Client) CreateQoS(ctx context.Context, qosName string, qosData map[string]int) error {
	data := map[string]interface{}{
		"qosName":     qosName,
		"qosSpecInfo": qosData,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("create QoS %v error: %s", data, errorCode)
	}

	return nil
}

func (cli *Client) DeleteQoS(ctx context.Context, qosName string) error {
	data := map[string]interface{}{
		"qosNames": []string{qosName},
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/delete", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("delete QoS %v error: %s", data, errorCode)
	}

	return nil
}

func (cli *Client) AssociateQoSWithVolume(ctx context.Context, volName, qosName string) error {
	data := map[string]interface{}{
		"keyNames": []string{volName},
		"qosName":  qosName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/volume/associate", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("associate QoS %s with volume %s error: %s", qosName, volName, errorCode)
	}

	return nil
}

func (cli *Client) DisassociateQoSWithVolume(ctx context.Context, volName, qosName string) error {
	data := map[string]interface{}{
		"keyNames": []string{volName},
		"qosName":  qosName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/volume/disassociate", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("disassociate QoS %s with volume %s error: %s", qosName, volName, errorCode)
	}

	return nil
}

func (cli *Client) GetQoSNameByVolume(ctx context.Context, volName string) (string, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/volume/qos?volName=%s", volName)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return "", err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return "", fmt.Errorf("get qos by volume %s error: %s", volName, errorCode)
	}

	qosName, exist := resp["qosName"].(string)
	if !exist {
		return "", nil
	}

	return qosName, nil
}

func (cli *Client) GetAssociateCountOfQoS(ctx context.Context, qosName string) (int, error) {
	storagePools, err := cli.getAllPools(ctx)
	if err != nil {
		return 0, err
	}
	if storagePools == nil {
		return 0, nil
	}

	associatePools, err := cli.getAssociatePoolOfQoS(ctx, qosName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get associate snapshot of QoS %s error: %v", qosName, err)
		return 0, err
	}
	pools, ok := associatePools["pools"].([]interface{})
	if !ok {
		msg := fmt.Sprintf("There is no pools info in response %v.", associatePools)
		log.AddContext(ctx).Errorln(msg)
		return 0, errors.New(msg)
	}
	storagePoolsCount := len(pools)

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The storage pool %v's format is not map[string]interface{}", p)
			log.AddContext(ctx).Errorln(msg)
			return 0, errors.New(msg)
		}
		poolId := int64(pool["poolId"].(float64))
		volumes, err := cli.getAssociateObjOfQoS(ctx, qosName, "volume", poolId)
		if err != nil {
			log.AddContext(ctx).Errorf("Get associate volume of QoS %s error: %v", qosName, err)
			return 0, err
		}

		snapshots, err := cli.getAssociateObjOfQoS(ctx, qosName, "snapshot", poolId)
		if err != nil {
			log.AddContext(ctx).Errorf("Get associate snapshot of QoS %s error: %v", qosName, err)
			return 0, err
		}

		volumeCount := int(volumes["totalNum"].(float64))
		snapshotCount := int(snapshots["totalNum"].(float64))
		totalCount := volumeCount + snapshotCount + storagePoolsCount
		if totalCount != 0 {
			return totalCount, nil
		}
	}

	return 0, nil
}

func (cli *Client) getAssociateObjOfQoS(ctx context.Context,
	qosName, objType string,
	poolId int64) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"qosName": qosName,
		"poolId":  poolId,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/volume/list?type=associated", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return nil, fmt.Errorf("get qos %s associate obj %s error: %s", qosName, objType, errorCode)
	}

	return resp, nil
}

func (cli *Client) getAssociatePoolOfQoS(ctx context.Context, qosName string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"qosName": qosName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/qos/storagePool/list?type=associated", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return nil, fmt.Errorf("get qos %s associate storagePool error: %s", qosName, errorCode)
	}

	return resp, nil
}
