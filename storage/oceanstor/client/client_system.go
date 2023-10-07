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

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

type System interface {
	// GetPoolByName used for get pool by name
	GetPoolByName(ctx context.Context, name string) (map[string]interface{}, error)
	// GetAllPools used for get all pools
	GetAllPools(ctx context.Context) (map[string]interface{}, error)
	// GetSystem used for get system info
	GetSystem(ctx context.Context) (map[string]interface{}, error)
	// GetLicenseFeature used for get license feature
	GetLicenseFeature(ctx context.Context) (map[string]int, error)
	// GetRemoteDeviceBySN used for get remote device by sn
	GetRemoteDeviceBySN(ctx context.Context, sn string) (map[string]interface{}, error)
	// GetAllRemoteDevices used for get all remote devices
	GetAllRemoteDevices(ctx context.Context) ([]map[string]interface{}, error)
	// GetDeviceSN used for get device sn
	GetDeviceSN() string
	// GetStorageVersion used for get storage version
	GetStorageVersion() string
}

// GetPoolByName used for get pool by name
func (cli *BaseClient) GetPoolByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/storagepool?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get pool %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Pool %s does not exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert resp.Data to arr failed, data: %v", resp.Data)
	}
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Pool %s does not exist", name)
		return nil, nil
	}

	pool, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData[0] to map failed, data: %v", respData[0])
	}
	return pool, nil
}

// GetAllPools used for get all pools
func (cli *BaseClient) GetAllPools(ctx context.Context) (map[string]interface{}, error) {
	resp, err := cli.Get(ctx, "/storagepool", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get all pools info error: %d", code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("There's no pools exist")
		return nil, nil
	}

	pools := make(map[string]interface{})

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert resp.Data to arr failed, data: %v", resp.Data)
	}
	for _, p := range respData {
		pool, ok := p.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf(fmt.Sprintf("convert pool to map failed, data: %v", p))
			continue
		}
		name, ok := pool["NAME"].(string)
		if !ok {
			log.AddContext(ctx).Warningf(fmt.Sprintf("convert name to map failed, data: %v", pool["NAME"]))
			continue
		}
		pools[name] = pool
	}

	return pools, nil
}

// GetLicenseFeature used for get license feature
func (cli *BaseClient) GetLicenseFeature(ctx context.Context) (map[string]int, error) {
	resp, err := cli.Get(ctx, "/license/feature", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get license feature error: %d", code)
		return nil, errors.New(msg)
	}

	result := map[string]int{}

	if resp.Data == nil {
		return result, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert resp.Data to arr failed, data: %v", resp.Data)
	}
	for _, i := range respData {
		feature, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf(fmt.Sprintf("convert feature to map failed, data: %v", i))
			continue
		}
		for k, v := range feature {
			result[k] = int(v.(float64))
		}
	}
	return result, nil
}

// GetSystem used for get system info
func (cli *BaseClient) GetSystem(ctx context.Context) (map[string]interface{}, error) {
	resp, err := cli.Get(ctx, "/system/", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get system info error: %d", code)
		return nil, errors.New(msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}
	cli.setStorageVersion(respData)
	return respData, nil
}

// GetRemoteDeviceBySN used for get remote device by sn
func (cli *BaseClient) GetRemoteDeviceBySN(ctx context.Context, sn string) (map[string]interface{}, error) {
	resp, err := cli.Get(ctx, "/remote_device", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get remote device %s error: %d", sn, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Remote device %s does not exist", sn)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert resp.Data to arr failed, data: %v", resp.Data)
	}
	for _, i := range respData {
		device, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert device to map failed, data: %v", i)
			continue
		}
		if device["SN"] == sn {
			return device, nil
		}
	}

	return nil, nil
}

// GetAllRemoteDevices used for get all remote devices
func (cli *BaseClient) GetAllRemoteDevices(ctx context.Context) ([]map[string]interface{}, error) {
	return cli.getBatchObjs(ctx, "/remote_device", true)
}

// GetDeviceSN used for get device sn
func (cli *BaseClient) GetDeviceSN() string {
	return cli.DeviceId
}

func (cli *BaseClient) setStorageVersion(systemInfo map[string]interface{}) {
	storagePointVersion, ok := systemInfo["pointRelease"].(string)
	if ok {
		cli.StorageVersion = storagePointVersion
	}
}

// GetStorageVersion used for get storage version
func (cli *BaseClient) GetStorageVersion() string {
	return cli.StorageVersion
}
