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
	hostnameAlreadyExist int64 = 50157019
)

// GetHostByName used to get host by name
func (cli *RestClient) GetHostByName(ctx context.Context, hostName string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"hostName": hostName,
	}

	resp, err := cli.get(ctx, "/dsware/service/iscsi/queryAllHost", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Get host of name %s error: %d", hostName, result)
	}

	hostList, exist := resp["hostList"].([]interface{})
	if !exist {
		log.AddContext(ctx).Infof("Host %s does not exist", hostName)
		return nil, nil
	}

	for _, i := range hostList {
		host, ok := i.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The host %v's format is not map[string]interface{}", i)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}
		if host["hostName"] == hostName {
			return host, nil
		}
	}

	return nil, nil
}

// CreateHost used to create host
func (cli *RestClient) CreateHost(ctx context.Context,
	hostName string,
	alua map[string]interface{}) error {
	data := map[string]interface{}{
		"hostName": hostName,
	}

	if switchoverMode, ok := alua["switchoverMode"]; ok {
		data["switchoverMode"] = switchoverMode
	}

	if pathType, ok := alua["pathType"]; ok {
		data["pathType"] = pathType
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/createHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(ctx, resp, hostnameAlreadyExist) {
			return fmt.Errorf("Create host %s error", hostName)
		}
	}

	return nil
}

// UpdateHost used to update host
func (cli *RestClient) UpdateHost(ctx context.Context, hostName string, alua map[string]interface{}) error {
	data := map[string]interface{}{
		"hostName": hostName,
	}

	if switchoverMode, ok := alua["switchoverMode"]; ok {
		data["switchoverMode"] = switchoverMode
	}

	if pathType, ok := alua["pathType"]; ok {
		data["pathType"] = pathType
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/modifyHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("update host %s by %v error", hostName, data)
	}

	return nil
}

// QueryHostByPort used query host by port
func (cli *RestClient) QueryHostByPort(ctx context.Context, port string) (string, error) {
	data := map[string]interface{}{
		"portName": []string{port},
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/queryHostByPort", data)
	if err != nil {
		return "", err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(ctx, resp, initiatorNotExist) {
			return "", fmt.Errorf("Get host initiator %s belongs error", port)
		}

		log.AddContext(ctx).Infof("Initiator %s does not belong to any host", port)
		return "", nil
	}

	portHostMap, exist := resp["portHostMap"].(map[string]interface{})
	if !exist {
		log.AddContext(ctx).Infof("Initiator %s does not belong to any host", port)
		return "", nil
	}

	hosts, exist := portHostMap[port].([]interface{})
	if !exist || len(hosts) == 0 {
		log.AddContext(ctx).Infof("Initiator %s does not belong to any host", port)
		return "", nil
	}

	return hosts[0].(string), nil
}

// AddPortToHost used add port to host
func (cli *RestClient) AddPortToHost(ctx context.Context, initiatorName, hostName string) error {
	data := map[string]interface{}{
		"hostName":  hostName,
		"portNames": []string{initiatorName},
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/addPortToHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(ctx, resp, initiatorAddedToHost) {
			return fmt.Errorf("Add initiator %s to host %s error", initiatorName, hostName)
		}
	}

	return nil
}

// AddLunToHost usd to add lun to host
func (cli *RestClient) AddLunToHost(ctx context.Context, lunName, hostName string) error {
	data := map[string]interface{}{
		"hostName": hostName,
		"lunNames": []string{lunName},
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/addLunsToHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Add lun %s to host %s error: %d", lunName, hostName, result)
	}

	return nil
}

// DeleteLunFromHost used to delete lun from host
func (cli *RestClient) DeleteLunFromHost(ctx context.Context, lunName, hostName string) error {
	data := map[string]interface{}{
		"hostName": hostName,
		"lunNames": []string{lunName},
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/deleteLunFromHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Delete lun %s from host %s error: %d", lunName, hostName, result)
	}

	return nil
}

// QueryHostOfVolume used to query host of volume
func (cli *RestClient) QueryHostOfVolume(ctx context.Context, lunName string) ([]map[string]interface{}, error) {
	data := map[string]interface{}{
		"lunName": lunName,
	}

	resp, err := cli.post(ctx, "/dsware/service/iscsi/queryHostFromVolume", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Query hosts which lun %s mapped error: %d", lunName, result)
	}

	var hostList []map[string]interface{}

	respData, exist := resp["hostList"].([]interface{})
	if exist {
		for _, i := range respData {
			hostList = append(hostList, i.(map[string]interface{}))
		}
	}

	return hostList, nil
}
