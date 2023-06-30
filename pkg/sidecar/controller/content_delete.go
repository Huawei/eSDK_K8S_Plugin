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

// Package controller used deal with the backend backend content resources
package controller

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

func (ctrl *backendController) removeProviderBackend(ctx context.Context,
	content *xuanwuv1.StorageBackendContent) error {

	log.AddContext(ctx).Infof("removeProviderBackend %s started", content.Name)
	if content.Status == nil || content.Status.ContentName == "" {
		return nil
	}

	if err := ctrl.handler.DeleteStorageBackend(ctx, content.Spec.BackendClaim); err != nil {
		ctrl.eventRecorder.Event(content, v1.EventTypeWarning, "DeleteStorageBackend",
			"Failed to remove storage backend")
		return fmt.Errorf("failed to remove storage backend %#v, err: %v", content.Name, err)
	}

	if err := ctrl.clearContentStatus(ctx, content.Name); err != nil {
		ctrl.eventRecorder.Event(content, v1.EventTypeWarning, "DeleteStorageBackend",
			"Failed to clear content status")
		return err
	}

	return nil
}

func (ctrl *backendController) clearContentStatus(ctx context.Context, contentName string) error {
	content, err := utils.GetContent(ctx, ctrl.clientSet, contentName)
	if err != nil && apiErrors.IsNotFound(err) {
		log.AddContext(ctx).Infof("clearContentStatus content %s does not exist.", contentName)
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to get storageBackendContent %s from api server: %v", contentName, err)
	}

	if content.Status != nil {
		content.Status = &xuanwuv1.StorageBackendContentStatus{
			ContentName:     "",
			VendorName:      "",
			ProviderVersion: "",
			Capacity:        nil,
			Capabilities:    nil,
		}
	}

	_, err = ctrl.updateContentStatusWithEvent(
		ctx, content, "ClearContentStatus", "Successful clear content status")
	if err != nil && apiErrors.IsNotFound(err) {
		log.AddContext(ctx).Infof("clearContentStatus content %s does not exist.", contentName)
		return nil
	}

	if err != nil {
		log.AddContext(ctx).Errorf("update storageBackendContent status %s failed, error %v", content.Name, err)
		return err
	}
	return nil
}
