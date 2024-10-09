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
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	pkgutils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	// ProtectClaimFinalizer name of finalizer on VolumeModifyClaim when be created
	ProtectClaimFinalizer = "modify.xuanwu.huawei.io/volumemodifyclaim-protection"

	// SourceStorageClassKind source kind of StorageClass
	SourceStorageClassKind = "StorageClass"

	// HyperMetroFeatures hyperMetro features
	HyperMetroFeatures = "hyperMetro"

	// MetroPairSyncSpeed creates hyper metro pair synchronization speed
	MetroPairSyncSpeed = "metroPairSyncSpeed"

	// CreateFailedReason reason of created claim failed
	CreateFailedReason = "CreatingFailed"

	// StartCreatingContentReason reason of claim is creating
	StartCreatingContentReason = "CreatingContent"

	// ReCreatingScReason reason of storageclass is recreating
	ReCreatingScReason = "RecreatingStorageClass"

	// ContentReclaimPolicyKey content reclaim policy
	ContentReclaimPolicyKey = "modify.xuanwu.io/reclaimPolicy"

	// ContentDeletePolicy delete content reclaim policy
	ContentDeletePolicy = "delete"

	// ContentRollbackPolicy rollback content reclaim policy
	ContentRollbackPolicy = "rollback"
)

var (
	supportFeatures        = []string{HyperMetroFeatures, MetroPairSyncSpeed}
	syncFuncList           []func(context.Context, *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error)
	onceInitSyncFuncList   sync.Once
	deleteFuncList         []func(context.Context, *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error)
	onceInitDeleteFuncList sync.Once
	featureCheckFunc       = map[string]func(string, map[string]string) error{
		HyperMetroFeatures: checkHyperMetro,
		MetroPairSyncSpeed: checkMetroPairSyncSpeed,
	}
)

func (ctrl *VolumeModifyController) syncClaimWork(ctx context.Context, name string) error {
	log.AddContext(ctx).Debugf("start sync VolumeModifyClaim: %s", name)
	defer log.AddContext(ctx).Debugf("finish sync VolumeModifyClaim: %s", name)
	claim, err := ctrl.claimLister.Get(name)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.AddContext(ctx).Infof("claim:%s is no longer exists, end this work", name)
			return nil
		}
		return fmt.Errorf("get claim:%s from the indexer cache error: %w", name, err)
	}

	if claim.DeletionTimestamp != nil {
		return ctrl.syncDeleteClaim(ctx, claim)
	}

	return ctrl.syncClaim(ctx, claim)
}

func (ctrl *VolumeModifyController) syncClaim(ctx context.Context, claim *xuanwuv1.VolumeModifyClaim) error {
	var err error
	syncFunctions := ctrl.initSyncFunctions()
	for _, syncFunction := range syncFunctions {
		if claim, err = syncFunction(ctx, claim); err != nil {
			return err
		}
	}
	return nil
}

func (ctrl *VolumeModifyController) initSyncFunctions() []func(context.Context, *xuanwuv1.VolumeModifyClaim) (
	*xuanwuv1.VolumeModifyClaim, error) {
	onceInitSyncFuncList.Do(func() {
		syncFuncList = append(syncFuncList,
			ctrl.setClaimPending,
			ctrl.preCreateClaim,
			ctrl.setClaimFinalizers,
			ctrl.setClaimCreating,
			ctrl.createClaimStatus,
			ctrl.waitClaimCompleted,
		)
	})
	return syncFuncList
}

func (ctrl *VolumeModifyController) setClaimPending(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error) {
	if claim.Status.Phase != "" {
		return claim, nil
	}

	defer log.AddContext(ctx).Infof("set claim %s to %s", claim.Name, xuanwuv1.VolumeModifyClaimPending)
	claimClone := claim.DeepCopy()
	claimClone.Status.Ready = generateReadyString(0, 0)
	return ctrl.updateClaimPhase(ctx, claimClone, xuanwuv1.VolumeModifyClaimPending)
}

