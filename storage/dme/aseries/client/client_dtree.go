/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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
	createDTreeUrl            = "/rest/fileservice/v1-sync/dtrees"
	deleteDTreeUrl            = "/rest/fileservice/v1-sync/dtrees/%s"
	queryDTreeListUrl         = "/rest/fileservice/v1/dtrees/query"
	createQuotaUrl            = "/rest/fileservice/v1-sync/quotas"
	deleteQuotaUrl            = "/rest/fileservice/v1-sync/quotas/%s"
	updateQuotaUrl            = "/rest/fileservice/v1-sync/quotas/%s"
	queryQuotaListUrl         = "/rest/fileservice/v1/quotas/query"
	dtreeQueryNfsShareListUrl = "/rest/fileservice/v1/nfs-shares/query"
	dtreeCreateNfsShareUrl    = "/rest/fileservice/v1-sync/nfs-shares"
	dtreeDeleteNfsShareUrl    = syncDeleteNfsShareUrl
	dtreeCreateDpcShareUrl    = "/rest/fileservice/v1-sync/dpc-shares"
	dtreeQueryDpcShareListUrl = "/rest/fileservice/v1/dpc-shares/query"
	dtreeDeleteDpcShareUrl    = "/rest/fileservice/v1-sync/dpc-shares/%s"

	quotaType = "directory_quota"
)

// DTree defines interfaces for DTree operations
type DTree interface {
	CreateDTree(ctx context.Context, params *CreateDTreeParams) (*CreateDTreeResponse, error)
	DeleteDTreeByID(ctx context.Context, dtreeID string) error
	GetDTreeByName(ctx context.Context, fsID, dtreeName string) (*DTreeInfo, error)
	CreateDTreeQuota(ctx context.Context, params *CreateQuotaParams) (*QuotaInfo, error)
	GetDTreeQuotaByRawID(ctx context.Context, parentRawID string) (*QuotaInfo, error)
	UpdateDTreeQuota(ctx context.Context, quotaID string, params *UpdateQuotaParams) error
	DeleteDTreeQuota(ctx context.Context, quotaID string) error
	GetNfsShareByDTreePath(ctx context.Context, sharePath string) (*DTreeNfsShareInfo, error)
	CreateDTreeNfsShare(ctx context.Context, params CreateNfsShareRequestBody) (*DTreeNfsShareInfo, error)
	DeleteDTreeNfsShare(ctx context.Context, nfsShareID string) error
	GetDataTurboShareByDTreePath(ctx context.Context, sharePath string) (*DTreeDpcShareInfo, error)
	CreateDTreeDpcShare(ctx context.Context, params CreateDpcShareParams) (*DTreeDpcShareInfo, error)
	DeleteDTreeDataTurboShare(ctx context.Context, dpcShareID string) error
}

// DTreeClient defines client implements the DTree interface
type DTreeClient struct {
	BaseClientInterface
}

// CreateDtreeParam defines a single DTree entry in create_dtrees_param
type CreateDtreeParam struct {
	DtreeName string `json:"dtree_name"`
	Count     int32  `json:"count"`
}

// DTreeNfsShareParam defines DTree nfs share param (NfsShareParam in API doc)
type DTreeNfsShareParam struct {
	SharePath         string              `json:"share_path"`
	Description       string              `json:"description,omitempty"`
	NfsClientAddition []NfsClientAddition `json:"nfs_share_client_addition"`
}

// DTreeDpcShareParam defines DTree DataTurbo share param (BaseCreateDpcShareRequest in API doc)
type DTreeDpcShareParam struct {
	Description  string    `json:"description,omitempty"`
	Charset      string    `json:"charset"`
	DpcShareAuth []DpcAuth `json:"dpc_share_auth"`
}

// CreateNfsShareRequestBody defines the outer request body for creating NFS share
type CreateNfsShareRequestBody struct {
	CreateNfsShareParam DTreeCreateNfsShareParam `json:"create_nfs_share_param"`
}

// DTreeCreateNfsShareParam defines create NFS share param (inner object) for DTree
type DTreeCreateNfsShareParam struct {
	SharePath         string              `json:"share_path"`
	FsID              string              `json:"fs_id"`
	Description       string              `json:"description,omitempty"`
	NfsClientAddition []NfsClientAddition `json:"nfs_share_client_addition"`
}

