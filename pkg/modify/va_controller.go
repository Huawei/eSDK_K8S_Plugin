/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

// Package modify contains claim and content controller
package modify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	storageV1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	storageV1Informer "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/manage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/scanner"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// VAController watches VolumeModifyContent resources
// and configures metro business links when status is Configuring
type VAController struct {
	kubeClient    kubernetes.Interface
	eventRecorder record.EventRecorder

	vaQueue          workqueue.TypedRateLimitingInterface[*storageV1.VolumeAttachment]
	vaInformer       storageV1Informer.VolumeAttachmentInformer
	vaInformerSynced cache.InformerSynced
	hostName         string
}

// VAControllerRequest is a request for new VA controller
type VAControllerRequest struct {
	KubeClient    kubernetes.Interface
	EventRecorder record.EventRecorder
	VaInformer    storageV1Informer.VolumeAttachmentInformer
	HostName      string
}

// NewVAController creates a new VAController
func NewVAController(request VAControllerRequest) *VAController {
	queue := workqueue.NewTypedRateLimitingQueue[*storageV1.VolumeAttachment](
		workqueue.DefaultTypedControllerRateLimiter[*storageV1.VolumeAttachment]())

	ctrl := &VAController{
		kubeClient:    request.KubeClient,
		eventRecorder: request.EventRecorder,
		vaQueue:       queue,
		vaInformer:    request.VaInformer,
		hostName:      request.HostName,
	}

	ctrl.addEventFunc()
	ctrl.vaInformerSynced = request.VaInformer.Informer().HasSynced
	return ctrl
}

func (c *VAController) addEventFunc() {
	c.vaInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { c.enqueueVA(obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { c.enqueueVA(newObj) },
		},
	)
}

func (c *VAController) enqueueVA(obj interface{}) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}

	if va, ok := obj.(*storageV1.VolumeAttachment); ok {
		if va.Spec.NodeName == c.hostName && va.Labels != nil && va.Labels[constants.RescanLabelKey] == "true" {
			c.vaQueue.Add(va)
		}
	}
}

// Run starts the workers to process events
func (c *VAController) Run(ctx context.Context, workers int, stopCh <-chan struct{}) {
	defer c.vaQueue.ShutDown()

	log.AddContext(ctx).Infoln("starting volume attachment controller")
	defer log.AddContext(ctx).Infoln("shutting down volume attachment controller")

	// Wait for all caches to sync
	if !cache.WaitForCacheSync(stopCh, c.vaInformerSynced) {
		log.AddContext(ctx).Errorln("cannot sync caches")
		return
	}

	log.AddContext(ctx).Infoln("starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(func() { c.runVAWorker(ctx) }, time.Second, stopCh)
	}

	log.AddContext(ctx).Infoln("started workers")
	defer log.AddContext(ctx).Infoln("shutting down workers")

	if stopCh != nil {
		sign := <-stopCh
		log.AddContext(ctx).Infof("Event controller exited, reason: %v", sign)
	}
}

func (c *VAController) runVAWorker(ctx context.Context) {
	for c.processNextVAItem(ctx) {
	}
}

func (c *VAController) processNextVAItem(ctx context.Context) bool {
	ctx, err := log.SetRequestInfoWithTag(ctx, vaResource)
	if err != nil {
		log.Warningf("Set request id error %v", err)
	}

	va, shutdown := c.vaQueue.Get()
	if shutdown {
		log.AddContext(ctx).Infof("processNextVAItem VA: %v, shutdown: %v", va, shutdown)
		return false
	}
	defer c.vaQueue.Done(va)

	log.AddContext(ctx).Infof("Start to handle VA %s", va.Name)
	err = c.handleVA(ctx, va)
	if err != nil {
		log.AddContext(ctx).Errorf("Handle VA %s failed, err: %v", va.Name, err)
		c.vaQueue.AddRateLimited(va)
		return true
	}

	log.AddContext(ctx).Infof("Successfully handled VA %s", va.Name)
	c.vaQueue.Forget(va)
	return true
}

func (c *VAController) handleVA(ctx context.Context, va *storageV1.VolumeAttachment) error {
	publishInfoStr, ok := va.Status.AttachmentMetadata["publishInfo"]
	if !ok {
		return fmt.Errorf("publishInfo not found in VA %s", va.Name)
	}

	var publishInfo manage.ControllerPublishInfo
	if err := json.Unmarshal([]byte(publishInfoStr), &publishInfo); err != nil {
		return fmt.Errorf("unmarshal publishInfo failed: %w", err)
	}

	log.AddContext(ctx).Infof("Start to scan volume %s on node %s", publishInfo.TgtLunWWN, va.Spec.NodeName)

	if err := scanner.GetFactory().Scan(ctx, &publishInfo); err != nil {
		c.eventRecorder.Event(va, corev1.EventTypeWarning, StageFailedReason, err.Error())
		return fmt.Errorf("scan volume failed: %w", err)
	}

	if err := c.updateVALabelWithRetry(ctx, va.Name); err != nil {
		return fmt.Errorf("update VA label failed: %w", err)
	}

	log.AddContext(ctx).Infof("Successfully completed volume scan for VA %s", va.Name)
	return nil
}

func (c *VAController) updateVALabelWithRetry(ctx context.Context, vaName string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		va, err := c.vaInformer.Lister().Get(vaName)
		if err != nil {
			return fmt.Errorf("get VA %s failed: %w", vaName, err)
		}

		vaNew := va.DeepCopy()
		delete(vaNew.Labels, constants.RescanLabelKey)
		_, err = c.kubeClient.StorageV1().VolumeAttachments().Update(ctx, vaNew, metav1.UpdateOptions{})
		return err
	})
}
