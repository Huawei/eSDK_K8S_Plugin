/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	coreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	clientset "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned/scheme"
	external "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/informers/externalversions"
	modifyinformers "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/informers/externalversions/xuanwu/v1"
	modifylisters "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/listers/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	// defaultReSyncPeriod is used when option function ReSyncPeriod is omitted
	defaultReSyncPeriod = 15 * time.Minute

	// defaultRetryMaxDelay is used when option function RetryMaxDelay is omitted
	defaultRetryMaxDelay = 5 * time.Minute

	// defaultRetryBaseDelay is used when option function RetryBaseDelay is omitted
	defaultRetryBaseDelay = 5 * time.Second

	// defaultReconcileClaimStatusDelay is used when option function ReconcileClaimStatusDelay is omitted
	defaultReconcileClaimStatusDelay = 100 * time.Millisecond

	// defaultProvisioner is used when option function ReconcileClaimStatusDelay is omitted
	defaultProvisioner = "csi.huawei.com"

	// defaultWorkerThreads is used when option function WorkerThreads is omitted
	defaultWorkerThreads = 4

	// claimResource is used uniquely identifies claim work queue
	claimResource = "vmc"

	// contentResource is used uniquely identifies content work queue
	contentResource = "vmct"

	// eventResourceName is used to record event
	eventResourceName = "modify-volume-mgnt"
)

// VolumeModifyController controller of volume modify
type VolumeModifyController struct {
	clientSet                 clientset.Interface
	contentClient             clientset.Interface
	client                    kubernetes.Interface
	modifyClient              drcsi.ModifyVolumeInterfaceClient
	eventRecorder             record.EventRecorder
	claimQueue                workqueue.RateLimitingInterface
	contentQueue              workqueue.RateLimitingInterface
	claimInformer             modifyinformers.VolumeModifyClaimInformer
	contentInformer           modifyinformers.VolumeModifyContentInformer
	claimLister               modifylisters.VolumeModifyClaimLister
	contentLister             modifylisters.VolumeModifyContentLister
	claimListerSync           cache.InformerSynced
	contentListerSync         cache.InformerSynced
	claimWorker               *ObjectWorker
	contentWorker             *ObjectWorker
	reSyncPeriod              time.Duration
	retryMaxDelay             time.Duration
	retryBaseDelay            time.Duration
	reconcileClaimStatusDelay time.Duration
	workerThreads             int
	provisioner               string
}

// NewVolumeModifyController instance a controller
func NewVolumeModifyController(ctx context.Context, client kubernetes.Interface, clientSet clientset.Interface,
	factory external.SharedInformerFactory,
	options ...func(controller *VolumeModifyController)) *VolumeModifyController {
	ctr := &VolumeModifyController{
		client:                    client,
		clientSet:                 clientSet,
		reSyncPeriod:              defaultReSyncPeriod,
		retryBaseDelay:            defaultRetryBaseDelay,
		retryMaxDelay:             defaultRetryMaxDelay,
		workerThreads:             defaultWorkerThreads,
		reconcileClaimStatusDelay: defaultReconcileClaimStatusDelay,
		provisioner:               defaultProvisioner,
	}

	// add custom options
	for _, option := range options {
		option(ctr)
	}

	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&coreV1.EventSinkImpl{Interface: client.CoreV1().Events(v1.NamespaceAll)})
	ctr.eventRecorder = broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: eventResourceName})
	ctr.claimInformer = factory.Xuanwu().V1().VolumeModifyClaims()
	ctr.contentInformer = factory.Xuanwu().V1().VolumeModifyContents()
	ctr.claimLister = ctr.claimInformer.Lister()
	ctr.contentLister = ctr.contentInformer.Lister()
	ctr.claimListerSync = ctr.claimInformer.Informer().HasSynced
	ctr.contentListerSync = ctr.contentInformer.Informer().HasSynced
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(ctr.retryBaseDelay, ctr.retryMaxDelay)
	claimQueueConfig := workqueue.RateLimitingQueueConfig{Name: claimResource}
	ctr.claimQueue = workqueue.NewRateLimitingQueueWithConfig(rateLimiter, claimQueueConfig)
	contentQueueConfig := workqueue.RateLimitingQueueConfig{Name: contentResource}
	ctr.contentQueue = workqueue.NewRateLimitingQueueWithConfig(rateLimiter, contentQueueConfig)

	// add event handler
	ctr.AddClaimHandler(ctx).AddContentHandler(ctx)

	// add workers
	ctr.claimWorker = NewObjectWorker(claimResource, ctr.claimQueue, SyncFunc(ctr.syncClaimWork))
	ctr.contentWorker = NewObjectWorker(contentResource, ctr.contentQueue, SyncFunc(ctr.syncContentWork))

	var err error
	_, ctr.contentClient, err = utils.GetK8SAndCrdClient(ctx)
	if err != nil {
		ctr.contentClient = clientSet
	}
	return ctr
}

