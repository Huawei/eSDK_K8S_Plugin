/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

// V6Client provides base client of clientv6
type V6Client struct {
	client.OceanstorClient
}

// NewClientV6 inits a new client of clientv6
func NewClientV6(ctx context.Context, param *client.NewClientConfig) (*V6Client, error) {
	cli, err := client.NewClient(ctx, param)
	if err != nil {
		return nil, err
	}

	return &V6Client{OceanstorClient: *cli}, nil
}

// SplitCloneFS used to split clone for dorado or oceantor v6
func (cli *V6Client) SplitCloneFS(ctx context.Context,
	fsID, vStoreId string, splitSpeed int, deleteParentSnapshot bool) error {
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
func (cli *V6Client) MakeLunName(name string) string {
	return name
}
