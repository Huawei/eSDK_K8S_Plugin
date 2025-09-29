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

// Package client provides oceanstor A-series storage client
package client

import (
	"context"
	"fmt"

	pkgutils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/api"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/api/rest"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// ASeriesFilesystem defines interfaces for file system operations
type ASeriesFilesystem interface {
	base.Filesystem
	// GetFileSystemByName used for get filesystem list by name
	GetFileSystemByName(ctx context.Context, name, vstoreId string) (map[string]interface{}, error)
	// CreateFileSystem used for create file system
	CreateFileSystem(ctx context.Context, params *CreateFilesystemParams,
		advancedOptions map[string]interface{}) (map[string]interface{}, error)
	// CreateDataTurboShare used for create DataTurbo share
	CreateDataTurboShare(ctx context.Context, params *CreateDataTurboShareParams) (map[string]interface{}, error)
	// GetDataTurboShareByPath used for get DataTurbo share by path
	GetDataTurboShareByPath(ctx context.Context, path, vstoreId string) (map[string]interface{}, error)
	// DeleteDataTurboShare used for delete DataTurbo share by id
	DeleteDataTurboShare(ctx context.Context, id, vstoreId string) error
	// AddDataTurboShareUser used for add DataTurbo share user
	AddDataTurboShareUser(ctx context.Context, params *AddDataTurboShareUserParams) error
	// RemoveDataTurboShareUser used for delete DataTurbo share user
	RemoveDataTurboShareUser(ctx context.Context, objID, vstoreId string) error
}

// GetFileSystemByName used for get filesystem list by name
func (cli *OceanASeriesClient) GetFileSystemByName(ctx context.Context,
	name, vstoreId string) (map[string]interface{}, error) {
	path := rest.NewRequestPath(api.ManageFileSystemPath)
	path.SetQuery("vstoreId", vstoreId)
	path.AddFilter("NAME", name)
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

	if code != base.SuccessCode {
		return nil, fmt.Errorf("get filesystem %s failed, error code: %d, error msg: %s", name, code, msg)
	}

	if resp.Data == nil {
		return map[string]interface{}{}, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("convert respData to array failed, data: %v", resp.Data)
	}
	if len(respData) == 0 {
		return map[string]interface{}{}, nil
	}

	filesystem, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert filesystem to map failed, data: %v", respData[0])
	}
	return filesystem, nil
}

// CreateFilesystemParams defines create filesystem params
type CreateFilesystemParams struct {
	Name            string
	ParentId        string
	Capacity        int64
	Description     string
	WorkLoadTypeId  string
	UnixPermissions string
	VstoreId        string
}

// CreateFileSystem used for create file system
func (cli *OceanASeriesClient) CreateFileSystem(ctx context.Context,
	params *CreateFilesystemParams, advancedOptions map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        params.Name,
		"PARENTID":    params.ParentId,
		"CAPACITY":    params.Capacity,
		"DESCRIPTION": params.Description,
	}

	if params.WorkLoadTypeId != "" {
		data["workloadTypeId"] = params.WorkLoadTypeId
	}

	if params.UnixPermissions != "" {
		data["unixPermissions"] = params.UnixPermissions
	}

	if params.VstoreId != "" {
		data["vstoreId"] = params.VstoreId
	}

	// Adapt A-series storage interface and set the snapshot directory to be invisible.
	data["ISSHOWSNAPDIR"] = false

	if len(advancedOptions) != 0 {
		data = pkgutils.CombineMap(advancedOptions, data)
	}

	resp, err := cli.Post(ctx, api.ManageFileSystemPath, data)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != base.SuccessCode {
		return nil, fmt.Errorf("create filesystem %v failed, error code: %d, error msg: %s", data, code, msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert filesystem to map failed, data: %v", resp.Data)
	}

	return respData, nil
}

// CreateDataTurboShareParams defines create DataTurbo share params
type CreateDataTurboShareParams struct {
	SharePath   string
	FsId        string
	Description string
	VstoreId    string
}

