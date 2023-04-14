/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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
	"strconv"
	"strings"

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/storage/oceanstor/clientv6"
	"huawei-csi-driver/storage/oceanstor/smartx"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	DORADO_V6_POOL_USAGE_TYPE = "0"
)

type OceanstorPlugin struct {
	basePlugin

	cli          client.BaseClientInterface
	product      string
	capabilities map[string]interface{}
}

func (p *OceanstorPlugin) init(config map[string]interface{}, keepLogin bool) error {
	configUrls, exist := config["urls"].([]interface{})
	if !exist || len(configUrls) <= 0 {
		return errors.New("urls must be provided")
	}

	var urls []string
	for _, i := range configUrls {
		urls = append(urls, i.(string))
	}

	user, exist := config["user"].(string)
	if !exist {
		return errors.New("user must be provided")
	}

	secretName, exist := config["secretName"].(string)
	if !exist {
		return errors.New("SecretName must be provided")
	}

	secretNamespace, exist := config["secretNamespace"].(string)
	if !exist {
		return errors.New("SecretNamespace must be provided")
	}

	backendID, exist := config["backendID"].(string)
	if !exist {
		return errors.New("backendID must be provided")
	}

	vstoreName, _ := config["vstoreName"].(string)
	parallelNum, _ := config["maxClientThreads"].(string)

	cli := client.NewClient(urls, user, secretName, secretNamespace, vstoreName, parallelNum, backendID)
	err := cli.Login(context.Background())
	if err != nil {
		return err
	}

	system, err := cli.GetSystem(context.Background())
	if err != nil {
		log.Errorf("Get system info error: %v", err)
		return err
	}

	p.product, err = utils.GetProductVersion(system)
	if err != nil {
		log.Errorf("Get product version error: %v", err)
		return err
	}

	if !keepLogin {
		cli.Logout(context.Background())
	}

	if p.product == utils.OceanStorDoradoV6 {
		clientV6 := clientv6.NewClientV6(urls, user, secretName, secretNamespace, vstoreName, parallelNum, backendID)
		cli.Logout(context.Background())
		err := p.switchClient(clientV6)
		if err != nil {
			return err
		}
	} else {
		p.cli = cli
	}

	return nil
}

func (p *OceanstorPlugin) updateBackendCapabilities() (map[string]interface{}, error) {
	features, err := p.cli.GetLicenseFeature(context.Background())
	if err != nil {
		log.Errorf("Get license feature error: %v", err)
		return nil, err
	}

	log.Debugf("Get license feature: %v", features)

	supportThin := utils.IsSupportFeature(features, "SmartThin")
	supportThick := p.product != "Dorado" && p.product != "DoradoV6"
	supportQoS := utils.IsSupportFeature(features, "SmartQoS")
	supportMetro := utils.IsSupportFeature(features, "HyperMetro")
	supportMetroNAS := utils.IsSupportFeature(features, "HyperMetroNAS")
	supportReplication := utils.IsSupportFeature(features, "HyperReplication")
	supportClone := utils.IsSupportFeature(features, "HyperClone") || utils.IsSupportFeature(features, "HyperCopy")
	supportApplicationType := p.product == "DoradoV6"

	capabilities := map[string]interface{}{
		"SupportThin":            supportThin,
		"SupportThick":           supportThick,
		"SupportQoS":             supportQoS,
		"SupportMetro":           supportMetro,
		"SupportReplication":     supportReplication,
		"SupportApplicationType": supportApplicationType,
		"SupportClone":           supportClone,
		"SupportMetroNAS":        supportMetroNAS,
	}

	return capabilities, nil
}

func (p *OceanstorPlugin) getRemoteDevices() (string, error) {
	devices, err := p.cli.GetAllRemoteDevices(context.Background())
	if err != nil {
		log.Errorf("Get remote devices error: %v", err)
		return "", err
	}

	var devicesSN []string
	for _, dev := range devices {
		deviceSN := dev["SN"].(string)
		devicesSN = append(devicesSN, deviceSN)
	}
	return strings.Join(devicesSN, ";"), nil
}

func (p *OceanstorPlugin) updateBackendSpecifications() (map[string]interface{}, error) {
	devicesSN, err := p.getRemoteDevices()
	if err != nil {
		log.Errorf("Get remote devices error: %v", err)
		return nil, err
	}

	specifications := map[string]interface{}{
		"LocalDeviceSN":   p.cli.GetDeviceSN(),
		"RemoteDevicesSN": devicesSN,
	}
	return specifications, nil
}

func (p *OceanstorPlugin) UpdateBackendCapabilities() (map[string]interface{}, map[string]interface{}, error) {
	capabilities, err := p.updateBackendCapabilities()
	if err != nil {
		return nil, nil, err
	}

	specifications, err := p.updateBackendSpecifications()
	if err != nil {
		return nil, nil, err
	}

	p.capabilities = capabilities
	return capabilities, specifications, nil
}

