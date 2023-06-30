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
	"fmt"

	coreV1 "k8s.io/api/core/v1"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/pkg/finalizers"
	"huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

func (ctrl *BackendController) updateContent(ctx context.Context, content *xuanwuv1.StorageBackendContent) error {
	log.AddContext(ctx).Infof("updateContent %s", content.Name)
	updated, err := ctrl.updateContentStore(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("updateContentStore error %v", err)
	}
	if !updated {
		return nil
	}

	if err = ctrl.syncContent(ctx, content); err != nil {
		log.AddContext(ctx).Warningf("syncContent %s failed, error: %v", content.Name, err)
		return err
	}

	return nil
}

func (ctrl *BackendController) syncContent(ctx context.Context, content *xuanwuv1.StorageBackendContent) error {
	log.AddContext(ctx).Infof("Start to syncContent %s.", content.Name)
	if utils.NeedAddContentBoundFinalizers(content) {
		if err := ctrl.addContentFinalizer(ctx, content); err != nil {
			msg := fmt.Sprintf("Failed to add bound Finalizer to StorageBackendContent %s,"+
				" error: %v", content.Name, err)
			log.AddContext(ctx).Errorln(msg)
			ctrl.eventRecorder.Event(content, coreV1.EventTypeWarning, "ErrorAddBoundFinalizer", msg)
			return err
		}
	}

	if utils.NeedRemoveContentBoundFinalizers(content) {
		log.AddContext(ctx).Infof("remove Content Finalizer %v", content.Finalizers)
		err := ctrl.removeContentFinalizer(ctx, content)
		if err != nil {
			msg := fmt.Sprintf("Failed to remove %s finalizer, error: %v", content.Name, err)
			log.AddContext(ctx).Errorln(msg)
			return err
		}
	}

	claim, err := ctrl.getClaimFromStore(content.Spec.BackendClaim)
	if err != nil {
		return err
	}

	if claim != nil && ctrl.needUpdateClaimStatus(claim, content) {
		ctrl.claimQueue.Add(utils.StorageBackendClaimKey(claim))
	}

	return nil
}

func (ctrl *BackendController) addContentFinalizer(ctx context.Context, content *xuanwuv1.StorageBackendContent) error {
	finalizers.SetFinalizer(content, utils.ContentBoundFinalizer)
	newObj, err := utils.UpdateContent(ctx, ctrl.clientSet, content)
	if err != nil {
		log.AddContext(ctx).Errorf("update storageBackendContent failed, error %v", err)
		return err
	}

	if _, err = ctrl.updateContentStore(ctx, newObj); err != nil {
		log.AddContext(ctx).Errorf("update content store failed, error: %v", err)
		return err
	}

	return nil
}

func (ctrl *BackendController) getClaimFromStore(objKey string) (*xuanwuv1.StorageBackendClaim, error) {
	obj, exist, err := ctrl.claimStore.GetByKey(objKey)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, nil
	}

	claim, ok := obj.(*xuanwuv1.StorageBackendClaim)
	if !ok {
		return nil, fmt.Errorf("expected StorageBackendClaim, got %+v", obj)
	}
	return claim, nil
}

func (ctrl *BackendController) needUpdateClaimStatus(claim *xuanwuv1.StorageBackendClaim,
	content *xuanwuv1.StorageBackendContent) bool {

	if content.Status == nil {
		return false
	}

	return (claim.Status == nil) ||
		(claim.Status.BoundContentName == "") ||
		(claim.Status.StorageBackendId == "" && content.Status.ContentName != "") ||
		claim.Status.Phase != xuanwuv1.BackendBound
}
