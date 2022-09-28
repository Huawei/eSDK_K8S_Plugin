/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.
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
	"encoding/json"
	"errors"
	"fmt"
	fusionURL "net/url"

	"huawei-csi-driver/utils/log"
)

const (
	clientAlreadyExist int64 = 1077939727
	fileSystemNotExist int64 = 33564678
	notForbidden       int   = 0
)

func (cli *Client) CreateFileSystem(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"name":            params["name"].(string),
		"storage_pool_id": params["poolId"].(int64),
		"account_id":      params["accountid"].(string),
	}

	if params["protocol"] == "dpc" {
		data["forbidden_dpc"] = notForbidden
	}

	if params["fspermission"] != nil && params["fspermission"] != "" {
		data["unix_permission"] = params["fspermission"]
	}

	if val, exist := params["isshowsnapdir"].(bool); exist {
		data["is_show_snap_dir"] = val
	}
	resp, err := cli.post(ctx, "/api/v2/converged_service/namespaces", data)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Create filesystem %v error: %d", data, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	respData, ok := resp["data"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The data of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}
	if respData != nil {
		return respData, nil
	}

	return nil, fmt.Errorf("failed to create filesystem %v", data)
}

func (cli *Client) DeleteFileSystem(ctx context.Context, id string) error {
	url := fmt.Sprintf("/api/v2/converged_service/namespaces/%s", id)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Delete filesystem %v error: %d", id, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) GetFileSystemByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/api/v2/converged_service/namespaces?name=%s", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode == fileSystemNotExist {
		return nil, nil
	}

	if errorCode != 0 {
		msg := fmt.Sprintf("Get filesystem %v error: %d", name, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	respData, ok := resp["data"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The data of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}
	if respData != nil {
		return respData, nil
	}
	return nil, nil
}

func (cli *Client) CreateNfsShare(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"share_path":     params["sharepath"].(string),
		"file_system_id": params["fsid"].(string),
		"description":    params["description"].(string),
		"account_id":     params["accountid"].(string),
	}

	resp, err := cli.post(ctx, "/api/v2/nas_protocol/nfs_share", data)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Create nfs share %v error: %d", data, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	respData, ok := resp["data"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The data of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if respData != nil {
		return respData, nil
	}
	return nil, fmt.Errorf("failed to create NFS share %v", data)
}

func (cli *Client) DeleteNfsShare(ctx context.Context, id, accountId string) error {
	url := fmt.Sprintf("/api/v2/nas_protocol/nfs_share?id=%s&account_id=%s", id, accountId)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Delete NFS share %v error: %d", id, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) GetNfsShareByPath(ctx context.Context, path, accountId string) (map[string]interface{}, error) {
	bytesPath, err := json.Marshal([]map[string]string{{"share_path": path}})
	if err != nil {
		return nil, err
	}

	sharePath := fusionURL.QueryEscape(fmt.Sprintf("%s", bytesPath))
	url := fmt.Sprintf("/api/v2/nas_protocol/nfs_share_list?account_id=%s&filter=%s", accountId, sharePath)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Get NFS share path %s error: %d", path, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	respData, ok := resp["data"].([]interface{})
	if !ok {
		msg := fmt.Sprintf("There is no data info in response %v.", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	for _, s := range respData {
		share, ok := s.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}

		if share["share_path"].(string) == path {
			return share, nil
		}
	}
	return nil, nil
}

// AllowNfsShareAccessRequest used for AllowNfsShareAccess request
type AllowNfsShareAccessRequest struct {
	AccessName  string
	ShareId     string
	AccessValue int
	AllSquash   int
	RootSquash  int
	AccountId   string
}

// AllowNfsShareAccess used for create nfs share client
func (cli *Client) AllowNfsShareAccess(ctx context.Context, req *AllowNfsShareAccessRequest) error {
	data := map[string]interface{}{
		"access_name":  req.AccessName,
		"share_id":     req.ShareId,
		"access_value": req.AccessValue,
		"sync":         0,
		"all_squash":   req.AllSquash,
		"root_squash":  req.RootSquash,
		"type":         0,
		"account_id":   req.AccountId,
	}

	resp, err := cli.Post(ctx, "/api/v2/nas_protocol/nfs_share_auth_client", data)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode == clientAlreadyExist {
		log.AddContext(ctx).Warningf("The nfs share auth client %s is already exist.", req.AccessName)
		return nil
	} else if errorCode != 0 {
		msg := fmt.Sprintf("Allow nfs share %v access error: %d", data, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) DeleteNfsShareAccess(ctx context.Context, accessID string) error {
	url := fmt.Sprintf("/api/v2/nas_protocol/nfs_share_auth_client?id=%s", accessID)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Delete nfs share %v access error: %d", accessID, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) GetNfsShareAccess(ctx context.Context, shareID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/api/v2/nas_protocol/nfs_share_auth_client_list?filter=share_id::%s", shareID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Get nfs share %v access error: %d", shareID, errorCode)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}
	respData, ok := resp["data"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The data of response %v's format is not map[string]interface{}", resp)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}
	if respData != nil {
		return respData, nil
	}
	return nil, err
}
