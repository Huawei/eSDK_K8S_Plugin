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

// Package modify contains claim and content controller
package modify

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// syncObjectFunc is a sync object function
type syncObjectFunc func(ctx context.Context, name string) error

// ObjectWorker defines the object worker
type ObjectWorker struct {
	name  string
	queue workqueue.RateLimitingInterface
	sync  syncObjectFunc
}

// NewObjectWorker return a new ObjectWorker
func NewObjectWorker(name string, queue workqueue.RateLimitingInterface,
	options ...func(*ObjectWorker)) *ObjectWorker {
	consumer := &ObjectWorker{
		name:  name,
		queue: queue,
		sync:  func(context.Context, string) error { return nil },
	}
	for _, option := range options {
		option(consumer)
	}
	return consumer
}

// Run is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the work queue.
func (h *ObjectWorker) Run(ctx context.Context) {
	for {
		if processNext := h.processNextWorkItem(ctx); !processNext {
			break
		}
	}
}

// processNextWorkItem will read a single work item off the work queue and attempt to process it,
// by calling the SyncHandler.
func (h *ObjectWorker) processNextWorkItem(ctx context.Context) bool {
	ctx, err := log.SetRequestInfoWithTag(ctx, h.name)
	if err != nil {
		log.Infof("set request id error %v", err)
	}

	obj, shutdown := h.queue.Get()
	if shutdown {
		log.AddContext(ctx).Infof("handle %s:%v, shutdown: %v", h.name, obj, shutdown)
		return false
	}

	defer h.queue.Done(obj)
	if err := h.SyncHandler(ctx, obj); err != nil {
		log.AddContext(ctx).Errorf("handle %s error: %v", h.name, err)
		runtime.HandleError(err)
		return true
	}
	return true
}

// SyncHandler passing it the namespace/name string of the resource to be synced.
func (h *ObjectWorker) SyncHandler(ctx context.Context, obj interface{}) error {
	objKey, ok := obj.(string)
	if !ok {
		h.queue.Forget(obj)
		return fmt.Errorf("expected string key in workqueue but got %#v", obj)
	}

	_, name, err := cache.SplitMetaNamespaceKey(objKey)
	if err != nil {
		return fmt.Errorf("split %s meta name:%s failed: %w", h.name, objKey, err)
	}

	if err := h.sync(ctx, name); err != nil {
		h.queue.AddRateLimited(objKey)
		log.AddContext(ctx).Warningf("retrying syncing %s:%s, failure %v", h.name, objKey, h.queue.NumRequeues(objKey))
		return fmt.Errorf("sync %s:%s error: %w", h.name, objKey, err)
	}

	h.queue.Forget(obj)
	return nil
}

// SyncFunc is the configuration of sync function for queue of object. If set, this function will be called
// when object popping. If not set, do-nothings.
func SyncFunc(sync syncObjectFunc) func(*ObjectWorker) {
	return func(consumer *ObjectWorker) {
		consumer.sync = sync
	}
}
