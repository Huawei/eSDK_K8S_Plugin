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

// Package handler contains all helper functions with backend process
package handler

import (
	"context"
	"huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/csi/backend/cache"
	"huawei-csi-driver/csi/backend/model"
	"huawei-csi-driver/utils/log"
)

// BackendCacheWrapperInterface wrapping interface of the backend cache,
// which is used to provide combined operation cache interfaces.
type BackendCacheWrapperInterface interface {
	cache.BackendCacheInterface
	AddBackendToCache(ctx context.Context, sbct v1.StorageBackendContent) (*model.Backend, error)
	UpdateCacheBackend(ctx context.Context, name string, sbct v1.StorageBackendContent)
	UpdateCacheBackendMetro(ctx context.Context)
	UpdateCacheBackendStatus(ctx context.Context, name string, online bool)
	LoadCacheStoragePools(ctx context.Context) []*model.StoragePool
	LoadCacheBackendTopologies(ctx context.Context, name string) []map[string]string
}

// CacheWrapper cache wrapper
type CacheWrapper struct {
	cache.BackendCacheInterface
}

// NewCacheWrapper init instance of CacheWrapper
func NewCacheWrapper() *CacheWrapper {
	return &CacheWrapper{cache.BackendCacheProvider}
}

// AddBackendToCache init a backend and add to cache
func (b *CacheWrapper) AddBackendToCache(ctx context.Context, sbct v1.StorageBackendContent) (*model.Backend, error) {
	newBackend, err := backend.BuildBackend(ctx, sbct)
	if err != nil {
		log.AddContext(ctx).Errorf("failed to initialize the backend when adding backend to cache,"+
			" backend: %s, error: %v.", sbct.Spec.BackendClaim, err)
		return nil, err
	}

	b.updateCacheBackend(ctx, *newBackend, sbct)
	return newBackend, nil
}

// UpdateCacheBackend update cache backend
// step 1: update storage pool
// step 2: update hyperMetro relationships
func (b *CacheWrapper) UpdateCacheBackend(ctx context.Context, name string, sbct v1.StorageBackendContent) {
	bk, exists := b.Load(name)
	if !exists || len(bk.Pools) == 0 {
		log.AddContext(ctx).Infof("the specified backend %s or backend's storage pool was not found in cache "+
			"when updating the backend, so updating it is skipped", name)
		return
	}

	b.updateCacheBackend(ctx, bk, sbct)
}

func (b *CacheWrapper) updateCacheBackend(ctx context.Context, bk model.Backend, sbct v1.StorageBackendContent) {

	bk.UpdatePools(ctx, &sbct)
	bk.SetAvailable(ctx, true)
	b.Store(ctx, bk.Name, bk)

	b.UpdateCacheBackendMetro(ctx)
}

// UpdateCacheBackendStatus update backend status
func (b *CacheWrapper) UpdateCacheBackendStatus(ctx context.Context, name string, online bool) {
	bk, exists := b.Load(name)
	if !exists {
		return
	}

	bk.SetAvailable(ctx, online)
	b.Store(ctx, bk.Name, bk)
}

// UpdateCacheBackendMetro update hyperMetro relationships
func (b *CacheWrapper) UpdateCacheBackendMetro(ctx context.Context) {
	backends := b.List(ctx)
	for _, i := range backends {
		if (i.MetroDomain == "" && i.MetrovStorePairID == "") || i.MetroBackend != nil {
			continue
		}

		for _, j := range backends {
			if i.Name == j.Name || i.Storage != j.Storage {
				continue
			}

			if ((i.MetroDomain != "" && i.MetroDomain == j.MetroDomain) ||
				(i.MetrovStorePairID != "" && i.MetrovStorePairID == j.MetrovStorePairID)) &&
				(i.MetroBackendName == j.Name && j.MetroBackendName == i.Name) {
				i.MetroBackend, j.MetroBackend = &j, &i
				i.Plugin.UpdateMetroRemotePlugin(ctx, j.Plugin)
				j.Plugin.UpdateMetroRemotePlugin(ctx, i.Plugin)
				b.Store(ctx, i.Name, i)
				b.Store(ctx, j.Name, j)
			}
		}
	}
}

// LoadCacheStoragePools load all cached storage pools
func (b *CacheWrapper) LoadCacheStoragePools(ctx context.Context) []*model.StoragePool {
	var candidatePools []*model.StoragePool
	backends := b.List(ctx)
	for _, bk := range backends {
		if bk.Available {
			candidatePools = append(candidatePools, bk.Pools...)
		}
	}
	return candidatePools
}

// LoadCacheBackendTopologies load specify backend's pools
func (b *CacheWrapper) LoadCacheBackendTopologies(ctx context.Context, name string) []map[string]string {
	bk, exists := b.Load(name)
	if !exists {
		log.AddContext(ctx).Warningf("backend [%s] does not exist when loading topologies", name)
		return make([]map[string]string, 0)
	}
	return bk.SupportedTopologies
}