func (ctrl *VolumeModifyController) preCreateClaim(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error) {
	if claim.Status.Phase != xuanwuv1.VolumeModifyClaimPending {
		return claim, nil
	}
	if err := ctrl.checkSource(ctx, claim); err != nil {
		return claim, err
	}

	err := checkParameters(claim.Spec.Parameters)
	if err != nil {
		ctrl.eventRecorder.Event(claim, corev1.EventTypeWarning, CreateFailedReason, err.Error())
		return claim, err
	}
	return claim, nil
}

func (ctrl *VolumeModifyController) setClaimFinalizers(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error) {
	if claim.Status.Phase != xuanwuv1.VolumeModifyClaimPending {
		return claim, nil
	}

	if utils.Contains(claim.Finalizers, ProtectClaimFinalizer) {
		return claim, nil
	}

	claimClone := claim.DeepCopy()
	claimClone.Finalizers = append(claimClone.Finalizers, ProtectClaimFinalizer)
	return ctrl.clientSet.XuanwuV1().VolumeModifyClaims().Update(ctx, claimClone, metav1.UpdateOptions{})
}

func (ctrl *VolumeModifyController) setClaimCreating(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error) {
	if claim.Status.Phase != xuanwuv1.VolumeModifyClaimPending {
		return claim, nil
	}

	defer log.AddContext(ctx).Infof("set claim %s to %s", claim.Name, xuanwuv1.VolumeModifyClaimCreating)
	ctrl.eventRecorder.Event(claim, corev1.EventTypeNormal, StartCreatingContentReason,
		"External sidecar is processing volume modify")
	claimClone := claim.DeepCopy()
	claimClone.Status.StartedAt = &metav1.Time{Time: time.Now().Local()}
	claimClone.Status.Phase = xuanwuv1.VolumeModifyClaimCreating
	return ctrl.clientSet.XuanwuV1().VolumeModifyClaims().UpdateStatus(ctx, claimClone, metav1.UpdateOptions{})
}

func (ctrl *VolumeModifyController) createClaimStatus(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error) {
	if len(claim.Status.Contents) != 0 ||
		claim.Status.Phase != xuanwuv1.VolumeModifyClaimCreating {
		return claim, nil
	}

	pvList, class, err := ctrl.fetchRefResources(ctx, claim)
	if err != nil {
		return claim, fmt.Errorf("fetchRefResources error: %w", err)
	}
	return ctrl.createContents(ctx, claim, pvList, class)
}

func (ctrl *VolumeModifyController) createContents(ctx context.Context, claim *xuanwuv1.VolumeModifyClaim,
	pvList *corev1.PersistentVolumeList, class *storagev1.StorageClass) (*xuanwuv1.VolumeModifyClaim, error) {
	claimClone := claim.DeepCopy()
	for _, volume := range pvList.Items {
		if volume.Spec.StorageClassName != claim.Spec.Source.Name {
			continue
		}

		if volume.Status.Phase != corev1.VolumeBound || volume.Spec.CSI == nil ||
			volume.Spec.CSI.VolumeHandle == "" || volume.Spec.ClaimRef == nil {
			log.AddContext(ctx).Infof("volume %s is not bound or claimRef does not exist.",
				pkgutils.MakeMetaWithNamespace(volume.Namespace, volume.Name))
			continue
		}

		newClaim, err := ctrl.claimLister.Get(claim.Name)
		if err == nil && newClaim.DeletionTimestamp != nil {
			log.AddContext(ctx).Infof("current claim is deleted, abandon content creation")
			break
		}

		content, err := ctrl.createContent(ctx, claim, volume, class)
		if err != nil {
			return nil, fmt.Errorf("create content error: %w", err)
		}
		log.AddContext(ctx).Infof("create content %s successful", content.Name)
		claimClone.Status.Contents = append(claimClone.Status.Contents, xuanwuv1.ModifyContents{
			ModifyContentName: content.Name,
			SourceVolume:      content.Spec.SourceVolume,
		})
	}

	if len(claimClone.Status.Contents) == 0 {
		msg := fmt.Sprintf("storageclass %s is not associsted with any bound pvc", claim.Spec.Source.Name)
		ctrl.eventRecorder.Event(claim, corev1.EventTypeNormal, CreateFailedReason, msg)
		log.AddContext(ctx).Infof(msg)
	}
	claimClone.Status.Parameters = claim.Spec.Parameters
	claimClone.Status.Ready = generateReadyString(0, len(claimClone.Status.Contents))
	return ctrl.updateClaimStatusWithRetry(ctx, claimClone, UpdateRetryTimes)
}