// CreateDpcShareParams defines create DPC share request params
type CreateDpcShareParams struct {
	DtreeID      string    `json:"dtree_id"`
	FsID         string    `json:"fs_id"`
	Description  string    `json:"description,omitempty"`
	Charset      string    `json:"charset"`
	DpcShareAuth []DpcAuth `json:"dpc_share_auth"`
}

// DTreeCreateQuotaParam defines DTree create quota param (CreateQuotaParam in API doc)
type DTreeCreateQuotaParam struct {
	QuotaType      string `json:"quota_type"`
	SpaceHardQuota int64  `json:"space_hard_quota"`
}

// CreateDTreeParams defines create DTree request params
type CreateDTreeParams struct {
	CreateDtreesParam   []*CreateDtreeParam      `json:"create_dtrees_param"`
	QuotaSwitch         bool                     `json:"quota_switch"`
	StorageID           string                   `json:"storage_id"`
	FsID                string                   `json:"fs_id"`
	UnixPermissions     string                   `json:"unix_permissions,omitempty"`
	CreateQuotaParam    []*DTreeCreateQuotaParam `json:"create_quota_param,omitempty"`
	CreateNfsShareParam *DTreeNfsShareParam      `json:"create_nfs_share_param,omitempty"`
	DataturboShare      *DTreeDpcShareParam      `json:"dataturbo_share,omitempty"`
}

// CreateDTreeResponse defines create DTree response
type CreateDTreeResponse struct {
	DtreeID             string `json:"dtree_id"`
	DtreeRawID          string `json:"dtree_raw_id"`
	DtreeName           string `json:"dtree_name"`
	NfsShareID          string `json:"nfs_share_id"`
	NfsShareRawID       string `json:"nfs_share_raw_id"`
	NfsSharePath        string `json:"nfs_share_path"`
	DataturboShareID    string `json:"dataturbo_share_id"`
	DataturboShareRawID string `json:"dataturbo_share_raw_id"`
	DataturboSharePath  string `json:"dataturbo_share_path"`
}

// DTreeInfo defines DTree info
type DTreeInfo struct {
	ID          string `json:"id"`
	RawID       string `json:"id_in_storage"`
	Name        string `json:"name"`
	FsID        string `json:"fs_id"`
	FsName      string `json:"fs_name"`
	QuotaSwitch bool   `json:"quota_switch"`
}

// QueryDTreeListParam defines query DTree list param
type QueryDTreeListParam struct {
	StorageID string `json:"storage_id"`
	FsID      string `json:"fs_id"`
	Name      string `json:"name"`
}

// QueryDTreeListResponse defines query DTree list response
type QueryDTreeListResponse struct {
	Total  int32        `json:"total"`
	Dtrees []*DTreeInfo `json:"dtrees"`
}

// DTreeNfsShareInfo defines DTree NFS share info
type DTreeNfsShareInfo struct {
	ID        string `json:"id"`
	SharePath string `json:"share_path"`
}

// QueryDTreeNfsShareListParam defines query DTree NFS share list param
type QueryDTreeNfsShareListParam struct {
	ExactSharePath string `json:"exact_share_path"`
	StorageID      string `json:"storage_id"`
}

// QueryDTreeNfsShareListResponse defines query DTree NFS share list response
type QueryDTreeNfsShareListResponse struct {
	Total int32                `json:"total"`
	Data  []*DTreeNfsShareInfo `json:"nfs_share_info_list"`
}

// DTreeDpcShareInfo defines DTree DPC share info
type DTreeDpcShareInfo struct {
	ID        string `json:"id"`
	SharePath string `json:"share_path"`
}

// QueryDTreeDpcShareListParam defines query DTree DPC share list param
type QueryDTreeDpcShareListParam struct {
	SharePath string `json:"share_path"`
	StorageID string `json:"storage_id"`
}

// QueryDTreeDpcShareListResponse defines query DTree DPC share list response
type QueryDTreeDpcShareListResponse struct {
	Total int32                `json:"total"`
	Data  []*DTreeDpcShareInfo `json:"data"`
}

// CreateQuotaParams defines create quota request params
type CreateQuotaParams struct {
	ParentID       string `json:"parent_id"`
	ParentType     string `json:"parent_type"`
	QuotaType      string `json:"quota_type"`
	SpaceHardQuota int64  `json:"space_hard_quota"`
}

