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

func (ctrl *BackendController) syncSecret(ctx context.Context, storageBackend *xuanwuv1.StorageBackendClaim) (
	*coreV1.Secret, error) {
	secret, err := ctrl.checkSecret(ctx, storageBackend)
	if err != nil {
		msg := fmt.Sprintf("check secret %s for claim %s failed, error: %v",
			storageBackend.Spec.SecretMeta, utils.StorageBackendClaimKey(storageBackend), err)
		ctrl.eventRecorder.Event(storageBackend, coreV1.EventTypeWarning, "ErrorCheckSecret", msg)
		return nil, errors.New(msg)
	}

	if err = ctrl.setSecretFinalizer(ctx, secret); err != nil {
		msg := fmt.Sprintf("Failed to check and set Secret Finalizer for StorageBackendClaim %s,"+
			" error: %v", utils.StorageBackendClaimKey(storageBackend), err)
		log.AddContext(ctx).Errorln(msg)
		ctrl.eventRecorder.Event(storageBackend, coreV1.EventTypeWarning, "ErrorSetSecretFinalizer", msg)
		return nil, errors.New(msg)
	}

	return secret, nil
}

func (ctrl *BackendController) setSecretFinalizer(ctx context.Context, secret *coreV1.Secret) error {
	if secret == nil {
		return nil
	}

	if finalizers.ContainsFinalizer(secret, utils.SecretFinalizer) {
		return nil
	}

	k8sUtils := app.GetGlobalConfig().K8sUtils
	finalizers.SetFinalizer(secret, utils.SecretFinalizer)
	_, err := k8sUtils.UpdateSecret(ctx, secret)
	if err != nil {
		log.AddContext(ctx).Errorf("setSecretFinalizer: update secret failed, error %v", err)
		return err
	}
	return nil
}

func (ctrl *BackendController) removeSecretFinalizer(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim) error {

	log.AddContext(ctx).Infof("removeSecretFinalizer with claim %s.",
		utils.StorageBackendClaimKey(storageBackend))
	if storageBackend.Status.SecretMeta == "" {
		return nil
	}

	secret, err := utils.GetSecret(storageBackend.Spec.SecretMeta)
	if err != nil && !apiErrors.IsNotFound(err) {
		log.AddContext(ctx).Errorf("getting secret, error %v", err)
		return err
	}

	if secret == nil {
		return nil
	}

	k8sUtils := app.GetGlobalConfig().K8sUtils

	if finalizers.ContainsFinalizer(secret, utils.SecretFinalizer) {
		if ctrl.isSecretUsed(ctx, secret, storageBackend, true) {
			return nil
		}
		finalizers.RemoveFinalizer(secret, utils.SecretFinalizer)
		_, err = k8sUtils.UpdateSecret(ctx, secret)
		if err != nil {
			log.AddContext(ctx).Errorf("update secret failed, error %v", err)
			return err
		}
	}

	return nil
}

func (ctrl *BackendController) checkSecret(ctx context.Context, storageBackend *xuanwuv1.StorageBackendClaim) (
	*coreV1.Secret, error) {

	if storageBackend.Spec.SecretMeta == "" {
		log.AddContext(ctx).Infoln("Configure secret is empty, no need to get.")
		return nil, nil
	}

	return utils.GetSecret(storageBackend.Spec.SecretMeta)
}

func (ctrl *BackendController) isSecretUsed(ctx context.Context, secret *coreV1.Secret,
	storageBackend *xuanwuv1.StorageBackendClaim, skipCurObj bool) bool {

	log.AddContext(ctx).Debugf("checking secret used for storageBackend %s",
		utils.StorageBackendClaimKey(storageBackend))
	claims, err := ctrl.claimLister.StorageBackendClaims(storageBackend.Namespace).List(labels.Everything())
	if err != nil {
		return false
	}

	for _, claim := range claims {
		if skipCurObj && claim.Name == storageBackend.Name {
			continue
		}

		secretMeta, err := utils.GenObjectMetaKey(secret)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to get secret %s meta info", secret.Name)
			return false
		}

		if secretMeta == claim.Status.SecretMeta {
			return true
		}
	}
	return false
}