func (ctrl *VolumeModifyController) fetchRefResources(ctx context.Context, claim *xuanwuv1.VolumeModifyClaim) (
	*corev1.PersistentVolumeList, *storagev1.StorageClass, error) {
	list, err := ctrl.client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("fetch ref PVs error: %w", err)
	}

	class, err := ctrl.client.StorageV1().StorageClasses().Get(ctx, claim.Spec.Source.Name, metav1.GetOptions{})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			msg := fmt.Sprintf("create content failed: no storageclass available for this claim."+
				" current storageClass name is %s", claim.Spec.Source.Name)
			ctrl.eventRecorder.Event(claim, corev1.EventTypeWarning, CreateFailedReason, msg)
		}
		return nil, nil, fmt.Errorf("fetch storageclass error: %w", err)
	}

	return list, class, nil
}

func (ctrl *VolumeModifyController) waitClaimCompleted(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error) {
	if claim.Status.Phase != xuanwuv1.VolumeModifyClaimCreating {
		return claim, nil
	}

	var completed int
	claimClone := claim.DeepCopy()
	for i, content := range claim.Status.Contents {
		modifyContent, err := ctrl.contentLister.Get(content.ModifyContentName)
		if err != nil {
			return nil, fmt.Errorf("query content failed during wait claim completed, error: %w", err)
		}

		claimClone.Status.Contents[i].Status = modifyContent.Status.Phase
		if modifyContent.Status.Phase == xuanwuv1.VolumeModifyContentCompleted {
			completed++
		}
	}

	claimClone.Status.Ready = generateReadyString(completed, len(claim.Status.Contents))
	if completed != len(claim.Status.Contents) {
		if claim.Status.Ready != claimClone.Status.Ready {
			return ctrl.updateClaimStatusWithRetry(ctx, claimClone, UpdateRetryTimes)
		}
		ctrl.claimQueue.AddAfter(claim.Name, ctrl.reconcileClaimStatusDelay)
		return claim, nil
	}

	err := ctrl.replaceStorageClass(ctx, claim)
	if err != nil {
		msg := "replace storageclass failed, waiting for next retry"
		ctrl.eventRecorder.Event(claim, corev1.EventTypeWarning, ReCreatingScReason, msg)
		return nil, fmt.Errorf("replace storageclass error: %w", err)
	}

	claimClone.Status.Phase = xuanwuv1.VolumeModifyClaimCompleted
	claimClone.Status.CompletedAt = &metav1.Time{Time: time.Now().Local()}
	return ctrl.updateClaimStatusWithRetry(ctx, claimClone, UpdateRetryTimes)
}

func (ctrl *VolumeModifyController) replaceStorageClass(ctx context.Context, claim *xuanwuv1.VolumeModifyClaim) error {
	class, err := ctrl.getStorageClass(ctx, claim.Name, claim.Spec.Source.Name)
	if err != nil {
		return err
	}

	classClone := class.DeepCopy()
	needReplace := false
	for key, values := range claim.Status.Parameters {
		if classValue, ok := classClone.Parameters[key]; !ok || values != classValue {
			needReplace = true
			classClone.Parameters[key] = values
		}
	}

	if !needReplace {
		log.AddContext(ctx).Infof("storageclass:%s params are contained and does not need to be replaced, "+
			"params:%+v", classClone.Name, classClone.Parameters)
		return nil
	}

	return ctrl.deleteAndreCreateStorageClass(ctx, claim.Name, claim.Spec.Source.Name, class, classClone)
}

