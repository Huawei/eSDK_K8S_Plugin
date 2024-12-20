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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/finalizers"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// syncClaimByKey processes a StorageBackendClaim request.
func (ctrl *BackendController) syncClaimByKey(ctx context.Context, objKey string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(objKey)
	log.AddContext(ctx).Infof("syncClaimByKey: namespace [%s] storageBackendClaim name [%s]", namespace, name)
	if err != nil {
		log.AddContext(ctx).Errorf("getting namespace & name of storageBackendClaim %s from "+
			"informer failed: %v", objKey, err)
		return nil
	}

	claim, err := ctrl.claimLister.StorageBackendClaims(namespace).Get(name)
	if err == nil {
		// the claim exists in informer cache, the handle event must be one of "create/update/sync"
		return ctrl.updateClaim(ctx, claim)
	}

	if err != nil && !apiErrors.IsNotFound(err) {
		log.AddContext(ctx).Errorf("getting storageBackendClaim %s from informer failed: %v", objKey, err)
		return err
	}

	claimObj, found, err := ctrl.claimStore.GetByKey(objKey)
	// the claim not in informer cache, the event must have been "delete"
	if err != nil || !found {
		log.AddContext(ctx).Warningf("the storageBackendClaim %s already deleted, found %v, error: %v",
			objKey, found, err)
		return nil
	}

	storageBackendClaim, ok := claimObj.(*xuanwuv1.StorageBackendClaim)
	if !ok {
		log.AddContext(ctx).Warningf("except StorageBackendClaim, got %+v", claimObj)
		return nil
	}

	return ctrl.deleteStorageBackendClaim(ctx, storageBackendClaim)
}

func (ctrl *BackendController) updateClaim(ctx context.Context, storageBackend *xuanwuv1.StorageBackendClaim) error {
	log.AddContext(ctx).Infof("updateClaim %s", utils.StorageBackendClaimKey(storageBackend))
	updated, err := ctrl.updateClaimStore(ctx, storageBackend)
	if err != nil {
		log.AddContext(ctx).Errorf("updateClaimStore error %v", err)
	}
	if !updated {
		return nil
	}

	if err = ctrl.syncClaim(ctx, storageBackend); err != nil {
		log.AddContext(ctx).Warningf("syncClaim %s failed, error: %v",
			utils.StorageBackendClaimKey(storageBackend), err)
		return err
	}

	return nil
}

func (ctrl *BackendController) syncClaim(ctx context.Context, storageBackend *xuanwuv1.StorageBackendClaim) error {
	log.AddContext(ctx).Infof("Start to syncClaim %s.", utils.StorageBackendClaimKey(storageBackend))
	defer log.AddContext(ctx).Infof("Finished syncClaim %s.", utils.StorageBackendClaimKey(storageBackend))

	syncTask := flow.NewTaskFlow(ctx, "Sync-StorageBackendClaim")
	syncTask.AddTask("Set-Claim-Status-Pending", ctrl.setClaimStatusTask, nil)
	syncTask.AddTask("Remove-Configmap-Finalizer", ctrl.removeConfigmapFinalizerTask, nil)
	syncTask.AddTask("Remove-Secret-Finalizer", ctrl.removeSecretFinalizerTask, nil)
	syncTask.AddTask("Delete-Claim", ctrl.deleteClaimTask, nil)
	syncTask.AddTask("Add-Claim-Finalizers", ctrl.addClaimFinalizersTask, nil)
	syncTask.AddTask("Create-Content", ctrl.createContentTask, nil)
	syncTask.AddTask("Update-Claim-Status", ctrl.updateClaimStatusTask, nil)
	syncTask.AddTask("Update-Claim", ctrl.updateClaimTask, nil)

	_, err := syncTask.Run(map[string]interface{}{
		"storageBackendClaim": storageBackend,
	})
	if err != nil {
		log.AddContext(ctx).Errorf("Run sync claim failed, error: %v", err)
		syncTask.Revert()
		return err
	}
	return nil
}

func (ctrl *BackendController) setClaimStatusPending(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim) (*xuanwuv1.StorageBackendClaim, error) {

	log.AddContext(ctx).Infof("setClaimStatusPending with claim %s.",
		utils.StorageBackendClaimKey(storageBackend))
	if storageBackend.Status == nil {
		storageBackend.Status = &xuanwuv1.StorageBackendClaimStatus{
			Phase: xuanwuv1.BackendPending,
		}
		return utils.UpdateClaimStatus(ctx, ctrl.clientSet, storageBackend)
	}

	return storageBackend, nil
}

