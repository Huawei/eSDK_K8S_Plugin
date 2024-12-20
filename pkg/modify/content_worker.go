/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package modify contains claim resource controller definitions and synchronization functions
package modify

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	// ProtectContentFinalizer name of finalizer on VolumeModifyContent when be created
	ProtectContentFinalizer = "modify.xuanwu.huawei.io/volumemodifycontent-protection"

	// UpdateRetryTimes retry times
	UpdateRetryTimes = 10

	// UpdateRetryDelay retry delay
	UpdateRetryDelay = 100 * time.Millisecond
)

var (
	syncContentFunctions     []syncContentFunction
	onceInitContentFunctions sync.Once
)

type syncContentFunction func(context.Context, *xuanwuv1.VolumeModifyContent) (*xuanwuv1.VolumeModifyContent, error)

func (ctrl *VolumeModifyController) syncContentWork(ctx context.Context, name string) error {
	log.AddContext(ctx).Debugf("start sync VolumeModifyContent: %s", name)
	defer log.AddContext(ctx).Debugf("finish sync VolumeModifyContent: %s", name)
	content, err := ctrl.contentLister.Get(name)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.AddContext(ctx).Infof("content:%s is no longer exists, end this work", name)
			return nil
		}
		return fmt.Errorf("get content:%s from the indexer cache error: %v", name, err)
	}

	if content.ObjectMeta.DeletionTimestamp != nil {
		return ctrl.syncDeleteContent(ctx, content)
	}

	return ctrl.syncContent(ctx, content)
}

func (ctrl *VolumeModifyController) syncContent(ctx context.Context, content *xuanwuv1.VolumeModifyContent) error {
	// if claim is deleted, canceling content sync
	if content.Spec.VolumeModifyClaimName != "" {
		claim, err := ctrl.claimLister.Get(content.Spec.VolumeModifyClaimName)
		if err != nil && apiErrors.IsNotFound(err) || (err == nil && claim.DeletionTimestamp != nil) {
			log.AddContext(ctx).Infof("claim: %s is deleted, canceling content: %s sync", claim.Name, content.Name)
			return nil
		}
	}

	var err error
	syncFunctions := ctrl.initContentFunctions()
	for _, syncFunction := range syncFunctions {
		if content, err = syncFunction(ctx, content); err != nil {
			return err
		}
	}
	return nil
}

func (ctrl *VolumeModifyController) initContentFunctions() []syncContentFunction {
	onceInitContentFunctions.Do(func() {
		syncContentFunctions = append(syncContentFunctions,
			ctrl.setContentPending,
			ctrl.preCreateContent,
			ctrl.setContentFinalizers,
			ctrl.createContentStatus,
			ctrl.callVolumeModify,
		)
	})
	return syncContentFunctions
}

func (ctrl *VolumeModifyController) setContentPending(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent) (*xuanwuv1.VolumeModifyContent, error) {
	if content.Status.Phase != "" {
		return content, nil
	}

	defer log.AddContext(ctx).Infof("set content %s to %s", content.Name, xuanwuv1.VolumeModifyContentPending)
	contentClone := content.DeepCopy()
	contentClone.Status.Phase = xuanwuv1.VolumeModifyContentPending
	return ctrl.contentClient.XuanwuV1().VolumeModifyContents().UpdateStatus(ctx, contentClone, metav1.UpdateOptions{})
}

func (ctrl *VolumeModifyController) preCreateContent(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent) (*xuanwuv1.VolumeModifyContent, error) {
	if content.Status.Phase != xuanwuv1.VolumeModifyContentPending {
		return content, nil
	}
	if len(content.Spec.Parameters) == 0 {
		msg := "check spec failed: parameters is empty"
		ctrl.eventRecorder.Event(content, corev1.EventTypeWarning, CreateFailedReason, msg)
		return nil, errors.New(msg)
	}

	err := checkParameters(content.Spec.Parameters)
	if err != nil {
		ctrl.eventRecorder.Event(content, corev1.EventTypeWarning, CreateFailedReason, err.Error())
		return nil, fmt.Errorf("check content:%s parameters error: %w", content.Name, err)
	}
	return content, nil
}

func (ctrl *VolumeModifyController) setContentFinalizers(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent) (*xuanwuv1.VolumeModifyContent, error) {
	if content.Status.Phase != xuanwuv1.VolumeModifyContentPending {
		return content, nil
	}

	defer log.AddContext(ctx).Infof("content %s added finalizer ", content.Name)
	contentClone := content.DeepCopy()
	if utils.Contains(content.Finalizers, ProtectContentFinalizer) {
		return contentClone, nil
	}
	contentClone.ObjectMeta.Finalizers = append(contentClone.ObjectMeta.Finalizers, ProtectContentFinalizer)
	return ctrl.contentClient.XuanwuV1().VolumeModifyContents().Update(ctx, contentClone, metav1.UpdateOptions{})
}

