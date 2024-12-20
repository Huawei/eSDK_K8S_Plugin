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

// Package cache for backend cache
package cache

import (
	"context"
	"sync"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// BackendCacheProvider provider for backend cache
var BackendCacheProvider = BackendCacheInterface(nil)

// BackendCacheInterface interface for backend cache
type BackendCacheInterface interface {
	// Store save backend to cache
	Store(ctx context.Context, backendName string, backend model.Backend)

	// Load get backend from cache
	Load(backendName string) (model.Backend, bool)

	// Delete delete backend cache by backendName
	Delete(ctx context.Context, backendName string)

	// Clear set backend cache empty
	Clear(ctx context.Context)

	// List get all backend cache
	List(ctx context.Context) []model.Backend

	// Count get backend cache length
	Count() int

	// PrintCacheContent print current backend cache
	PrintCacheContent(ctx context.Context)
}

// BackendCache contains backendItems and mutex
type BackendCache struct {
	backends map[string]model.Backend
	mutex    sync.RWMutex
}

func init() {
	BackendCacheProvider = NewBackendCache()
}

// NewBackendCache init backend backend
func NewBackendCache() *BackendCache {
	return &BackendCache{
		backends: make(map[string]model.Backend),
		mutex:    sync.RWMutex{},
	}
}

// Store save backend to cache
func (b *BackendCache) Store(ctx context.Context, backendName string, backend model.Backend) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	log.AddContext(ctx).Debugf("store backend cache, backendName: [%v] backend: [%+v]", backendName, backend)
	b.backends[backendName] = backend
}

// Load get backend from cache
func (b *BackendCache) Load(backendName string) (model.Backend, bool) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	val, exists := b.backends[backendName]
	return val, exists
}

// Delete delete backend cache by backendName
func (b *BackendCache) Delete(ctx context.Context, backendName string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	defer b.PrintCacheContent(ctx)
	bk, ok := b.backends[backendName]
	if ok && bk.Plugin != nil {
		bk.Plugin.Logout(ctx)
	}
	log.AddContext(ctx).Debugf("delete backend cache, backendName: [%v]", backendName)
	delete(b.backends, backendName)
}

// Clear set backend cache empty
func (b *BackendCache) Clear(ctx context.Context) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	defer b.PrintCacheContent(ctx)
	for name, bk := range b.backends {
		if bk.Plugin != nil {
			bk.Plugin.Logout(ctx)
		}
		delete(b.backends, name)
	}
	log.AddContext(ctx).Infoln("clear backend cache")
	b.backends = make(map[string]model.Backend)
}

// List get all backend cache
func (b *BackendCache) List(ctx context.Context) []model.Backend {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	r := make([]model.Backend, 0)
	for _, v := range b.backends {
		r = append(r, v)
	}
	return r
}

// Count get backend cache length
func (b *BackendCache) Count() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	return len(b.backends)
}

// PrintCacheContent print current backend cache
func (b *BackendCache) PrintCacheContent(ctx context.Context) {
	for _, bk := range b.backends {
		log.AddContext(ctx).Debugf("backend: %s,  values: %+v", bk.Name, bk)
		for _, pool := range bk.Pools {
			log.AddContext(ctx).Debugf("backend: %s,  poolName: %s, Capabilities: %+v, Capacities: %+v",
				bk.Name, pool.Name, pool.Capabilities, pool.Capacities)
		}
	}
}