func (ctrl *VolumeModifyController) getStorageClass(ctx context.Context, claimName,
	scName string) (*storagev1.StorageClass, error) {
	class, err := ctrl.client.StorageV1().StorageClasses().Get(ctx, scName, metav1.GetOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		return nil, fmt.Errorf("get storageclass name: %s error: %w", scName, err)
	}
	if err == nil {
		// storageclass information will be deleted later
		log.AddContext(ctx).Infof("get storageclass info: %+v", class)
		return class, nil
	}

	backupScName := generateBackupScName(claimName, scName)
	class, err = ctrl.client.StorageV1().StorageClasses().Get(ctx, backupScName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get backup storageclass name: %s error: %w", scName, err)
	}

	log.AddContext(ctx).Infof("get backup storageclass info: %+v", class)
	return class, nil
}

func (ctrl *VolumeModifyController) deleteAndreCreateStorageClass(ctx context.Context, claimName, oldScName string,
	old, create *storagev1.StorageClass) error {
	backupScName := generateBackupScName(claimName, oldScName)
	backupClassClone := old.DeepCopy()
	backupClassClone.Name = backupScName
	backupClassClone.ResourceVersion = ""
	sc, err := ctrl.client.StorageV1().StorageClasses().Create(ctx, backupClassClone, metav1.CreateOptions{})
	if err != nil && !apiErrors.IsAlreadyExists(err) {
		return fmt.Errorf("create backup storageclass:%s error: %w", backupClassClone.Name, err)
	}
	log.AddContext(ctx).Infof("create backup sc %s successful, params: %+v", sc.Name, sc.Parameters)

	err = ctrl.client.StorageV1().StorageClasses().Delete(ctx, oldScName, metav1.DeleteOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		return fmt.Errorf("delete old storageclass:%s error: %w", oldScName, err)
	}
	log.AddContext(ctx).Infof("delete source sc %s successful", oldScName)

	createCopy := create.DeepCopy()
	createCopy.ResourceVersion = ""
	newSc, err := ctrl.client.StorageV1().StorageClasses().Create(ctx, createCopy, metav1.CreateOptions{})
	if err != nil && !apiErrors.IsAlreadyExists(err) {
		return fmt.Errorf("create new storageclass:%s error: %w", createCopy.Name, err)
	}
	log.AddContext(ctx).Infof("create new sc %s successful, params: %+v", newSc.Name, newSc.Parameters)

	// delete backup sc
	err = ctrl.client.StorageV1().StorageClasses().Delete(ctx, backupScName, metav1.DeleteOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		// If the backup class fails to be deleted, an error is returned, causing the SC to be deleted and recreated.
		// Therefore, only logs are recorded and a success message is returned.
		log.AddContext(ctx).Warningf("delete backup sc name: %s, error: %v", backupScName, err)
	}

	log.AddContext(ctx).Infof("delete backup sc %s successful", backupScName)
	return err
}

func (ctrl *VolumeModifyController) createContent(ctx context.Context, claim *xuanwuv1.VolumeModifyClaim,
	volume corev1.PersistentVolume, class *storagev1.StorageClass) (*xuanwuv1.VolumeModifyContent, error) {
	contentName := claim.Name + "-" + string(volume.Spec.ClaimRef.UID)
	existContent, err := ctrl.contentLister.Get(contentName)
	if err != nil && !apiErrors.IsNotFound(err) {
		return nil, fmt.Errorf("get content from lister error: %w", err)
	}

	if existContent != nil {
		return existContent.DeepCopy(), nil
	}

	content := &xuanwuv1.VolumeModifyContent{
		ObjectMeta: metav1.ObjectMeta{Name: contentName},
		Spec: xuanwuv1.VolumeModifyContentSpec{
			VolumeModifyClaimName:  claim.Name,
			VolumeHandle:           volume.Spec.CSI.VolumeHandle,
			Parameters:             claim.Spec.Parameters,
			StorageClassParameters: class.Parameters,
			SourceVolume: pkgutils.MakeMetaWithNamespace(volume.Spec.ClaimRef.Namespace,
				volume.Spec.ClaimRef.Name),
		},
	}
	return ctrl.clientSet.XuanwuV1().VolumeModifyContents().Create(ctx, content, metav1.CreateOptions{})
}