func (ctrl *VolumeModifyController) createContentStatus(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent) (*xuanwuv1.VolumeModifyContent, error) {
	if content.Status.Phase != xuanwuv1.VolumeModifyContentPending {
		return content, nil
	}

	defer log.AddContext(ctx).Infof("set content %s to %s", content.Name, xuanwuv1.VolumeModifyContentCreating)
	ctrl.eventRecorder.Event(content, corev1.EventTypeNormal, StartCreatingContentReason,
		"External sidecar is processing volume modify")
	contentClone := content.DeepCopy()
	contentClone.Status.StartedAt = &metav1.Time{Time: time.Now().Local()}
	contentClone.Status.Phase = xuanwuv1.VolumeModifyContentCreating
	return ctrl.contentClient.XuanwuV1().VolumeModifyContents().UpdateStatus(ctx, contentClone, metav1.UpdateOptions{})
}

func (ctrl *VolumeModifyController) callVolumeModify(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent) (*xuanwuv1.VolumeModifyContent, error) {
	if content.Status.Phase != xuanwuv1.VolumeModifyContentCreating {
		return content, nil
	}

	request := &drcsi.ModifyVolumeRequest{
		VolumeId:               content.Spec.VolumeHandle,
		StorageClassParameters: content.Spec.StorageClassParameters,
		MutableParameters:      content.Spec.Parameters,
	}
	log.AddContext(ctx).Infof("call modify interface start, content:%s, request body: %+v",
		content.Name, request)
	response, err := ctrl.modifyClient.ModifyVolume(ctx, request)
	if err != nil {
		msg := fmt.Sprintf("call modify volume insterface error: %v", err)
		ctrl.eventRecorder.Event(content, corev1.EventTypeWarning, CreateFailedReason, msg)
		return nil, errors.New(msg)
	}
	log.AddContext(ctx).Infof("call modify interface end, content:%s, response body: %+v",
		content.Name, response)

	completedContent, err := ctrl.setContentToCompleted(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("set content to completed failed, error: %v", err)
		return nil, err
	}

	msg := fmt.Sprintf("the %s is modified successfully", completedContent.Spec.SourceVolume)
	ctrl.eventRecorder.Event(content, corev1.EventTypeNormal, StartCreatingContentReason, msg)
	log.AddContext(ctx).Infof("content:%s be updated to completed", completedContent.Name)
	return completedContent, nil
}

func (ctrl *VolumeModifyController) setContentToCompleted(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent) (*xuanwuv1.VolumeModifyContent, error) {
	contentClone := content.DeepCopy()
	contentClone.Status.CompletedAt = &metav1.Time{Time: time.Now().Local()}
	contentClone.Status.Phase = xuanwuv1.VolumeModifyContentCompleted
	updatedContent, err := ctrl.contentClient.XuanwuV1().VolumeModifyContents().UpdateStatus(ctx, contentClone,
		metav1.UpdateOptions{})
	if err == nil {
		return updatedContent, nil
	}
	log.AddContext(ctx).Infof("update content: %s status failed, error: %v", content.Name, err)

	// the update fails because the content is deleted or update, retry to update content
	content, retryErr := ctrl.updateStatusWithRetry(ctx, contentClone, UpdateRetryTimes)
	if retryErr == nil {
		return content, nil
	}

	// the content spec may be modified, the current change may not meet the content claim.
	// Therefore, the current change is rolled back.
	log.AddContext(ctx).Infof("retry update content: %s failed, start do rollback", content.Name)
	_, rollbackErr := ctrl.doRollback(ctx, contentClone)
	if rollbackErr != nil {
		log.AddContext(ctx).Errorf("do rollback failed, error: %v", rollbackErr)
	}
	return nil, fmt.Errorf("update content to completed error: %w", err)
}

func (ctrl *VolumeModifyController) updateStatusWithRetry(ctx context.Context, content *xuanwuv1.VolumeModifyContent,
	retryTimes int) (*xuanwuv1.VolumeModifyContent, error) {
	var err error
	var resultContent *xuanwuv1.VolumeModifyContent
	log.AddContext(ctx).Infof("start update content: %s", content.Name)

	for i := 0; i < retryTimes; i++ {
		resultContent, err = ctrl.contentLister.Get(content.Name)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				log.AddContext(ctx).Errorf("content is not in lister, return failed")
				return nil, err
			}
			log.AddContext(ctx).Infof("get content from lister failed, error: %v", err)
			continue
		}

		if err == nil && !canRetry(resultContent, content) {
			msg := "content spec is modified, abort retry"
			log.AddContext(ctx).Infoln(msg)
			return nil, errors.New(msg)
		}

		contentCopy := resultContent.DeepCopy()
		contentCopy.Status = content.Status
		resultContent, err = ctrl.contentClient.XuanwuV1().VolumeModifyContents().UpdateStatus(ctx, contentCopy,
			metav1.UpdateOptions{})
		if err == nil {
			log.AddContext(ctx).Infof("update content: %s success", content.Name)
			return resultContent, nil
		}

		// wait cache refreshing
		time.Sleep(UpdateRetryDelay)
		log.AddContext(ctx).Infof("retry to update content: %s, retry times: %d", content.Name, i+1)
	}

	log.AddContext(ctx).Errorf("retry to update content failed, error: %v", err)
	return resultContent, err
}

