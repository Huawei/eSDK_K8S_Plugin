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

// Package model package for backend model
package model

import (
	"context"
	"huawei-csi-driver/utils/log"

	xuanwuV1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/backend/plugin"
)

// StorageBackendTuple contains sbc and sbct
type StorageBackendTuple struct {
	Claim   *xuanwuV1.StorageBackendClaim
	Content *xuanwuV1.StorageBackendContent
}

// Backend for storage
type Backend struct {
	Name                string
	Storage             string
	Available           bool
	Plugin              plugin.Plugin
	Pools               []*StoragePool
	Parameters          map[string]interface{}
	SupportedTopologies []map[string]string
	AccountName         string

	MetroDomain       string
	MetrovStorePairID string
	MetroBackendName  string
	MetroBackend      *Backend

	ReplicaBackendName string
	ReplicaBackend     *Backend
}

// SetAvailable set Backend available
func (b *Backend) SetAvailable(ctx context.Context, available bool) {
	if b.Available != available {
		log.AddContext(ctx).Infof("change cache backend %s online to %v", b.Name, available)
	}
	b.Available = available
}

// UpdatePools update Backend pools
func (b *Backend) UpdatePools(ctx context.Context, sbct *xuanwuV1.StorageBackendContent) {
	for _, pool := range b.Pools {
		pool.UpdatePoolBySBCT(ctx, sbct)
	}
}

// SelectPoolPair for pool pair
type SelectPoolPair struct {
	Local  *StoragePool
	Remote *StoragePool
}
