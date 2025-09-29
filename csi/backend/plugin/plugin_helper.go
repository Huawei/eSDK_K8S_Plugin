/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.
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

// Package plugin provide storage function
package plugin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"text/template"

	xuanwuV1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	oceanstor "github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// verifyProtocolAndPortals verifyProtocolAndPortals
func verifyProtocolAndPortals(parameters map[string]interface{}, storage string) (string, []string, error) {
	protocol, exist := parameters["protocol"].(string)
	if !exist {
		return "", []string{}, fmt.Errorf("protocol must be provided for %s backend", storage)
	}

	if (storage == constants.OceanStorNas || storage == constants.OceanStorDtree) &&
		(protocol != ProtocolNfs && protocol != ProtocolNfsPlus) {
		return "", []string{}, fmt.Errorf("protocol must be %s or %s for %s backend", ProtocolNfs,
			ProtocolNfsPlus, storage)
	}

	if (storage == constants.FusionNas || storage == constants.FusionDTree) &&
		(protocol != constants.ProtocolNfs && protocol != constants.ProtocolDpc) {
		return "", []string{}, fmt.Errorf("protocol must be %s or %s for %s backend", constants.ProtocolNfs,
			constants.ProtocolDpc, storage)
	}

	if protocol == constants.ProtocolDpc {
		return "", nil, nil
	}

	portals, exist := parameters["portals"].([]interface{})
	if !exist || len(portals) == 0 {
		return "", []string{}, fmt.Errorf("portals must be provided for %s backend", storage)
	}
	portalsStrs := pkgUtils.ConvertToStringSlice(portals)
	if protocol == ProtocolNfs && len(portalsStrs) != 1 {
		return "", []string{}, fmt.Errorf("portals just support one portal for %s backend nfs", storage)
	}
	if protocol == ProtocolNfsPlus && !checkNfsPlusPortalsFormat(portalsStrs) {
		return "", []string{}, errors.New("portals must be ip or domain and can't both exist")
	}

	return protocol, portalsStrs, nil
}

func checkNfsPlusPortalsFormat(portals []string) bool {
	var portalsTypeIP bool
	var portalsTypeDomain bool

	for _, portal := range portals {
		ip := net.ParseIP(portal)
		if ip != nil {
			portalsTypeIP = true
			if portalsTypeDomain {
				return false
			}
		} else {
			portalsTypeDomain = true
			if portalsTypeIP {
				return false
			}
		}
	}

	return true
}

