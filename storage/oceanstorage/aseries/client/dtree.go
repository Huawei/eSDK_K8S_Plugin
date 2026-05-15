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

// Package client provides oceanstor A-series storage client
package client

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/api"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/api/rest"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

const (
	dtreeNotExist   = 1077955336
	dtreeNotFound   = 1077955080
	dtreeParentType = "16445"
	filesystemType  = 40
)

// ASeriesDtree defines interfaces for DTree operations on A-series storage
type ASeriesDtree interface {
	// GetDTreeByName gets DTree by parent filesystem name and dtree name
	GetDTreeByName(ctx context.Context, parentName, dtreeName, vstoreId string) (map[string]interface{}, error)
	// CreateDTree creates a DTree with the specified data
	CreateDTree(ctx context.Context, req *DTreeCreateRequest) (map[string]interface{}, error)
	// DeleteDTreeByID deletes a DTree by ID
	DeleteDTreeByID(ctx context.Context, vStoreID, dtreeID string) error
	// DeleteDTreeByName deletes a DTree by name parameters
	DeleteDTreeByName(ctx context.Context, vStoreID, parentName, dtreeName string) error
	// UpdateDTree modifies a DTree by ID
	UpdateDTree(ctx context.Context, dtreeID string, req *DTreeUpdateRequest) error
	// GetDTreeByID gets DTree information by ID
	GetDTreeByID(ctx context.Context, dtreeID string) (map[string]interface{}, error)
	// CreateDTreeQuota creates a quota for DTree
	CreateDTreeQuota(ctx context.Context, req *DTreeQuotaRequest) (map[string]interface{}, error)
	// GetDTreeQuota gets quota for DTree by parent ID
	GetDTreeQuota(ctx context.Context, parentID, vStoreID string) (map[string]interface{}, error)
	// UpdateDTreeQuota updates quota for DTree
	UpdateDTreeQuota(ctx context.Context, quotaID string, req *DTreeQuotaUpdateRequest) error
	// DeleteDTreeQuota deletes a DTree quota
	DeleteDTreeQuota(ctx context.Context, quotaID, vStoreID string) error
	// DeleteDTreeQuotaByParentID deletes all quotas for a specific DTree
	DeleteDTreeQuotaByParentID(ctx context.Context, parentID, vStoreID string) error
}

// GetDTreeByName gets DTree by parent filesystem name and dtree name
func (cli *OceanASeriesClient) GetDTreeByName(ctx context.Context,
	parentName, dtreeName, vstoreId string) (map[string]interface{}, error) {
	restPath := rest.NewRequestPath(api.ManageDTreePath)
	restPath.SetQuery("PARENTNAME", parentName)
	restPath.SetQuery("NAME", dtreeName)
	restPath.SetQuery("vstoreId", vstoreId)
	encodedPath, err := restPath.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode path and queries for DTree '%s' under parent '%s': %w", dtreeName,
			parentName, err)
	}
	resp, err := cli.Get(ctx, encodedPath, nil)
	if err != nil {
		return nil, fmt.Errorf("http get request failed for DTree '%s' under parent '%s' with vStoreID '%s': %w",
			dtreeName, parentName, cli.GetvStoreID(), err)
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, fmt.Errorf("failed to format response error for DTree '%s' under parent '%s' [code: %d, msg: %s]: %w",
			dtreeName, parentName, code, msg, err)
	}

	if code == dtreeNotExist || code == dtreeNotFound || resp.Data == nil {
		return nil, nil
	}

	if code != storage.SuccessCode {
		return nil, fmt.Errorf("get DTree by name '%s' failed [code: %d]: %s", dtreeName, code, msg)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok || len(respData) == 0 {
		return nil, nil
	}

	return respData, nil
}

// DTreeCreateRequest defines the request structure for creating a DTree
type DTreeCreateRequest struct {
	Name            string `json:"name"`
	ParentName      string `json:"parentName"`
	UnixPermissions string `json:"unixPermissions,omitempty"`
	VStoreID        string `json:"vstoreId,omitempty"`
}

