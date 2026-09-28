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

const (
	storagePoolUrl      = "/rest/storagemgmt/v1/hyperscale-pools/query"
	storagePoolQueryUrl = "/rest/storagemgmt/v1/storagepools/query"
	zoneQueryUrl        = "/rest/storageclusterservice/v1/zones/query"
	vstoreQueryUrl      = "/rest/fileservice/v1/vstores/query"
)

// System defines interfaces for system operations
type System interface {
	GetHyperScalePoolByName(ctx context.Context, name string) (*HyperScalePool, error)
	GetHyperScalePools(ctx context.Context) ([]*HyperScalePool, error)
	GetStoragePools(ctx context.Context, zoneID string) ([]*StoragePool, error)
	QueryZoneBySN(ctx context.Context, sn string) (*ZoneInfo, error)
	QueryVstores(ctx context.Context, params *VstoreQueryParams) ([]*VstoreInfo, error)
	GetStoragePoolByName(ctx context.Context, name, zoneID string) (*StoragePool, error)
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

// PoolRef is the interface for pool types that provide a raw ID
type PoolRef interface {
	GetRawID() string
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

// GetRawID returns the raw ID of the HyperScalePool
func (p *HyperScalePool) GetRawID() string { return p.RawId }

// ZoneQueryParams defines query zone parameters
type ZoneQueryParams struct {
	SN string `json:"sn"`
}

// ZoneListResponse is the response of zone query
type ZoneListResponse struct {
	Total int         `json:"total"`
	Datas []*ZoneInfo `json:"datas"`
}

// ZoneInfo defines zone information
type ZoneInfo struct {
	ID       string `json:"id"`
	NativeID string `json:"native_id"`
	Name     string `json:"name"`
	SN       string `json:"sn"`
}

// QueryZoneBySN queries zone by serial number
func (cli *SystemClient) QueryZoneBySN(ctx context.Context, sn string) (*ZoneInfo, error) {
	param := &ZoneQueryParams{SN: sn}
	resp, err := gracefulCall[ZoneListResponse](ctx, cli, http.MethodPost, zoneQueryUrl, param)
	if err != nil {
		return nil, fmt.Errorf("query zone by SN %s failed: %w", sn, err)
	}
	if resp.Total == 0 || len(resp.Datas) == 0 {
		return nil, fmt.Errorf("zone with SN %s not found", sn)
	}
	return resp.Datas[0], nil
}

// VstoreQueryParams defines query vstore parameters
type VstoreQueryParams struct {
	Name      string `json:"name"`
	StorageID string `json:"storage_id"`
	ZoneID    string `json:"zone_id"`
}

// VstoreListResponse is the response of vstore query
type VstoreListResponse struct {
	Total   int           `json:"total"`
	Vstores []*VstoreInfo `json:"vstores"`
}

// VstoreInfo defines vstore information
type VstoreInfo struct {
	ID    string `json:"id"`
	RawID string `json:"raw_id"`
	Name  string `json:"name"`
}

// QueryVstores queries vstores by parameters
func (cli *SystemClient) QueryVstores(ctx context.Context, params *VstoreQueryParams) ([]*VstoreInfo, error) {
	resp, err := gracefulCall[VstoreListResponse](ctx, cli, http.MethodPost, vstoreQueryUrl, params)
	if err != nil {
		return nil, fmt.Errorf("query vstores failed: %w", err)
	}
	if resp.Total == 0 || len(resp.Vstores) == 0 {
		return nil, fmt.Errorf("vstore not found with params: name=%s, storageID=%s, zoneID=%s",
			params.Name, params.StorageID, params.ZoneID)
	}
	return resp.Vstores, nil
}

// StoragePoolQueryParams defines query storage pool parameters
type StoragePoolQueryParams struct {
	StorageID string `json:"storage_id"`
	ZoneID    string `json:"zone_id"`
}

// StoragePoolListResponse is the response of storage pool query
type StoragePoolListResponse struct {
	Total int            `json:"total"`
	Datas []*StoragePool `json:"datas"`
}

// StoragePool defines storage pool information (zone-scoped)
type StoragePool struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	RawID         string  `json:"raw_id"`
	TotalCapacity float64 `json:"total_capacity"` // Total capacity, unit: MB
	FreeCapacity  float64 `json:"free_capacity"`  // Free capacity, unit: MB
}

// GetRawID returns the raw ID of the StoragePool
func (p *StoragePool) GetRawID() string { return p.RawID }

// GetStoragePools used for get all zone-scoped storage pools
func (cli *SystemClient) GetStoragePools(ctx context.Context, zoneID string) ([]*StoragePool, error) {
	params := &StoragePoolQueryParams{
		StorageID: cli.GetStorageID(),
		ZoneID:    zoneID,
	}
	resp, err := gracefulCall[StoragePoolListResponse](ctx, cli, http.MethodPost, storagePoolQueryUrl, params)
	if err != nil {
		return nil, fmt.Errorf("get storage pools failed: %w", err)
	}
	return resp.Datas, nil
}

// GetStoragePoolByName queries a zone-scoped storage pool by name
func (cli *SystemClient) GetStoragePoolByName(ctx context.Context, name, zoneID string) (*StoragePool, error) {
	pools, err := cli.GetStoragePools(ctx, zoneID)
	if err != nil {
		return nil, fmt.Errorf("get storage pool by name %s failed: %w", name, err)
	}
	for _, pool := range pools {
		if pool.Name == name {
			return pool, nil
		}
	}
	return nil, nil
}
