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
	"errors"
	"flag"
	"fmt"
	"time"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"huawei-csi-driver/pkg/utils"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	backendInformers "huawei-csi-driver/pkg/client/informers/externalversions/xuanwu/v1"
	backendListers "huawei-csi-driver/pkg/client/listers/xuanwu/v1"
	storageBackend "huawei-csi-driver/pkg/storage-backend/handle"
	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/taskflow"
)

var (
	retryIntervalStart = flag.Duration(
		"retry-interval-start",
		5*time.Second,
		"Initial retry interval of failed storageBackend creation or deletion. "+
			"It doubles with each failure, up to retry-interval-max.")
	retryIntervalMax = flag.Duration(
		"retry-interval-max",
		5*time.Minute,
		"Maximum retry interval of failed storageBackend creation or deletion.")
	provisionTimeout = flag.Duration(
		"provision-timeout",
		5*time.Minute,
		"The timeout of the provision storage backend.")
)

type backendController struct {
	providerName string

	clientSet     clientSet.Interface
	eventRecorder record.EventRecorder
	reSyncPeriod  time.Duration

	contentQueue      workqueue.RateLimitingInterface
	contentListerSync cache.InformerSynced
	contentLister     backendListers.StorageBackendContentLister
	contentStore      cache.Store

	handler Handler
}

// BackendControllerRequest is a request for new controller
type BackendControllerRequest struct {
	// provider name
	ProviderName string
	// storage backend client
	ClientSet clientSet.Interface
	// storageBackend interfaces
	Backend storageBackend.BackendInterfaces
	// provider time out
	TimeOut time.Duration
	// storage backend content informer
	ContentInformer backendInformers.StorageBackendContentInformer
	// reSync period time
	ReSyncPeriod time.Duration
	// event recorder
	EventRecorder record.EventRecorder
}

// NewSideCarBackendController return a new *backendController
func NewSideCarBackendController(request BackendControllerRequest) *backendController {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(*retryIntervalStart, *retryIntervalMax)
	ctrl := &backendController{
		providerName:  request.ProviderName,
		clientSet:     request.ClientSet,
		eventRecorder: request.EventRecorder,
		reSyncPeriod:  request.ReSyncPeriod,
		contentQueue:  workqueue.NewNamedRateLimitingQueue(rateLimiter, "sidecar-backend-controller-content"),
		contentStore:  cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		handler:       NewCDRHandler(request.Backend, request.TimeOut),
	}

	request.ContentInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) { ctrl.enqueueContent(obj) },
			UpdateFunc: func(oldObj, newObj interface{}) {
				newContent, ok := newObj.(*xuanwuv1.StorageBackendContent)
				if !ok {
					log.Warningf("newObj %v can not convert to StorageBackendContent", newObj)
					return
				}

				oldContent, ok := oldObj.(*xuanwuv1.StorageBackendContent)
				if !ok {
					log.Warningf("oldObj %v can not convert to StorageBackendContent", oldObj)
					return
				}

				if oldContent.ResourceVersion == newContent.ResourceVersion {
					// Periodic resync will send update events for all known StorageBackendContent.
					// Two different versions of the same StorageBackendContent will always have different RVs.
					return
				}
				ctrl.enqueueContent(newObj)
			},
			DeleteFunc: func(obj interface{}) { ctrl.enqueueContent(obj) },
		},
	)
	ctrl.contentLister = request.ContentInformer.Lister()
	ctrl.contentListerSync = request.ContentInformer.Informer().HasSynced
	return ctrl
}

func (ctrl *backendController) enqueueContent(obj interface{}) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}

	if content, ok := obj.(*xuanwuv1.StorageBackendContent); ok {
		objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(content)
		if err != nil {
			log.Errorf("failed to get key from object: %v, %v", content, err)
			return
		}
		log.Infof("enqueued StorageBackendContent %q for sync", objName)
		ctrl.contentQueue.Add(objName)
	}
}

// Run defines the sidecar controller process
func (ctrl *backendController) Run(ctx context.Context, workers int, stopCh <-chan struct{}) {
	defer ctrl.contentQueue.ShutDown()

	log.AddContext(ctx).Infoln("Starting sidecar storage backend")
	defer log.AddContext(ctx).Infoln("Shutting down sidecar storage backend")

	if !cache.WaitForCacheSync(stopCh, ctrl.contentListerSync) {
		log.AddContext(ctx).Errorln("Cannot sync caches")
		return
	}

	ctrl.initializeCaches(ctx, ctrl.contentLister)
	for i := 0; i < workers; i++ {
		go wait.Until(ctrl.runContentWorker, time.Second, stopCh)
	}

	if stopCh != nil {
		sign := <-stopCh
		log.AddContext(ctx).Infof("Backend Sidecar exited, reason: %v", sign)
	}
}