// CreateDTree creates a DTree with the specified configuration data
func (cli *OceanASeriesClient) CreateDTree(ctx context.Context,
	req *DTreeCreateRequest) (map[string]interface{}, error) {

	reqMap := map[string]interface{}{
		"NAME":       req.Name,
		"PARENTNAME": req.ParentName,
		"TYPE":       dtreeParentType,
		"PARENTTYPE": filesystemType,
	}
	if req.UnixPermissions != "" {
		reqMap["unixPermissions"] = req.UnixPermissions
	}
	if req.VStoreID != "" {
		reqMap["vstoreId"] = req.VStoreID
	}

	resp, err := cli.Post(ctx, api.ManageDTreePath, reqMap)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != storage.SuccessCode {
		return nil, fmt.Errorf("create DTree failed, error code: %d, error msg: %s", code, msg)
	}

	if resp.Data == nil {
		return nil, fmt.Errorf("create DTree response data is nil")
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert DTree response to map failed, data: %v", resp.Data)
	}

	return respData, nil
}

// DeleteDTreeByID deletes a DTree by ID
func (cli *OceanASeriesClient) DeleteDTreeByID(ctx context.Context, vStoreID, dtreeID string) error {
	url := fmt.Sprintf(api.GetDTreeByIDPath, dtreeID)
	data := map[string]interface{}{}
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Delete(ctx, url, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != storage.SuccessCode {
		return fmt.Errorf("delete DTree by ID %s failed, error code: %d, error msg: %s", dtreeID, code, msg)
	}

	return nil
}

// DeleteDTreeByName deletes a DTree by name parameters
func (cli *OceanASeriesClient) DeleteDTreeByName(ctx context.Context, vStoreID, parentName, dtreeName string) error {
	data := map[string]interface{}{
		"PARENTNAME": parentName,
		"NAME":       dtreeName,
	}
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Delete(ctx, api.ManageDTreePath, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != storage.SuccessCode {
		return fmt.Errorf("delete DTree by name %s failed, error code: %d, error msg: %s", dtreeName, code, msg)
	}

	return nil
}

// GetDTreeByID gets DTree information by ID
func (cli *OceanASeriesClient) GetDTreeByID(ctx context.Context, dtreeID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s?ID=%s", api.ManageDTreePath, dtreeID)
	encodePath, err := rest.NewRequestPath(url).Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode path: %w", err)
	}

	resp, err := cli.Get(ctx, encodePath, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != storage.SuccessCode {
		return nil, fmt.Errorf("get DTree by ID %s failed, error code: %d, error msg: %s", dtreeID, code, msg)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok || len(respData) == 0 {
		return nil, nil
	}

	return respData, nil
}

// DTreeUpdateRequest defines the request structure for updating a DTree
type DTreeUpdateRequest struct {
	VStoreID string `json:"vstoreId,omitempty"`
}

// UpdateDTree modifies a DTree by ID
func (cli *OceanASeriesClient) UpdateDTree(ctx context.Context, dtreeID string, req *DTreeUpdateRequest) error {
	url := fmt.Sprintf(api.GetDTreeByIDPath, dtreeID)

	// Convert struct to map for JSON marshaling
	data := map[string]interface{}{}
	if req.VStoreID != "" {
		data["vstoreId"] = req.VStoreID
	}

	resp, err := cli.Put(ctx, url, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != storage.SuccessCode {
		return fmt.Errorf("update DTree %s failed, error code: %d, error msg: %s", dtreeID, code, msg)
	}

	return nil
}

// DTreeQuotaRequest defines the request structure for DTree quota operations
type DTreeQuotaRequest struct {
	PARENTTYPE     string `json:"PARENTTYPE"`
	PARENTID       string `json:"PARENTID"`
	QUOTATYPE      string `json:"QUOTATYPE"`
	SPACEHARDQUOTA string `json:"SPACEHARDQUOTA"`
	SPACEUNITTYPE  string `json:"SPACEUNITTYPE"`
	VStoreID       string `json:"vstoreId,omitempty"`
}

// CreateDTreeQuota creates a quota for DTree
func (cli *OceanASeriesClient) CreateDTreeQuota(ctx context.Context,
	req *DTreeQuotaRequest) (map[string]interface{}, error) {
	reqMap := map[string]interface{}{
		"PARENTTYPE":     req.PARENTTYPE,
		"PARENTID":       req.PARENTID,
		"QUOTATYPE":      req.QUOTATYPE,
		"SPACEHARDQUOTA": req.SPACEHARDQUOTA,
		"SPACEUNITTYPE":  req.SPACEUNITTYPE,
	}
	if req.VStoreID != "" {
		reqMap["vstoreId"] = req.VStoreID
	}

	resp, err := cli.Post(ctx, api.ManageFileSystemQuotaPath, reqMap)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != storage.SuccessCode {
		return nil, fmt.Errorf("create DTree quota failed, error code: %d, error msg: %s", code, msg)
	}

	if resp.Data == nil {
		return nil, fmt.Errorf("create DTree quota response data is nil")
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert DTree quota response to map failed, data: %v", resp.Data)
	}

	return respData, nil
}

// GetDTreeQuota gets quota for DTree by parent ID
func (cli *OceanASeriesClient) GetDTreeQuota(ctx context.Context,
	parentID, vStoreID string) (map[string]interface{}, error) {
	path := rest.NewRequestPath(api.ManageFileSystemQuotaPath)
	path.SetQuery("vstoreId", vStoreID)
	path.SetQuery("PARENTID", parentID)
	path.SetQuery("PARENTTYPE", dtreeParentType)
	path.SetDefaultListRange()
	encodePath, err := path.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode path and queries: %w", err)
	}

	resp, err := cli.Get(ctx, encodePath, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != storage.SuccessCode {
		return nil, fmt.Errorf("get DTree quota by parent ID %s failed, error code: %d, error msg: %s",
			parentID, code, msg)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("convert respData to array failed, data: %v", resp.Data)
	}

	if len(respData) == 0 {
		return nil, nil
	}

	// Handle nested data structure: respData[0] -> map["data"] -> data map
	quotaData, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert quota to map failed, data: %v", respData[0])
	}

	return quotaData, nil
}

// DeleteDTreeQuotaByParentID deletes a DTree quota by parent ID
func (cli *OceanASeriesClient) DeleteDTreeQuotaByParentID(ctx context.Context, parentID, vStoreID string) error {
	data := map[string]interface{}{
		"PARENTID":   parentID,
		"PARENTTYPE": dtreeParentType,
	}
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Delete(ctx, api.ManageFileSystemQuotaPath, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != storage.SuccessCode {
		return fmt.Errorf("delete DTree quota by parent ID %s failed, error code: %d, error msg: %s",
			parentID, code, msg)
	}

	return nil
}

// DTreeQuotaUpdateRequest defines the request structure for updating DTree quota
type DTreeQuotaUpdateRequest struct {
	VStoreID       string `json:"vstoreId,omitempty"`
	SPACEHARDQUOTA string `json:"SPACEHARDQUOTA"`
	SPACEUNITTYPE  string `json:"SPACEUNITTYPE"`
}

// UpdateDTreeQuota updates quota for DTree
func (cli *OceanASeriesClient) UpdateDTreeQuota(ctx context.Context,
	quotaID string, req *DTreeQuotaUpdateRequest) error {
	url := fmt.Sprintf(api.UpdateOrDeleteFileSystemQuotaPath, quotaID)

	data := map[string]interface{}{
		"SPACEHARDQUOTA": req.SPACEHARDQUOTA,
		"SPACEUNITTYPE":  req.SPACEUNITTYPE,
	}
	if req.VStoreID != "" {
		data["vstoreId"] = req.VStoreID
	}

	resp, err := cli.Put(ctx, url, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != storage.SuccessCode {
		return fmt.Errorf("update DTree quota %s failed, error code: %d, error msg: %s", quotaID, code, msg)
	}

	return nil
}

// DeleteDTreeQuota deletes a DTree quota
func (cli *OceanASeriesClient) DeleteDTreeQuota(ctx context.Context, quotaID, vStoreID string) error {
	url := fmt.Sprintf(api.UpdateOrDeleteFileSystemQuotaPath, quotaID)

	resp, err := cli.Delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != storage.SuccessCode {
		return fmt.Errorf("delete DTree quota %s failed, error code: %d, error msg: %s", quotaID, code, msg)
	}

	return nil
}
