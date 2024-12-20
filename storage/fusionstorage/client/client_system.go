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
	"strings"

	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// GetAccountIdByName gets account id by account name
func (cli *RestClient) GetAccountIdByName(ctx context.Context, accountName string) (string, error) {
	url := fmt.Sprintf("/dfv/service/obsPOE/query_accounts?name=%s", accountName)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return "", err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return "", fmt.Errorf("get account name by id error: %d", result)
	}

	respData, exist := resp["data"].(map[string]interface{})
	if !exist {
		return "", fmt.Errorf("get account name by id response data is empty")
	}
	accountId, exist := respData["id"].(string)
	if !exist || accountId == "" {
		return "", fmt.Errorf("get account name by id response data dose not have accountId")
	}

	return accountId, nil
}

// GetPoolByName gets pool by pool name
func (cli *RestClient) GetPoolByName(ctx context.Context, poolName string) (map[string]interface{}, error) {
	resp, err := cli.get(ctx, "/dsware/service/v1.3/storagePool", nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Get all pools error: %d", result)
	}

	storagePools, exist := resp["storagePools"].([]interface{})
	if !exist || len(storagePools) <= 0 {
		return nil, nil
	}

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The pool %v's format is not map[string]interface{}", p)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}

		if pool["poolName"].(string) == poolName {
			return pool, nil
		}
	}

	return nil, nil
}

// GetPoolById gets pool by pool id
func (cli *RestClient) GetPoolById(ctx context.Context, poolId int64) (map[string]interface{}, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/storagePool?poolId=%d", poolId)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("get pool by id %d error: %d", poolId, result)
	}

	storagePools, exist := resp["storagePools"].([]interface{})
	if !exist || len(storagePools) <= 0 {
		return nil, nil
	}

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The pool %v's format is not map[string]interface{}", p)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}

		if int64(pool["poolId"].(float64)) == poolId {
			return pool, nil
		}
	}

	return nil, nil
}

// GetAllAccounts gets all accounts
func (cli *RestClient) GetAllAccounts(ctx context.Context) ([]string, error) {
	resp, err := cli.get(ctx, "/dfv/service/obsPOE/accounts", nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("get all accounts error: %d", result)
	}

	respData, exist := resp["data"].([]interface{})
	if !exist {
		return nil, fmt.Errorf("get all accounts response data is empty")
	}

	var accounts []string
	for _, d := range respData {
		data, ok := d.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert responseData to map failed, data: %v", d)
			continue
		}
		accountName, exist := data["name"].(string)
		if !exist || accountName == "" {
			continue
		}
		accounts = append(accounts, accountName)
	}

	return accounts, nil
}

// GetAllPools gets all pools
func (cli *RestClient) GetAllPools(ctx context.Context) (map[string]interface{}, error) {
	resp, err := cli.get(ctx, "/dsware/service/v1.3/storagePool", nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Get all pools error: %d", result)
	}

	storagePools, exist := resp["storagePools"].([]interface{})
	if !exist || len(storagePools) <= 0 {
		return nil, nil
	}

	pools := make(map[string]interface{})

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The pool %v's format is not map[string]interface{}", p)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}
		name, ok := pool["poolName"].(string)
		if !ok {
			return nil, pkgUtils.Errorf(ctx, "convert poolName to string failed, data: %v", pool["poolName"])
		}
		pools[name] = pool
	}

	return pools, nil
}

func (cli *RestClient) getAllPools(ctx context.Context) ([]interface{}, error) {
	resp, err := cli.get(ctx, "/dsware/service/v1.3/storagePool", nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("get all pools error: %d", result)
	}

	storagePools, exist := resp["storagePools"].([]interface{})
	if !exist || len(storagePools) <= 0 {
		return nil, nil
	}
	return storagePools, nil
}

// GetNFSServiceSetting gets nfs service settings
func (cli *RestClient) GetNFSServiceSetting(ctx context.Context) (map[string]bool, error) {
	setting := map[string]bool{"SupportNFS41": false}

	req := make(map[string]interface{})
	if cli.accountName != "" {
		req["account_name"] = cli.accountName
	} else {
		req = nil
	}

	resp, err := cli.get(ctx, "/api/v2/nas_protocol/nfs_service_config", req)
	if err != nil {
		// Pacific 8.1.0/8.1.1 does not have this interface, ignore this error.
		if strings.Contains(err.Error(), "invalid character '<' looking for beginning of value") {
			log.AddContext(ctx).Debugln("Backend dose not have interface: /api/v2/nas_protocol/nfs_service_config")
			return setting, nil
		}

		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		return nil, utils.Errorf(ctx, "The format of NFS service setting result is incorrect.")
	}

	code := int64(result["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get NFS service setting failed. errorCode: %d", code)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return nil, utils.Errorf(ctx, "The format of NFS service setting data is incorrect.")
	}
	if data == nil {
		log.AddContext(ctx).Infoln("NFS service setting is empty.")
		return nil, nil
	}

	for k, v := range data {
		if k == "nfsv41_status" {
			setting["SupportNFS41"], ok = v.(bool)
			if !ok {
				log.AddContext(ctx).Warningf("convert map[SupportNFS41] to bool failed, data: %v", v)
			}
			break
		}
	}

	return setting, nil
}
