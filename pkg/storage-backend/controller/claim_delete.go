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

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/pkg/finalizers"
	"huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

func (ctrl *BackendController) deleteStorageBackendClaim(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim) error {

	_ = ctrl.claimStore.Delete(storageBackend)
	log.AddContext(ctx).Infof("storageBackendClaim %s deleted", utils.StorageBackendClaimKey(storageBackend))

	backendContentName := ""
	if storageBackend.Status != nil && storageBackend.Status.BoundContentName != "" {
		backendContentName = storageBackend.Status.BoundContentName
	}

	if backendContentName == "" {
		log.AddContext(ctx).Infof("deleteStorageBackendClaim %s, there is no content bound.",
			utils.StorageBackendClaimKey(storageBackend))
		return nil
	}

	ctrl.contentQueue.Add(backendContentName)
	return nil
}

func (ctrl *BackendController) processWithDeletionTimeStamp(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim) error {

	log.AddContext(ctx).Infof("processWithDeletionTimeStamp StorageBackendClaim %s",
		utils.StorageBackendClaimKey(storageBackend))
	backendContentName := ""
	if storageBackend.Status != nil && storageBackend.Status.BoundContentName != "" {
		backendContentName = storageBackend.Status.BoundContentName
	}

	backendContent, err := ctrl.getContentFromStore(backendContentName)
	if err != nil {
		log.AddContext(ctx).Errorf("getContentFromStore %s failed, error: %v", backendContentName, err)
		return err
	}

	if backendContent != nil && backendContent.Spec.BackendClaim == utils.StorageBackendClaimKey(storageBackend) {
		log.AddContext(ctx).Infof("Check to delete content, content spec claim %s, claim info %s",
			backendContent.Spec.BackendClaim, utils.StorageBackendClaimKey(storageBackend))
		err = ctrl.deleteContent(ctx, backendContentName)
		if err != nil {
			ctrl.eventRecorder.Eventf(backendContent, coreV1.EventTypeWarning,
				"ErrorDeleteStorageBackendContent", err.Error())
			return err
		}
	}

	if !utils.NeedRemoveClaimBoundFinalizers(storageBackend) {
		return nil
	}

	return ctrl.removeClaimFinalizer(ctx, storageBackend)
}

func (ctrl *BackendController) getContentFromStore(contentName string) (*xuanwuv1.StorageBackendContent, error) {
	obj, exist, err := ctrl.contentStore.GetByKey(contentName)
	if err != nil {
		return nil, err
	}

	if !exist {
		return nil, nil
	}

	content, ok := obj.(*xuanwuv1.StorageBackendContent)
	if !ok {
		return nil, fmt.Errorf("expected StorageBackendContent, got %+v", obj)
	}
	return content, nil
}

func (ctrl *BackendController) removeContentFinalizer(ctx context.Context,
	content *xuanwuv1.StorageBackendContent) error {

	log.AddContext(ctx).Infof("Start to remove content %s finalizer.", content.Name)
	finalizers.RemoveFinalizer(content, utils.ContentBoundFinalizer)
	newObj, err := utils.UpdateContent(ctx, ctrl.clientSet, content)
	if err != nil && apiErrors.IsResourceExpired(err) {
		log.AddContext(ctx).Warningf("Update storageBackendContent finalizer %s failed, error %v",
			content.Name, err)
		content, err = utils.GetContent(ctx, ctrl.clientSet, content.Name)
		if err != nil {
			log.AddContext(ctx).Errorf("get content %s from cluster failed, error: %v", content.Name, err)
			return err
		}
		finalizers.RemoveFinalizer(content, utils.ContentBoundFinalizer)
		newObj, err = utils.UpdateContent(ctx, ctrl.clientSet, content)
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Update storageBackendContent finalizer %s failed, error %v",
			content.Name, err)
		return err
	}

	if _, err = ctrl.updateContentStore(ctx, newObj); err != nil {
		log.AddContext(ctx).Errorf("update content store failed, error: %v", err)
		return err
	}
	return nil
}

func (ctrl *BackendController) updateStorageBackendContent(ctx context.Context,
	content *xuanwuv1.StorageBackendContent) (

	*xuanwuv1.StorageBackendContent, error) {
	content, err := utils.GetContent(ctx, ctrl.clientSet, content.Name)
	if err != nil {
		log.AddContext(ctx).Errorf("get content %s from cluster failed, error: %v", content.Name, err)
		return nil, err
	}

	newObj, err := utils.UpdateContent(ctx, ctrl.clientSet, content)
	if err != nil && apiErrors.IsResourceExpired(err) {
		log.AddContext(ctx).Errorf("Update storageBackendContent finalizer %s failed, error %v",
			content.Name, err)
		content, err = utils.GetContent(ctx, ctrl.clientSet, content.Name)
		if err != nil {
			log.AddContext(ctx).Errorf("get content %s from cluster failed, error: %v", content.Name, err)
			return nil, err
		}
		newObj, err = utils.UpdateContent(ctx, ctrl.clientSet, content)
	}

	if err != nil {
		return nil, err
	}

	return newObj, nil
}

func (ctrl *BackendController) deleteContent(ctx context.Context, backendContentName string) error {
	err := utils.DeleteContent(ctx, ctrl.clientSet, backendContentName)
	if err != nil && !apiErrors.IsNotFound(err) {
		msg := fmt.Sprintf("Failed to delete %s API object, error: %v", backendContentName, err)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (ctrl *BackendController) removeClaimFinalizer(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim) error {

	finalizers.RemoveFinalizer(storageBackend, utils.ClaimBoundFinalizer)
	log.AddContext(ctx).Infof("remove claim finalizer %s", utils.StorageBackendClaimKey(storageBackend))
	newObj, err := utils.UpdateClaim(ctx, ctrl.clientSet, storageBackend)
	if err != nil {
		log.AddContext(ctx).Errorf("update storageBackendClaim %s failed, error %v",
			utils.StorageBackendClaimKey(storageBackend), err)
		return err
	}

	if _, err = ctrl.updateClaimStore(ctx, newObj); err != nil {
		log.AddContext(ctx).Errorf("update claim store failed, error: %v", err)
		return err
	}

	return nil
}