// Run will sync informer caches and starting workers. It will block until stopCh is closed
func (ctrl *VolumeModifyController) Run(ctx context.Context, stopCh <-chan struct{}) {
	defer ctrl.claimQueue.ShutDown()
	defer ctrl.contentQueue.ShutDown()

	log.AddContext(ctx).Infoln("starting volume modify controller")
	defer log.AddContext(ctx).Infoln("shutting down volume modify controller")

	if !cache.WaitForCacheSync(stopCh, ctrl.claimListerSync, ctrl.contentListerSync) {
		log.AddContext(ctx).Errorln("cannot sync caches")
		return
	}

	log.AddContext(ctx).Infoln("starting workers")
	for i := 0; i < ctrl.workerThreads; i++ {
		go wait.Until(func() { ctrl.claimWorker.Run(ctx) }, time.Second, stopCh)
		go wait.Until(func() { ctrl.contentWorker.Run(ctx) }, time.Second, stopCh)
	}

	log.AddContext(ctx).Infoln("started workers")
	defer log.AddContext(ctx).Infoln("shutting down workers")
	if stopCh != nil {
		sign := <-stopCh
		log.AddContext(ctx).Infof("volume modify controller exited, reason: [%v]", sign)
	}
}

// AddClaimHandler add claim event handler
func (ctrl *VolumeModifyController) AddClaimHandler(ctx context.Context) *VolumeModifyController {
	_, err := ctrl.claimInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueClaim(obj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueClaim(obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueClaim(newObj) },
		},
	)
	if err != nil {
		log.AddContext(ctx).Errorf("Add claim event handler failed, error: %v", err)
	}
	return ctrl
}

// AddContentHandler add claim event handler
func (ctrl *VolumeModifyController) AddContentHandler(ctx context.Context) *VolumeModifyController {
	_, err := ctrl.contentInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { ctrl.enqueueContent(obj) },
			DeleteFunc: func(obj interface{}) { ctrl.enqueueContent(obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { ctrl.enqueueContent(newObj) },
		},
	)
	if err != nil {
		log.AddContext(ctx).Errorf("Add content event handler failed, error: %v", err)
	}
	return ctrl
}

func (ctrl *VolumeModifyController) enqueueClaim(obj interface{}) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}
	if modifyClaim, ok := obj.(*xuanwuv1.VolumeModifyClaim); ok {
		objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(modifyClaim)
		if err != nil {
			log.Errorf("failed to get claim key from object [%v] err: [%v]", modifyClaim, err)
			return
		}
		log.Infof("enqueued claim [%v] for sync", objName)
		ctrl.claimQueue.Add(objName)
	}
}

func (ctrl *VolumeModifyController) enqueueContent(obj interface{}) {
	if unknown, ok := obj.(cache.DeletedFinalStateUnknown); ok && unknown.Obj != nil {
		obj = unknown.Obj
	}
	if modifyContent, ok := obj.(*xuanwuv1.VolumeModifyContent); ok {
		objName, err := cache.DeletionHandlingMetaNamespaceKeyFunc(modifyContent)
		if err != nil {
			log.Errorf("failed to get content key from object [%v] err: [%v]", modifyContent, err)
			return
		}
		log.Infof("enqueued content [%v] for sync", objName)
		ctrl.contentQueue.Add(objName)
	}
}

// WorkerThreads used to configure the number of working threads.
func WorkerThreads(workerThreads int) func(controller *VolumeModifyController) {
	return func(ctr *VolumeModifyController) {
		ctr.workerThreads = workerThreads
	}
}

// ReSyncPeriod used to configure re-sync period.
func ReSyncPeriod(reSyncPeriod time.Duration) func(controller *VolumeModifyController) {
	return func(ctr *VolumeModifyController) {
		ctr.reSyncPeriod = reSyncPeriod
	}
}

// RetryMaxDelay used to configure the max interval of retry.
func RetryMaxDelay(retryMaxDelay time.Duration) func(controller *VolumeModifyController) {
	return func(ctr *VolumeModifyController) {
		ctr.retryMaxDelay = retryMaxDelay
	}
}

// RetryBaseDelay used to configure the start interval of retry.
func RetryBaseDelay(retryBaseDelay time.Duration) func(controller *VolumeModifyController) {
	return func(ctr *VolumeModifyController) {
		ctr.retryBaseDelay = retryBaseDelay
	}
}

// ReconcileClaimStatusDelay used to configure the interval of reconcile claim status.
func ReconcileClaimStatusDelay(reconcileClaimStatusDelay time.Duration) func(controller *VolumeModifyController) {
	return func(ctr *VolumeModifyController) {
		ctr.reconcileClaimStatusDelay = reconcileClaimStatusDelay
	}
}

// Provisioner used to configure the driver name.
func Provisioner(provisioner string) func(controller *VolumeModifyController) {
	return func(ctr *VolumeModifyController) {
		ctr.provisioner = provisioner
	}
}

// ClientOfModify used to configure the modify client.
func ClientOfModify(modifyClient drcsi.ModifyVolumeInterfaceClient) func(controller *VolumeModifyController) {
	return func(ctr *VolumeModifyController) {
		ctr.modifyClient = modifyClient
	}
}
