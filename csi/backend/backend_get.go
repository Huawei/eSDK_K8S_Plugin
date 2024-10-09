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

	xuanwuV1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
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

// GetBackendInfoArgs is the arguments to get backend info.
type GetBackendInfoArgs struct {
	contentName   string
	configmapMeta string
	secretMeta    string
	certSecret    string
	useCert       bool
}

// NewGetBackendInfoArgsFromClaim used to new get backend info arguments from StorageBackendClaim
func NewGetBackendInfoArgsFromClaim(claim *xuanwuV1.StorageBackendClaim) GetBackendInfoArgs {
	return GetBackendInfoArgs{
		configmapMeta: claim.Spec.ConfigMapMeta,
		secretMeta:    claim.Spec.SecretMeta,
		certSecret:    claim.Spec.CertSecret,
		useCert:       claim.Spec.UseCert,
	}
}

// NewGetBackendInfoArgsFromContent used to new get backend info arguments from StorageBackendContent
func NewGetBackendInfoArgsFromContent(content *xuanwuV1.StorageBackendContent) GetBackendInfoArgs {
	return GetBackendInfoArgs{
		contentName:   content.Name,
		configmapMeta: content.Spec.ConfigmapMeta,
		secretMeta:    content.Spec.SecretMeta,
		certSecret:    content.Spec.CertSecret,
		useCert:       content.Spec.UseCert,
	}
}

// GetStorageBackendInfo used to get storage config info
func GetStorageBackendInfo(ctx context.Context, backendID string, args GetBackendInfoArgs) (map[string]any, error) {
	log.AddContext(ctx).Infof("start GetStorageBackendInfo: %s.", backendID)
	backendMapData, err := GetBackendConfigmapMap(ctx, args.configmapMeta)
	if err != nil {
		msg := fmt.Sprintf("get backend %s failed, error %v", args.configmapMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	secret, err := pkgUtils.GetBackendSecret(ctx, args.secretMeta)
	if err != nil {
		msg := fmt.Sprintf("GetBackendSecret for secret %s failed, error %v", args.secretMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	err = addSecretInfo(secret, backendMapData)
	if err != nil {
		msg := fmt.Sprintf("addSecretInfo for secret %s failed, error %v", args.secretMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	backendMapData["backendID"] = backendID
	backendMapData["useCert"] = args.useCert
	backendMapData["certSecret"] = args.certSecret
	backendMapData["contentName"] = args.contentName

	return backendMapData, nil
}
