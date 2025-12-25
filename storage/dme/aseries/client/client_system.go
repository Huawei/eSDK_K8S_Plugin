/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

// Package client provides DME A-series storage client
package client

import (
	"context"
	"fmt"
	"net/http"
)

const storagePoolUrl = "/rest/storagemgmt/v1/hyperscale-pools/query"

// System defines interfaces for system operations
type System interface {
	GetHyperScalePoolByName(ctx context.Context, name string) (*HyperScalePool, error)
	GetHyperScalePools(ctx context.Context) ([]*HyperScalePool, error)
}

// SystemClient defines client implements the System interface
type SystemClient struct {
	BaseClientInterface
}

// GetPoolParams defines query storage pool param
type GetPoolParams struct {
	StorageId string `json:"storage_id"`
}

// GetHyperScalePoolByName used for get pool by name
func (cli *SystemClient) GetHyperScalePoolByName(ctx context.Context, name string) (*HyperScalePool, error) {
	pools, err := cli.GetHyperScalePools(ctx)
	if err != nil {
		return nil, fmt.Errorf("get storage pool by name:%s failed: %w", name, err)
	}
	for _, pool := range pools {
		if pool.Name == name {
			return pool, nil
		}
	}
	return nil, nil
}

// GetHyperScalePools used for get all pools
func (cli *SystemClient) GetHyperScalePools(ctx context.Context) ([]*HyperScalePool, error) {
	params := &GetPoolParams{
		StorageId: cli.GetStorageID(),
	}
	resp, err := gracefulCall[HyperScalePoolResponse](ctx, cli, http.MethodPost, storagePoolUrl, params)
	if err != nil {
		return nil, fmt.Errorf("get storage pool failed: %w", err)
	}
	return resp.Data, nil
}

// HyperScalePoolResponse is the response of get storage pool request
type HyperScalePoolResponse struct {
	Total int64             `json:"total"`
	Data  []*HyperScalePool `json:"data"`
}

// HyperScalePool defines storage pool information
type HyperScalePool struct {
	ID            string  `json:"id"`
	RawId         string  `json:"raw_id"`
	Name          string  `json:"name"`
	TotalCapacity float64 `json:"total_capacity"` // Total capacity, unit: MB.
	CapacityUsage float64 `json:"capacity_usage"`
	FreeCapacity  float64 `json:"free_capacity"` // Free capacity, unit: MB
}
