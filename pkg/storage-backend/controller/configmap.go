/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package controller used deal with the backend claim and backend content resources
package controller

import (
	"context"
	"errors"
	"fmt"

	coreV1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/pkg/finalizers"
	"huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

func (ctrl *BackendController) syncConfigmap(ctx context.Context, storageBackend *xuanwuv1.StorageBackendClaim) (
	*coreV1.ConfigMap, error) {

	configmap, err := ctrl.checkConfigMap(ctx, storageBackend)
	if err != nil {
		msg := fmt.Sprintf("check configMap %s for claim %s failed, error: %v",
			storageBackend.Spec.ConfigMapMeta, utils.StorageBackendClaimKey(storageBackend), err)
		ctrl.eventRecorder.Event(storageBackend, coreV1.EventTypeWarning, "ErrorCheckConfigmap", msg)
		return nil, errors.New(msg)
	}

	if err = ctrl.setConfigmapFinalizer(ctx, configmap); err != nil {
		msg := fmt.Sprintf("Failed to check and set ConfigMap Finalizer for StorageBackendClaim %s,"+
			" error: %v", utils.StorageBackendClaimKey(storageBackend), err)
		log.AddContext(ctx).Errorln(msg)
		ctrl.eventRecorder.Event(storageBackend, coreV1.EventTypeWarning, "ErrorSetConfigMapFinalizer", msg)
		return nil, errors.New(msg)
	}

	return configmap, nil
}

func (ctrl *BackendController) setConfigmapFinalizer(ctx context.Context, configmap *coreV1.ConfigMap) error {
	if configmap == nil {
		return nil
	}

	if finalizers.ContainsFinalizer(configmap, utils.ConfigMapFinalizer) {
		return nil
	}

	finalizers.SetFinalizer(configmap, utils.ConfigMapFinalizer)
	k8sUtils := app.GetGlobalConfig().K8sUtils
	_, err := k8sUtils.UpdateConfigmap(ctx, configmap)
	if err != nil {
		log.AddContext(ctx).Errorf("setConfigmapFinalizer: update configmap failed, error %v", err)
		return err
	}
	return nil
}

func (ctrl *BackendController) removeConfigmapFinalizer(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim) error {

	log.AddContext(ctx).Infof("removeConfigmapFinalizer with claim %s.",
		utils.StorageBackendClaimKey(storageBackend))

	if storageBackend.Status.ConfigmapMeta == "" {
		return nil
	}

	configmap, err := utils.GetConfigMap(storageBackend.Spec.ConfigMapMeta)
	if err != nil && !apiErrors.IsNotFound(err) {
		log.AddContext(ctx).Errorf("getting configmap, error %v", err)
		return err
	}

	if configmap == nil {
		return nil
	}

	if !finalizers.ContainsFinalizer(configmap, utils.ConfigMapFinalizer) {
		return nil
	}

	if ctrl.isConfigmapUsed(ctx, configmap, storageBackend, true) {
		return nil
	}

	finalizers.RemoveFinalizer(configmap, utils.ConfigMapFinalizer)
	k8sUtils := app.GetGlobalConfig().K8sUtils
	_, err = k8sUtils.UpdateConfigmap(ctx, configmap)
	if err != nil {
		log.AddContext(ctx).Errorf("removeConfigmapFinalizer: update configmap failed, error %v", err)
		return err
	}

	return nil
}

func (ctrl *BackendController) checkConfigMap(ctx context.Context, storageBackend *xuanwuv1.StorageBackendClaim) (
	*coreV1.ConfigMap, error) {

	if storageBackend.Spec.ConfigMapMeta == "" {
		log.AddContext(ctx).Infoln("Configure configmap is empty, no need to get.")
		return nil, nil
	}

	return utils.GetConfigMap(storageBackend.Spec.ConfigMapMeta)
}

func (ctrl *BackendController) isConfigmapUsed(ctx context.Context, configmap *coreV1.ConfigMap,
	storageBackend *xuanwuv1.StorageBackendClaim, skipCurObj bool) bool {
	log.AddContext(ctx).Infof("checking configmap used for storageBackend %s",
		utils.StorageBackendClaimKey(storageBackend))

	claims, err := ctrl.claimLister.StorageBackendClaims(storageBackend.Namespace).List(labels.Everything())
	if err != nil {
		return false
	}

	for _, claim := range claims {
		if skipCurObj && claim.Name == storageBackend.Name {
			continue
		}

		configmapMeta, err := utils.GenObjectMetaKey(configmap)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to get configmap %s meta info", configmap.Name)
			return false
		}

		if claim.Status != nil && configmapMeta == claim.Status.ConfigmapMeta {
			return true
		}
	}
	return false
}
