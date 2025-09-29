/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.
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

// Package base provide base operations for oceanstor base storage
package base

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	queryNfsSharePerPage int64 = 100
)

// Filesystem defines interfaces for file system operations
type Filesystem interface {
	// GetFileSystemByID used for get file system by id
	GetFileSystemByID(ctx context.Context, id string) (map[string]interface{}, error)
	// GetNfsShareByPath used for get nfs share by path
	GetNfsShareByPath(ctx context.Context, path, vStoreID string) (map[string]interface{}, error)
	// GetNfsShareAccess used for get nfs share access
	GetNfsShareAccess(ctx context.Context, parentID, name, vStoreID string) (map[string]interface{}, error)
	// GetNfsShareAccessCount used for get nfs share access count by id
	GetNfsShareAccessCount(ctx context.Context, parentID, vStoreID string) (int64, error)
	// GetNfsShareAccessRange used for get nfs share access
	GetNfsShareAccessRange(ctx context.Context, parentID, vStoreID string, startRange, endRange int64) ([]any, error)
	// UpdateFileSystem used for update file system
	UpdateFileSystem(ctx context.Context, fsID string, params map[string]interface{}) error
	// ExtendFileSystem used for extend file system by new capacity
	ExtendFileSystem(ctx context.Context, fsID string, newCapacity int64) error
	// AllowNfsShareAccess used for allow nfs share access
	AllowNfsShareAccess(ctx context.Context, req *AllowNfsShareAccessRequest) error
	// CreateNfsShare used for create nfs share
	CreateNfsShare(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error)
	// DeleteFileSystem used for delete file system
	DeleteFileSystem(ctx context.Context, params map[string]interface{}) error
	// DeleteNfsShareAccess used for delete nfs share access
	DeleteNfsShareAccess(ctx context.Context, accessID, vStoreID string) error
	// DeleteNfsShare used for delete nfs share by id
	DeleteNfsShare(ctx context.Context, id, vStoreID string) error
	// GetNFSServiceSetting used for get nfs service setting
	GetNFSServiceSetting(ctx context.Context) (map[string]bool, error)
}

// FilesystemClient defines client implements the Filesystem interface
type FilesystemClient struct {
	RestClientInterface
}

// DeleteFileSystem used for delete file system
func (cli *FilesystemClient) DeleteFileSystem(ctx context.Context, params map[string]interface{}) error {
	resp, err := cli.Delete(ctx, "/filesystem", params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == FilesystemNotExist {
		log.AddContext(ctx).Infof("Filesystem %s does not exist while deleting", params)
		return nil
	}

	if code != 0 {
		return utils.Errorf(ctx, "Delete filesystem %s error: %d", params, code)
	}

	return nil
}

// GetFileSystemByID used for get file system by id
func (cli *FilesystemClient) GetFileSystemByID(ctx context.Context, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/filesystem/%s", id)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get filesystem of ID %s error: %d", id, code)
		return nil, errors.New(msg)
	}

	fs, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to map[string]interface{} failed")
	}
	return fs, nil
}

// GetNfsShareByPath used for get nfs share by path
func (cli *FilesystemClient) GetNfsShareByPath(ctx context.Context,
	path, vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/NFSHARE?filter=SHAREPATH::%s&range=[0-100]", path)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Get(ctx, url, data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SharePathInvalid {
		log.AddContext(ctx).Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}
	if code != 0 {
		return nil, fmt.Errorf("Get nfs share of path %s error: %d", path, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}

	share, ok := respData[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("convert respData[0] to map[string]interface{} failed")
	}
	return share, nil
}

// GetNfsShareAccess used for get nfs share access
func (cli *FilesystemClient) GetNfsShareAccess(ctx context.Context,
	parentID, name, vStoreID string) (map[string]interface{}, error) {
	count, err := cli.GetNfsShareAccessCount(ctx, parentID, vStoreID)
	if err != nil {
		return nil, err
	}

	var i int64
	for i = 0; i < count; i += queryNfsSharePerPage { // Query per page 100
		clients, err := cli.GetNfsShareAccessRange(ctx, parentID, vStoreID, i, i+queryNfsSharePerPage)
		if err != nil {
			return nil, err
		}

		if clients == nil {
			return nil, nil
		}

		for _, ac := range clients {
			access, ok := ac.(map[string]interface{})
			if !ok {
				log.AddContext(ctx).Warningf("convert ac: %v to map[string]interface{} failed.", ac)
				continue
			}
			if access["NAME"].(string) == name {
				return access, nil
			}
		}
	}

	return nil, nil
}

// GetNfsShareAccessCount used for get nfs share access count by id
func (cli *FilesystemClient) GetNfsShareAccessCount(ctx context.Context, parentID, vStoreID string) (int64, error) {
	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT/count?filter=PARENTID::%s", parentID)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}
	resp, err := cli.Get(ctx, url, data)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return 0, fmt.Errorf("Get nfs share access count of %s error: %d", parentID, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return 0, errors.New("convert resp.Data to map[string]interface{} failed")
	}
	countStr, ok := respData["COUNT"].(string)
	if !ok {
		return 0, errors.New("convert respData[\"COUNT\"] to string failed")
	}
	count := utils.ParseIntWithDefault(countStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)
	return count, nil
}