func (p *OceanstorPlugin) getParams(ctx context.Context, name string,
	parameters map[string]interface{}) map[string]interface{} {

	params := map[string]interface{}{
		"name":        name,
		"description": parameters["description"].(string),
		"capacity":    utils.RoundUpSize(parameters["size"].(int64), 512),
	}

	paramKeys := []string{
		"storagepool",
		"allocType",
		"qos",
		"authClient",
		"cloneFrom",
		"cloneSpeed",
		"metroDomain",
		"remoteStoragePool",
		"sourceSnapshotName",
		"sourceVolumeName",
		"snapshotParentId",
		"applicationType",
		"allSquash",
		"rootSquash",
		"fsPermission",
		"snapshotDirectoryVisibility",
		"reservedSnapshotSpaceRatio",
	}

	for _, key := range paramKeys {
		if v, exist := parameters[key]; exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	if v, exist := parameters["hyperMetro"].(string); exist && v != "" {
		params["hypermetro"] = utils.StrToBool(ctx, v)
	}

	// Add new bool parameter here
	for _, i := range []string{
		"replication",
	} {
		if v, exist := parameters[i].(string); exist && v != "" {
			params[i] = utils.StrToBool(ctx, v)
		}
	}

	// Add new string parameter here
	for _, i := range []string{
		"replicationSyncPeriod",
		"vStorePairID",
	} {
		if v, exist := parameters[i].(string); exist && v != "" {
			params[i] = v
		}
	}

	return params
}

func (p *OceanstorPlugin) updatePoolCapabilities(poolNames []string,
	usageType string) (map[string]interface{}, error) {
	pools, err := p.cli.GetAllPools(context.Background())
	if err != nil {
		log.Errorf("Get all pools error: %v", err)
		return nil, err
	}

	log.Debugf("Get pools: %v", pools)

	var validPools []map[string]interface{}
	for _, name := range poolNames {
		if pool, exist := pools[name].(map[string]interface{}); exist {
			poolType, exist := pool["NEWUSAGETYPE"].(string)
			if (pool["USAGETYPE"] == usageType || pool["USAGETYPE"] == DORADO_V6_POOL_USAGE_TYPE) ||
				(exist && poolType == DORADO_V6_POOL_USAGE_TYPE) {
				validPools = append(validPools, pool)
			} else {
				log.Warningf("Pool %s is not for %s", name, usageType)
			}
		} else {
			log.Warningf("Pool %s does not exist", name)
		}
	}

	capabilities := p.analyzePoolsCapacity(validPools)
	return capabilities, nil
}

func (p *OceanstorPlugin) analyzePoolsCapacity(pools []map[string]interface{}) map[string]interface{} {
	capabilities := make(map[string]interface{})

	for _, pool := range pools {
		name := pool["NAME"].(string)
		freeCapacity, _ := strconv.ParseInt(pool["USERFREECAPACITY"].(string), 10, 64)

		capabilities[name] = map[string]interface{}{
			"FreeCapacity": freeCapacity * 512,
		}
	}

	return capabilities
}

func (p *OceanstorPlugin) duplicateClient(ctx context.Context) (client.BaseClientInterface, error) {
	err := p.cli.Login(ctx)
	if err != nil {
		return nil, err
	}

	return p.cli, nil
}

// SupportQoSParameters checks requested QoS parameters support by Oceanstor plugin
func (p *OceanstorPlugin) SupportQoSParameters(ctx context.Context, qosConfig string) error {
	return smartx.CheckQoSParameterSupport(ctx, p.product, qosConfig)
}

// Logout is to logout the storage session
func (p *OceanstorPlugin) Logout(ctx context.Context) {
	if p.cli != nil {
		p.cli.Logout(ctx)
	}
}
func (p *OceanstorPlugin) switchClient(newClient client.BaseClientInterface) error {
	log.Infoln("Using OceanStor V6 or Dorado V6 BaseClient.")
	p.cli = newClient
	if err := p.cli.Login(context.Background()); err != nil {
		return err
	}

	_, err := p.cli.GetSystem(context.Background())
	if err != nil {
		log.Errorf("Get system info error: %v", err)
		return err
	}
	return nil
}

func (p *OceanstorPlugin) getNewClientConfig(ctx context.Context, param map[string]interface{}) (*client.NewClientConfig, error) {
	data := &client.NewClientConfig{}
	configUrls, exist := param["urls"].([]interface{})
	if !exist || len(configUrls) <= 0 {
		msg := fmt.Sprintf("Verify urls: [%v] failed. urls must be provided.", param["urls"])
		log.AddContext(ctx).Errorln(msg)
		return data, errors.New(msg)
	}
	for _, configUrl := range configUrls {
		url, ok := configUrl.(string)
		if !ok {
			msg := fmt.Sprintf("Verify url: [%v] failed. url convert to string failed.", configUrl)
			log.AddContext(ctx).Errorln(msg)
			return data, errors.New(msg)
		}
		data.Urls = append(data.Urls, url)
	}

	var urls []string
	for _, i := range configUrls {
		urls = append(urls, i.(string))
	}

	data.User, exist = param["user"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify user: [%v] failed. user must be provided.", data.User)
		log.AddContext(ctx).Errorln(msg)
		return data, errors.New(msg)
	}

	data.SecretName, exist = param["secretName"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify SecretName: [%v] failed. SecretName must be provided.", data.SecretName)
		log.AddContext(ctx).Errorln(msg)
		return data, errors.New(msg)
	}

	data.SecretNamespace, exist = param["secretNamespace"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify SecretNamespace: [%v] failed. SecretNamespace must be provided.",
			data.SecretNamespace)
		log.AddContext(ctx).Errorln(msg)
		return data, errors.New(msg)
	}

	data.BackendID, exist = param["backendID"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify backendID: [%v] failed. backendID must be provided.",
			param["backendID"])
		return data, pkgUtils.Errorln(ctx, msg)
	}

	data.VstoreName, _ = param["vstoreName"].(string)
	data.ParallelNum, _ = param["maxClientThreads"].(string)

	return data, nil
}
