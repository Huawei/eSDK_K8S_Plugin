/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

package handler

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/csi/backend/model"
)

func TestBackendSelector_SelectBackend(t *testing.T) {
	// arrange
	instance := NewBackendSelector()

	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "Load",
		func(*CacheWrapper, string) (model.Backend, bool) {
			return model.Backend{Name: "test-name"}, true
		})
	defer patches.Reset()

	// action
	_, err := instance.SelectBackend(context.Background(), "test-name")
	if err != nil {
		t.Errorf("SelectBackend want err is nil, but got error is %v", err)
	}
}

func TestBackendSelector_SelectLocalPool_CacheFailed(t *testing.T) {
	// arrange
	instance := NewBackendSelector()
	params := map[string]interface{}{}

	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "LoadCacheStoragePools",
		func(*CacheWrapper, context.Context) []*model.StoragePool {
			return nil
		})
	defer patches.Reset()

	// action
	_, err := instance.SelectLocalPool(context.Background(), int64(10), params)
	if err == nil {
		t.Error("SelectBackend want an error, but got error is nil")
	}
}

func TestBackendSelector_SelectLocalPool_CapabilityFailed(t *testing.T) {
	// arrange
	instance := NewBackendSelector()
	params := map[string]interface{}{}

	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "LoadCacheStoragePools",
		func(*CacheWrapper, context.Context) []*model.StoragePool {
			return []*model.StoragePool{{Name: "pool-1"}}
		}).ApplyFunc(backend.FilterByCapability, func(_ context.Context, _ map[string]interface{},
		pool []*model.StoragePool, _ [][]interface{}) ([]*model.StoragePool, error) {
		return pool, errors.New("capability filter failed")
	})
	defer patches.Reset()

	// action
	_, err := instance.SelectLocalPool(context.Background(), int64(10), params)
	if err == nil {
		t.Error("SelectBackend want an error, but got error is nil")
	}
}

func TestBackendSelector_SelectLocalPool_TopologyFailed(t *testing.T) {
	// arrange
	instance := NewBackendSelector()
	params := map[string]interface{}{}

	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "LoadCacheStoragePools",
		func(*CacheWrapper, context.Context) []*model.StoragePool {
			return []*model.StoragePool{{Name: "pool-1"}}
		}).ApplyFunc(backend.FilterByCapability, func(_ context.Context, _ map[string]interface{},
		pool []*model.StoragePool, _ [][]interface{}) ([]*model.StoragePool, error) {
		return pool, nil
	}).ApplyFunc(backend.FilterByTopology, func(_ map[string]interface{},
		_ []*model.StoragePool) ([]*model.StoragePool, error) {
		return nil, errors.New("topology filter failed")
	})
	defer patches.Reset()

	// action
	_, err := instance.SelectLocalPool(context.Background(), int64(10), params)
	if err == nil {
		t.Error("SelectBackend want an error, but got error is nil")
	}
}
