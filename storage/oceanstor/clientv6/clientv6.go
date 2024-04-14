/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package clientv6

import (
	"context"
	"fmt"
	"strconv"

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// ClientV6 provides base client of clientv6
type ClientV6 struct {
	client.BaseClient
}

// NewClientV6 inits a new client of clientv6
func NewClientV6(ctx context.Context, param *client.NewClientConfig) (*ClientV6, error) {
	var err error
	var parallelCount int

	if len(param.ParallelNum) > 0 {
		parallelCount, err = strconv.Atoi(param.ParallelNum)
		if err != nil || parallelCount > client.MaxParallelCount || parallelCount < client.MinParallelCount {
			log.Warningf("The config parallelNum %d is invalid, set it to the default value %d",
				parallelCount, client.DefaultParallelCount)
			parallelCount = client.DefaultParallelCount
		}
	} else {
		parallelCount = client.DefaultParallelCount
	}

	log.AddContext(ctx).Infof("Init parallel count is %d", parallelCount)
	client.ClientSemaphore = utils.NewSemaphore(parallelCount)

	cli, err := client.NewClient(ctx, param)
	if err != nil {
		return nil, err
	}

	return &ClientV6{
		*cli,
	}, nil
}

// SplitCloneFS used to split clone for dorado or oceantor v6
func (cli *ClientV6) SplitCloneFS(ctx context.Context, fsID, vStoreId string, splitSpeed int, deleteParentSnapshot bool) error {
	data := map[string]interface{}{
		"ID":                   fsID,
		"SPLITSPEED":           splitSpeed,
		"deleteParentSnapshot": deleteParentSnapshot,
		"action":               1,
		"vstoreId":             vStoreId,
	}

	resp, err := cli.Put(ctx, SplitCloneFileSystem, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("split clone fs failed. fsId: %s, error code: %d", fsID, code)
	}

	return nil
}

// MakeLunName  v6 storage lun name support 1 to 255 characters
func (cli *ClientV6) MakeLunName(name string) string {
	return name
}
