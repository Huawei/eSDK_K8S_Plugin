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

	"huawei-csi-driver/utils/log"
)

const (
	initiatorAlreadyExist int64 = 50155102
	initiatorAddedToHost  int64 = 50157021
	initiatorNotExist     int64 = 50155103
)

// GetInitiatorByName used to get initiator by name
func (cli *RestClient) GetInitiatorByName(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"portName": name,
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/queryPortInfo", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(ctx, resp, initiatorNotExist) {
			return nil, fmt.Errorf("Get initiator %s error", name)
		}

		log.AddContext(ctx).Infof("Initiator %s does not exist", name)
		return nil, nil
	}

	portList, exist := resp["portList"].([]interface{})
	if !exist || len(portList) == 0 {
		log.AddContext(ctx).Infof("Initiator %s does not exist", name)
		return nil, nil
	}

	return portList[0].(map[string]interface{}), nil
}

// CreateInitiator used to create initiator by name
func (cli *RestClient) CreateInitiator(ctx context.Context, name string) error {
	data := map[string]interface{}{
		"portName": name,
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/createPort", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(ctx, resp, initiatorAlreadyExist) {
			return fmt.Errorf("Create initiator %s error", name)
		}
	}

	return nil
}

// QueryIscsiPortal used to query iscsi portal
func (cli *RestClient) QueryIscsiPortal(ctx context.Context) ([]map[string]interface{}, error) {
	data := make(map[string]interface{})
	resp, err := cli.post(ctx, "/dsware/service/cluster/dswareclient/queryIscsiPortal", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Query iscsi portal error: %d", result)
	}

	var nodeResultList []map[string]interface{}

	respData, exist := resp["nodeResultList"].([]interface{})
	if exist {
		for _, i := range respData {
			nodeResultList = append(nodeResultList, i.(map[string]interface{}))
		}
	}

	return nodeResultList, nil
}
