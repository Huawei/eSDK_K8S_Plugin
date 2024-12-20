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
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// BackendRegisterInterface register backend operation set
type BackendRegisterInterface interface {
	FetchAndRegisterAllBackend(ctx context.Context)
	FetchAndRegisterOneBackend(ctx context.Context, name string, checkOnline bool) (*model.Backend, error)
	LoadOrRegisterOneBackend(ctx context.Context, name string) (*model.Backend, error)
	LoadOrRebuildOneBackend(ctx context.Context, name, contentName string) (*model.Backend, error)
	RemoveRegisteredOneBackend(ctx context.Context, name string)
	UpdateOrRegisterOneBackend(ctx context.Context, sbct *v1.StorageBackendContent) error
}

// BackendRegister backend register
type BackendRegister struct {
	fetchHandler BackendFetchInterface
	cacheHandler BackendCacheWrapperInterface
}

// NewBackendRegister init instance of BackendRegister
func NewBackendRegister() *BackendRegister {
	return &BackendRegister{
		fetchHandler: NewBackendFetcher(),
		cacheHandler: NewCacheWrapper(),
	}
}

// RemoveRegisteredOneBackend remove registered backend from cache
func (b *BackendRegister) RemoveRegisteredOneBackend(ctx context.Context, name string) {
	b.cacheHandler.Delete(ctx, name)
}

// LoadOrRegisterOneBackend if the cache is hit, the cache backend is directly returned.
// If the cache is not hit, the Kubernetes is queried for registration again.
func (b *BackendRegister) LoadOrRegisterOneBackend(ctx context.Context, name string) (*model.Backend, error) {
	bk, exists := b.cacheHandler.Load(name)
	if exists {
		return &bk, nil
	}

	return b.FetchAndRegisterOneBackend(ctx, name, true)
}

// LoadOrRebuildOneBackend if the backend has been already in cache, and doesn't need to rebuild, return it directly.
// Otherwise, fetch and register the backend again.
func (b *BackendRegister) LoadOrRebuildOneBackend(ctx context.Context,
	name, contentName string) (*model.Backend, error) {
	bk, exists := b.cacheHandler.Load(name)
	if exists && !bk.NeedRebuild(contentName) {
		return &bk, nil
	}

	if bk.NeedRebuild(contentName) {
		log.AddContext(ctx).Infof("The content name of backend [%s] has changed from [%s] to [%s], need rebuild",
			bk.Name, bk.ContentName, contentName)
		b.cacheHandler.Delete(ctx, name)
	}

	return b.FetchAndRegisterOneBackend(ctx, name, true)
}

// FetchAndRegisterAllBackend fetch all backends in the kubernetes and register them to cache.
func (b *BackendRegister) FetchAndRegisterAllBackend(ctx context.Context) {
	contents, err := b.fetchHandler.FetchAllBackends(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("fetch and register all backend failed, error: %v", err)
		return
	}

	err = b.UpdateOrRegisterOnlineBackend(ctx, contents)
	if err != nil {
		return
	}

	// if backend online is false need delete memory backend
	b.CheckConsistency(ctx, contents)
}

// FetchAndRegisterOneBackend fetch one backend in the kubernetes and register them to cache.
func (b *BackendRegister) FetchAndRegisterOneBackend(ctx context.Context, name string,
	checkOnline bool) (*model.Backend, error) {
	sbct, err := b.fetchHandler.FetchBackendByName(ctx, name, checkOnline)
	if err != nil {
		log.AddContext(ctx).Errorf("fetch backend %s failed, error: %v", name, err)
		return nil, err
	}

	bk, err := b.UpdateAndAddBackend(ctx, *sbct)
	if err != nil {
		log.AddContext(ctx).Errorf("add backend %s to cache failed, error: %v", name, err)
		return nil, err
	}
	return bk, nil
}

// UpdateAndAddBackend if the cache is hit, the cache backend is directly updated.
// If the cache is not hit, the Kubernetes is queried for registration again.
func (b *BackendRegister) UpdateAndAddBackend(ctx context.Context,
	sbct v1.StorageBackendContent) (*model.Backend, error) {
	_, name, err := pkgUtils.SplitMetaNamespaceKey(sbct.Spec.BackendClaim)
	if err != nil {
		log.AddContext(ctx).Errorf("get backend name failed, error: %v", err)
		return nil, err
	}

	bk, exists := b.cacheHandler.Load(name)
	if exists {
		b.cacheHandler.UpdateCacheBackend(ctx, name, sbct)
		return &bk, nil
	}
	return b.cacheHandler.AddBackendToCache(ctx, sbct)
}

// UpdateOrRegisterOnlineBackend update or register all online backend.
func (b *BackendRegister) UpdateOrRegisterOnlineBackend(ctx context.Context,
	contents []v1.StorageBackendContent) error {
	if len(contents) == 0 {
		return nil
	}

	var err error
	for _, content := range contents {
		if content.Status == nil || !content.Status.Online {
			continue
		}
		if _, err = b.UpdateAndAddBackend(ctx, content); err != nil {
			log.AddContext(ctx).Errorf("sync backend failed, backend: %s, error: %v",
				content.Spec.BackendClaim, err)
		}
	}
	return err
}

// CheckConsistency if storage backend deleted, but memory, however, the backend still exists in the memory.
// so need to delete the backend from the memory.
func (b *BackendRegister) CheckConsistency(ctx context.Context, contents []v1.StorageBackendContent) {
	existBackends := map[string]v1.StorageBackendContent{}
	for _, content := range contents {
		_, name, err := pkgUtils.SplitMetaNamespaceKey(content.Spec.BackendClaim)
		if err != nil {
			continue
		}
		existBackends[name] = content
	}

	backends := b.cacheHandler.List(ctx)
	for _, bk := range backends {
		sbct, ok := existBackends[bk.Name]
		if !ok || !sbct.Status.Online {
			b.cacheHandler.Delete(ctx, bk.Name)
		}
	}
}

// UpdateOrRegisterOneBackend register one backend by sbct
func (b *BackendRegister) UpdateOrRegisterOneBackend(ctx context.Context, sbct *v1.StorageBackendContent) error {
	_, err := b.UpdateAndAddBackend(ctx, *sbct)
	if err != nil {
		return fmt.Errorf("add backend %s to cache failed, error: %w", sbct.Spec.BackendClaim, err)
	}
	return nil
}