func (ctrl *BackendController) setClaimStatusUnavailable(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim) (*xuanwuv1.StorageBackendClaim, error) {
	log.AddContext(ctx).Infof("setClaimStatusUnavailable with claim %s.",
		utils.StorageBackendClaimKey(storageBackend))

	storageBackend.Status = &xuanwuv1.StorageBackendClaimStatus{
		Phase: xuanwuv1.BackendUnavailable,
	}

	return utils.UpdateClaimStatus(ctx, ctrl.clientSet, storageBackend)
}

func (ctrl *BackendController) updateClaimStatusWithEvent(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim, reason, message string) (*xuanwuv1.StorageBackendClaim, error) {

	newClaim, err := utils.UpdateClaimStatus(ctx, ctrl.clientSet, storageBackend)
	if err != nil {
		return nil, err
	}

	ctrl.eventRecorder.Event(newClaim, coreV1.EventTypeNormal, reason, message)
	if _, err = ctrl.updateClaimStore(ctx, newClaim); err != nil {
		log.AddContext(ctx).Errorf("update claim %s status error: failed to update internal cache %v",
			utils.StorageBackendClaimKey(storageBackend), err)
		return nil, err
	}
	return newClaim, nil
}

func (ctrl *BackendController) addClaimFinalizer(ctx context.Context,
	storageBackend *xuanwuv1.StorageBackendClaim) error {

	finalizers.SetFinalizer(storageBackend, utils.ClaimBoundFinalizer)
	log.AddContext(ctx).Infof("add Claim %s Finalizer %s",
		utils.StorageBackendClaimKey(storageBackend), utils.ClaimBoundFinalizer)
	newObj, err := utils.UpdateClaim(ctx, ctrl.clientSet, storageBackend)
	if err != nil {
		log.AddContext(ctx).Errorf("update storageBackendClaim failed, error %v", err)
		return err
	}

	if _, err = ctrl.updateClaimStore(ctx, newObj); err != nil {
		log.AddContext(ctx).Errorf("update claim store failed, error: %v", err)
		return err
	}

	return nil
}

func (ctrl *BackendController) createContent(ctx context.Context, storageBackend *xuanwuv1.StorageBackendClaim) (
	*xuanwuv1.StorageBackendClaim, error) {

	log.AddContext(ctx).Infof("createContent with claim %s.", utils.StorageBackendClaimKey(storageBackend))
	configmap, err := ctrl.syncConfigmap(ctx, storageBackend)
	if err != nil {
		return storageBackend, err
	}

	var needUpdate bool
	configmapMeta, err := utils.GenObjectMetaKey(configmap)
	if err != nil {
		return storageBackend, err
	}

	if configmap != nil && storageBackend.Status.ConfigmapMeta != configmapMeta {
		storageBackend.Status.ConfigmapMeta = configmapMeta
		needUpdate = true
	}

	secret, err := ctrl.syncSecret(ctx, storageBackend)
	if err != nil {
		return storageBackend, err
	}

	secretMeta, err := utils.GenObjectMetaKey(secret)
	if err != nil {
		return storageBackend, err
	}

	if secret != nil && storageBackend.Status.SecretMeta != secretMeta {
		storageBackend.Status.SecretMeta = secretMeta
		needUpdate = true
	}

	if storageBackend.Spec.MaxClientThreads != "" &&
		storageBackend.Status.MaxClientThreads != storageBackend.Spec.MaxClientThreads {
		storageBackend.Status.MaxClientThreads = storageBackend.Spec.MaxClientThreads
		needUpdate = true
	}

	storageBackendContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.GenDynamicContentName(storageBackend),
		},

		Spec: xuanwuv1.StorageBackendContentSpec{
			Provider:         storageBackend.Spec.Provider,
			ConfigmapMeta:    configmapMeta,
			SecretMeta:       secretMeta,
			BackendClaim:     utils.StorageBackendClaimKey(storageBackend),
			MaxClientThreads: storageBackend.Spec.MaxClientThreads,
			Parameters:       storageBackend.Spec.Parameters,
		},
	}

	updateContent, err := utils.CreateContent(ctx, ctrl.clientSet, storageBackendContent)
	var getErr error
	if err != nil {
		updateContent, getErr = utils.GetContent(ctx, ctrl.clientSet, storageBackendContent.Name)
		if getErr != nil {
			log.AddContext(ctx).Errorf("Get storageBackendContent %s failed, error: %v",
				storageBackendContent.Name, err)
			return storageBackend, getErr
		}
		log.AddContext(ctx).Infof("storageBackendContent %s for storageBackendClaim %s already exist, reusing",
			storageBackendContent.Name, utils.StorageBackendClaimKey(storageBackend))
		err = nil
	}
	storageBackend.Status.BoundContentName = updateContent.Name

	ctrl.eventRecorder.Eventf(storageBackend, coreV1.EventTypeNormal, "CreatingStorageBackend",
		"Waiting for provider register the backend %s", utils.StorageBackendClaimKey(storageBackend))

	if _, err = ctrl.updateContentStore(ctx, updateContent); err != nil {
		log.AddContext(ctx).Errorf("failed to update content store, error: %v", err)
	}

	newClaim := storageBackend
	if needUpdate {
		newClaim, err = ctrl.updateClaimStatusWithEvent(ctx, storageBackend, "CreatedContent",
			"Successful created content for storageBackendClaim")
		if err != nil {
			log.AddContext(ctx).Errorf("update claim %s status failed, error: %v",
				utils.StorageBackendClaimKey(storageBackend), err)
			return storageBackend, err
		}
	}

	return newClaim, nil
}

