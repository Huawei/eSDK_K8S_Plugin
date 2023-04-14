/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.

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
	coreV1 "k8s.io/api/core/v1"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/lib/drcsi"
	"huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

func (ctrl *backendController) initContentStatus(ctx context.Context, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	newContent, err := utils.GetContent(ctx, ctrl.clientSet, content.Name)
	if err != nil {
		return nil, fmt.Errorf("initContentStatus: failed to get storageBackendContent %s from api server: %v",
			content.Name, err)
	}

	if newContent.Status != nil {
		return newContent, nil
	}

	if newContent.Status == nil {
		newContent.Status = &xuanwuv1.StorageBackendContentStatus{
			ContentName:     "",
			VendorName:      "",
			ProviderVersion: "",
			Online:          true,
			Capacity:        make(map[xuanwuv1.CapacityType]string),
			Capabilities:    make(map[string]bool),
		}
	}

	log.AddContext(ctx).Infof("Init content %s with status %v.", newContent.Name, newContent.Status)
	return utils.UpdateContentStatus(ctx, ctrl.clientSet, newContent)
}

func (ctrl *backendController) createContent(ctx context.Context, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	log.AddContext(ctx).Infof("createStorageBackendContent for content [%s]: started", content.Name)
	// step1. create content with the provider
	backendId, err := ctrl.createContentWrapper(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("createContent for content [%s]: error occurred in createContentWrapper: %v",
			content.Name, err)
		return nil, err
	}

	if !ctrl.shouldUpdateContent(ctx, content, nil, backendId) {
		return content, nil
	}

	newContent, err := ctrl.updateContentStatusWithEvent(
		ctx, content, "CreatedContent", "Successful created content with provider")
	if err != nil {
		log.AddContext(ctx).Errorf("createContent: update content %s status failed, error: %v",
			content.Name, err)
		return content, err
	}

	return newContent, nil
}

func (ctrl *backendController) createContentWrapper(ctx context.Context,
	content *xuanwuv1.StorageBackendContent) (string, error) {

	log.AddContext(ctx).Infof("Start to create content %s within backend handler", content.Name)
	providerName, backendId, err := ctrl.handler.CreateStorageBackend(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("createContentWrapper: create storage backend for content %s, "+
			"return error: %v", content.Name, err)
		return "", err
	}

	log.AddContext(ctx).Infof("Create storage backend: provider %s, backendId %s.", providerName, backendId)
	return backendId, nil
}

func (ctrl *backendController) shouldUpdateContent(ctx context.Context, content *xuanwuv1.StorageBackendContent,
	status *drcsi.GetBackendStatsResponse, backendId string) bool {
	defer log.AddContext(ctx).Infof("Update content status %s", content.Status)

	var needUpdate bool
	if backendId != "" && content.Status.ContentName != backendId {
		content.Status.ContentName = backendId
		needUpdate = true
	}

	if content.Status.SecretMeta != content.Spec.SecretMeta {
		content.Status.SecretMeta = content.Spec.SecretMeta
		needUpdate = true
	}

	if content.Status.MaxClientThreads != content.Spec.MaxClientThreads {
		content.Status.MaxClientThreads = content.Spec.MaxClientThreads
		needUpdate = true
	}

	if content.Status.ConfigmapMeta != content.Spec.ConfigmapMeta {
		content.Status.ConfigmapMeta = content.Spec.ConfigmapMeta
		needUpdate = true
	}

	if status == nil {
		log.AddContext(ctx).Infof("shouldUpdateContent: provider status is nil, needUpdate %v", needUpdate)
		return needUpdate
	}

	if content.Status.VendorName != status.VendorName {
		content.Status.VendorName = status.VendorName
	}

	if content.Status.ProviderVersion != status.ProviderVersion {
		content.Status.ProviderVersion = status.ProviderVersion
	}

	if content.Status.Online != status.Online {
		content.Status.Online = status.Online
	}

	log.AddContext(ctx).Infof("content.Status.Capabilities: [%v]; status.Capabilities: [%v]",
		content.Status, status.Capabilities)
	if status.Capabilities != nil {
		content.Status.Capabilities = status.Capabilities
	}

	if status.Specifications != nil {
		content.Status.Specification = status.Specifications
	}

	return true
}

func (ctrl *backendController) getContentStats(ctx context.Context, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	log.AddContext(ctx).Infof("Start to get content status %s backendId %s within backend handler",
		content.Name, content.Status.ContentName)

	status, err := ctrl.handler.GetStorageBackendStats(ctx, content.Name, content.Spec.BackendClaim)
	if err != nil {
		log.AddContext(ctx).Errorf("getContentStats: get storage backend status for content %s, "+
			"return error: %v", content.Name, err)
		return nil, err
	}

	log.AddContext(ctx).Infof("getContentStats status %v", status)
	if !ctrl.shouldUpdateContent(ctx, content, status, "") {
		return content, nil
	}

	newContent, err := ctrl.updateContentStatusWithEvent(
		ctx, content, "UpdateContentStatus", "Successful update content status")
	if err != nil {
		log.AddContext(ctx).Errorf("getContentStats: update content %s status failed, error: %v",
			content.Name, err)
		return content, err
	}

	return newContent, nil
}

func (ctrl *backendController) updateContentStatusWithEvent(ctx context.Context,
	content *xuanwuv1.StorageBackendContent, reason, message string) (*xuanwuv1.StorageBackendContent, error) {

	newContent, err := utils.UpdateContentStatus(ctx, ctrl.clientSet, content)
	if err != nil {
		return nil, err
	}

	ctrl.eventRecorder.Event(newContent, coreV1.EventTypeNormal, reason, message)
	if _, err = ctrl.updateContentStore(ctx, newContent); err != nil {
		log.AddContext(ctx).Errorf("update content %s status error: failed to update internal cache %v",
			newContent.Name, err)
		return nil, err
	}
	return newContent, nil
}

func (ctrl *backendController) updateContentObj(
	ctx context.Context, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	err := ctrl.updateContentWrapper(ctx, content)
	if err != nil {
		msg := fmt.Sprintf("Update the content %s from storage backend", content.Name)
		log.AddContext(ctx).Errorln(msg)
		return nil, err
	}

	if !ctrl.shouldUpdateContent(ctx, content, nil, "") {
		return content, nil
	}

	newContent, err := ctrl.updateContentStatusWithEvent(
		ctx, content, "UpdateContent", "Successful update content")
	if err != nil {
		log.AddContext(ctx).Errorf("updateContentObj: update content %s status failed, error: %v",
			content.Name, err)
		return nil, err
	}
	return newContent, nil
}

func (ctrl *backendController) updateContentWrapper(ctx context.Context,
	content *xuanwuv1.StorageBackendContent) error {

	log.AddContext(ctx).Infof("Start to update content %s within backend handler", content.Name)
	err := ctrl.handler.UpdateStorageBackend(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("updateContentWrapper: update storage backend for content %s, "+
			"return error: %v", content.Name, err)
		return err
	}

	log.AddContext(ctx).Infof("Update storage backend with content %s successful", content.Name)
	return nil
}
