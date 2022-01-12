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
	"errors"
	"fmt"
	"strconv"
	"time"

	"utils/log"
)

// CreateFileSystem to create file system on enterprise storage
func (cli *Client) CreateFileSystem(params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":          params["name"].(string),
		"PARENTID":      params["parentid"].(string),
		"CAPACITY":      params["capacity"].(int64),
		"DESCRIPTION":   params["description"].(string),
		"ALLOCTYPE":     params["alloctype"].(int),
		"ISSHOWSNAPDIR": false,
	}

	if val, ok := params["workloadTypeID"].(string); ok {
		res, err := strconv.ParseUint(val, 0, 32)
		if err != nil {
			return nil, fmt.Errorf("cannot convert workloadtype value %s to uint32: %v", val, err)
		}

		data["workloadTypeId"] = uint32(res)
	}
	resp, err := cli.post("/filesystem", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SYSTEM_BUSY || code == MSG_TIME_OUT {
		for i := 0; i < 10; i++ {
			time.Sleep(time.Second * GET_INFO_WAIT_INTERNAL)
			log.Infof("Create filesystem timeout, try to get info. The %d time", i+1)
			fsInfo, err := cli.GetFileSystemByName(params["name"].(string))
			if err != nil || fsInfo == nil {
				log.Warningf("get filesystem error, fs: %v, error: %v", fsInfo, err)
				continue
			}
			return fsInfo, nil
		}
	}

	err = dealCreateFSError(code)
	if err != nil {
		return nil, err
	}

	return cli.getResponseDataMap(resp.Data)
}

func dealCreateFSError(code int64) error {
	suggestMsg := "Suggestion: delete current PVC and specify the proper capacity of the file system and try again."
	if code == exceedFSCapacityUpper {
		return fmt.Errorf("create filesystem error. ErrorCode: %d. Reason: the entered capacity is greater "+
			"than the maximum capacity of the file system. %s", code, suggestMsg)
	}

	if code == lessFSCapacityLower {
		return fmt.Errorf("create filesystem error. ErrorCode: %d. Reason: the entered capacity is less "+
			"than the minimum capacity of the file system. %s", code, suggestMsg)
	}

	if code != 0 {
		msg := fmt.Sprintf("Create filesystem error. ErrorCode: %d. Please contact technical support.", code)
		return errors.New(msg)
	}
	return nil
}
