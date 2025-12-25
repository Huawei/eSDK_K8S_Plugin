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

// Package volume defines operations of volumes
package volume

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

// QueryVolumeModel is used to query a volume
type QueryVolumeModel struct {
	Name string
}

// Querier is used to query a filesystem volume
type Querier struct {
	capacity int64
	ctx      context.Context
	cli      client.DMEASeriesClientInterface
	params   *QueryVolumeModel
}

// NewQuerier inits a new filesystem volume querier
func NewQuerier(ctx context.Context, cli client.DMEASeriesClientInterface, params *QueryVolumeModel) *Querier {
	return &Querier{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Query get a filesystem resource from storage and returns a volume object
func (c *Querier) Query() (utils.Volume, error) {
	if err := c.queryFilesystemInfo(); err != nil {
		return nil, err
	}

	vol := utils.NewVolume(c.params.Name)
	vol.SetSize(c.capacity)
	return vol, nil
}

func (c *Querier) queryFilesystemInfo() error {
	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.Name)
	if err != nil {
		return err
	}
	if fs == nil {
		return fmt.Errorf("filesystem %s does not exist", c.params.Name)
	}

	c.capacity = fs.TotalCapacityInByte
	return nil
}