func formatOceanstorInitParam(config map[string]interface{}) (res *oceanstor.NewClientConfig, err error) {
	res = &oceanstor.NewClientConfig{}

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

func formatBaseClientConfig(config map[string]interface{}) (*base.NewClientConfig, error) {
	res := &base.NewClientConfig{}
	configUrls, ok := utils.GetValue[[]interface{}](config, "urls")
	if !ok || len(configUrls) <= 0 {
		return nil, fmt.Errorf("urls is not provided in config, or it is invalid, config: %v", config)
	}

	for _, configUrl := range configUrls {
		url, ok := configUrl.(string)
		if !ok {
			return nil, fmt.Errorf("url %s convert to string failed", configUrl)
		}
		res.Urls = append(res.Urls, url)
	}

	res.User, ok = utils.GetValue[string](config, "user")
	if !ok {
		return nil, fmt.Errorf("user is not provided in config, or it is invalid, config: %v", config)
	}

	res.SecretName, ok = utils.GetValue[string](config, "secretName")
	if !ok {
		return nil, fmt.Errorf("secretName is not provided in config, or it is invalid, config: %v", config)
	}

	res.SecretNamespace, ok = utils.GetValue[string](config, "secretNamespace")
	if !ok {
		return nil, fmt.Errorf("secretNamespace is not provided in config, or it is invalid, config: %v", config)
	}

	res.BackendID, ok = utils.GetValue[string](config, "backendID")
	if !ok {
		return nil, fmt.Errorf("backendID is not provided in config, or it is invalid, config: %v", config)
	}

	res.ParallelNum, _ = utils.GetValue[string](config, "maxClientThreads")
	res.UseCert, _ = utils.GetValue[bool](config, "useCert")
	res.CertSecretMeta, _ = utils.GetValue[string](config, "certSecret")

	res.Storage, ok = utils.GetValue[string](config, "storage")
	if !ok {
		return nil, fmt.Errorf("storage is not provided in config, or it is invalid, config: %v", config)
	}

	res.Name, ok = utils.GetValue[string](config, "name")
	if !ok {
		return nil, fmt.Errorf("name is not provided in config, or it is invalid, config: %v", config)
	}

	return res, nil
}

func analyzePoolsCapacity(ctx context.Context, pools []map[string]interface{},
	vStoreQuotaMap map[string]interface{}) map[string]interface{} {
	capacities := make(map[string]interface{})

	for _, pool := range pools {
		name, ok := pool["NAME"].(string)
		if !ok {
			continue
		}
		var err error
		var freeCapacity, totalCapacity int64
		if freeStr, ok := pool["USERFREECAPACITY"].(string); ok {
			freeCapacity, err = strconv.ParseInt(freeStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
		}
		if totalStr, ok := pool["USERTOTALCAPACITY"].(string); ok {
			totalCapacity, err = strconv.ParseInt(totalStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
		}
		if err != nil {
			log.AddContext(ctx).Warningf("parse capacity failed, error: %v", err)
		}
		poolCapacityMap := map[string]interface{}{
			string(xuanwuV1.FreeCapacity):  freeCapacity * constants.AllocationUnitBytes,
			string(xuanwuV1.TotalCapacity): totalCapacity * constants.AllocationUnitBytes,
			string(xuanwuV1.UsedCapacity):  (totalCapacity - freeCapacity) * constants.AllocationUnitBytes,
		}
		if len(vStoreQuotaMap) == 0 {
			capacities[name] = poolCapacityMap
			continue
		}
		log.AddContext(ctx).Debugf("analyzePoolsCapacity poolName: %s, poolCapacity: %+v, vstoreQuota: %+v",
			name, poolCapacityMap, vStoreQuotaMap)
		free, ok := vStoreQuotaMap[string(xuanwuV1.FreeCapacity)].(int64)
		if ok && free < freeCapacity*constants.AllocationUnitBytes {
			capacities[name] = vStoreQuotaMap
		} else {
			capacities[name] = poolCapacityMap
		}
	}

	return capacities
}

func verifyDTreeParam(ctx context.Context, config map[string]any, storageType string) error {
	// verify storage
	storage, exist := utils.ToStringWithFlag(config["storage"])
	if !exist || (storage != storageType) {
		return fmt.Errorf("verify storage %v failed: storage must be %q", config["storage"], storageType)
	}

	// verify parameters
	parameters, exist := config["parameters"].(map[string]any)
	if !exist {
		return errors.New("parameters of backend must be provided, but got empty")
	}

	// verify protocol portals
	_, _, err := verifyProtocolAndPortals(parameters, storageType)
	if err != nil {
		return pkgUtils.Errorf(ctx, "check fusionstorage-dtree parameter failed, err: %v", err)
	}

	return nil
}

// getValidParentname gets the valid parent name of dtree by compare StorageClass.parentname and backend.parentname
// returns:
//
// error if StorageClass.parentname == "" && backend.parentname == ""
// backend.parentname if StorageClass.parentname == "" && backend.parentname != ""
// StorageClass.parentname if StorageClass.parentname != "" && backend.parentname == ""
// error if StorageClass.parentname != "" && backend.parentname != "" && StorageClass.parentname != backend.parentname
// StorageClass.parentname if StorageClass.parentname == backend.parentname
func getValidParentname(scParentname, bkParentname string) (string, error) {
	if scParentname == "" {
		if bkParentname == "" {
			return "", errors.New("parentname must be provided in StorageClass or backend")
		}

		return bkParentname, nil
	}

	if bkParentname == "" {
		return scParentname, nil
	}

	if scParentname != bkParentname {
		return "", errors.New(fmt.Sprintf("parentname %q in StorageClass is not equal to %q in backend",
			scParentname, bkParentname))
	}

	return scParentname, nil
}

// DTreeAttachVolumeParameter is the parameter for attaching volume
type DTreeAttachVolumeParameter struct {
	VolumeContext map[string]string `json:"volumeContext"`
}

// attachDTreeVolume attach dtree volume to node and return storage mapping info.
func attachDTreeVolume(parameters map[string]any) (map[string]any, error) {
	params, err := utils.ConvertMapToStruct[DTreeAttachVolumeParameter](parameters)
	if err != nil {
		return nil, fmt.Errorf("convert parameters to struct failed when attach dtree: %w", err)
	}

	dtreeParentName, exists := params.VolumeContext[constants.DTreeParentKey]
	if exists {
		return map[string]any{constants.DTreeParentKey: dtreeParentName}, nil
	}

	return map[string]any{}, nil
}

func updateCapabilityByNfsServiceSetting(capabilities map[string]interface{}, nfsServiceSetting map[string]bool) {
	// NFS3 is enabled by default.
	capabilities["SupportNFS3"] = true
	capabilities["SupportNFS4"] = false
	capabilities["SupportNFS41"] = false
	capabilities["SupportNFS42"] = false

	if !nfsServiceSetting["SupportNFS3"] {
		capabilities["SupportNFS3"] = false
	}

	if nfsServiceSetting["SupportNFS4"] {
		capabilities["SupportNFS4"] = true
	}

	if nfsServiceSetting["SupportNFS41"] {
		capabilities["SupportNFS41"] = true
	}

	if nfsServiceSetting["SupportNFS42"] {
		capabilities["SupportNFS42"] = true
	}
}

func getVolumeNameFromPVNameOrParameters(pvName string, parameters map[string]any) (string, error) {
	volumeNameTpl, _ := utils.GetValue[string](parameters, constants.ScVolumeNameKey)
	if volumeNameTpl == "" {
		return pvName, nil
	}

	if err := validateVolumeName(volumeNameTpl); err != nil {
		return "", err
	}

	metadata, err := newExtraCreateMetadataFromParameters(parameters)
	if err != nil {
		return "", err
	}

	tpl, err := template.New(constants.ScVolumeNameKey).Parse(volumeNameTpl + volumeNameSuffix)
	if err != nil {
		return "", fmt.Errorf("failed to parse volume name template %s: %w", volumeNameTpl, err)
	}

	var volumeName strings.Builder
	if err := tpl.Execute(&volumeName, metadata); err != nil {
		return "", fmt.Errorf("failed to excute template: %w", err)
	}

	return volumeName.String(), nil
}
