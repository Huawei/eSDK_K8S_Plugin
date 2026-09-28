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
	"reflect"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func (ctrl *backendController) initContentStatus(ctx context.Context, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	var newContent *xuanwuv1.StorageBackendContent
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var getErr error
		newContent, getErr = utils.GetContent(ctx, ctrl.clientSet, content.Name)
		if getErr != nil {
			return fmt.Errorf("initContentStatus: failed to get storageBackendContent %s from api server: %v",
				content.Name, getErr)
		}

		if newContent.Status != nil {
			return nil
		}

		newContent.Status = &xuanwuv1.StorageBackendContentStatus{
			ContentName:     "",
			VendorName:      "",
			ProviderVersion: "",
			Online:          true,
			Capabilities:    make(map[string]bool),
		}

		log.AddContext(ctx).Infof("Init content %s with status %+v.", newContent.Name, newContent.Status)
		var updateErr error
		newContent, updateErr = utils.UpdateContentStatus(ctx, ctrl.clientSet, newContent)
		return updateErr
	})
	if err != nil {
		return nil, err
	}
	return newContent, nil
}

// updateEvent holds the reason and message for the event recorded by updateContentStatusWithEvent.
type updateEvent struct {
	reason  string
	message string
}

// fetchFreshAndUpdateContent fetches fresh content from the API server, computes
// the diff via shouldUpdateContent, and calls updateContentStatusWithEvent if needed.
// This avoids stale informer cache values overwriting fields concurrently modified
// by other components (e.g., Online set by controller).
func (ctrl *backendController) fetchFreshAndUpdateContent(ctx context.Context,
	content *xuanwuv1.StorageBackendContent, status *drcsi.GetBackendStatsResponse,
	backendId string, event updateEvent) (*xuanwuv1.StorageBackendContent, error) {

	freshContent, err := utils.GetContent(ctx, ctrl.clientSet, content.Name)
	if err != nil {
		log.AddContext(ctx).Errorf("failed to get fresh content %s: %v", content.Name, err)
		return nil, err
	}
	if freshContent.Status == nil {
		freshContent.Status = &xuanwuv1.StorageBackendContentStatus{}
	}
	if !ctrl.shouldUpdateContent(ctx, freshContent, status, backendId) {
		return freshContent, nil
	}

	newContent, err := ctrl.updateContentStatusWithEvent(ctx, freshContent, event.reason, event.message)
	if err != nil {
		log.AddContext(ctx).Errorf("update content %s status failed, error: %v", content.Name, err)
		return nil, err
	}
	return newContent, nil
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

	return ctrl.fetchFreshAndUpdateContent(ctx, content, nil, backendId,
		updateEvent{reason: "CreatedContent", message: "Successful created content with provider"})
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
	defer log.AddContext(ctx).Debugf("Update content status %v", content.Status)

	var needUpdate bool
	if backendId != "" && content.Status.ContentName != backendId {
		content.Status.ContentName = backendId
		needUpdate = true
	}

	if content.Status.SecretMeta != content.Spec.SecretMeta {
		content.Status.SecretMeta = content.Spec.SecretMeta
		needUpdate = true
	}

	if content.Status.UseCert != content.Spec.UseCert {
		content.Status.UseCert = content.Spec.UseCert
		needUpdate = true
	}

	if content.Status.CertSecret != content.Spec.CertSecret {
		content.Status.CertSecret = content.Spec.CertSecret
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

	return ctrl.shouldUpdateContentStatus(ctx, content, status)
}
func (ctrl *backendController) shouldUpdateContentStatus(ctx context.Context, content *xuanwuv1.StorageBackendContent,
	status *drcsi.GetBackendStatsResponse) bool {

	log.AddContext(ctx).Debugf("content.Status: [%+v]; status: [%+v]",
		content.Status, status)

	if content.Status.VendorName != status.VendorName {
		content.Status.VendorName = status.VendorName
	}

	if content.Status.ProviderVersion != status.ProviderVersion {
		content.Status.ProviderVersion = status.ProviderVersion
	}

	if content.Status.Online != status.Online {
		content.Status.Online = status.Online
	}

	if content.Status.SN != status.Specifications["LocalDeviceSN"] {
		content.Status.SN = status.Specifications["LocalDeviceSN"]
	}

	if !reflect.DeepEqual(content.Status.Capabilities, status.Capabilities) {
		content.Status.Capabilities = status.Capabilities
	}

	if !reflect.DeepEqual(content.Status.Specification, status.Specifications) {
		content.Status.Specification = status.Specifications
	}

	if !reflect.DeepEqual(content.Status.Pools, status.Pools) {
		pools := make([]xuanwuv1.Pool, 0)
		for _, pool := range status.Pools {
			pools = append(pools, xuanwuv1.Pool{
				Name:       pool.GetName(),
				Capacities: pool.GetCapacities(),
			})
		}
		content.Status.Pools = pools
	}

	return true
}

func (ctrl *backendController) getContentStats(ctx context.Context, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	log.AddContext(ctx).Debugf("Start to get content status %s backendId %s within backend handler",
		content.Name, content.Status.ContentName)

	status, err := ctrl.handler.GetStorageBackendStats(ctx, content.Name, content.Spec.BackendClaim)
	if err != nil {
		log.AddContext(ctx).Errorf("getContentStats: get storage backend status for content %s, "+
			"return error: %v", content.Name, err)
		return nil, err
	}

	log.AddContext(ctx).Debugf("getContentStats status %+v", status)

	return ctrl.fetchFreshAndUpdateContent(ctx, content, status, "",
		updateEvent{reason: "UpdateContentStatus", message: "Successful update content status"})
}

func (ctrl *backendController) updateContentStatusWithEvent(ctx context.Context,
	content *xuanwuv1.StorageBackendContent, reason, message string) (*xuanwuv1.StorageBackendContent, error) {

	desiredStatus := content.Status.DeepCopy()
	var newContent *xuanwuv1.StorageBackendContent

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var getErr error
		newContent, getErr = utils.GetContent(ctx, ctrl.clientSet, content.Name)
		if getErr != nil {
			return getErr
		}

		newContent.Status = desiredStatus
		var updateErr error
		newContent, updateErr = utils.UpdateContentStatus(ctx, ctrl.clientSet, newContent)
		return updateErr
	})
	if err != nil {
		return nil, err
	}

	ctrl.eventRecorder.Event(newContent, coreV1.EventTypeNormal, reason, message)
	if _, err = ctrl.updateContentStore(ctx, newContent); err != nil {
		log.AddContext(ctx).Errorf("update content %s status error: failed to update internal cache %v",
			content.Name, err)
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

	return ctrl.fetchFreshAndUpdateContent(ctx, content, nil, "",
		updateEvent{reason: "UpdateContent", message: "Successful update content"})
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
