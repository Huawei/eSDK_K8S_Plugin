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

	"huawei-csi-driver/csi/app"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
	coreV1 "k8s.io/api/core/v1"
)

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

// ConvertConfigmapToMap converts a configmap to a map object
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