func (ctrl *backendController) initializeCaches(ctx context.Context,
	contentLister backendListers.StorageBackendContentLister) {

	contentList, err := contentLister.List(labels.Everything())
	if err != nil {
		log.AddContext(ctx).Errorf("StorageBackend claim initialize failed, error: %v", err)
		return
	}

	for _, content := range contentList {
		if !ctrl.isMatchProvider(content) {
			continue
		}
		contentClone := content.DeepCopy()
		if _, err := ctrl.updateContentStore(ctx, contentClone); err != nil {
			log.AddContext(ctx).Errorf("Update content cache failed, error: %v", err)
		}
	}
	log.AddContext(ctx).Infoln("sidecar controller initialized")
}

func (ctrl *backendController) isMatchProvider(content *xuanwuv1.StorageBackendContent) bool {
	return content.Spec.BackendClaim != "" && content.Spec.Provider == ctrl.providerName
}

func (ctrl *backendController) updateContentStore(ctx context.Context, content interface{}) (bool, error) {
	return utils.StoreObjectUpdate(ctx, ctrl.contentStore, content, "storageBackendContent")
}

func (ctrl *backendController) runContentWorker() {
	for ctrl.processNextContentWorkItem() {
	}
}

func (ctrl *backendController) processNextContentWorkItem() bool {
	obj, shutdown := ctrl.contentQueue.Get()
	if shutdown {
		log.Infof("processNextContentWorkItem obj: [%v], shutdown: [%v]", obj, shutdown)
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), *provisionTimeout)
	defer cancel()
	defer ctrl.contentQueue.Done(obj)
	if err := ctrl.handleContentWork(ctx, obj); err != nil {
		utilRuntime.HandleError(err)
		return true
	}
	return true
}

func (ctrl *backendController) handleContentWork(ctx context.Context, obj interface{}) error {
	objKey, ok := obj.(string)
	if !ok {
		ctrl.contentQueue.Forget(obj)
		msg := fmt.Sprintf("expected string in content workqueue but got %#v", obj)
		log.AddContext(ctx).Errorf(msg)
		return errors.New(msg)
	}

	if err := ctrl.syncContentByKey(ctx, objKey); err != nil {
		log.AddContext(ctx).Errorf("handleContentWork: sync storageBackendContent %s failed,"+
			" error: %v", objKey, err)
		ctrl.contentQueue.AddRateLimited(objKey)
		return err
	}
	ctrl.contentQueue.Forget(obj)
	return nil
}

// syncContentByKey processes a StorageBackendContent request.
func (ctrl *backendController) syncContentByKey(ctx context.Context, objKey string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(objKey)
	log.AddContext(ctx).Infof("syncContentByKey: namespace [%s] storageBackendContent name [%s]",
		namespace, name)
	if err != nil {
		log.AddContext(ctx).Errorf("getting namespace & name of storageBackendContent %s from "+
			"informer failed: %v", objKey, err)
		return nil
	}

	content, err := ctrl.contentLister.Get(name)
	if err == nil {
		if ctrl.isMatchProvider(content) {
			// the content exists in informer cache, the handle event must be one of "create/update/sync"
			return ctrl.updateContent(ctx, content)
		}
	}

	if !apiErrors.IsNotFound(err) {
		log.AddContext(ctx).Errorf("getting storageBackendContent %s from informer failed: %v", objKey, err)
		return err
	}

	contentObj, found, err := ctrl.contentStore.GetByKey(objKey)
	// the content not in informer cache, the event must have been "delete"
	if err != nil {
		log.AddContext(ctx).Errorf("get storageBackendContent %s from store, error: %v", objKey, err)
		return nil
	}

	if !found {
		log.AddContext(ctx).Infof("the storageBackendContent %s already deleted, found %v", objKey, found)
		return nil
	}

	storageBackendContent, ok := contentObj.(*xuanwuv1.StorageBackendContent)
	if !ok {
		log.AddContext(ctx).Warningf("except StorageBackendContent, got %+v", contentObj)
		return nil
	}

	return ctrl.deleteContentCache(ctx, storageBackendContent)
}

func (ctrl *backendController) updateContent(ctx context.Context, content *xuanwuv1.StorageBackendContent) error {
	log.AddContext(ctx).Infof("updateContent %s", content.Name)
	updated, err := ctrl.updateContentStore(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("updateContentStore error %v", err)
	}

	if !updated {
		return nil
	}

	if err := ctrl.syncContent(ctx, content); err != nil {
		log.AddContext(ctx).Warningf("syncContent %s failed, error: %v", content.Name, err)
		return err
	}

	return nil
}