// CreateDataTurboShare used for create DataTurbo share
func (cli *OceanASeriesClient) CreateDataTurboShare(ctx context.Context,
	params *CreateDataTurboShareParams) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"sharePath":   params.SharePath,
		"fsId":        params.FsId,
		"description": params.Description,
	}

	if params.VstoreId != "" {
		data["vstoreId"] = params.VstoreId
	}

	resp, err := cli.Post(ctx, api.ManageDataTurboShare, data)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != base.SuccessCode {
		return nil, fmt.Errorf("create DataTurbo share %v failed, error code: %d, error msg: %s", data, code, msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert DataTurbo share to map failed, data: %v", resp.Data)
	}

	return respData, nil
}

// GetDataTurboShareByPath used for get DataTurbo share by path
func (cli *OceanASeriesClient) GetDataTurboShareByPath(ctx context.Context,
	path, vstoreId string) (map[string]interface{}, error) {
	rp := rest.NewRequestPath(api.ManageDataTurboShare)
	rp.SetQuery("vstoreId", vstoreId)
	rp.AddFilter("sharePath", path)
	rp.SetDefaultListRange()
	encodePath, err := rp.Encode()
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

	if code != base.SuccessCode {
		return nil, fmt.Errorf("get DataTurbo share %s failed, error code: %d, error msg: %s", path, code, msg)
	}

	if resp.Data == nil {
		return map[string]interface{}{}, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("convert respData to array failed, data: %v", resp.Data)
	}
	if len(respData) == 0 {
		return map[string]interface{}{}, nil
	}

	return matchBySharePath(ctx, respData, path), nil
}

func matchBySharePath(ctx context.Context, shareArr []interface{}, sharePath string) map[string]interface{} {
	for _, share := range shareArr {
		shareInfo, ok := share.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert share info to map failed, data: %v", share)
			continue
		}

		getPath, _ := utils.GetValue[string](shareInfo, "sharePath")
		if getPath == sharePath {
			return shareInfo
		}
	}

	return nil
}

// DeleteDataTurboShare used for delete DataTurbo share by id
func (cli *OceanASeriesClient) DeleteDataTurboShare(ctx context.Context, id, vstoreId string) error {
	url := fmt.Sprintf(api.DeleteDataTurboShare, id)
	var data = make(map[string]interface{})
	if vstoreId != "" {
		data["vstoreId"] = vstoreId
	}

	resp, err := cli.Delete(ctx, url, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code == base.ShareNotExist {
		log.AddContext(ctx).Infof("DataTurbo share %s does not exist while deleting", id)
		return nil
	}

	if code != base.SuccessCode {
		return fmt.Errorf("delete DataTurbo share %s failed, error code: %d, error msg: %s", id, code, msg)
	}

	return nil
}

// AddDataTurboShareUserParams defines add DataTurbo share user params
type AddDataTurboShareUserParams struct {
	UserName   string
	ShareId    string
	Permission uint32
	VstoreId   string
}

// AddDataTurboShareUser used for add DataTurbo share user
func (cli *OceanASeriesClient) AddDataTurboShareUser(ctx context.Context, params *AddDataTurboShareUserParams) error {
	data := map[string]interface{}{
		"userName":   params.UserName,
		"shareId":    params.ShareId,
		"permission": params.Permission,
	}

	if params.VstoreId != "" {
		data["vstoreId"] = params.VstoreId
	}

	resp, err := cli.Post(ctx, api.ManageDataTurboShareUser, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code != base.SuccessCode {
		return fmt.Errorf("create DataTurbo share %v failed, error code: %d, error msg: %s", data, code, msg)
	}

	return nil
}

// RemoveDataTurboShareUser used for remove DataTurbo share user
func (cli *OceanASeriesClient) RemoveDataTurboShareUser(ctx context.Context, objID, vstoreId string) error {
	url := fmt.Sprintf(api.RemoveDataTurboShareUser, objID)
	var data = make(map[string]interface{})
	if vstoreId != "" {
		data["vstoreId"] = vstoreId
	}

	resp, err := cli.Delete(ctx, url, data)
	if err != nil {
		return err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return err
	}

	if code == base.AuthUserNotExist {
		log.AddContext(ctx).Infof("DataTurbo share auth user %s does not exist while deleting", objID)
		return nil
	}

	if code != base.SuccessCode {
		return fmt.Errorf("remove DataTurbo share auth user %s failed, "+
			"error code: %d, error msg: %s", objID, code, msg)
	}

	return nil
}