func (ctrl *BackendController) setStorageBackendClaimStatus(ctx context.Context,
	newClaim *xuanwuv1.StorageBackendClaim) error {
	configmapData, err := backend.GetBackendConfigmapMap(ctx, newClaim.Spec.ConfigMapMeta)
	if err != nil {
		msg := fmt.Sprintf("GetBackendConfigmapMap: [%s] failed, error: %v", newClaim.Spec.ConfigMapMeta, err)
		return utils.Errorln(ctx, msg)
	}

	param, ok := configmapData["parameters"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("Parse parameters: [%s] to map[string]interface{} failed, error: %v",
			newClaim.Spec.ConfigMapMeta, err)
		return utils.Errorln(ctx, msg)
	}

	newClaim.Status.StorageType, _ = configmapData["storage"].(string)
	newClaim.Status.Protocol, _ = param["protocol"].(string)
	newClaim.Status.MetroBackend, _ = configmapData["metroBackend"].(string)

	log.AddContext(ctx).Infof("setStorageBackendClaimStatus, StorageType: [%s], Protocol: [%s], MetroBackend: [%s]",
		newClaim.Status.StorageType, newClaim.Status.Protocol, newClaim.Status.MetroBackend)
	return nil
}

func (ctrl *BackendController) updateStorageBackendClaimStatus(ctx context.Context,
	newClaim *xuanwuv1.StorageBackendClaim) (*xuanwuv1.StorageBackendClaim, error) {

	oldClaim, err := utils.GetClaim(ctx, ctrl.clientSet, newClaim)
	if err != nil {
		msg := fmt.Sprintf("get storageBackendClaim %s failed, error: %v", utils.StorageBackendClaimKey(newClaim), err)
		return nil, utils.Errorln(ctx, msg)
	}

	content, err := utils.GetContent(ctx, ctrl.clientSet, oldClaim.Status.BoundContentName)
	if err != nil {
		msg := fmt.Sprintf("get StorageBackendContents %s failed, error: %v", oldClaim.Status.BoundContentName, err)
		return nil, utils.Errorln(ctx, msg)
	}

	err = ctrl.setStorageBackendClaimStatus(ctx, newClaim)
	if err != nil {
		return nil, err
	}

	if !ctrl.isUpdateFinalClaimStatus(newClaim, content) {
		return newClaim, nil
	}

	newClaimObj, err := ctrl.updateClaimStatusWithEvent(ctx, newClaim, "UpdateStatus",
		"Successful update status for storageBackendClaim")
	if err != nil {
		log.AddContext(ctx).Errorf("updateStorageBackendClaimStatus: update claim %s status failed,"+
			" error: %v", newClaim.Name, err)
		return nil, err
	}

	return newClaimObj, nil
}

