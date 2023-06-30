/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

// Package utils to provide k8s resource utils
package utils

import (
	"context"
	"errors"
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

func IsSBCTExist(ctx context.Context, backendID string) bool {
	content, err := GetContentByClaimMeta(ctx, backendID)
	if err != nil {
		log.AddContext(ctx).Infof("IsSBCTExist err: [%v]", err)
		return false
	}

	return content != nil
}

func IsSBCTOnline(ctx context.Context, backendID string) bool {
	online, err := GetSBCTOnlineStatusByClaim(ctx, backendID)
	if err != nil {
		log.AddContext(ctx).Infof("GetSBCTOnlineStatusByClaim failed, err: [%v]", err)
		return false
	}

	return online
}

func GetPasswordFromBackendID(ctx context.Context, backendID string) (string, error) {
	_, secretMeta, err := GetConfigMeta(ctx, backendID)
	if err != nil {
		return "", err
	}

	namespace, secretName, err := SplitMetaNamespaceKey(secretMeta)
	if err != nil {
		return "", fmt.Errorf("split secret secretMeta %s namespace failed, error: %v", secretMeta, err)
	}

	return utils.GetPasswordFromSecret(ctx, secretName, namespace)
}

func GetBackendConfigmapByClaimName(ctx context.Context, claimNameMeta string) (*coreV1.ConfigMap, error) {
	log.AddContext(ctx).Infof("Get configmap meta data by claim meta: [%s]", claimNameMeta)
	configmapMeta, _, err := GetConfigMeta(ctx, claimNameMeta)
	if err != nil {
		msg := fmt.Sprintf("GetConfigMeta: [%s] failed, error: [%v].", claimNameMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	ns, configmapName, err := SplitMetaNamespaceKey(configmapMeta)
	if err != nil {
		msg := fmt.Sprintf("SplitMetaNamespaceKey ConfigmapMeta: [%s] failed, error: [%v].",
			configmapMeta, err)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return app.GetGlobalConfig().K8sUtils.GetConfigmap(ctx, configmapName, ns)
}

func GetClaimByMeta(ctx context.Context, claimNameMeta string) (*xuanwuv1.StorageBackendClaim, error) {
	ns, claimName, err := SplitMetaNamespaceKey(claimNameMeta)
	if err != nil {
		msg := fmt.Sprintf("SplitMetaNamespaceKey: [%s] failed, error: [%v].", claimNameMeta, err)
		return nil, Errorln(ctx, msg)
	}

	claim, err := GetClaim(ctx, app.GetGlobalConfig().BackendUtils,
		&xuanwuv1.StorageBackendClaim{
			ObjectMeta: v1.ObjectMeta{
				Namespace: ns,
				Name:      claimName,
			},
		})
	if err != nil {
		msg := fmt.Sprintf("Get storageBackendClaim: [%s] failed, error: [%v].", claimNameMeta, err)
		return nil, Errorln(ctx, msg)
	}

	if claim == nil {
		msg := fmt.Sprintf("StorageBackendClaim: [%s] is nil, get claim failed.", claimName)
		return nil, Errorln(ctx, msg)
	}

	return claim, nil
}

func GetConfigMeta(ctx context.Context, claimNameMeta string) (string, string, error) {
	log.AddContext(ctx).Infof("Get claim: [%s] config meta.", claimNameMeta)

	claim, err := GetClaimByMeta(ctx, claimNameMeta)
	if err != nil {
		return "", "", err
	}

	if claim == nil {
		msg := fmt.Sprintf("Get claim failed, claim: [%s] is nil", claimNameMeta)
		return "", "", Errorln(ctx, msg)
	}

	return claim.Spec.ConfigMapMeta, claim.Spec.SecretMeta, nil
}

func GetContentByClaimMeta(ctx context.Context, claimNameMeta string) (*xuanwuv1.StorageBackendContent, error) {
	log.AddContext(ctx).Debugf("Start to get storageBackendContent by claimMeta: [%s].", claimNameMeta)

	claim, err := GetClaimByMeta(ctx, claimNameMeta)
	if err != nil {
		return nil, err
	}

	if claim.Status == nil {
		msg := fmt.Sprintf("StorageBackendClaim: [%s] status is nil, can not get content name.", claimNameMeta)
		return nil, Errorln(ctx, msg)
	}

	return GetContent(ctx, app.GetGlobalConfig().BackendUtils, claim.Status.BoundContentName)
}

func GetBackendSecret(ctx context.Context, secretMeta string) (*coreV1.Secret, error) {
	namespace, name, err := SplitMetaNamespaceKey(secretMeta)
	if err != nil {
		return nil, fmt.Errorf("split secret secretMeta %s namespace failed, error: %v", secretMeta, err)
	}

	secret, err := app.GetGlobalConfig().K8sUtils.GetSecret(ctx, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("get secret with name %s and namespace %s failed, error: %v",
			name, namespace, err)
	}

	return secret, nil
}

func GetSBCTOnlineStatusByClaim(ctx context.Context, backendID string) (bool, error) {
	content, err := GetContentByClaimMeta(ctx, backendID)
	if err != nil {
		msg := fmt.Sprintf("GetContentByClaimMeta: [%s] failed, err: [%v]", backendID, err)
		return false, Errorln(ctx, msg)
	}

	if content == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] content is nil, GetSBCTOnlineStatusByClaim failed.",
			content.Name)
		return false, Errorln(ctx, msg)
	}

	if content.Status == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] content.status is nil, GetSBCTOnlineStatusByClaim failed.",
			content.Name)
		return false, Errorln(ctx, msg)
	}

	return content.Status.Online, nil
}

func SetSBCTOnlineStatus(ctx context.Context, content *xuanwuv1.StorageBackendContent, status bool) error {
	content.Status.Online = status

	_, err := app.GetGlobalConfig().BackendUtils.XuanwuV1().StorageBackendContents().UpdateStatus(ctx,
		content, v1.UpdateOptions{})
	if err != nil {
		msg := fmt.Sprintf("Update storageBackendContent Status: [%s] failed， err: [%v]", content.Name, err)
		return Errorln(ctx, msg)
	}

	return nil
}

func SetStorageBackendContentOnlineStatus(ctx context.Context, backendID string, online bool) error {
	content, err := GetContentByClaimMeta(ctx, backendID)
	if err != nil {
		msg := fmt.Sprintf("GetContentByClaimMeta: [%s] failed, err: [%v]", backendID, err)
		return Errorln(ctx, msg)
	}

	if content.Status == nil {
		msg := fmt.Sprintf("StorageBackendContent: [%s] status is nil, SetStorageBackendContentOnlineStatus failed.",
			content.Name)
		return Errorln(ctx, msg)
	}

	err = SetSBCTOnlineStatus(ctx, content, online)
	if err != nil {
		return err
	}

	log.AddContext(ctx).Infof("SetStorageBackendContentOnlineStatus [%s] to [%b] succeeded.",
		backendID, online)
	return nil
}