// GetNfsShareAccessRange used for get nfs share access
func (cli *FilesystemClient) GetNfsShareAccessRange(ctx context.Context, parentID, vStoreID string, startRange,
	endRange int64) ([]interface{}, error) {

	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT?filter=PARENTID::%s&range=[%d-%d]", parentID, startRange, endRange)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}
	resp, err := cli.Get(ctx, url, data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get nfs share access of %s error: %d", parentID, code)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	return respData, nil
}

// UpdateFileSystem used for update file system
func (cli *FilesystemClient) UpdateFileSystem(ctx context.Context, fsID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/filesystem/%s", fsID)
	resp, err := cli.Put(ctx, url, params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Update filesystem %s by params %v error: %d", fsID, params, code)
	}

	return nil
}

// ExtendFileSystem used for extend file system by new capacity
func (cli *FilesystemClient) ExtendFileSystem(ctx context.Context, fsID string, newCapacity int64) error {
	url := fmt.Sprintf("/filesystem/%s", fsID)
	data := map[string]interface{}{
		"CAPACITY": newCapacity,
	}

	resp, err := cli.Put(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Extend FS capacity to %d error: %d", newCapacity, code)
	}

	return nil
}

// AllowNfsShareAccessRequest used for AllowNfsShareAccess request
type AllowNfsShareAccessRequest struct {
	Name        string
	ParentID    string
	VStoreID    string
	AccessVal   int
	Sync        int
	AllSquash   int
	RootSquash  int
	AccessKrb5  int
	AccessKrb5i int
	AccessKrb5p int
}

// AllowNfsShareAccess used for allow nfs share access
func (cli *FilesystemClient) AllowNfsShareAccess(ctx context.Context, req *AllowNfsShareAccessRequest) error {
	data := map[string]interface{}{
		"NAME":       req.Name,
		"PARENTID":   req.ParentID,
		"ACCESSVAL":  req.AccessVal,
		"SYNC":       req.Sync,
		"ALLSQUASH":  req.AllSquash,
		"ROOTSQUASH": req.RootSquash,
	}
	if req.AccessKrb5 != -1 {
		data["ACCESSKRB5"] = req.AccessKrb5
	}
	if req.AccessKrb5i != -1 {
		data["ACCESSKRB5I"] = req.AccessKrb5i
	}
	if req.AccessKrb5p != -1 {
		data["ACCESSKRB5P"] = req.AccessKrb5p
	}
	if req.VStoreID != "" {
		data["vstoreId"] = req.VStoreID
	}

	resp, err := cli.Post(ctx, "/NFS_SHARE_AUTH_CLIENT", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("allow nfs share %v access error: %d", data, code)
	}

	return nil
}

// CreateNfsShare used for create nfs share
func (cli *FilesystemClient) CreateNfsShare(ctx context.Context,
	params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"SHAREPATH":   params["sharepath"].(string),
		"FSID":        params["fsid"].(string),
		"DESCRIPTION": params["description"].(string),
	}

	vStoreID, _ := params["vStoreID"].(string)
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Post(ctx, "/NFSHARE", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == ShareAlreadyExist || code == SharePathAlreadyExist {
		sharePath, ok := params["sharepath"].(string)
		if !ok {
			return nil, errors.New("convert sharepath to string failed")
		}
		log.AddContext(ctx).Infof("Nfs share %s already exists while creating", sharePath)

		share, err := cli.GetNfsShareByPath(ctx, sharePath, vStoreID)
		return share, err
	}

	if code == SystemBusy || code == MsgTimeOut {
		for i := 0; i < 10; i++ {
			time.Sleep(GetInfoWaitInternal)
			log.AddContext(ctx).Infof("Create nfs share timeout, try to Get info. The %d time", i+1)
			share, err := cli.GetNfsShareByPath(ctx, params["sharepath"].(string), vStoreID)
			if err != nil || share == nil {
				log.AddContext(ctx).Warningf("Get nfs share error, share: %v, error: %v", share, err)
				continue
			}
			return share, nil
		}
	}

	if code != 0 {
		return nil, fmt.Errorf("create nfs share %v error: %d", data, code)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to map[string]interface{} failed")
	}
	return respData, nil
}

// DeleteNfsShareAccess used for delete nfs share access
func (cli *FilesystemClient) DeleteNfsShareAccess(ctx context.Context, accessID, vStoreID string) error {
	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT/%s", accessID)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}
	resp, err := cli.Delete(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Delete nfs share %s access error: %d", accessID, code)
	}

	return nil
}

// DeleteNfsShare used for delete nfs share by id
func (cli *FilesystemClient) DeleteNfsShare(ctx context.Context, id, vStoreID string) error {
	url := fmt.Sprintf("/NFSHARE/%s", id)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.Delete(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == ShareNotExist || code == NFSShareNotExist {
		log.AddContext(ctx).Infof("Nfs share %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete nfs share %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

// GetNFSServiceSetting used for get nfs service setting
func (cli *FilesystemClient) GetNFSServiceSetting(ctx context.Context) (map[string]bool, error) {
	resp, err := cli.Get(ctx, "/nfsservice", nil)
	if err != nil {
		// All enterprise storage support this interface.
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get NFS service setting failed. errorCode: %d", code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infoln("NFS service setting is empty.")
		return nil, nil
	}

	setting := map[string]bool{
		// NFS3 is enabled by default.
		"SupportNFS3":  true,
		"SupportNFS4":  false,
		"SupportNFS41": false,
		"SupportNFS42": false,
	}
	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to map[string]interface{} failed")
	}
	for k, v := range respData {
		var err error
		if k == "SUPPORTV3" {
			setting["SupportNFS3"], err = strconv.ParseBool(v.(string))
		} else if k == "SUPPORTV4" {
			setting["SupportNFS4"], err = strconv.ParseBool(v.(string))
		} else if k == "SUPPORTV41" {
			setting["SupportNFS41"], err = strconv.ParseBool(v.(string))
		} else if k == "SUPPORTV42" {
			setting["SupportNFS42"], err = strconv.ParseBool(v.(string))
		}

		if err != nil {
			log.AddContext(ctx).Errorf("Convert [%v] to bool failed. error: %v", v, err)
			return nil, err
		}
	}

	return setting, nil
}