func (ctrl *BackendController) isUpdateFinalClaimStatus(
	claimObj *xuanwuv1.StorageBackendClaim, content *xuanwuv1.StorageBackendContent) bool {
	newStatus := claimObj.Status.DeepCopy()
	var changed bool
	if content.Status == nil || (content.Status.ContentName == "" && content.Status.VendorName == "") {
		return false
	}

	if content.Status.ContentName != "" && newStatus.StorageBackendId != content.Status.ContentName {
		newStatus.StorageBackendId = content.Status.ContentName
		changed = true
	}

	if content.Status.VendorName != "" && newStatus.Phase != xuanwuv1.BackendBound {
		newStatus.Phase = xuanwuv1.BackendBound
		changed = true
	}

	if newStatus.StorageType != "" || newStatus.Protocol != "" || newStatus.MetroBackend != "" {
		changed = true
	}

	claimObj.Status = newStatus
	return changed
}

func (ctrl *BackendController) setClaimStatusTask(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {

	storageBackend, ok := params["storageBackendClaim"].(*xuanwuv1.StorageBackendClaim)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain storageBackendClaim field.", params)
		return nil, utils.Errorln(ctx, msg)
	}

	newClaim, err := ctrl.setClaimStatusPending(ctx, storageBackend)
	if err != nil {
		msg := fmt.Sprintf("Update claim %s status to pending failed, error: %v",
			utils.StorageBackendClaimKey(storageBackend), err)
		return nil, utils.Errorln(ctx, msg)
	}

	return map[string]interface{}{
		"storageBackendClaim": newClaim,
	}, nil
}

func (ctrl *BackendController) removeConfigmapFinalizerTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	storageBackend, ok := taskResult["storageBackendClaim"].(*xuanwuv1.StorageBackendClaim)
	if !ok {
		msg := fmt.Sprintf("taskResult %v does not contain storageBackendClaim field.", taskResult)
		return nil, utils.Errorln(ctx, msg)
	}

	if err := ctrl.removeConfigmapFinalizer(ctx, storageBackend); err != nil {
		msg := fmt.Sprintf("Failed to check and remove ConfigMap Finalizer for StorageBackendClaim %s,"+
			" error: %v", utils.StorageBackendClaimKey(storageBackend), err)
		ctrl.eventRecorder.Event(storageBackend, coreV1.EventTypeWarning, "ErrorRemoveConfigMapFinalizer", msg)
		return nil, utils.Errorln(ctx, msg)
	}

	return nil, nil
}

func (ctrl *BackendController) removeSecretFinalizerTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	storageBackend, ok := taskResult["storageBackendClaim"].(*xuanwuv1.StorageBackendClaim)
	if !ok {
		msg := fmt.Sprintf("taskResult %v does not contain storageBackendClaim field.", taskResult)
		return nil, utils.Errorln(ctx, msg)
	}

	if err := ctrl.removeSecretFinalizer(ctx, storageBackend); err != nil {
		msg := fmt.Sprintf("Failed to check and remove Secret Finalizer for StorageBackendClaim %s,"+
			" error: %v", utils.StorageBackendClaimKey(storageBackend), err)
		ctrl.eventRecorder.Event(storageBackend, coreV1.EventTypeWarning, "ErrorRemoveSecretFinalizer", msg)
		return nil, utils.Errorln(ctx, msg)
	}

	return nil, nil
}

func (ctrl *BackendController) deleteClaimTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	storageBackend, ok := taskResult["storageBackendClaim"].(*xuanwuv1.StorageBackendClaim)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain storageBackendClaim field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if storageBackend != nil && storageBackend.ObjectMeta.DeletionTimestamp == nil {
		return nil, nil
	}

	log.AddContext(ctx).Infof("syncClaim: delete with claim %s.", utils.StorageBackendClaimKey(storageBackend))
	return nil, ctrl.processWithDeletionTimeStamp(ctx, storageBackend)
}

func (ctrl *BackendController) addClaimFinalizersTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	storageBackend, ok := taskResult["storageBackendClaim"].(*xuanwuv1.StorageBackendClaim)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain storageBackendClaim field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if !utils.NeedAddClaimBoundFinalizers(storageBackend) {
		return nil, nil
	}

	log.AddContext(ctx).Infof("syncClaim: addClaimFinalizer with claim %s.",
		utils.StorageBackendClaimKey(storageBackend))
	if err := ctrl.addClaimFinalizer(ctx, storageBackend); err != nil {
		msg := fmt.Sprintf("Failed to add bound Finalizer to StorageBackendClaim %s,"+
			" error: %v", utils.StorageBackendClaimKey(storageBackend), err)
		ctrl.eventRecorder.Event(storageBackend, coreV1.EventTypeWarning, "ErrorAddBoundFinalizer", msg)
		return nil, utils.Errorln(ctx, msg)
	}
	return nil, nil
}

