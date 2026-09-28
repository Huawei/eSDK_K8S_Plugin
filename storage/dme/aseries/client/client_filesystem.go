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
	"errors"
	"fmt"
	"net/http"
)

const (
	createFilesystemUrl       = "/rest/fileservice/v1/filesystems/customize-filesystems"
	filesystemWithFsIDUrl     = "/rest/fileservice/v1/filesystems/%s"
	syncFilesystemWithFsIDUrl = "/rest/fileservice/v1-sync/filesystems/%s"
	deleteFilesystemUrl       = "/rest/fileservice/v1/filesystems/delete"
	batchQueryFilesystemUrl   = "/rest/fileservice/v1/filesystems/query"
	queryNfsShareUrl          = "/rest/fileservice/v1/nfs-shares/query"
	deleteNfsShareUrl         = "/rest/fileservice/v1/nfs-shares/delete"
	queryDataTurboShareUrl    = "/rest/fileservice/v1/dpc-shares/query"
	deleteDataTurboShareUrl   = "/rest/fileservice/v1/dpc-shares/delete"
	queryDataTurboUserUrl     = "/rest/fileservice/v1/dpc-administrators/query"
	syncDeleteNfsShareUrl     = "/rest/fileservice/v1-sync/nfs-shares/%s" // shared with DTree
	createKVCacheUrl          = "/rest/fileservice/v1-sync/kv-cache-stores"
	deleteKVCacheUrl          = "/rest/fileservice/v1-sync/kv-cache-stores/%s"
	queryKVCacheUrl           = "/rest/kvcachemgmt/v1/kv-cache-stores/query"

	privateShareMode = "1"
)

// Filesystem defines interfaces for file system operations
type Filesystem interface {
	UpdateFileSystem(ctx context.Context, fsID string, params *UpdateFileSystemParams) error
	DeleteFileSystem(ctx context.Context, fsID string) error
	GetFileSystemByID(ctx context.Context, fsID string) (*FileSystemInfo, error)
	GetFileSystemByName(ctx context.Context, name string) (*FileSystemInfo, error)
	CreateFileSystem(ctx context.Context, params *CreateFilesystemParams) error
	GetDataTurboShareByPath(ctx context.Context, path string) (*DataTurboShare, error)
	DeleteDataTurboShare(ctx context.Context, id string) error
	GetDataTurboUserByName(ctx context.Context, name string) (*DataTurboAdmin, error)
	GetNfsShareByPath(ctx context.Context, path string) (*NfsShareInfo, error)
	DeleteNfsShare(ctx context.Context, id string) error
	DeleteNfsPrivateShare(ctx context.Context, id string) error
	CreateKVCache(ctx context.Context, params *CreateKVCacheParams) (*KVCacheStore, error)
	DeleteKVCache(ctx context.Context, kvcacheStoreId string) error
	QueryKVCache(ctx context.Context, params *QueryKVCacheParams) (*KVCacheStore, error)
	SyncDeleteFileSystem(ctx context.Context, fsID string) error
	SyncDeleteNfsShare(ctx context.Context, id string) error
}

// FilesystemClient defines client implements the Filesystem interface
type FilesystemClient struct {
	BaseClientInterface
}

// UpdateFileSystem used for update file system
func (cli *FilesystemClient) UpdateFileSystem(ctx context.Context, fsID string, params *UpdateFileSystemParams) error {
	if params == nil {
		return errors.New("params is nil")
	}
	err := gracefulCallWithSyncFallback(ctx, cli, http.MethodPut, &SyncFallbackUrls{
		SyncUrl:  fmt.Sprintf(syncFilesystemWithFsIDUrl, fsID),
		AsyncUrl: fmt.Sprintf(filesystemWithFsIDUrl, fsID),
	}, params)
	if err != nil {
		return fmt.Errorf("update filesystem for fsId: %s failed: %w", fsID, err)
	}
	return nil
}

// DeleteFileSystem used for delete file system
func (cli *FilesystemClient) DeleteFileSystem(ctx context.Context, fsID string) error {
	param := &DeleteFilesystemParam{
		FilesystemIds: []string{fsID},
	}
	err := gracefulCallWithTaskWait(ctx, cli, http.MethodPost, deleteFilesystemUrl, param)
	if err != nil {
		return fmt.Errorf("delete filesystem for fsId: %s failed: %w", fsID, err)
	}
	return nil
}

// GetFileSystemByID used for get file system by id
func (cli *FilesystemClient) GetFileSystemByID(ctx context.Context, fsID string) (*FileSystemInfo, error) {
	reqUrl := fmt.Sprintf(filesystemWithFsIDUrl, fsID)
	resp, err := gracefulCall[FileSystemInfo](ctx, cli, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("get filesystem for fsId: %s failed: %w", fsID, err)
	}
	return resp, nil
}