func (ctrl *VolumeModifyController) checkSource(ctx context.Context, claim *xuanwuv1.VolumeModifyClaim) error {
	if claim.Spec.Source.Kind != SourceStorageClassKind {
		msg := "check spec failed: spec.source.kind is not 'StorageClass'"
		ctrl.eventRecorder.Event(claim, corev1.EventTypeWarning, CreateFailedReason, msg)
		return errors.New(msg)
	}

	class, err := ctrl.client.StorageV1().StorageClasses().Get(ctx, claim.Spec.Source.Name, metav1.GetOptions{})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			notFoundErr := fmt.Errorf("check spec failed: the storageclass: %s does not exist",
				claim.Spec.Source.Name)
			ctrl.eventRecorder.Event(claim, corev1.EventTypeWarning, CreateFailedReason, notFoundErr.Error())
		}
		return fmt.Errorf("query storage class error: %w", err)
	}

	if class != nil && class.Provisioner != ctrl.provisioner {
		msg := fmt.Sprintf("check spec failed: storageclass:%s provisioner is not %s",
			class.Name, ctrl.provisioner)
		ctrl.eventRecorder.Event(claim, corev1.EventTypeWarning, CreateFailedReason, msg)
		return errors.New(msg)
	}

	return nil
}

func (ctrl *VolumeModifyController) updateClaimPhase(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim, phase xuanwuv1.VolumeModifyClaimPhase) (*xuanwuv1.VolumeModifyClaim, error) {
	claimClone := claim.DeepCopy()
	claimClone.Status.Phase = phase
	return ctrl.clientSet.XuanwuV1().VolumeModifyClaims().UpdateStatus(ctx, claimClone, metav1.UpdateOptions{})
}

func (ctrl *VolumeModifyController) findNoSupportFeatures(params map[string]string) []string {
	var notSupports []string
	supports := make(map[string]string)
	for key, value := range params {
		if !utils.Contains(supportFeatures, key) {
			notSupports = append(notSupports, key)
			continue
		}
		supports[key] = value
	}

	return notSupports
}

func generateReadyString(completed, total int) string {
	return strconv.Itoa(completed) + "/" + strconv.Itoa(total)
}

func generateBackupScName(claimName, scName string) string {
	return scName + "-" + claimName
}

func (ctrl *VolumeModifyController) syncDeleteClaim(ctx context.Context, claim *xuanwuv1.VolumeModifyClaim) error {
	onceInitDeleteFuncList.Do(func() {
		deleteFuncList = append(deleteFuncList,
			ctrl.deleteContent,
			ctrl.waitClaimDelete,
		)
	})

	var err error
	for _, deleteFunction := range deleteFuncList {
		if claim, err = deleteFunction(ctx, claim); err != nil {
			return err
		}
	}
	return nil
}

func (ctrl *VolumeModifyController) getRefContentsFromLister(ctx context.Context,
	claimName string) ([]*xuanwuv1.VolumeModifyContent, error) {
	allContents, err := ctrl.contentLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to get reference contents of cliam from lister, error: %w", err)
	}

	var contentsOfClaim []*xuanwuv1.VolumeModifyContent
	for _, content := range allContents {
		if content.Spec.VolumeModifyClaimName == claimName {
			contentsOfClaim = append(contentsOfClaim, content)
		}
	}

	return contentsOfClaim, nil
}