func (ctrl *BackendController) createContentTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	storageBackend, ok := taskResult["storageBackendClaim"].(*xuanwuv1.StorageBackendClaim)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain storageBackendClaim field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if utils.IsClaimBoundContent(storageBackend) {
		return nil, nil
	}

	log.AddContext(ctx).Infof("syncClaim: createContent with claim %s.",
		utils.StorageBackendClaimKey(storageBackend))
	newClaim, err := ctrl.createContent(ctx, storageBackend)
	if err != nil {
		return nil, utils.Errorf(ctx, "Create StorageBackendContent %s failed, error %v",
			utils.GenDynamicContentName(storageBackend), err)
	}

	return map[string]interface{}{
		"storageBackendClaim": newClaim,
	}, nil
}

func (ctrl *BackendController) updateClaimStatusTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	storageBackend, ok := taskResult["storageBackendClaim"].(*xuanwuv1.StorageBackendClaim)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain storageBackendClaim field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if utils.IsClaimReady(storageBackend) {
		return nil, nil
	}

	log.AddContext(ctx).Infof("syncClaim: updateStorageBackendClaimStatus with claim %s.",
		utils.StorageBackendClaimKey(storageBackend))
	newClaim, err := ctrl.updateStorageBackendClaimStatus(ctx, storageBackend)
	if err != nil {
		return nil, utils.Errorf(ctx, "Update %s status with content failed, error %v",
			utils.StorageBackendClaimKey(storageBackend), err)
	}
	return map[string]interface{}{
		"storageBackendClaim": newClaim,
	}, nil
}

func (ctrl *BackendController) updateClaimTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	storageBackend, ok := taskResult["storageBackendClaim"].(*xuanwuv1.StorageBackendClaim)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain storageBackendClaim field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if !utils.NeedChangeContent(storageBackend) {
		return nil, nil
	}

	log.AddContext(ctx).Infof("syncClaim: updateClaimTask with claim %s.",
		utils.StorageBackendClaimKey(storageBackend))
	newClaim, err := ctrl.updateStorageBackendClaim(ctx, storageBackend)
	if err != nil {
		return nil, utils.Errorf(ctx, "Update claim %s with content failed, error %v",
			utils.StorageBackendClaimKey(storageBackend), err)
	}
	return map[string]interface{}{
		"storageBackendClaim": newClaim,
	}, nil
}

func (ctrl *BackendController) updateStorageBackendClaim(ctx context.Context, claim *xuanwuv1.StorageBackendClaim) (
	*xuanwuv1.StorageBackendClaim, error) {
	claim.Status.MaxClientThreads = claim.Spec.MaxClientThreads
	claim.Status.SecretMeta = claim.Spec.SecretMeta
	claim.Status.UseCert = claim.Spec.UseCert
	claim.Status.CertSecret = claim.Spec.CertSecret
	newClaim, err := ctrl.updateClaimStatusWithEvent(ctx, claim, "UpdateClaim",
		"Successful update claim for storageBackendClaim")
	if err != nil {
		log.AddContext(ctx).Errorf("updateStorageBackendClaim: update claim %s failed, error: %v",
			utils.StorageBackendClaimKey(claim), err)
		return nil, err
	}

	content, err := utils.GetContent(ctx, ctrl.clientSet, claim.Status.BoundContentName)
	if err != nil {
		log.AddContext(ctx).Errorf("updateStorageBackendClaim: get storageBackendContent %s failed, error: %v",
			claim.Status.BoundContentName, err)
		return nil, err
	}

	content.Spec.MaxClientThreads = claim.Spec.MaxClientThreads
	content.Spec.SecretMeta = claim.Spec.SecretMeta
	content.Spec.UseCert = claim.Spec.UseCert
	content.Spec.CertSecret = claim.Spec.CertSecret
	_, err = utils.UpdateContent(ctx, ctrl.clientSet, content)
	if err != nil {
		log.AddContext(ctx).Errorf("updateStorageBackendClaim: update storageBackendContent %s failed, "+
			"error: %v", claim.Status.BoundContentName, err)
		return nil, err
	}
	return newClaim, nil
}
