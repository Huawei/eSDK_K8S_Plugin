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

package handler

import (
	"context"
	"strconv"

	"huawei-csi-driver/lib/drcsi"
	"huawei-csi-driver/pkg/constants"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
)

// StorageBackendDetails backend details
type StorageBackendDetails struct {
	Capabilities   map[string]bool
	Specifications map[string]string
	Pools          []*drcsi.Pool
}

// StorageServiceInterface query backend operation set
type StorageServiceInterface interface {
	GetBackendDetails(ctx context.Context, name, contentName string) (StorageBackendDetails, error)
}

// StorageHandler backend query handler
type StorageHandler struct {
	cacheHandler BackendCacheWrapperInterface
	register     BackendRegisterInterface
	fetchHandler BackendFetchInterface
}

// NewStorageHandler init instance of StorageHandler
func NewStorageHandler() *StorageHandler {
	return &StorageHandler{
		cacheHandler: NewCacheWrapper(),
		register:     NewBackendRegister(),
		fetchHandler: NewBackendFetcher(),
	}
}

// GetBackendDetails query backend details
func (s *StorageHandler) GetBackendDetails(ctx context.Context,
	name, contentName string) (StorageBackendDetails, error) {
	bk, err := s.register.LoadOrRebuildOneBackend(ctx, name, contentName)
	if err != nil {
		log.AddContext(ctx).Warningf("load cache backend %s failed, error: %v", name, err)
		return StorageBackendDetails{}, err
	}

	capabilities, specifications, err := bk.Plugin.UpdateBackendCapabilities(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("query backend %s capabilities failed, error: %v", name, err)
		return StorageBackendDetails{}, err
	}

	var poolNames []string
	for _, pool := range bk.Pools {
		poolNames = append(poolNames, pool.Name)
	}

	poolCapabilities, err := bk.Plugin.UpdatePoolCapabilities(ctx, poolNames)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot update pool capabilities of backend %s: %v", name, err)
		return StorageBackendDetails{}, err
	}

	poolCapabilityMap := pkgUtils.ConvertToMapValueX[map[string]interface{}](ctx, poolCapabilities)
	poolCapacities := make([]*drcsi.Pool, 0)
	for _, pool := range bk.Pools {
		capacities := make(map[string]string)
		poolCapabilityInt64Map := pkgUtils.ConvertToMapValueX[int64](ctx, poolCapabilityMap[pool.GetName()])
		for k, v := range poolCapabilityInt64Map {
			capacities[k] = strconv.FormatInt(v, constants.DefaultIntBase)
		}
		poolCapacities = append(poolCapacities, &drcsi.Pool{
			Name:       pool.Name,
			Capacities: capacities,
		})
	}
	return StorageBackendDetails{
		Capabilities:   pkgUtils.ConvertToMapValueX[bool](ctx, capabilities),
		Specifications: pkgUtils.ConvertToMapValueX[string](ctx, specifications),
		Pools:          poolCapacities,
	}, nil
}