func (ctrl *VolumeModifyController) deleteContent(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error) {
	if claim.Status.Phase == "" || claim.Status.Phase == xuanwuv1.VolumeModifyClaimDeleting ||
		claim.Status.Phase == xuanwuv1.VolumeModifyClaimRollback {
		return claim, nil
	}

	log.AddContext(ctx).Infof("start delete ref content")
	contents, err := ctrl.getRefContentsFromLister(ctx, claim.Name)
	if err != nil {
		return nil, fmt.Errorf("query content list error: %w", err)
	}

	phase := xuanwuv1.VolumeModifyClaimDeleting
	if claim.Status.Phase == xuanwuv1.VolumeModifyClaimCreating {
		phase = xuanwuv1.VolumeModifyClaimRollback
		for _, content := range contents {
			if content.DeletionTimestamp != nil {
				continue
			}

			if err := ctrl.addAnnotationToContent(ctx, content, ContentRollbackPolicy, UpdateRetryTimes); err != nil {
				return nil, fmt.Errorf("add annotation to content failed: %w", err)
			}

			if err := ctrl.deleteRefContent(ctx, content); err != nil {
				return nil, fmt.Errorf("delete reference content failed: %w", err)
			}
		}
	} else {
		for _, content := range contents {
			if content.DeletionTimestamp != nil {
				continue
			}
			if err := ctrl.deleteRefContent(ctx, content); err != nil {
				return nil, fmt.Errorf("delete reference content failed: %w", err)
			}
		}
	}

	claimCopy := claim.DeepCopy()
	claimCopy.Status.Phase = phase
	claim, err = ctrl.updateClaimStatusWithRetry(ctx, claimCopy, UpdateRetryTimes)
	if err != nil {
		log.AddContext(ctx).Infof("update claim to %s failed, error: %v", phase, err)
		return nil, err
	}

	log.AddContext(ctx).Infof("set claim %s to %s", claim.Name, claim.Status.Phase)
	return claim, nil
}

func (ctrl *VolumeModifyController) addAnnotationToContent(ctx context.Context,
	content *xuanwuv1.VolumeModifyContent, value string, retryTimes int) error {
	log.AddContext(ctx).Infof("start to add annotation to content %s", content.Name)
	if policy, ok := content.Annotations[ContentReclaimPolicyKey]; ok && policy == value {
		return nil
	}

	var err error
	var resultContent *xuanwuv1.VolumeModifyContent
	for i := 0; i < retryTimes; i++ {
		resultContent, err = ctrl.contentLister.Get(content.Name)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				log.AddContext(ctx).Errorf("content is not in lister, return failed")
				return err
			}
			log.AddContext(ctx).Infof("get content from lister failed, error: %v", err)
			continue
		}

		if policy, ok := resultContent.Annotations[ContentReclaimPolicyKey]; ok && policy == value {
			return nil
		}

		contentCopy := resultContent.DeepCopy()
		if contentCopy.Annotations == nil {
			contentCopy.Annotations = map[string]string{}
		}

		contentCopy.Annotations[ContentReclaimPolicyKey] = value
		_, err = ctrl.clientSet.XuanwuV1().VolumeModifyContents().Update(ctx, contentCopy, metav1.UpdateOptions{})
		if err == nil {
			log.AddContext(ctx).Infof("update content: %s annotation success", content.Name)
			return nil
		}

		// wait cache refreshing
		time.Sleep(UpdateRetryDelay)
		log.AddContext(ctx).Infof("retry to update content: %s annotation, retry times: %d", content.Name, i+1)
	}

	return err
}

func (ctrl *VolumeModifyController) deleteRefContent(ctx context.Context, content *xuanwuv1.VolumeModifyContent) error {
	log.AddContext(ctx).Infof("start delete claim:%s ref content:%s", content.Spec.VolumeModifyClaimName,
		content.Name)

	err := ctrl.clientSet.XuanwuV1().VolumeModifyContents().Delete(ctx, content.Name,
		metav1.DeleteOptions{})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.AddContext(ctx).Infof("claim:%s ref content:%s has been deleted",
				content.Spec.VolumeModifyClaimName, content.Name)
			return nil
		}
		return fmt.Errorf("claim:%s delete ref content:%s error:%w", content.Spec.VolumeModifyClaimName,
			content.Name, err)
	}

	log.AddContext(ctx).Infof("delete claim:%s ref content:%s finish", content.Spec.VolumeModifyClaimName,
		content.Name)
	return nil
}

