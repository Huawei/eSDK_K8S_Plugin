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
	"strconv"
	"strings"

	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/pkg/constants"
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

	vStoreId string

	cli          client.BaseClientInterface
	product      string
	capabilities map[string]interface{}
}

func (p *OceanstorPlugin) init(config map[string]interface{}, keepLogin bool) error {
	backendClientConfig, err := p.formatInitParam(config)
	if err != nil {
		return err
	}

	cli, err := client.NewClient(backendClientConfig)
	if err != nil {
		return err
	}

	if err = cli.Login(context.Background()); err != nil {
		log.Errorf("plugin init login failed, err: %v", err)
		return err
	}

	system, err := cli.GetSystem(context.Background())
	if err != nil {
		log.Errorf("get system info error: %v", err)
		return err
	}

	p.product, err = utils.GetProductVersion(system)
	if err != nil {
		log.Errorf("get product version error: %v", err)
		return err
	}
	if !keepLogin {
		cli.Logout(context.Background())
	}

	if p.product == constants.OceanStorDoradoV6 {
		clientV6, err := clientv6.NewClientV6(backendClientConfig)
		if err != nil {
			return err
		}

		cli.Logout(context.Background())
		err = p.switchClient(clientV6)
		if err != nil {
			return err
		}
	} else {
		p.cli = cli
	}
	p.vStoreId = cli.VStoreID
	return nil
}

func (p *OceanstorPlugin) formatInitParam(config map[string]interface{}) (res *client.NewClientConfig, err error) {
	res = &client.NewClientConfig{}

	configUrls, exist := config["urls"].([]interface{})
	if !exist || len(configUrls) <= 0 {
		err = errors.New("urls must be provided")
		return
	}
	for _, i := range configUrls {
		res.Urls = append(res.Urls, i.(string))
	}
	res.User, exist = config["user"].(string)
	if !exist {
		err = errors.New("user must be provided")
		return
	}
	res.SecretName, exist = config["secretName"].(string)
	if !exist {
		err = errors.New("SecretName must be provided")
		return
	}
	res.SecretNamespace, exist = config["secretNamespace"].(string)
	if !exist {
		err = errors.New("SecretNamespace must be provided")
		return
	}
	res.BackendID, exist = config["backendID"].(string)
	if !exist {
		err = errors.New("backendID must be provided")
		return
	}
	res.VstoreName, _ = config["vstoreName"].(string)
	res.ParallelNum, _ = config["maxClientThreads"].(string)

	res.UseCert, _ = config["useCert"].(bool)
	res.CertSecretMeta, _ = config["certSecret"].(string)

	return
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
	supportLabel := app.GetGlobalConfig().EnableLabel && p.cli.GetStorageVersion() >= constants.MinVersionSupportLabel

	log.Debugf("enableLabel: %v, storageVersion: %v", app.GetGlobalConfig().EnableLabel, p.cli.GetStorageVersion())

	capabilities := map[string]interface{}{
		"SupportThin":            supportThin,
		"SupportThick":           supportThick,
		"SupportQoS":             supportQoS,
		"SupportMetro":           supportMetro,
		"SupportReplication":     supportReplication,
		"SupportApplicationType": supportApplicationType,
		"SupportClone":           supportClone,
		"SupportMetroNAS":        supportMetroNAS,
		"SupportLabel":           supportLabel,
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
		deviceSN, ok := dev["SN"].(string)
		if !ok {
			continue
		}
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
		log.Errorf("updateBackendCapabilities failed, err: %v", err)
		return nil, nil, err
	}

	specifications, err := p.updateBackendSpecifications()
	if err != nil {
		log.Errorf("updateBackendSpecifications failed, err: %v", err)
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
		"vstoreId":    "0",
	}
	for _, key := range []string{
		"storagepool",
		"allocType",
		"qos",
		"authClient",
		"backend",
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
		"parentname",
		"vstoreId",
	} {
		if v, exist := parameters[key]; exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	// Add new bool parameter here
	for _, i := range []string{
		"replication",
		"hyperMetro",
	} {
		if v, exist := parameters[i].(string); exist && v != "" {
			params[strings.ToLower(i)] = utils.StrToBool(ctx, v)
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
		name, ok := pool["NAME"].(string)
		if !ok {
			continue
		}
		freeCapacity, err := strconv.ParseInt(pool["USERFREECAPACITY"].(string), 10, 64)
		if err != nil {
			log.Warningf("analysisPoolsCapacity parseInt failed, data: %v, err: %v", pool["USERFREECAPACITY"], err)
		}

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
		return data, pkgUtils.Errorln(ctx, msg)
	}
	for _, configUrl := range configUrls {
		url, ok := configUrl.(string)
		if !ok {
			msg := fmt.Sprintf("Verify url: [%v] failed. url convert to string failed.", configUrl)
			return data, pkgUtils.Errorln(ctx, msg)
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
		return data, pkgUtils.Errorln(ctx, msg)
	}

	data.SecretName, exist = param["secretName"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify SecretName: [%v] failed. SecretName must be provided.", data.SecretName)
		return data, pkgUtils.Errorln(ctx, msg)
	}

	data.SecretNamespace, exist = param["secretNamespace"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify SecretNamespace: [%v] failed. SecretNamespace must be provided.",
			data.SecretNamespace)
		return data, pkgUtils.Errorln(ctx, msg)
	}

	data.BackendID, exist = param["backendID"].(string)
	if !exist {
		msg := fmt.Sprintf("Verify backendID: [%v] failed. backendID must be provided.",
			param["backendID"])
		return data, pkgUtils.Errorln(ctx, msg)
	}

	data.VstoreName, _ = param["vstoreName"].(string)
	data.ParallelNum, _ = param["maxClientThreads"].(string)

	data.UseCert, _ = param["useCert"].(bool)
	data.CertSecretMeta, _ = param["certSecret"].(string)

	return data, nil
}
