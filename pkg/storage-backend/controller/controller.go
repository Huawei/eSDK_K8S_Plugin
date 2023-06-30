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
	"flag"
	"fmt"
	"time"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	backendInformers "huawei-csi-driver/pkg/client/informers/externalversions/xuanwu/v1"
	backendListers "huawei-csi-driver/pkg/client/listers/xuanwu/v1"
	"huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
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

// BackendController defines the backend controller parameters
type BackendController struct {
	clientSet     clientSet.Interface
	client        kubernetes.Interface
	eventRecorder record.EventRecorder
	reSyncPeriod  time.Duration

	claimQueue        workqueue.RateLimitingInterface
	contentQueue      workqueue.RateLimitingInterface
	claimListerSync   cache.InformerSynced
	contentListerSync cache.InformerSynced
	claimLister       backendListers.StorageBackendClaimLister
	contentLister     backendListers.StorageBackendContentLister
	claimStore        cache.Store
	contentStore      cache.Store
}

// BackendControllerRequest is a request for new controller
type BackendControllerRequest struct {
	// storage backend client
	ClientSet clientSet.Interface
	// storage backend claim informer
	ClaimInformer backendInformers.StorageBackendClaimInformer
	// storage backend content informer
	ContentInformer backendInformers.StorageBackendContentInformer
	// reSync period time
	ReSyncPeriod time.Duration
	// event recorder
	EventRecorder record.EventRecorder
}

// NewBackendController return a new NewBackendController
func NewBackendController(request BackendControllerRequest) *BackendController {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(*retryIntervalStart, *retryIntervalMax)
	ctrl := &BackendController{
		clientSet:     request.ClientSet,
		claimQueue:    workqueue.NewNamedRateLimitingQueue(rateLimiter, "backend-controller-claim"),
		contentQueue:  workqueue.NewNamedRateLimitingQueue(rateLimiter, "backend-controller-content"),
		claimStore:    cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		contentStore:  cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc),
		reSyncPeriod:  request.ReSyncPeriod,
		eventRecorder: request.EventRecorder,
	}

	request.ClaimInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) { ctrl.enqueueClaim(obj) },
			UpdateFunc: func(oldObj, newObj interface{}) {
				newClaim, ok := newObj.(*xuanwuv1.StorageBackendClaim)
				if !ok {
					log.Warningf("newObj %v can not convert to StorageBackendClaim", newObj)
					return
				}

				oldClaim, ok := oldObj.(*xuanwuv1.StorageBackendClaim)
				if !ok {
					log.Warningf("oldObj %v can not convert to StorageBackendClaim", oldObj)
					return
				}

				if oldClaim.ResourceVersion == newClaim.ResourceVersion {
					// Periodic resync will send update events for all known StorageBackendClaim.
					// Two different versions of the same StorageBackendClaim will always have different RVs.
					return
				}
				ctrl.enqueueClaim(newObj)
			},
			DeleteFunc: func(obj interface{}) { ctrl.enqueueClaim(obj) },
		},
	)
	ctrl.claimLister = request.ClaimInformer.Lister()
	ctrl.claimListerSync = request.ClaimInformer.Informer().HasSynced

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

// Run defines the controller process
func (ctrl *BackendController) Run(ctx context.Context, workers int, stopCh <-chan struct{}) {
	defer ctrl.claimQueue.ShutDown()
	defer ctrl.contentQueue.ShutDown()

	log.AddContext(ctx).Infoln("Starting storage backend controller")
	defer log.AddContext(ctx).Infoln("Shutting down storage backend controller")

	if !cache.WaitForCacheSync(stopCh, ctrl.claimListerSync, ctrl.contentListerSync) {
		log.AddContext(ctx).Errorln("Cannot sync caches")
		return
	}

	ctrl.initializeCaches(ctx, ctrl.claimLister, ctrl.contentLister)

	for i := 0; i < workers; i++ {
		go wait.Until(func() { ctrl.runClaimWorker(ctx) }, time.Second, stopCh)
		go wait.Until(func() { ctrl.runContentWorker(ctx) }, time.Second, stopCh)
	}

	if stopCh != nil {
		sign := <-stopCh
		log.AddContext(ctx).Infof("Backend Controller exited, reason: %v", sign)
	}
}

func (ctrl *BackendController) enqueueClaim(obj interface{}) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}

	if claim, ok := obj.(*xuanwuv1.StorageBackendClaim); ok {
		objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(claim)
		if err != nil {
			log.Errorf("failed to get key from object: %v, %v", err, claim)
			return
		}
		log.Infof("enqueued StorageBackendClaim %q for sync", objName)
		ctrl.claimQueue.Add(objName)
	}
}

func (ctrl *BackendController) enqueueContent(obj interface{}) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}

	if content, ok := obj.(*xuanwuv1.StorageBackendContent); ok {
		objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(content)
		if err != nil {
			log.Errorf("failed to get key from object: %v, %v", err, content)
			return
		}
		log.Infof("enqueued StorageBackendContent %q for sync", objName)
		ctrl.contentQueue.Add(objName)
	}
}

func (ctrl *BackendController) runClaimWorker(ctx context.Context) {
	for !ctrl.processNextClaimWorkItem(ctx) {
		return
	}
}