func (ctrl *VolumeModifyController) waitClaimDelete(ctx context.Context,
	claim *xuanwuv1.VolumeModifyClaim) (*xuanwuv1.VolumeModifyClaim, error) {
	if claim.Status.Phase != xuanwuv1.VolumeModifyClaimDeleting &&
		claim.Status.Phase != xuanwuv1.VolumeModifyClaimRollback {
		return claim, nil
	}

	contents, err := ctrl.getRefContentsFromLister(ctx, claim.Name)
	if err != nil {
		return nil, fmt.Errorf("query content list error: %w", err)
	}
	if len(contents) > 0 {
		ctrl.claimQueue.AddAfter(claim.Name, ctrl.reconcileClaimStatusDelay)
		return claim, nil
	}

	defer log.AddContext(ctx).Infof("remove claim %s finalizers successful", claim.Name)
	claimClone := claim.DeepCopy()
	claimClone.Finalizers = utils.RemoveString(claimClone.Finalizers, ProtectClaimFinalizer)
	return ctrl.clientSet.XuanwuV1().VolumeModifyClaims().Update(ctx, claimClone, metav1.UpdateOptions{})
}

func checkParameters(params map[string]string) error {
	if len(params) == 0 {
		return errors.New("check spec failed: parameters is empty")
	}

	var notSupports []string
	supports := make(map[string]string)
	for key, value := range params {
		if !utils.Contains(supportFeatures, key) {
			notSupports = append(notSupports, key)
			continue
		}
		if check, ok := featureCheckFunc[key]; ok {
			if err := check(value, params); err != nil {
				return err
			}
		}
		supports[key] = value
	}

	if len(notSupports) == 0 {
		return nil
	}

	return fmt.Errorf("check spec failed: parameters [%s] are not supported. only [%s] are supported",
		strings.Join(notSupports, ","), strings.Join(supportFeatures, ","))
}

func checkHyperMetro(value string, _ map[string]string) error {
	if value == "true" {
		return nil
	}
	return fmt.Errorf("check spec failed: paramter hyperMetro can only be set to 'true'")
}

func checkMetroPairSyncSpeed(value string, params map[string]string) error {
	if hyperMetro, exist := params[HyperMetroFeatures]; !exist || hyperMetro != "true" {
		return fmt.Errorf("check spec failed: " +
			"parameter metroPairSyncSpeed can be configured only when hyperMetro is set to true")
	}

	speed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("check spec failed: paramter metroPairSyncSpeed can only be an integer")
	}

	if speed < client.MetroPairSyncSpeedLow || speed > client.MetroPairSyncSpeedHighest {
		return fmt.Errorf(
			"check spec failed: paramter metroPairSyncSpeed must be between %d and %d, but got [%d]",
			client.MetroPairSyncSpeedLow, client.MetroPairSyncSpeedHighest, speed)
	}

	return nil
}

func (ctrl *VolumeModifyController) updateClaimStatusWithRetry(ctx context.Context, claim *xuanwuv1.VolumeModifyClaim,
	retryTimes int) (*xuanwuv1.VolumeModifyClaim, error) {
	var err error
	var resultClaim *xuanwuv1.VolumeModifyClaim
	log.AddContext(ctx).Infof("start update claim: %s", claim.Name)

	for i := 0; i < retryTimes; i++ {
		resultClaim, err = ctrl.claimLister.Get(claim.Name)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				log.AddContext(ctx).Errorf("claim is not in lister, return failed")
				return nil, err
			}
			log.AddContext(ctx).Infof("get claim from lister failed, error: %v", err)
			continue
		}

		claimCopy := resultClaim.DeepCopy()
		claimCopy.Status = claim.Status
		resultClaim, err = ctrl.contentClient.XuanwuV1().VolumeModifyClaims().UpdateStatus(ctx, claimCopy,
			metav1.UpdateOptions{})
		if err == nil {
			log.AddContext(ctx).Infof("update claim: %s success", claim.Name)
			return resultClaim, nil
		}

		// wait cache refreshing
		time.Sleep(UpdateRetryDelay)
		log.AddContext(ctx).Infof("retry to update claim: %s, retry times: %d", claim.Name, i+1)
	}

	log.AddContext(ctx).Errorf("update claim failed, error: %v", err)
	return resultClaim, err
}
