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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// ExpandFilesystemModel is used to expand a filesystem volume
type ExpandFilesystemModel struct {
	Name     string
	Capacity int64
}

// Expander is used to expand a filesystem volume
type Expander struct {
	vstoreId string
	ctx      context.Context
	cli      client.OceanASeriesClientInterface
	params   *ExpandFilesystemModel
}

// NewExpander inits a new filesystem volume expander
func NewExpander(ctx context.Context, cli client.OceanASeriesClientInterface, params *ExpandFilesystemModel) *Expander {
	return &Expander{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Expand expands a filesystem resource capacity on storage
func (c *Expander) Expand() error {
	tr := flow.NewTransaction()
	tr.Then(c.prepareParams, nil)

	tr.Then(c.expandFilesystem, nil)

	err := tr.Commit()
	if err != nil {
		tr.Rollback()
		return err
	}

	return nil
}

func (c *Expander) prepareParams() error {
	c.vstoreId = c.cli.GetvStoreID()
	return nil
}

func (c *Expander) expandFilesystem() error {
	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.Name, c.vstoreId)
	if err != nil {
		return err
	}
	if len(fs) == 0 {
		return fmt.Errorf("filesystem %s does not exist", c.params.Name)
	}

	curSizeStr, ok := utils.GetValue[string](fs, "CAPACITY")
	if !ok {
		return fmt.Errorf("failed to expand filesystem %s, get filesystem info with invalid CAPACITY", c.params.Name)
	}

	curSize := utils.ParseIntWithDefault(curSizeStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)
	if c.params.Capacity == curSize {
		log.AddContext(c.ctx).Infof("the size of filesystem %s has not changed and the current size is %d",
			c.params.Name, curSize)
		return nil
	} else if c.params.Capacity < curSize {
		return fmt.Errorf("failed to expand filesystem %s, new size %d is less than current size %d",
			c.params.Name, c.params.Capacity, curSize)
	}

	poolName, ok := utils.GetValue[string](fs, "PARENTNAME")
	if !ok {
		return fmt.Errorf("failed to expand filesystem %s, get filesystem info with invalid PARENTNAME", c.params.Name)
	}

	pool, err := c.cli.GetPoolByName(c.ctx, poolName)
	if err != nil {
		return err
	}

	if pool == nil {
		return fmt.Errorf("failed to expand filesystem %s, pool %s is not exist", c.params.Name, poolName)
	}

	fsId, ok := utils.GetValue[string](fs, "ID")
	if !ok {
		return fmt.Errorf("failed to expand filesystem %s, get filesystem info with invalid ID", c.params.Name)
	}

	params := map[string]interface{}{"CAPACITY": c.params.Capacity, "vstoreId": c.vstoreId}
	return c.cli.UpdateFileSystem(c.ctx, fsId, params)
}
