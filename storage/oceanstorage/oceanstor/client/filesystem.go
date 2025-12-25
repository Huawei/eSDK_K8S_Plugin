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
	"errors"
	"fmt"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	filesystemNotExist    int64 = 1073752065
	shareNotExist         int64 = 1077939717
	systemBusy            int64 = 1077949006
	msgTimeOut            int64 = 1077949001
	exceedFSCapacityUpper int64 = 1073844377
	lessFSCapacityLower   int64 = 1073844376
)

// OceanstorFilesystem defines interfaces for file system operations
type OceanstorFilesystem interface {
	base.Filesystem
	// SafeDeleteFileSystem used for delete file system
	SafeDeleteFileSystem(ctx context.Context, params map[string]interface{}) error
	// SafeDeleteNfsShare used for delete nfs share by id
	SafeDeleteNfsShare(ctx context.Context, id, vStoreID string) error
	// GetFileSystemByName used for get file system by name
	GetFileSystemByName(ctx context.Context, name string) (map[string]interface{}, error)
	// CreateFileSystem used for create file system
	CreateFileSystem(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
	// ModifyNfsShareAccess modifies nfs share auth client access value
	ModifyNfsShareAccess(ctx context.Context, accessID, vStoreID string, accessVal constants.AuthClientAccessVal) error
	// CheckNfsShareAccessStatus checks access status of nfs share
	CheckNfsShareAccessStatus(ctx context.Context, sharePath, client, vStoreID string,
		accessVal constants.AuthClientAccessVal) (bool, error)
}

// SafeDeleteFileSystem used for delete file system
func (cli *OceanstorClient) SafeDeleteFileSystem(ctx context.Context, params map[string]interface{}) error {
	resp, err := cli.SafeDelete(ctx, "/filesystem", params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == filesystemNotExist {
		log.AddContext(ctx).Infof("Filesystem %s does not exist while deleting", params)
		return nil
	}

	if code != 0 {
		return utils.Errorf(ctx, "Delete filesystem %s error: %d", params, code)
	}

	return nil
}

// SafeDeleteNfsShare used for delete nfs share by id
func (cli *OceanstorClient) SafeDeleteNfsShare(ctx context.Context, id, vStoreID string) error {
	url := fmt.Sprintf("/NFSHARE/%s", id)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.SafeDelete(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == shareNotExist {
		log.AddContext(ctx).Infof("Nfs share %s does not exist while deleting", id)
		return nil
	}

	if code != 0 {
		return fmt.Errorf("delete nfs share %s error: %d", id, code)
	}

	return nil
}

// GetFileSystemByName used for get file system by name
func (cli *OceanstorClient) GetFileSystemByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/filesystem?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get filesystem %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Filesystem %s does not exist", name)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	return cli.getObjByvStoreName(respData), nil
}

// CreateFileSystem used for create file system
func (cli *OceanstorClient) CreateFileSystem(ctx context.Context, params map[string]interface{}) (
	map[string]interface{}, error) {
	resp, err := cli.Post(ctx, "/filesystem", params)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == systemBusy || code == msgTimeOut {
		for i := 0; i < 10; i++ {
			time.Sleep(storage.GetInfoWaitInternal)
			log.AddContext(ctx).Infof("Create filesystem timeout, try to get info. The %d time", i+1)
			fsInfo, err := cli.GetFileSystemByName(ctx, params["name"].(string))
			if err != nil || fsInfo == nil {
				log.AddContext(ctx).Warningf("Get filesystem error, fs: %v, error: %v", fsInfo, err)
				continue
			}
			return fsInfo, nil
		}
	}

	err = dealCreateFSError(ctx, code)
	if err != nil {
		return nil, err
	}
	return cli.getResponseDataMap(ctx, resp.Data)
}

// ModifyNfsShareAccess modifies nfs share auth client access value
func (cli *OceanstorClient) ModifyNfsShareAccess(ctx context.Context, accessID, vStoreID string,
	accessVal constants.AuthClientAccessVal) error {
	req := map[string]any{
		"ID":        accessID,
		"vstoreId":  vStoreID,
		"ACCESSVAL": accessVal,
	}
	resp, err := cli.Put(ctx, "/NFS_SHARE_AUTH_CLIENT", req)
	if err != nil {
		return err
	}

	return resp.AssertErrorCode()
}

// CheckNfsShareAccessStatus checks access status of nfs share
func (cli *OceanstorClient) CheckNfsShareAccessStatus(ctx context.Context, sharePath, client, vStoreID string,
	accessVal constants.AuthClientAccessVal) (bool, error) {
	req := map[string]any{
		"vstoreId":  vStoreID,
		"NAME":      client,
		"SHAREPATH": sharePath,
	}
	if accessVal != constants.AuthClientNoAccess {
		req["ACCESSVAL"] = accessVal
	}

	resp, err := cli.Get(ctx, "/NFS_SHARE_AUTH_CLIENT/test_effective", req)
	if err != nil {
		return false, err
	}

	if err := resp.AssertErrorCode(); err != nil {
		return false, err
	}

	respData, ok := resp.Data.(map[string]any)
	if !ok {
		return false, fmt.Errorf("failed to convert response data: %v", resp)
	}
	status, ok := utils.GetValue[string](respData, "status")
	if !ok {
		return false, fmt.Errorf("failed to get status from response data, response: %v", respData)
	}

	return status == "0", nil
}

func dealCreateFSError(ctx context.Context, code int64) error {
	suggestMsg := "Suggestion: Delete current PVC and specify the proper capacity of the file system and try again."
	if code == exceedFSCapacityUpper {
		return utils.Errorf(ctx, "create filesystem error. ErrorCode: %d. Reason: the entered capacity is "+
			"greater than the maximum capacity of the file system. %s", code, suggestMsg)
	}

	if code == lessFSCapacityLower {
		return utils.Errorf(ctx, "create filesystem error. ErrorCode: %d. Reason: the entered capacity is "+
			"less than the minimum capacity of the file system. %s", code, suggestMsg)
	}

	if code != 0 {
		return utils.Errorf(ctx, "Create filesystem error. ErrorCode: %d.", code)
	}

	return nil
}