func (ctrl *backendController) deleteContentCache(ctx context.Context,
	content *xuanwuv1.StorageBackendContent) error {

	err := ctrl.contentStore.Delete(content)
	if err != nil {
		msg := fmt.Sprintf("delete content from store failed, error: %v", err)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	log.AddContext(ctx).Infof("Delete storageBackendContent %s finished.", content.Name)
	return nil
}

func (ctrl *backendController) syncContent(ctx context.Context, content *xuanwuv1.StorageBackendContent) error {
	log.AddContext(ctx).Infof("Start to sync content %s.", content.Name)
	defer log.AddContext(ctx).Infof("Finished sync content %s.", content.Name)

	syncTask := taskflow.NewTaskFlow(ctx, "Sync-StorageBackendContent")
	syncTask.AddTask("Init-Content-Status", ctrl.initContentStatusTask, nil)
	syncTask.AddTask("Delete-Content", ctrl.deleteContentTask, nil)
	syncTask.AddTask("Create-Content", ctrl.createContentTask, nil)
	syncTask.AddTask("Update-Content", ctrl.updateContentTask, nil)
	syncTask.AddTask("Get-Content", ctrl.getContentTask, nil)
	_, err := syncTask.Run(map[string]interface{}{
		"storageBackendContent": content,
	})
	if err != nil {
		log.AddContext(ctx).Errorf("Run sync content failed, error: %v.", err)
		syncTask.Revert()
		return err
	}
	return nil
}

func (ctrl *backendController) initContentStatusTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	content, ok := params["storageBackendContent"].(*xuanwuv1.StorageBackendContent)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain StorageBackendContent field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	newContent, err := ctrl.initContentStatus(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("Init StorageBackendContent %s failed, error %v", content.Name, err)
		return nil, err
	}
	return map[string]interface{}{
		"storageBackendContent": newContent,
	}, nil
}

func (ctrl *backendController) deleteContentTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	content, ok := taskResult["storageBackendContent"].(*xuanwuv1.StorageBackendContent)
	if !ok {
		msg := fmt.Sprintf("taskResult %v does not contain StorageBackendContent field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if content.ObjectMeta.DeletionTimestamp == nil {
		return nil, nil
	}

	log.AddContext(ctx).Infof("Start to delete content %v.", content.Name)
	if err := ctrl.removeProviderBackend(ctx, content); err != nil {
		log.AddContext(ctx).Errorf("removeProviderBackend %s failed, error: %v", content.Name, err)
		return nil, err
	}
	return nil, nil
}

func (ctrl *backendController) createContentTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	content, ok := taskResult["storageBackendContent"].(*xuanwuv1.StorageBackendContent)
	if !ok {
		msg := fmt.Sprintf("taskResult %v does not contain StorageBackendContent field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if utils.IsContentReady(ctx, content) {
		return nil, nil
	}

	log.AddContext(ctx).Infof("Start to create content %v.", content.Name)
	newContent, err := ctrl.createContent(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("Create StorageBackendContent %s failed, error %v", content.Name, err)
		return nil, err
	}

	if _, err = ctrl.getContentStats(ctx, newContent); err != nil {
		log.AddContext(ctx).Errorf("Get StorageBackendContent %s status failed, error %v", newContent.Name, err)
		return nil, err
	}
	return map[string]interface{}{
		"storageBackendContent": newContent,
	}, nil
}

func (ctrl *backendController) updateContentTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	content, ok := taskResult["storageBackendContent"].(*xuanwuv1.StorageBackendContent)
	if !ok {
		msg := fmt.Sprintf("taskResult %v does not contain StorageBackendContent field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	// start to update the secret info, only secret or maxClientThreads changed, we will update
	if content.Status == nil || (content.Spec.SecretMeta == content.Status.SecretMeta &&
		content.Spec.MaxClientThreads == content.Status.MaxClientThreads &&
		content.Status.SN != "") {
		return nil, nil
	}

	log.AddContext(ctx).Infof("Start to update content %v.", content.Name)
	content.Status.SN = content.Status.Specification["LocalDeviceSN"]
	newContent, err := ctrl.updateContentObj(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("Update StorageBackendContent %s failed, error %v", content.Name, err)
		return nil, err
	}

	return map[string]interface{}{
		"storageBackendContent": newContent,
	}, nil
}

func (ctrl *backendController) getContentTask(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	content, ok := taskResult["storageBackendContent"].(*xuanwuv1.StorageBackendContent)
	if !ok {
		msg := fmt.Sprintf("taskResult %v does not contain StorageBackendContent field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	// validate deletion scene
	if content.DeletionTimestamp != nil {
		return map[string]interface{}{
			"storageBackendContent": content,
		}, nil
	}

	log.AddContext(ctx).Infof("Start to get content stats %v.", content.Name)
	newContent, err := ctrl.getContentStats(ctx, content)
	if err != nil {
		log.AddContext(ctx).Errorf("Get StorageBackendContent %s status failed, error %v", content.Name, err)
		return nil, err
	}

	return map[string]interface{}{
		"storageBackendContent": newContent,
	}, nil
}
