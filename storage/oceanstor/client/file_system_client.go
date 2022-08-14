/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
// Package client to restful client to access enterprise storage
package client

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

func (cli *Client) CreateFileSystem(ctx context.Context,
	params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":          params["name"].(string),
		"PARENTID":      params["parentid"].(string),
		"CAPACITY":      params["capacity"].(int64),
		"DESCRIPTION":   params["description"].(string),
		"ALLOCTYPE":     params["alloctype"].(int),
		"ISSHOWSNAPDIR": false,
	}

	if params["fspermission"] != nil && params["fspermission"] != "" {
		data["unixPermissions"] = params["fspermission"]
	}

	if hyperMetro, hyperMetroOK := params["hypermetro"].(bool); hyperMetroOK && hyperMetro {
		data["fileSystemMode"] = hyperMetroFilesystem
		if vstoreId, exist := params["vstoreId"].(string); exist && vstoreId != "" {
			data["vstoreId"] = vstoreId
		}
	} else {
		data["fileSystemMode"] = localFilesystem
	}

	if val, ok := params["workloadTypeID"].(string); ok {
		res, err := strconv.ParseUint(val, 0, 32)
		if err != nil {
			return nil, fmt.Errorf("cannot convert workloadtype to int32: %v", err)
		}

		data["workloadTypeId"] = uint32(res)
	}

	resp, err := cli.post(ctx, "/filesystem", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SYSTEM_BUSY || code == MSG_TIME_OUT {
		for i := 0; i < 10; i++ {
			time.Sleep(time.Second * GET_INFO_WAIT_INTERNAL)
			log.AddContext(ctx).Infof("Create filesystem timeout, try to get info. The %d time", i+1)
			fsInfo, err := cli.GetFileSystemByName(ctx, params["name"].(string))
			if err != nil || fsInfo == nil {
				log.AddContext(ctx).Warningf("get filesystem error, fs: %v, error: %v", fsInfo, err)
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

func dealCreateFSError(ctx context.Context, code int64) error {
	suggestMsg := "Suggestion: delete current PVC and specify the proper capacity of the file system and try again."
	if code == exceedFSCapacityUpper {
		return utils.Errorf(ctx, "create filesystem error. ErrorCode: %d. Reason: the entered capacity is "+
			"greater than the maximum capacity of the file system. %s", code, suggestMsg)
	}

	if code == lessFSCapacityLower {
		return utils.Errorf(ctx, "create filesystem error. ErrorCode: %d. Reason: the entered capacity is "+
			"less than the minimum capacity of the file system. %s", code, suggestMsg)
	}

	if code != 0 {
		return utils.Errorf(ctx, "Create filesystem error. ErrorCode: %d. Please contact technical "+
			"support.", code)
	}

	return nil
}

func (cli *Client) DeleteFileSystem(ctx context.Context, params map[string]interface{}) error {
	resp, err := cli.delete(ctx, "/filesystem", params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == FILESYSTEM_NOT_EXIST {
		log.AddContext(ctx).Infof("Filesystem %s does not exist while deleting", params)
		return nil
	}

	if code != 0 {
		return utils.Errorf(ctx, "Delete filesystem %s error: %d", params, code)
	}

	return nil
}
