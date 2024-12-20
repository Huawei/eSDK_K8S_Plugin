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
	"errors"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var logName = "volume"

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestObjectWorker_processNextWorkItem_withQueueShutdown(t *testing.T) {
	// arrange
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 5*time.Second)
	queue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter, workqueue.RateLimitingQueueConfig{Name: "test"})
	objectWorker := &ObjectWorker{queue: queue}

	// mock
	patches := gomonkey.ApplyFuncReturn(log.SetRequestInfo, context.Background(), nil).
		ApplyMethodReturn(queue, "Get", context.Background(), true)
	defer patches.Reset()

	// action
	result := objectWorker.processNextWorkItem(context.Background())
	// assert

	if result {
		t.Errorf("TestObjectWorker_processNextWorkItem failed, want false, but got true")
	}
}

func TestObjectWorker_processNextWorkItem_withSyncFailed(t *testing.T) {
	// arrange
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 5*time.Second)
	queue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter, workqueue.RateLimitingQueueConfig{Name: "test"})
	objectWorker := &ObjectWorker{queue: queue}

	// mock
	patches := gomonkey.ApplyFuncReturn(log.SetRequestInfo, context.Background(), nil).
		ApplyMethodReturn(queue, "Get", "test", false).
		ApplyMethodReturn(objectWorker, "SyncHandler", errors.New("sync failed"))
	defer patches.Reset()

	// action
	result := objectWorker.processNextWorkItem(context.Background())

	// assert
	if !result {
		t.Errorf("TestObjectWorker_processNextWorkItem_withSyncFailed failed, want true, but got false")
	}
}

func TestObjectWorker_processNextWorkItem_Successful(t *testing.T) {
	// arrange
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 5*time.Second)
	queue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter, workqueue.RateLimitingQueueConfig{Name: "test"})
	objectWorker := &ObjectWorker{queue: queue}

	// mock
	patches := gomonkey.ApplyFuncReturn(log.SetRequestInfo, context.Background(), nil).
		ApplyMethodReturn(queue, "Get", "test", false).
		ApplyMethodReturn(objectWorker, "SyncHandler", nil)
	defer patches.Reset()

	// action
	result := objectWorker.processNextWorkItem(context.Background())
	// assert

	if !result {
		t.Errorf("TestObjectWorker_processNextWorkItem_successful failed, want true, but got false")
	}
}

func TestObjectWorker_SyncHandler_WithKeyIsNotString(t *testing.T) {
	// arrange
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 5*time.Second)
	queue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter, workqueue.RateLimitingQueueConfig{Name: "test"})
	objectWorker := &ObjectWorker{queue: queue}

	// action
	err := objectWorker.SyncHandler(context.Background(), 0)

	// assert
	if err == nil {
		t.Errorf("TestObjectWorker_SyncHandler_WithKeyIsNotString failed, want an error, but got nil")
	}
}

func TestObjectWorker_SyncHandler_WithSplitMetaFailed(t *testing.T) {
	// arrange
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 5*time.Second)
	queue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter, workqueue.RateLimitingQueueConfig{Name: "test"})
	objectWorker := &ObjectWorker{queue: queue}

	// mock
	patches := gomonkey.ApplyFuncReturn(cache.SplitMetaNamespaceKey, "", "", errors.New("split failed"))
	defer patches.Reset()

	// action
	err := objectWorker.SyncHandler(context.Background(), "test")

	// assert
	if err == nil {
		t.Errorf("TestObjectWorker_SyncHandler_WithSplitMetaFailed failed, want an error, but got nil")
	}
}

func TestObjectWorker_SyncHandler_WithSyncFailed(t *testing.T) {
	// arrange
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 5*time.Second)
	queue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter, workqueue.RateLimitingQueueConfig{Name: "test"})
	objectWorker := &ObjectWorker{
		queue: queue,
		sync:  func(ctx context.Context, name string) error { return errors.New("sync failed") },
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(log.SetRequestInfo, context.Background(), nil).
		ApplyFuncReturn(cache.SplitMetaNamespaceKey, "", "test", nil)
	defer patches.Reset()

	// action
	err := objectWorker.SyncHandler(context.Background(), "test")

	// assert
	if err == nil {
		t.Errorf("TestObjectWorker_SyncHandler_WithSyncFailed failed, want an error, but got nil")
	}
}

func TestObjectWorker_SyncHandler_Successful(t *testing.T) {
	// arrange
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(5*time.Second, 5*time.Second)
	queue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter, workqueue.RateLimitingQueueConfig{Name: "test"})
	objectWorker := &ObjectWorker{
		queue: queue,
		sync:  func(ctx context.Context, name string) error { return nil },
	}

	// mock
	patches := gomonkey.ApplyFuncReturn(log.SetRequestInfo, context.Background(), nil).
		ApplyFuncReturn(cache.SplitMetaNamespaceKey, "", "test", nil)
	defer patches.Reset()

	// action
	err := objectWorker.SyncHandler(context.Background(), "test")

	// assert
	if err != nil {
		t.Errorf("TestObjectWorker_SyncHandler_WithSyncFailed failed, want nil, but got %v", err)
	}
}