// UpdateQuotaParams defines update quota params
type UpdateQuotaParams struct {
	SpaceHardQuota int64 `json:"space_hard_quota"`
}

// QuotaInfo defines quota info
type QuotaInfo struct {
	ID             string `json:"id"`
	SpaceHardQuota int64  `json:"space_hard_quota"`
	ParentRawID    string `json:"parent_raw_id"`
	ParentID       string `json:"parent_id"`
}

// QueryQuotaListParam defines query quota list param
type QueryQuotaListParam struct {
	ParentRawID string `json:"parent_raw_id"`
	QuotaType   string `json:"quota_type"`
	StorageID   string `json:"storage_id"`
}

// QueryQuotaListResponse defines query quota list response
type QueryQuotaListResponse struct {
	Total int32        `json:"total"`
	Datas []*QuotaInfo `json:"datas"`
}

// CreateDTree used for create DTree
func (cli *DTreeClient) CreateDTree(ctx context.Context, params *CreateDTreeParams) (*CreateDTreeResponse, error) {
	resp, err := gracefulCall[CreateDTreeResponse](ctx, cli, http.MethodPost, createDTreeUrl, params)
	if err != nil {
		return nil, fmt.Errorf("create DTree failed: %w", err)
	}
	return resp, nil
}

// DeleteDTreeByID used for delete DTree by ID
func (cli *DTreeClient) DeleteDTreeByID(ctx context.Context, dtreeID string) error {
	reqUrl := fmt.Sprintf(deleteDTreeUrl, dtreeID)
	_, err := gracefulCall[struct{}](ctx, cli, http.MethodDelete, reqUrl, nil)
	if err != nil {
		return fmt.Errorf("delete DTree for dtreeID: %s failed: %w", dtreeID, err)
	}
	return nil
}

// GetDTreeByName used for get DTree by name
func (cli *DTreeClient) GetDTreeByName(ctx context.Context, fsID, dtreeName string) (*DTreeInfo, error) {
	param := &QueryDTreeListParam{
		StorageID: cli.GetStorageID(),
		FsID:      fsID,
		Name:      dtreeName,
	}
	resp, err := gracefulCall[QueryDTreeListResponse](ctx, cli, http.MethodPost, queryDTreeListUrl, param)
	if err != nil {
		return nil, fmt.Errorf("get DTree by name: %s failed: %w", dtreeName, err)
	}
	if len(resp.Dtrees) == 0 {
		return nil, nil
	}
	for _, dtree := range resp.Dtrees {
		if dtree.Name == dtreeName {
			return dtree, nil
		}
	}
	return nil, nil
}

// CreateDTreeQuota used for create DTree quota
func (cli *DTreeClient) CreateDTreeQuota(ctx context.Context, params *CreateQuotaParams) (*QuotaInfo, error) {
	resp, err := gracefulCall[QuotaInfo](ctx, cli, http.MethodPost, createQuotaUrl, params)
	if err != nil {
		return nil, fmt.Errorf("create DTree quota failed: %w", err)
	}
	return resp, nil
}

// GetDTreeQuotaByRawID used for get DTree quota by parent raw ID (storage device ID)
func (cli *DTreeClient) GetDTreeQuotaByRawID(ctx context.Context, parentRawID string) (*QuotaInfo, error) {
	param := &QueryQuotaListParam{
		ParentRawID: parentRawID,
		QuotaType:   quotaType,
		StorageID:   cli.GetStorageID(),
	}
	resp, err := gracefulCall[QueryQuotaListResponse](ctx, cli, http.MethodPost, queryQuotaListUrl, param)
	if err != nil {
		return nil, fmt.Errorf("get DTree quota for parentRawID: %s failed: %w", parentRawID, err)
	}
	if len(resp.Datas) == 0 {
		return nil, nil
	}
	return resp.Datas[0], nil
}

// UpdateDTreeQuota used for update DTree quota
func (cli *DTreeClient) UpdateDTreeQuota(ctx context.Context, quotaID string, params *UpdateQuotaParams) error {
	reqUrl := fmt.Sprintf(updateQuotaUrl, quotaID)
	_, err := gracefulCall[struct{}](ctx, cli, http.MethodPut, reqUrl, params)
	if err != nil {
		return fmt.Errorf("update DTree quota for quotaID: %s failed: %w", quotaID, err)
	}
	return nil
}

