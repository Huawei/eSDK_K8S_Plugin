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
	volumeNameNotExist   int64 = 50150005
	deleteVolumeNotExist int64 = 32150005
	queryVolumeNotExist  int64 = 31000000
)

func (cli *Client) CreateVolume(ctx context.Context, params map[string]interface{}) error {
	data := map[string]interface{}{
		"volName": params["name"].(string),
		"volSize": params["capacity"].(int64),
		"poolId":  params["poolId"].(int64),
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/volume/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("Create volume %v error: %s", data, errorCode)
	}

	return nil
}

func (cli *Client) GetVolumeByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/volume/queryByName?volName=%s", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(float64)
		if int64(errorCode) == volumeNameNotExist {
			log.AddContext(ctx).Warningf("Volume of name %s doesn't exist", name)
			return nil, nil
		}

		// Compatible with FusionStorage 6.3
		if int64(errorCode) == queryVolumeNotExist {
			log.AddContext(ctx).Warningf("Volume of name %s doesn't exist", name)
			return nil, nil
		}

		return nil, fmt.Errorf("Get volume by name %s error: %d", name, int64(errorCode))
	}

	lun, ok := resp["lunDetailInfo"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	return lun, nil
}

func (cli *Client) DeleteVolume(ctx context.Context, name string) error {
	data := map[string]interface{}{
		"volNames": []string{name},
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/volume/delete", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		details, ok := resp["detail"].([]interface{})
		if !ok || len(details) == 0 {
			msg := fmt.Sprintf("There is no detail info in response %v.", resp)
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		detail, ok := details[0].(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The format of detail info %v is not map[string]interface{}.", details)
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		floatCode, ok := detail["errorCode"].(float64)
		if !ok {
			msg := fmt.Sprintf("There is no error code in detail %v.", detail)
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		errorCode := int64(floatCode)
		if errorCode == volumeNameNotExist {
			log.AddContext(ctx).Warningf("Volume %s doesn't exist while deleting.", name)
			return nil
		}

		// Compatible with FusionStorage 6.3
		if errorCode == deleteVolumeNotExist {
			log.AddContext(ctx).Warningf("Volume %s doesn't exist while deleting.", name)
			return nil
		}

		return fmt.Errorf("Delete volume %s error: %d", name, errorCode)
	}

	return nil
}

func (cli *Client) AttachVolume(ctx context.Context, name, ip string) error {
	data := map[string]interface{}{
		"volName": []string{name},
		"ipList":  []string{ip},
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/volume/attach", data)
	if err != nil {
		return err
	}

	result, ok := resp[name].([]interface{})
	if !ok || len(result) == 0 {
		return fmt.Errorf("Attach volume %s to %s error", name, ip)
	}

	attachResult, ok := result[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("attach volume %s to %s error", name, ip)
	}

	errorCode, exist := attachResult["errorCode"].(string)
	if !exist || errorCode != "0" {
		return fmt.Errorf("Attach volume %s to %s error: %s", name, ip, errorCode)
	}

	return nil
}

func (cli *Client) DetachVolume(ctx context.Context, name, ip string) error {
	data := map[string]interface{}{
		"volName": []string{name},
		"ipList":  []string{ip},
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/volume/detach", data)
	if err != nil {
		return err
	}

	result, ok := resp["volumeInfo"].([]interface{})
	if !ok || len(result) == 0 {
		return fmt.Errorf("Detach volume %s from %s error", name, ip)
	}

	detachResult, ok := result[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("detach volume %s from %s error", name, ip)
	}

	errorCode, exist := detachResult["errorCode"].(string)
	if !exist || errorCode != "0" {
		return fmt.Errorf("Detach volume %s from %s error: %s", name, ip, errorCode)
	}

	return nil
}

func (cli *Client) ExtendVolume(ctx context.Context, lunName string, newCapacity int64) error {
	data := map[string]interface{}{
		"volName":    lunName,
		"newVolSize": newCapacity,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/volume/expand", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("extend volume capacity to %d error: %d", newCapacity, result)
	}

	return nil
}

func (cli *Client) GetHostLunId(ctx context.Context, hostName, lunName string) (string, error) {
	data := map[string]interface{}{
		"hostName": hostName,
	}

	resp, err := cli.post(ctx, "/dsware/service/v1.3/host/lun/list", data)
	if err != nil {
		return "", err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return "", fmt.Errorf("get hostLun of hostName %s error: %d", hostName, result)
	}

	hostLunList, exist := resp["hostLunList"].([]interface{})
	if !exist {
		log.AddContext(ctx).Infof("Host %s does not exist", hostName)
		return "", nil
	}

	for _, i := range hostLunList {
		hostLun, ok := i.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The hostlun %v's format is not map[string]interface{}", i)
			log.AddContext(ctx).Errorln(msg)
			return "", errors.New(msg)
		}
		if hostLun["lunName"].(string) == lunName {
			return strconv.FormatInt(int64(hostLun["lunId"].(float64)), 10), nil
		}
	}
	return "", nil
}
