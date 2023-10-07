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

// Package backend get is related with storage backend get operation
package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"

	"huawei-csi-driver/csi/app"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

// GetBackendCapabilities used to get storage backend status, such as the license, capacity
func GetBackendCapabilities(ctx context.Context, storageBackendId string) (map[string]bool, map[string]string, error) {
	backend := GetBackendWithFresh(ctx, storageBackendId, true)
	if backend == nil {
		msg := fmt.Sprintf("Failed to get backend %s", storageBackendId)
		return nil, nil, pkgUtils.Errorln(ctx, msg)
	}

	// If sbct is offline, delete the backend from the csiBackends.
	backendID := pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backend.Name)
	online, err := pkgUtils.GetSBCTOnlineStatusByClaim(context.TODO(), backendID)
	if !online {
		RemoveOneBackend(ctx, backend.Name)
		msg := fmt.Sprintf("SBCT: [%s] online status is false, RemoveOneBackend: [%s]", backendID, backend.Name)
		return nil, nil, pkgUtils.Errorln(ctx, msg)
	}

	capabilities, specifications, err := backend.Plugin.UpdateBackendCapabilities()
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot update backend [%s] capabilities, ret: [%+v], error: [%v]",
			backend.Name, capabilities, err)
		return nil, nil, err
	}

	capabilityMap := map[string]bool{}
	for key, val := range capabilities {
		v, ok := val.(bool)
		if !ok {
			log.AddContext(ctx).Warningf("Convert capability [%s] val: [%v] to bool failed.", key, val)
			continue
		}
		capabilityMap[key] = v
	}

	specificationMap := map[string]string{}
	for key, val := range specifications {
		v, ok := val.(string)
		if !ok {
			log.AddContext(ctx).Warningf("Convert specifications [%s] val: [%v] to string failed.", key, val)
			continue
		}
		specificationMap[key] = v
	}

	return capabilityMap, specificationMap, nil
}

// GetBackendWithFresh used to obtain registered backends
var GetBackendWithFresh = func(ctx context.Context, backendName string, update bool) *Backend {
	// Registered backend exists in the cache.
	if csiBackends[backendName] != nil || !update {
		return csiBackends[backendName]
	}

	// The backend can be registered only when the storageBackendContent exists and [online: true].
	backendMeta := pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName)
	if !isBackendOnline(ctx, backendMeta) {
		return nil
	}

	configmapMeta, secretMeta, err := pkgUtils.GetConfigMeta(ctx, backendMeta)
	if err != nil {
		log.AddContext(ctx).Errorf("GetConfigMeta %s failed, error %v", backendMeta, err)
		return nil
	}

	useCert, certSecret, err := pkgUtils.GetCertMeta(ctx, backendMeta)
	if err != nil {
		log.AddContext(ctx).Errorf("GetCertMeta %s failed, error %v", backendMeta, err)
		return nil
	}

	_, err = RegisterOneBackend(ctx, backendMeta, configmapMeta, secretMeta, certSecret, useCert)
	if err != nil {
		msg := fmt.Sprintf("RegisterBackend %s failed, error %v", backendMeta, err)
		log.AddContext(ctx).Errorln(msg)
	}

	return csiBackends[backendName]
}

func isBackendOnline(ctx context.Context, claimNameMeta string) bool {
	log.AddContext(ctx).Infof("Start to check storageBackendContent: [%s] Online status.", claimNameMeta)

	content, err := pkgUtils.GetContentByClaimMeta(ctx, claimNameMeta)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.AddContext(ctx).Infof("Get storageBackendContent by claim: [%s] failed, sbct does not exist.", claimNameMeta)
			return false
		}
		log.AddContext(ctx).Errorf("Get storageBackendContent: [%s] failed, error: [%v].", claimNameMeta, err)
		return false
	}

	if content.Status == nil {
		log.AddContext(ctx).Errorf("StorageBackendContent: [%s] Status is nil.", content.Name)
		return false
	}

	log.AddContext(ctx).Infof("storageBackendContent status: [Online: %v]", content.Status.Online)
	return content.Status.Online
}

// GetBackendConfigmap used to get Configmap
func GetBackendConfigmap(ctx context.Context, configmapMeta string) (*coreV1.ConfigMap, error) {
	namespace, name, err := pkgUtils.SplitMetaNamespaceKey(configmapMeta)
	if err != nil {
		return nil, fmt.Errorf("split configmap meta %s namespace failed, error: %v", configmapMeta, err)
	}

	configmap, err := app.GetGlobalConfig().K8sUtils.GetConfigmap(ctx, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("get configmap for [%s] failed, error: %v", configmapMeta, err)
	}

	return configmap, nil
}

// GetBackendConfigmapMap used to get backend info by configmapMeta
func GetBackendConfigmapMap(ctx context.Context, configmapMeta string) (map[string]interface{}, error) {
	configmap, err := GetBackendConfigmap(ctx, configmapMeta)
	if err != nil {
		return nil, err
	}

	return ConvertConfigmapToMap(ctx, configmap)
}

func ConvertConfigmapToMap(ctx context.Context, configmap *coreV1.ConfigMap) (map[string]interface{}, error) {
	if configmap.Data == nil {
		msg := fmt.Sprintf("Configmap: [%s] the configmap.Data is nil", configmap.Name)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	var csiConfig CSIConfig
	err := json.Unmarshal([]byte(configmap.Data["csi.json"]), &csiConfig)
	if err != nil {
		msg := fmt.Sprintf("json.Unmarshal configmap.Data[\"csi.json\"] failed. err: %s", err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return csiConfig.Backends, nil
}

func addSecretInfo(secret *coreV1.Secret, storageConfig map[string]interface{}) error {
	if secret.Data == nil {
		return fmt.Errorf("the Data not exist in secret %s", secret.Name)
	}

	storageConfig["secretNamespace"] = secret.Namespace
	storageConfig["secretName"] = secret.Name
	storageConfig["user"] = string(secret.Data["user"])

	return nil
}

// GetStorageBackendInfo used to get storage config info
func GetStorageBackendInfo(ctx context.Context, backendID, configmapMeta, secretMeta, certSecret string,
	useCert bool) (map[string]interface{}, error) {
	log.AddContext(ctx).Infof("start GetStorageBackendInfo: %s.", backendID)
	backendMapData, err := GetBackendConfigmapMap(ctx, configmapMeta)
	if err != nil {
		msg := fmt.Sprintf("get backend %s failed, error %v", configmapMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	secret, err := pkgUtils.GetBackendSecret(ctx, secretMeta)
	if err != nil {
		msg := fmt.Sprintf("GetBackendSecret for secret %s failed, error %v", secretMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	err = addSecretInfo(secret, backendMapData)
	if err != nil {
		msg := fmt.Sprintf("addSecretInfo for secret %s failed, error %v", secretMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	backendMapData["backendID"] = backendID

	backendMapData["useCert"] = useCert
	backendMapData["certSecret"] = certSecret

	return backendMapData, nil
}