// DeleteDTreeQuota used for delete DTree quota
func (cli *DTreeClient) DeleteDTreeQuota(ctx context.Context, quotaID string) error {
	reqUrl := fmt.Sprintf(deleteQuotaUrl, quotaID)
	_, err := gracefulCall[struct{}](ctx, cli, http.MethodDelete, reqUrl, nil)
	if err != nil {
		return fmt.Errorf("delete DTree quota for quotaID: %s failed: %w", quotaID, err)
	}
	return nil
}

// GetNfsShareByDTreePath used for get NFS share by DTree path
func (cli *DTreeClient) GetNfsShareByDTreePath(ctx context.Context, sharePath string) (*DTreeNfsShareInfo, error) {
	param := &QueryDTreeNfsShareListParam{
		ExactSharePath: sharePath,
		StorageID:      cli.GetStorageID(),
	}
	resp, err := gracefulCall[QueryDTreeNfsShareListResponse](ctx, cli, http.MethodPost, dtreeQueryNfsShareListUrl, param)
	if err != nil {
		return nil, fmt.Errorf("get NFS share by DTree path: %s failed: %w", sharePath, err)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return resp.Data[0], nil
}

// DeleteDTreeNfsShare used for delete DTree NFS share by ID
func (cli *DTreeClient) DeleteDTreeNfsShare(ctx context.Context, nfsShareID string) error {
	reqUrl := fmt.Sprintf(dtreeDeleteNfsShareUrl, nfsShareID)
	_, err := gracefulCall[struct{}](ctx, cli, http.MethodDelete, reqUrl, nil)
	if err != nil {
		return fmt.Errorf("delete NFS share for nfsShareID: %s failed: %w", nfsShareID, err)
	}
	return nil
}

// CreateDTreeNfsShare used for create NFS share independently
func (cli *DTreeClient) CreateDTreeNfsShare(ctx context.Context,
	params CreateNfsShareRequestBody) (*DTreeNfsShareInfo, error) {
	resp, err := gracefulCall[DTreeNfsShareInfo](ctx, cli, http.MethodPost, dtreeCreateNfsShareUrl, params)
	if err != nil {
		return nil, fmt.Errorf("create NFS share for path %s failed: %w", params.CreateNfsShareParam.SharePath, err)
	}
	return resp, nil
}

// GetDataTurboShareByDTreePath used for get DataTurbo share by DTree path
func (cli *DTreeClient) GetDataTurboShareByDTreePath(ctx context.Context,
	sharePath string) (*DTreeDpcShareInfo, error) {
	param := &QueryDTreeDpcShareListParam{
		SharePath: sharePath,
		StorageID: cli.GetStorageID(),
	}
	resp, err := gracefulCall[QueryDTreeDpcShareListResponse](ctx, cli, http.MethodPost, dtreeQueryDpcShareListUrl, param)
	if err != nil {
		return nil, fmt.Errorf("get DataTurbo share by DTree path: %s failed: %w", sharePath, err)
	}
	if len(resp.Data) == 0 {
		return nil, nil
	}
	return resp.Data[0], nil
}

// DeleteDTreeDataTurboShare used for delete DTree DataTurbo share by ID
func (cli *DTreeClient) DeleteDTreeDataTurboShare(ctx context.Context, dpcShareID string) error {
	reqUrl := fmt.Sprintf(dtreeDeleteDpcShareUrl, dpcShareID)
	_, err := gracefulCall[struct{}](ctx, cli, http.MethodDelete, reqUrl, nil)
	if err != nil {
		return fmt.Errorf("delete DataTurbo share for dpcShareID: %s failed: %w", dpcShareID, err)
	}
	return nil
}

// CreateDTreeDpcShare used for create DPC share independently
func (cli *DTreeClient) CreateDTreeDpcShare(ctx context.Context,
	params CreateDpcShareParams) (*DTreeDpcShareInfo, error) {
	resp, err := gracefulCall[DTreeDpcShareInfo](ctx, cli, http.MethodPost, dtreeCreateDpcShareUrl, params)
	if err != nil {
		return nil, fmt.Errorf("create DPC share for DTree %s failed: %w", params.DtreeID, err)
	}
	return resp, nil
}