// GetFileSystemByName used for get filesystem list by name
func (cli *FilesystemClient) GetFileSystemByName(ctx context.Context, name string) (*FileSystemInfo, error) {
	param := &GetFilesystemParam{
		Name:      name,
		StorageId: cli.GetStorageID(),
	}
	resp, err := gracefulCall[BatchQueryFilesystemResponse](ctx, cli, http.MethodPost, batchQueryFilesystemUrl, param)
	if err != nil {
		return nil, fmt.Errorf("batch get filesystem for name: %s failed: %w", name, err)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	for _, info := range resp.Data {
		if info.Name == name {
			return info, nil
		}
	}
	return nil, nil
}

// CreateFileSystem used for create file system
func (cli *FilesystemClient) CreateFileSystem(ctx context.Context, params *CreateFilesystemParams) error {
	if params == nil {
		return errors.New("param is nil")
	}
	err := gracefulCallWithTaskWait(ctx, cli, http.MethodPost, createFilesystemUrl, params)
	if err != nil {
		return fmt.Errorf("create filesystem failed: %w", err)
	}
	return nil
}

// GetDataTurboShareByPath used for get DataTurbo share by path
func (cli *FilesystemClient) GetDataTurboShareByPath(ctx context.Context,
	path string) (*DataTurboShare, error) {
	param := &GetDataTurboShareParam{
		StorageId: cli.GetStorageID(),
		ZoneId:    cli.GetZoneID(),
		SharePath: path,
	}
	resp, err := gracefulCall[DataTurboShareResponse](ctx, cli, http.MethodPost, queryDataTurboShareUrl, param)
	if err != nil {
		return nil, fmt.Errorf("get DataTurbo share info by path: %s failed: %w", path, err)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return resp.Data[0], nil
}

// DeleteDataTurboShare used for delete DataTurbo share by id
func (cli *FilesystemClient) DeleteDataTurboShare(ctx context.Context, id string) error {
	param := &DeleteDataTurboShareParam{
		DpcShareIds: []string{id},
	}
	err := gracefulCallWithTaskWait(ctx, cli, http.MethodPost, deleteDataTurboShareUrl, param)
	if err != nil {
		return fmt.Errorf("delete DataTurbo share by id: %s failed: %w", id, err)
	}
	return nil
}

// GetDataTurboUserByName used for get DataTurbo share user
func (cli *FilesystemClient) GetDataTurboUserByName(ctx context.Context, name string) (*DataTurboAdmin, error) {
	param := &GetDataTurboAdminParam{
		StorageId: cli.GetStorageID(),
		ZoneId:    cli.GetZoneID(),
		Name:      name,
	}
	resp, err := gracefulCall[DataTurboAdminResponse](ctx, cli, http.MethodPost, queryDataTurboUserUrl, param)
	if err != nil {
		return nil, fmt.Errorf("get DataTurbo admin by name: %s failed: %w", name, err)
	}
	if len(resp.Administrators) == 0 {
		return nil, nil
	}
	return resp.Administrators[0], nil
}

// GetNfsShareByPath used for get nfs share by path
func (cli *FilesystemClient) GetNfsShareByPath(ctx context.Context, path string) (*NfsShareInfo, error) {
	param := &GetNfsShareParam{
		SharePath: path,
		StorageId: cli.GetStorageID(),
		ZoneId:    cli.GetZoneID(),
	}
	resp, err := gracefulCall[NfsShareInfoResponse](ctx, cli, http.MethodPost, queryNfsShareUrl, param)
	if err != nil {
		return nil, fmt.Errorf("get nfs share for path: %s failed: %w", path, err)
	}
	if len(resp.NfsShareInfoList) == 0 {
		return nil, nil
	}
	return resp.NfsShareInfoList[0], nil
}

// DeleteNfsShare used for delete nfs share by id
func (cli *FilesystemClient) DeleteNfsShare(ctx context.Context, id string) error {
	param := &DeleteNfsShareParam{
		NfsShareIds: []string{id},
	}
	err := gracefulCallWithTaskWait(ctx, cli, http.MethodPost, deleteNfsShareUrl, param)
	if err != nil {
		return fmt.Errorf("delete nfs share for share id: %s failed: %w", id, err)
	}
	return nil
}

// DeleteNfsPrivateShare used for delete nfs private share by id
func (cli *FilesystemClient) DeleteNfsPrivateShare(ctx context.Context, id string) error {
	param := &DeleteNfsShareParam{
		NfsShareIds:  []string{id},
		SharePrivate: privateShareMode,
	}
	err := gracefulCallWithTaskWait(ctx, cli, http.MethodPost, deleteNfsShareUrl, param)
	if err != nil {
		return fmt.Errorf("delete nfs share for share id: %s failed: %w", id, err)
	}
	return nil
}

// CreateKVCache creates a KVCache store (one-stop: FS + share + KVCache)
func (cli *FilesystemClient) CreateKVCache(ctx context.Context, params *CreateKVCacheParams) (*KVCacheStore, error) {
	resp, err := gracefulCall[KVCacheStore](ctx, cli, http.MethodPost, createKVCacheUrl, params)
	if err != nil {
		return nil, fmt.Errorf("create KVCache failed: %w", err)
	}
	return resp, nil
}

// DeleteKVCache deletes a KVCache store by raw_id (one-stop: deletes FS + share + KVCache)
func (cli *FilesystemClient) DeleteKVCache(ctx context.Context, kvcacheStoreId string) error {
	url := fmt.Sprintf(deleteKVCacheUrl, kvcacheStoreId)
	_, err := gracefulCall[struct{}](ctx, cli, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("delete KVCache %s failed: %w", kvcacheStoreId, err)
	}
	return nil
}

// QueryKVCache queries KVCache stores by parameters
func (cli *FilesystemClient) QueryKVCache(ctx context.Context, params *QueryKVCacheParams) (*KVCacheStore, error) {
	resp, err := gracefulCall[KVCacheQueryResponse](ctx, cli, http.MethodPost, queryKVCacheUrl, params)
	if err != nil {
		return nil, fmt.Errorf("query KVCache failed: %w", err)
	}
	if resp.Total == 0 || len(resp.KVCacheStores) == 0 {
		return nil, nil
	}
	return &resp.KVCacheStores[0], nil
}

// SyncDeleteFileSystem deletes a filesystem by ID using the synchronous API
func (cli *FilesystemClient) SyncDeleteFileSystem(ctx context.Context, fsID string) error {
	url := fmt.Sprintf(syncFilesystemWithFsIDUrl, fsID)
	_, err := gracefulCall[struct{}](ctx, cli, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("sync delete filesystem %s failed: %w", fsID, err)
	}
	return nil
}

// SyncDeleteNfsShare deletes an NFS share by ID using the synchronous API
func (cli *FilesystemClient) SyncDeleteNfsShare(ctx context.Context, id string) error {
	url := fmt.Sprintf(syncDeleteNfsShareUrl, id)
	_, err := gracefulCall[struct{}](ctx, cli, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("sync delete NFS share %s failed: %w", id, err)
	}
	return nil
}

// UpdateFileSystemParams defines update FileSystem params
type UpdateFileSystemParams struct {
	Capacity float64 `json:"capacity"`
}

// DeleteFilesystemParam defines delete FileSystem params
type DeleteFilesystemParam struct {
	FilesystemIds []string `json:"file_system_ids"`
}

// GetFilesystemParam defines get FileSystem params
type GetFilesystemParam struct {
	Name      string `json:"name"`
	StorageId string `json:"storage_id"`
}

// FilesystemSpec defines FileSystem spec
type FilesystemSpec struct {
	Name        string  `json:"name"`
	Capacity    float64 `json:"capacity"`
	Description string  `json:"description,omitempty"`
	Count       int32   `json:"count"`
}

// NfsClientAddition defines nfs client addition param
type NfsClientAddition struct {
	Name                     string `json:"name"`
	Permission               string `json:"permission"`
	WriteMode                string `json:"write_mode,omitempty"`
	PermissionConstraint     string `json:"permission_constraint"`
	RootPermissionConstraint string `json:"root_permission_constraint"`
}

// CreateNfsShareParam defines nfs share param
type CreateNfsShareParam struct {
	StorageId         string               `json:"storage_id"`
	SharePath         string               `json:"share_path"`
	Description       string               `json:"description,omitempty"`
	NfsClientAddition []*NfsClientAddition `json:"nfs_share_client_addition"`
}

// DpcAuth defines DataTurbo administrator list
type DpcAuth struct {
	DpcUserID  string `json:"dpc_user_id"`
	Permission string `json:"permission"`
}

// CreateDpcShareParam defines DataTurbo share param
type CreateDpcShareParam struct {
	Charset     string     `json:"charset"`
	Description string     `json:"description,omitempty"`
	DpcAuth     []*DpcAuth `json:"dpc_share_auth"`
}

// Tuning defines tuning param
type Tuning struct {
	AllocationType string `json:"allocation_type,omitempty"`
}

// CreateFilesystemParams defines create FileSystem param
type CreateFilesystemParams struct {
	SnapshotDirVisible  bool                 `json:"snapshot_dir_visible,omitempty"`
	StorageID           string               `json:"storage_id"`
	PoolRawID           string               `json:"pool_raw_id"`
	ZoneID              string               `json:"zone_id"`
	FilesystemSpecs     []*FilesystemSpec    `json:"filesystem_specs"`
	CreateNfsShareParam *CreateNfsShareParam `json:"create_nfs_share_param,omitempty"`
	CreateDpcShareParam *CreateDpcShareParam `json:"create_dpc_share_param,omitempty"`
	Tuning              *Tuning              `json:"tuning,omitempty"`
}

// BatchQueryFilesystemResponse is the response of get filesystem request
type BatchQueryFilesystemResponse struct {
	Total int64             `json:"total"`
	Data  []*FileSystemInfo `json:"data"`
}

// FileSystemInfo defines filesystem info
type FileSystemInfo struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Description             string `json:"description"`
	HealthStatus            string `json:"health_status"`
	RunningStatus           string `json:"running_status"`
	AllocType               string `json:"alloc_type"`
	Type                    string `json:"type"`
	StoragePoolName         string `json:"storage_pool_name"`
	TotalCapacityInByte     int64  `json:"total_capacity_in_byte"`     // Total file system capacity, unit: byte.
	AvailableCapacityInByte int64  `json:"available_capacity_in_byte"` // Available file system capacity, unit: byte.
}

// GetDataTurboShareParam defines get DataTurbo share info param
type GetDataTurboShareParam struct {
	StorageId string `json:"storage_id"`
	ZoneId    string `json:"zone_id"`
	SharePath string `json:"share_path"`
}

// DeleteDataTurboShareParam defines delete DataTurbo share info param
type DeleteDataTurboShareParam struct {
	DpcShareIds []string `json:"dpc_share_ids"`
}

// GetDataTurboAdminParam defines get DataTurbo admin info param
type GetDataTurboAdminParam struct {
	StorageId string `json:"storage_id"`
	ZoneId    string `json:"zone_id"`
	Name      string `json:"name"`
}

// DataTurboShareResponse is the response of get DataTurbo share request
type DataTurboShareResponse struct {
	Total int64             `json:"total"`
	Data  []*DataTurboShare `json:"data"`
}

// DataTurboShare defines DataTurbo info
type DataTurboShare struct {
	ID string `json:"id"`
}

// NfsShareInfoResponse is the response of get nfs share request
type NfsShareInfoResponse struct {
	Total            int64           `json:"total"`
	NfsShareInfoList []*NfsShareInfo `json:"nfs_share_info_list"`
}

// NfsShareInfo defines nfs share info
type NfsShareInfo struct {
	ID string `json:"id"`
}

// DataTurboAdminResponse is the response of get DataTurbo administrator request
type DataTurboAdminResponse struct {
	Total          int64             `json:"total"`
	Administrators []*DataTurboAdmin `json:"administrators"`
}

// DataTurboAdmin defines DataTurbo admin info
type DataTurboAdmin struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GetNfsShareParam defines get nfs share param
type GetNfsShareParam struct {
	SharePath string `json:"share_path"`
	StorageId string `json:"storage_id"`
	ZoneId    string `json:"zone_id"`
}

// DeleteNfsShareParam defines delete nfs share param
type DeleteNfsShareParam struct {
	NfsShareIds  []string `json:"nfs_share_ids"`
	SharePrivate string   `json:"share_private,omitempty"`
}

// CreateKVCacheParams defines parameters for creating KVCache
type CreateKVCacheParams struct {
	StorageID         string                 `json:"storage_id"`
	ZoneID            string                 `json:"zone_id"`
	PoolRawID         string                 `json:"pool_raw_id"`
	VstoreID          string                 `json:"vstore_id"`
	DataCleanupSwitch string                 `json:"data_cleanup_switch"`
	MaxSurvivalTime   int32                  `json:"max_survival_time,omitempty"`
	KVCacheStores     []KVCacheStoreBaseInfo `json:"kv_cache_stores"`
}

// KVCacheStoreBaseInfo defines a single KVCache store to create
type KVCacheStoreBaseInfo struct {
	Name        string `json:"name"`
	Capacity    int64  `json:"capacity"`
	Description string `json:"description,omitempty"`
}

// KVCacheStore defines KVCache store response
type KVCacheStore struct {
	ID       string `json:"id"`
	RawID    string `json:"raw_id"`
	Name     string `json:"name"`
	Capacity int64  `json:"capacity"`
}

// QueryKVCacheParams defines parameters for querying KVCache
type QueryKVCacheParams struct {
	ID        string `json:"id,omitempty"`
	StorageID string `json:"storage_id,omitempty"`
	Name      string `json:"name,omitempty"`
	VstoreID  string `json:"vstore_id,omitempty"`
	RawID     string `json:"raw_id,omitempty"`
	ZoneID    string `json:"zone_id,omitempty"`
}

// KVCacheQueryResponse defines the response of KVCache query
type KVCacheQueryResponse struct {
	Total         int            `json:"total"`
	KVCacheStores []KVCacheStore `json:"kv_cache_stores"`
}
