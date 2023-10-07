/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package plugin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/fusionstorage/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	CAPACITY_UNIT    int64 = 1024 * 1024
	fileCapacityUnit int64 = 1024
)

const (
	FusionStorageSan = iota
	FusionStorageNas
)

type FusionStoragePlugin struct {
	basePlugin
	cli *client.Client
}

func (p *FusionStoragePlugin) init(config map[string]interface{}, keepLogin bool) error {

	clientConfig, err := p.getNewClientConfig(context.Background(), config)
	if err != nil {
		return err
	}

	cli := client.NewClient(clientConfig)
	err = cli.Login(context.Background())
	if err != nil {
		return err
	}

	err = cli.SetAccountId(context.TODO())
	if err != nil {
		return pkgUtils.Errorln(context.TODO(), fmt.Sprintf("setAccountId failed, error: %v", err))
	}

	if !keepLogin {
		cli.Logout(context.Background())
	}

	p.cli = cli
	return nil
}

func (p *FusionStoragePlugin) getParams(name string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":        name,
		"description": parameters["description"].(string),
		"capacity":    utils.RoundUpSize(parameters["size"].(int64), CAPACITY_UNIT),
	}

	paramKeys := []string{
		"storagepool",
		"cloneFrom",
		"authClient",
		"storageQuota",
		"accountName",
		"allSquash",
		"rootSquash",
		"fsPermission",
		"snapshotDirectoryVisibility",
		"qos",
	}

	for _, key := range paramKeys {
		if v, exist := parameters[key].(string); exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	return params, nil
}

func (p *FusionStoragePlugin) UpdateBackendCapabilities() (map[string]interface{}, map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":  true,
		"SupportThick": false,
		"SupportQoS":   false,
	}

	return capabilities, nil, nil
}

func (p *FusionStoragePlugin) updatePoolCapabilities(poolNames []string, storageType int) (map[string]interface{}, error) {
	// To keep connection token alive
	p.cli.KeepAlive(context.Background())

	pools, err := p.cli.GetAllPools(context.Background())
	if err != nil {
		log.Errorf("Get fusionstorage pools error: %v", err)
		return nil, err
	}
	log.Debugf("Get pools: %v", pools)

	var accounts []string
	if storageType == FusionStorageNas {
		accounts, err = p.cli.GetAllAccounts(context.Background())
		if err != nil {
			log.Errorf("Get accounts error: %v", err)
			return nil, err
		}
	}

	capabilities := make(map[string]interface{})

	for _, name := range poolNames {
		if i, exist := pools[name]; exist {
			pool, ok := i.(map[string]interface{})
			if !ok {
				continue
			}

			totalCapacity := int64(pool["totalCapacity"].(float64))
			usedCapacity := int64(pool["usedCapacity"].(float64))
			freeCapacity := (totalCapacity - usedCapacity) * CAPACITY_UNIT

			capability := map[string]interface{}{"FreeCapacity": freeCapacity}
			if storageType == FusionStorageNas {
				capability["Accounts"] = accounts
			}
			capabilities[name] = capability
		}
	}

	return capabilities, nil
}

// SupportQoSParameters checks requested QoS parameters support by FusionStorage plugin
func (p *FusionStoragePlugin) SupportQoSParameters(ctx context.Context, qosConfig string) error {
	// do not verify qos parameter for FusionStorage
	return nil
}

// Logout is to logout the storage session
func (p *FusionStoragePlugin) Logout(ctx context.Context) {
	if p.cli != nil {
		p.cli.Logout(ctx)
	}
}

func (p *FusionStoragePlugin) getNewClientConfig(ctx context.Context, config map[string]interface{}) (*client.NewClientConfig, error) {
	newClientConfig := &client.NewClientConfig{}
	configUrls, exist := config["urls"].([]interface{})
	if !exist || len(configUrls) <= 0 {
		msg := fmt.Sprintf("Verify urls: [%v] failed. urls must be provided.", config["urls"])
		log.AddContext(ctx).Errorln(msg)
		return newClientConfig, errors.New(msg)
	}

	newClientConfig.Url, exist = configUrls[0].(string)
	if !exist {
		msg := fmt.Sprintf("Verify url: [%v] failed. convert url to string failed.", configUrls[0])
		log.AddContext(ctx).Errorln(msg)
		return newClientConfig, errors.New(msg)
	}

	newClientConfig.User, exist = config["user"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify User: [%v] failed. User must be provided.", config["user"])
		log.AddContext(ctx).Errorln(msg)
		return newClientConfig, errors.New(msg)
	}

	newClientConfig.SecretName, exist = config["secretName"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify SecretName: [%v] failed. SecretName must be provided.", config["secretName"])
		log.AddContext(ctx).Errorln(msg)
		return newClientConfig, errors.New(msg)
	}

	newClientConfig.SecretNamespace, exist = config["secretNamespace"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify SecretNamespace: [%v] failed. SecretNamespace must be provided.",
			config["SecretNamespace"])
		log.AddContext(ctx).Errorln(msg)
		return newClientConfig, errors.New(msg)
	}

	newClientConfig.BackendID, exist = config["backendID"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify backendID: [%v] failed. backendID must be provided.",
			config["backendID"])
		return newClientConfig, pkgUtils.Errorln(ctx, msg)
	}

	newClientConfig.AccountName, _ = config["accountName"].(string)
	newClientConfig.ParallelNum, _ = config["maxClientThreads"].(string)

	newClientConfig.UseCert, _ = config["useCert"].(bool)
	newClientConfig.CertSecretMeta, _ = config["certSecret"].(string)

	return newClientConfig, nil
}