func (ctrl *BackendController) processNextClaimWorkItem(ctx context.Context) bool {
	obj, shutdown := ctrl.claimQueue.Get()
	if shutdown {
		log.AddContext(ctx).Infof("processNextClaimWorkItem obj: [%v], shutdown: [%v]", obj, shutdown)
		return false
	}

	timeout, cancel := context.WithTimeout(ctx, *provisionTimeout)
	defer cancel()
	ctx = timeout

	defer ctrl.claimQueue.Done(obj)
	if err := ctrl.handleClaimWork(ctx, obj); err != nil {
		utilRuntime.HandleError(err)
		return true
	}
	return true
}

func (ctrl *BackendController) handleClaimWork(ctx context.Context, obj interface{}) error {
	objKey, ok := obj.(string)
	if !ok {
		ctrl.claimQueue.Forget(obj)
		msg := fmt.Sprintf("expected string in claim workqueue but got %#v", obj)
		log.AddContext(ctx).Errorf(msg)
		return errors.New(msg)
	}

	err := ctrl.syncClaimByKey(ctx, objKey)
	if err != nil {
		log.AddContext(ctx).Errorf("handleClaimWork: sync storageBackendClaim %s failed, error: %v", objKey, err)
		ctrl.claimQueue.AddRateLimited(objKey)
		return err
	}

	ctrl.claimQueue.Forget(obj)
	return nil
}

func (ctrl *BackendController) runContentWorker(ctx context.Context) {
	for !ctrl.processNextContentWorkItem(ctx) {
		return
	}
}

func (ctrl *BackendController) processNextContentWorkItem(ctx context.Context) bool {
	obj, shutdown := ctrl.contentQueue.Get()
	if shutdown {
		log.AddContext(ctx).Infof("processNextContentWorkItem obj: [%v], shutdown: [%v]", obj, shutdown)
		return false
	}

	timeout, cancel := context.WithTimeout(ctx, *provisionTimeout)
	defer cancel()
	ctx = timeout

	defer ctrl.contentQueue.Done(obj)
	if err := ctrl.handleContentWork(ctx, obj); err != nil {
		utilRuntime.HandleError(err)
		return true
	}
	return true
}

func (ctrl *BackendController) handleContentWork(ctx context.Context, obj interface{}) error {
	objKey, ok := obj.(string)
	if !ok {
		ctrl.contentQueue.Forget(obj)
		msg := fmt.Sprintf("expected string in content workqueue but got %#v", obj)
		log.AddContext(ctx).Errorf(msg)
		return errors.New(msg)
	}

	if err := ctrl.syncContentByKey(ctx, objKey); err != nil {
		log.AddContext(ctx).Errorf("handleContentWork: sync storageBackendContent %s failed, error: %v",
			objKey, err)
		ctrl.contentQueue.AddRateLimited(objKey)
		return err
	}

	ctrl.contentQueue.Forget(obj)
	return nil
}

func (ctrl *BackendController) initializeCaches(ctx context.Context,
	claimLister backendListers.StorageBackendClaimLister, contentLister backendListers.StorageBackendContentLister) {

	claimList, err := claimLister.List(labels.Everything())
	if err != nil {
		log.AddContext(ctx).Errorf("StorageBackend claim initialize failed, error: %v", err)
	}

	for _, claim := range claimList {
		claimClone := claim.DeepCopy()
		if _, err := ctrl.updateClaimStore(ctx, claimClone); err != nil {
			log.AddContext(ctx).Errorf("Update claim cache failed, error: %v", err)
		}
	}

	contentList, err := contentLister.List(labels.Everything())
	if err != nil {
		log.AddContext(ctx).Errorf("StorageBackend claim initialize failed, error: %v", err)
	}

	for _, content := range contentList {
		contentClone := content.DeepCopy()
		if _, err := ctrl.updateContentStore(ctx, contentClone); err != nil {
			log.AddContext(ctx).Errorf("Update content cache failed, error: %v", err)
		}
	}
}

func (ctrl *BackendController) updateClaimStore(ctx context.Context, claim interface{}) (bool, error) {
	return utils.StoreObjectUpdate(ctx, ctrl.claimStore, claim, "claim")
}

func (ctrl *BackendController) updateContentStore(ctx context.Context, content interface{}) (bool, error) {
	return utils.StoreObjectUpdate(ctx, ctrl.contentStore, content, "content")
}

// syncContentByKey processes a StorageBackendContent request.
func (ctrl *BackendController) syncContentByKey(ctx context.Context, objKey string) error {
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
		// the content exists in informer cache, the handle event must be one of "create/update/sync"
		return ctrl.updateContent(ctx, content)
	}

	if !apiErrors.IsNotFound(err) {
		log.AddContext(ctx).Errorf("getting storageBackendContent %s from informer failed: %v", objKey, err)
		return err
	}

	contentObj, found, err := ctrl.contentStore.GetByKey(objKey)
	// the content not in informer cache, the event must have been "delete"
	if err != nil || !found {
		log.AddContext(ctx).Warningf("the storageBackendContent %s already deleted, found %v, error: %v",
			objKey, found, err)
		return nil
	}

	storageBackendContent, ok := contentObj.(*xuanwuv1.StorageBackendContent)
	if !ok {
		log.AddContext(ctx).Warningf("except StorageBackendContent, got %+v", contentObj)
		return nil
	}
	return ctrl.deleteStorageBackendContent(ctx, storageBackendContent)
}