func (ctrl *VolumeModifyController) syncDeleteContent(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent) error {
	log.AddContext(ctx).Infof("start delete VolumeModifyContent: %s", content.Name)
	if content.Status.Phase == "" {
		return nil
	}
	contentClone := content.DeepCopy()
	if content.Status.Phase == xuanwuv1.VolumeModifyContentPending ||
		content.Status.Phase == xuanwuv1.VolumeModifyContentCreating {
		log.AddContext(ctx).Infof("content %s phase is %s, start deleting", content.Name, content.Status.Phase)
	} else {
		policy, ok := contentClone.Annotations[ContentReclaimPolicyKey]
		if ok && policy == ContentRollbackPolicy {
			var err error
			if contentClone, err = ctrl.rollbackContent(ctx, content); err != nil {
				return fmt.Errorf("rollback content:%s error: %w", content.Name, err)
			}
		} else {
			log.AddContext(ctx).Infof("content %s not rollback annotation, start deleting", content.Name)
		}
	}

	contentClone.ObjectMeta.Finalizers = utils.RemoveString(contentClone.ObjectMeta.Finalizers, ProtectContentFinalizer)
	_, err := ctrl.contentClient.XuanwuV1().VolumeModifyContents().Update(ctx, contentClone, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("remove finalizer failed during delete content:%s, error: %v", err, content.Name)
	}

	log.AddContext(ctx).Infof("finish remove VolumeModifyContent: %s finalizers", content.Name)
	return nil
}

func (ctrl *VolumeModifyController) rollbackContent(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent) (*xuanwuv1.VolumeModifyContent, error) {
	contentClone := content.DeepCopy()
	if contentClone.Status.Phase != xuanwuv1.VolumeModifyContentRollback {
		var err error
		contentClone.Status.Phase = xuanwuv1.VolumeModifyContentRollback
		contentClone, err = ctrl.updateStatusWithRetry(ctx, contentClone, UpdateRetryTimes)
		if err != nil {
			return contentClone, fmt.Errorf("update content:%s to rollback error: %v", content.Name, err)
		}
		log.AddContext(ctx).Infof("set content %s to %s", content.Name, xuanwuv1.VolumeModifyContentRollback)
	}

	return ctrl.doRollback(ctx, contentClone)
}

func (ctrl *VolumeModifyController) doRollback(ctx context.Context, content *xuanwuv1.VolumeModifyContent) (
	*xuanwuv1.VolumeModifyContent, error) {
	request := &drcsi.ModifyVolumeRequest{
		VolumeId:               content.Spec.VolumeHandle,
		StorageClassParameters: content.Spec.StorageClassParameters,
		MutableParameters:      generateRollbackParams(content.Spec.Parameters),
	}
	log.AddContext(ctx).Infof("start rollback content:%s, request body: %+v", content.Name, request)
	response, err := ctrl.modifyClient.ModifyVolume(ctx, request)
	if err != nil {
		msg := fmt.Sprintf("call modify volume interface to rollback error: %v", err)
		ctrl.eventRecorder.Event(content, corev1.EventTypeWarning, CreateFailedReason, msg)
		return content, errors.New(msg)
	}
	log.AddContext(ctx).Infof("end rollback content:%s, response body: %+v", content.Name, response)
	return content, nil
}

func generateRollbackParams(params map[string]string) map[string]string {
	rollback := make(map[string]string)
	if _, ok := params[HyperMetroFeatures]; ok {
		rollback[HyperMetroFeatures] = "false"
	}
	return rollback
}

func canRetry(new, old *xuanwuv1.VolumeModifyContent) bool {
	if new.DeletionTimestamp != nil {
		return true
	}

	if old.Spec.VolumeHandle != new.Spec.VolumeHandle {
		return false
	}

	if old.Spec.VolumeModifyClaimName != new.Spec.VolumeModifyClaimName {
		return false
	}

	if old.Spec.SourceVolume != new.Spec.SourceVolume {
		return false
	}

	if !reflect.DeepEqual(old.Spec.Parameters, new.Spec.Parameters) {
		return false
	}

	if !reflect.DeepEqual(old.Spec.StorageClassParameters, new.Spec.StorageClassParameters) {
		return false
	}

	return true
}
