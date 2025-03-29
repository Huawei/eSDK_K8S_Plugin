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

package client

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/utils"
)

const (
	manageDtreePath    = "/api/v2/file_service/dtrees"
	nfsShareListPath   = "/api/v2/nas_protocol/nfs_share_list"
	manageNfsSharePath = "/api/v2/nas_protocol/nfs_share"
	addAuthClientPath  = "/api/v2/nas_protocol/nfs_share_auth_client"
)

// DTree is the interface for Pacific DTree
type DTree interface {
	GetDTreeByName(ctx context.Context, parentName, name string) (*DTreeResponse, error)
	CreateDTree(ctx context.Context, parentName, name, unixPermission string) (*DTreeResponse, error)
	DeleteDTree(ctx context.Context, dtreeId string) error
	GetDTreeNfsShareByPath(ctx context.Context, sharePath string) (*GetDTreeNfsShareResponse, error)
	CreateDTreeNfsShare(ctx context.Context, req *CreateDTreeNfsShareRequest) (*CreateDTreeNfsShareResponse, error)
	DeleteDTreeNfsShare(ctx context.Context, shareId string) error
	AddNfsShareAuthClient(ctx context.Context, req *AddNfsShareAuthClientRequest) error
}

// DTreeResponse defines the fields of dtree response
type DTreeResponse struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

// GetDTreeByName gets dtree by its name
func (cli *RestClient) GetDTreeByName(ctx context.Context, parentName, name string) (*DTreeResponse, error) {
	restPath := utils.NewFusionRestPath(manageDtreePath)
	restPath.SetQuery("file_system_name", parentName)
	restPath.AddFilter("name", name)
	encodedPath, err := restPath.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode path and queries: %w", err)
	}

	resp, err := gracefulGet[[]*DTreeResponse](ctx, cli, encodedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get dtree by name: %w", err)
	}
	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("error %+v from get dtree by name restful response", resp.Result)
	}

	index := slices.IndexFunc(resp.Data, func(data *DTreeResponse) bool {
		return data.Name == name
	})
	if index == -1 {
		return nil, nil
	}

	return resp.Data[index], nil
}

// CreateDTreeRequest defines the fields to create dtree resource
type CreateDTreeRequest struct {
	Name           string `json:"name"`
	FileSystemName string `json:"file_system_name"`
	UnixPermission string `json:"unix_permission,omitempty"`
	AccountId      int    `json:"account_id"`
}

// CreateDTree creates a dtree resource
func (cli *RestClient) CreateDTree(ctx context.Context, parentName, name, unixPermission string) (*DTreeResponse,
	error) {
	req := CreateDTreeRequest{
		Name:           name,
		FileSystemName: parentName,
		UnixPermission: unixPermission,
		AccountId:      cli.accountId,
	}

	resp, err := gracefulPost[*DTreeResponse](ctx, cli, manageDtreePath, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create dtree: %w", err)
	}

	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("error %+v from create dtree restful response", resp.Result)
	}

	return resp.Data, nil
}

// DeleteDTree deletes dtree resource by its id
func (cli *RestClient) DeleteDTree(ctx context.Context, dtreeId string) error {
	restPath := utils.NewFusionRestPath(manageDtreePath)
	restPath.SetQuery("id", dtreeId)
	restPath.SetQuery("account_id", strconv.Itoa(cli.accountId))
	encodedPath, err := restPath.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode path and queries: %w", err)
	}

	resp, err := gracefulDelete[any](ctx, cli, encodedPath, nil)
	if err != nil {
		return fmt.Errorf("failed to delete dtree: %w", err)
	}

	if resp.GetErrorCode() != 0 {
		return fmt.Errorf("error %+v from delete dtree restful response", resp.Result)
	}

	return nil
}

// GetDTreeNfsShareResponse defines the fields of dtree nfs share response
type GetDTreeNfsShareResponse struct {
	Id        string `json:"id"`
	SharePath string `json:"share_path"`
}

// GetDTreeNfsShareByPath gets the nfs share of dtree by share path
func (cli *RestClient) GetDTreeNfsShareByPath(ctx context.Context, sharePath string) (*GetDTreeNfsShareResponse,
	error) {
	restPath := utils.NewFusionRestPath(nfsShareListPath)
	restPath.SetQuery("account_id", strconv.Itoa(cli.accountId))
	restPath.AddFilter("share_path", sharePath)
	encodedPath, err := restPath.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode path and queries: %w", err)
	}

	resp, err := gracefulGet[[]*GetDTreeNfsShareResponse](ctx, cli, encodedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get dtree nfs share by path: %w", err)
	}

	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("error %+v from get dtree nfs share by path restful response", resp.Result)
	}

	index := slices.IndexFunc(resp.Data, func(data *GetDTreeNfsShareResponse) bool {
		return data.SharePath == sharePath
	})
	if index == -1 {
		return nil, nil
	}

	return resp.Data[index], nil
}

// CreateDTreeNfsShareRequest defines the fields of create dtree nfs share request
type CreateDTreeNfsShareRequest struct {
	DtreeId     string `json:"dtree_id"`
	Sharepath   string `json:"share_path"`
	Description string `json:"description"`
	AccountId   int    `json:"account_id"`
}

// CreateDTreeNfsShareResponse defines the fields of create dtree nfs share response
type CreateDTreeNfsShareResponse struct {
	Id string `json:"id"`
}

// CreateDTreeNfsShare creates a nfs share of dtree
func (cli *RestClient) CreateDTreeNfsShare(ctx context.Context,
	req *CreateDTreeNfsShareRequest) (*CreateDTreeNfsShareResponse, error) {
	req.AccountId = cli.accountId
	resp, err := gracefulPost[*CreateDTreeNfsShareResponse](ctx, cli, manageNfsSharePath, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create dtree nfs share: %w", err)
	}

	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("error %+v from create dtree nfs share restful response", resp.Result)
	}

	return resp.Data, nil
}

// DeleteDTreeNfsShare deletes dtree nfs share by its id
func (cli *RestClient) DeleteDTreeNfsShare(ctx context.Context, shareId string) error {
	req := map[string]string{"account_id": strconv.Itoa(cli.accountId), "id": shareId}
	resp, err := gracefulDelete[any](ctx, cli, manageNfsSharePath, req)
	if err != nil {
		return fmt.Errorf("failed to delete dtree nfs share: %w", err)
	}

	if resp.GetErrorCode() != 0 {
		return fmt.Errorf("error %+v from delete nfs share restful response", resp.Result)
	}

	return nil
}

// AddNfsShareAuthClientRequest defines the fields to add nfs share auth client request
type AddNfsShareAuthClientRequest struct {
	AccessName  string `json:"access_name"`
	ShareId     string `json:"share_id"`
	AccessValue int    `json:"access_value"`
	Sync        int    `json:"sync"`
	AllSquash   int    `json:"all_squash"`
	RootSquash  int    `json:"root_squash"`
	AccountId   int    `json:"account_id"`
}

// AddNfsShareAuthClient adds nfs share auth client
func (cli *RestClient) AddNfsShareAuthClient(ctx context.Context, req *AddNfsShareAuthClientRequest) error {
	req.AccountId = cli.accountId
	resp, err := gracefulPost[any](ctx, cli, addAuthClientPath, req)
	if err != nil {
		return fmt.Errorf("failed to add nfs share auth client: %w", err)
	}

	if resp.GetErrorCode() != 0 {
		return fmt.Errorf("error %+v from add nfs share auth client restful response", resp.Result)
	}

	return nil
}
