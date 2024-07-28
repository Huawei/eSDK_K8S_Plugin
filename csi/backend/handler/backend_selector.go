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

	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/csi/backend/model"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// BackendSelectInterface all backend select operation set
type BackendSelectInterface interface {
	SelectBackend(context.Context, string) (*model.Backend, error)
	SelectPoolPair(context.Context, int64, map[string]interface{}) (*model.SelectPoolPair, error)
	SelectLocalPool(context.Context, int64, map[string]interface{}) ([]*model.StoragePool, error)
	SelectRemotePool(context.Context, int64, string, map[string]interface{}) (*model.StoragePool, error)
}

// BackendSelector backend selector
type BackendSelector struct {
	cacheHandler BackendCacheWrapperInterface
	register     BackendRegisterInterface
}

// NewBackendSelector init instance of BackendSelector
func NewBackendSelector() *BackendSelector {
	return &BackendSelector{
		cacheHandler: NewCacheWrapper(),
		register:     NewBackendRegister(),
	}
}

// SelectBackend select one backend by name
func (b *BackendSelector) SelectBackend(ctx context.Context, name string) (*model.Backend, error) {
	return b.register.LoadOrRegisterOneBackend(ctx, name)
}

// SelectPoolPair select local pool and remote pool
func (b *BackendSelector) SelectPoolPair(ctx context.Context, requestSize int64,
	params map[string]interface{}) (*model.SelectPoolPair, error) {
	localPools, err := b.SelectLocalPool(ctx, requestSize, params)
	if err != nil {
		return nil, err
	}
	var poolPairs []model.SelectPoolPair
	for _, localPool := range localPools {
		remotePool, err := b.SelectRemotePool(ctx, requestSize, localPool.Parent, params)
		if err != nil {
			return nil, err
		}
		log.AddContext(ctx).Debugf("Select remote pool is %v.", remotePool)
		poolPairs = append(poolPairs, model.SelectPoolPair{Local: localPool, Remote: remotePool})
	}

	local, remote, err := backend.WeightPools(ctx, requestSize, params, localPools, poolPairs)
	if err != nil {
		log.AddContext(ctx).Errorf("weight pools failed, error: %v", err)
		return nil, err
	}

	return &model.SelectPoolPair{Local: local, Remote: remote}, nil
}

// SelectLocalPool select local pool
func (b *BackendSelector) SelectLocalPool(ctx context.Context, requestSize int64,
	parameters map[string]interface{}) ([]*model.StoragePool, error) {
	candidatePools := b.cacheHandler.LoadCacheStoragePools(ctx)
	if len(candidatePools) == 0 {
		return nil, fmt.Errorf("no found any available storage pool for volume %v", parameters)
	}

	return filterPool(ctx, requestSize, candidatePools, parameters, backend.PrimaryFilterFuncs)
}

// SelectRemotePool select remote pool
func (b *BackendSelector) SelectRemotePool(ctx context.Context, requestSize int64, localBackendName string,
	parameters map[string]interface{}) (*model.StoragePool, error) {
	hyperMetro, hyperMetroOK := parameters["hyperMetro"].(string)
	replication, replicationOK := parameters["replication"].(string)

	if hyperMetroOK && utils.StrToBool(ctx, hyperMetro) &&
		replicationOK && utils.StrToBool(ctx, replication) {
		return nil, fmt.Errorf("cannot create volume with hyperMetro and replication properties: %v", parameters)
	}

	var err error
	var remotePools []*model.StoragePool
	if hyperMetroOK && utils.StrToBool(ctx, hyperMetro) {
		localBackend, exists := b.cacheHandler.Load(localBackendName)
		if !exists {
			return nil, fmt.Errorf("backend %s does not exist in cache", localBackendName)
		}
		if localBackend.MetroBackend == nil {
			return nil, fmt.Errorf("no metro backend of %s exists for volume: %v", localBackendName, parameters)
		}
		log.AddContext(ctx).Debugf("load backend %s success: %+v", localBackendName, localBackend)
		remotePools, err = filterPool(ctx,
			requestSize, localBackend.MetroBackend.Pools, parameters, backend.SecondaryFilterFuncs)
	}

	if replicationOK && utils.StrToBool(ctx, replication) {
		localBackend, exists := b.cacheHandler.Load(localBackendName)
		if exists && localBackend.ReplicaBackend == nil {
			return nil, fmt.Errorf("no replica backend exists for volume: %v", parameters)
		}
		remotePools, err = filterPool(ctx, requestSize, localBackend.Pools, parameters, backend.SecondaryFilterFuncs)
	}

	if err != nil {
		return nil, fmt.Errorf("select remote pool failed: %v", err)
	}

	if len(remotePools) == 0 {
		return nil, nil
	}

	// weight the remote pool
	return backend.WeightSinglePools(ctx, requestSize, parameters, remotePools)
}

func filterPool(ctx context.Context, requestSize int64, candidatePools []*model.StoragePool,
	parameters map[string]interface{}, filters [][]interface{}) ([]*model.StoragePool, error) {
	var err error
	if candidatePools, err = backend.FilterByCapability(ctx, parameters, candidatePools, filters); err != nil {
		return nil, err
	}

	if candidatePools, err = backend.FilterByTopology(parameters, candidatePools); err != nil {
		return nil, err
	}

	allocType, _ := parameters["allocType"].(string)
	return backend.FilterByCapacity(requestSize, allocType, candidatePools), nil
}
