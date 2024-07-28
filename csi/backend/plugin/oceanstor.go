/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

	xuanwuV1 "huawei-csi-driver/client/apis/xuanwu/v1"
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
	// DoradoV6PoolUsageType defines pool usage type of dorado v6
	DoradoV6PoolUsageType = "0"

	// ProtocolNfs defines protocol type nfs
	ProtocolNfs = "nfs"
	// ProtocolNfsPlus defines protocol type nfs+
	ProtocolNfsPlus = "nfs+"

	// SystemVStore default value is 0
	SystemVStore = "0"
)

// OceanstorPlugin provides oceanstor plugin base operations
type OceanstorPlugin struct {
	basePlugin

	vStoreId string

	cli          client.BaseClientInterface
	product      string
	capabilities map[string]interface{}
}

func (p *OceanstorPlugin) init(ctx context.Context, config map[string]interface{}, keepLogin bool) error {
	backendClientConfig, err := p.formatInitParam(config)
	if err != nil {
		return err
	}

	cli, err := client.NewClient(ctx, backendClientConfig)
	if err != nil {
		return err
	}

	if err = cli.Login(ctx); err != nil {
		log.AddContext(ctx).Errorf("plugin init login failed, err: %v", err)
		return err
	}

	if err = cli.SetSystemInfo(ctx); err != nil {
		cli.Logout(ctx)
		log.AddContext(ctx).Errorf("set client info failed, err: %v", err)
		return err
	}

	p.name = backendClientConfig.Name
	p.product = cli.Product

	if p.product == constants.OceanStorDoradoV6 {
		clientV6, err := clientv6.NewClientV6(ctx, backendClientConfig)
		if err != nil {
			cli.Logout(ctx)
			log.AddContext(ctx).Errorf("new OceanStor V6 client error: %v", err)
			return err
		}
		cli.Logout(ctx)
		clientV6.LastLif = cli.GetCurrentLif(ctx)
		clientV6.CurrentLifWwn = cli.CurrentLifWwn
		err = p.switchClient(ctx, clientV6)
		if err != nil {
			return err
		}
	} else {
		p.cli = cli
	}
	if !keepLogin {
		cli.Logout(ctx)
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

	res.Storage, exist = config["storage"].(string)
	if !exist {
		return nil, errors.New("storage type must be configured for backend")
	}

	res.Name, exist = config["name"].(string)
	if !exist {
		return nil, errors.New("storage name must be configured for backend")
	}
	return
}

func (p *OceanstorPlugin) updateBackendCapabilities(ctx context.Context) (map[string]interface{}, error) {
	features, err := p.cli.GetLicenseFeature(ctx)
	if err != nil {
		log.Errorf("Get license feature error: %v", err)
		return nil, err
	}

	log.AddContext(ctx).Debugf("Get license feature: %v", features)

	supportThin := utils.IsSupportFeature(features, "SmartThin")
	supportThick := p.product != "Dorado" && p.product != "DoradoV6"
	supportQoS := utils.IsSupportFeature(features, "SmartQoS")
	supportMetro := utils.IsSupportFeature(features, "HyperMetro")
	supportMetroNAS := utils.IsSupportFeature(features, "HyperMetroNAS")
	supportReplication := utils.IsSupportFeature(features, "HyperReplication")
	supportClone := utils.IsSupportFeature(features, "HyperClone") || utils.IsSupportFeature(features, "HyperCopy")
	supportApplicationType := p.product == "DoradoV6"

	supportLabel := app.GetGlobalConfig().EnableLabel &&
		p.cli.GetStorageVersion() >= constants.MinVersionSupportLabel &&
		p.cli.IsSupportContainer(ctx)
	log.AddContext(ctx).Debugf("enableLabel: %v, storageVersion: %v", app.GetGlobalConfig().EnableLabel,
		p.cli.GetStorageVersion())

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

func (p *OceanstorPlugin) getRemoteDevices(ctx context.Context) (string, error) {
	devices, err := p.cli.GetAllRemoteDevices(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Get remote devices error: %v", err)
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

func (p *OceanstorPlugin) updateBackendSpecifications(ctx context.Context) (map[string]interface{}, error) {
	devicesSN, err := p.getRemoteDevices(ctx)
	if err != nil {
		return nil, err
	}

	specifications := map[string]interface{}{
		"LocalDeviceSN":   p.cli.GetDeviceSN(),
		"RemoteDevicesSN": devicesSN,
		"VStoreID":        p.cli.GetvStoreID(),
		"VStoreName":      p.cli.GetvStoreName(),
	}
	return specifications, nil
}

// updateVStorePair update vStore pair info
func (p *OceanstorPlugin) updateVStorePair(ctx context.Context, specifications map[string]interface{}) {
	if specifications == nil {
		specifications = map[string]interface{}{}
	}

	if p.product != constants.OceanStorDoradoV6 || p.cli.GetvStoreID() == "" ||
		p.cli.GetStorageVersion() < constants.DoradoV615 {
		log.AddContext(ctx).Debugf("storage product is %s,version is %s, vStore id is %s, "+
			"do not update VStorePairId", p.product, p.cli.GetStorageVersion(), p.cli.GetvStoreID())
		return
	}

	vStorePairs, err := p.cli.GetVStorePairs(ctx)
	if err != nil {
		log.AddContext(ctx).Debugf("Get vStore pairs error: %v", err)
		return
	}

	if len(vStorePairs) == 0 {
		log.AddContext(ctx).Debugln("Get vStore pairs is empty")
		return
	}

	for _, pair := range vStorePairs {
		if data, ok := pair.(map[string]interface{}); ok {
			if localVStoreId, ok := data["LOCALVSTOREID"].(string); ok && localVStoreId == p.cli.GetvStoreID() {
				specifications["VStorePairId"] = data["ID"]
				specifications["HyperMetroDomainId"] = data["DOMAINID"]
				return
			}
		}
	}
	log.AddContext(ctx).Debugf("not found VStorePairId and HyperMetroDomainId, current vStoreId is %s",
		p.cli.GetvStoreID())
}

// for fileSystem on dorado storage, only Thin is supported
func (p *OceanstorNasPlugin) updateSmartThin(capabilities map[string]interface{}) error {
	if capabilities == nil {
		return nil
	}
	if p.product == "Dorado" || p.product == "DoradoV6" {
		capabilities["SupportThin"] = true
	}
	return nil
}

// UpdateBackendCapabilities used to update backend capabilities
func (p *OceanstorPlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities, err := p.updateBackendCapabilities(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("updateBackendCapabilities failed, err: %v", err)
		return nil, nil, err
	}

	specifications, err := p.updateBackendSpecifications(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("updateBackendSpecifications failed, err: %v", err)
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
		"capacity":    utils.RoundUpSize(parameters["size"].(int64), constants.AllocationUnitBytes),
		"vstoreId":    "0",
	}
	resetParams(parameters, params)
	toLowerParams(parameters, params)
	processBoolParams(ctx, parameters, params)
	return params
}

// resetParams process need reset param
func resetParams(source, target map[string]interface{}) {
	if source == nil || target == nil {
		return
	}
	if fileSystemName, ok := source["annVolumeName"]; ok {
		target["name"] = fileSystemName
	}
}

// processBoolParams process bool param
func processBoolParams(ctx context.Context, source, target map[string]interface{}) {
	if source == nil || target == nil {
		return
	}
	// Add new bool parameter here
	for _, i := range []string{
		"replication",
		"hyperMetro",
	} {
		if v, exist := source[i].(string); exist && v != "" {
			target[strings.ToLower(i)] = utils.StrToBool(ctx, v)
		}
	}
}

// toLowerParams convert params to lower
func toLowerParams(source, target map[string]interface{}) {
	if source == nil || target == nil {
		return
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
		"replicationSyncPeriod",
		"vStorePairID",
		"accesskrb5",
		"accesskrb5i",
		"accesskrb5p",
		"fileSystemMode",
	} {
		if v, exist := source[key]; exist && v != "" {
			target[strings.ToLower(key)] = v
		}
	}
}

func (p *OceanstorPlugin) updatePoolCapabilities(ctx context.Context, poolNames []string,
	vStoreQuotaMap map[string]interface{}, usageType string) (map[string]interface{}, error) {
	pools, err := p.cli.GetAllPools(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Get all pools error: %v", err)
		return nil, err
	}

	log.AddContext(ctx).Debugf("Get pools: %v", pools)

	var validPools []map[string]interface{}
	for _, name := range poolNames {
		if pool, exist := pools[name].(map[string]interface{}); exist {
			poolType, exist := pool["NEWUSAGETYPE"].(string)
			if (pool["USAGETYPE"] == usageType || pool["USAGETYPE"] == DoradoV6PoolUsageType) ||
				(exist && poolType == DoradoV6PoolUsageType) {
				validPools = append(validPools, pool)
			} else {
				log.AddContext(ctx).Warningf("Pool %s is not for %s", name, usageType)
			}
		} else {
			log.AddContext(ctx).Warningf("Pool %s does not exist", name)
		}
	}

	capabilities := p.analyzePoolsCapacity(ctx, validPools, vStoreQuotaMap)
	return capabilities, nil
}

func (p *OceanstorPlugin) analyzePoolsCapacity(ctx context.Context, pools []map[string]interface{},
	vStoreQuotaMap map[string]interface{}) map[string]interface{} {
	capabilities := make(map[string]interface{})

	for _, pool := range pools {
		name, ok := pool["NAME"].(string)
		if !ok {
			continue
		}
		var err error
		var freeCapacity, totalCapacity int64
		if freeStr, ok := pool["USERFREECAPACITY"].(string); ok {
			freeCapacity, err = strconv.ParseInt(freeStr, 10, 64)
		}
		if totalStr, ok := pool["USERTOTALCAPACITY"].(string); ok {
			totalCapacity, err = strconv.ParseInt(totalStr, 10, 64)
		}
		if err != nil {
			log.AddContext(ctx).Warningf("parse capacity failed, error: %v", err)
		}
		poolCapacityMap := map[string]interface{}{
			string(xuanwuV1.FreeCapacity):  freeCapacity * 512,
			string(xuanwuV1.TotalCapacity): totalCapacity * 512,
			string(xuanwuV1.UsedCapacity):  totalCapacity - freeCapacity,
		}
		if len(vStoreQuotaMap) == 0 {
			capabilities[name] = poolCapacityMap
			continue
		}
		log.AddContext(ctx).Debugf("analyzePoolsCapacity poolName: %s, poolCapacity: %+v, vstoreQuota: %+v",
			name, poolCapacityMap, vStoreQuotaMap)
		free, ok := vStoreQuotaMap[string(xuanwuV1.FreeCapacity)].(int64)
		if ok && free < freeCapacity*512 {
			capabilities[name] = vStoreQuotaMap
		} else {
			capabilities[name] = poolCapacityMap
		}
	}

	return capabilities
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
func (p *OceanstorPlugin) switchClient(ctx context.Context, newClient client.BaseClientInterface) error {
	log.AddContext(ctx).Infoln("Using OceanStor V6 or Dorado V6 BaseClient.")
	p.cli = newClient
	err := p.cli.Login(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			p.Logout(ctx)
		}
	}()
	// If a new URL is selected when the client is switched, the system information needs to be updated.
	if err = p.cli.SetSystemInfo(ctx); err != nil {
		log.AddContext(ctx).Errorf("set system info failed, error: %v", err)
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
