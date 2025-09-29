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
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
)

// QueryFilesystemModel is used to query a volume
type QueryFilesystemModel struct {
	Name         string
	WorkloadType string
}

// Querier is used to query a filesystem volume
type Querier struct {
	vstoreId string
	capacity int64
	ctx      context.Context
	cli      client.OceanASeriesClientInterface
	params   *QueryFilesystemModel
}

// NewQuerier inits a new filesystem volume querier
func NewQuerier(ctx context.Context, cli client.OceanASeriesClientInterface, params *QueryFilesystemModel) *Querier {
	return &Querier{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Query get a filesystem resource from storage and returns a volume object
func (c *Querier) Query() (utils.Volume, error) {
	tr := flow.NewTransaction()
	tr.Then(c.prepareParams, nil)
	tr.Then(c.queryAndInjectFilesystemInfo, nil)

	err := tr.Commit()
	if err != nil {
		tr.Rollback()
		return nil, err
	}

	vol := utils.NewVolume(c.params.Name)
	vol.SetSize(utils.TransK8SCapacity(c.capacity, constants.AllocationUnitBytes))

	return vol, nil
}

func (c *Querier) prepareParams() error {
	c.vstoreId = c.cli.GetvStoreID()
	return nil
}

func (c *Querier) queryAndInjectFilesystemInfo() error {
	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.Name, c.vstoreId)
	if err != nil {
		return err
	}
	if len(fs) == 0 {
		return fmt.Errorf("filesystem %s does not exist", c.params.Name)
	}

	err = c.validateWorkloadType(fs)
	if err != nil {
		return fmt.Errorf("failed to validate queried filesystem %s, err: %w", c.params.Name, err)
	}

	capacityStr, ok := utils.GetValue[string](fs, "CAPACITY")
	if !ok {
		return fmt.Errorf("failed to query filesystem %s, get filesystem info with empty CAPACITY", c.params.Name)
	}

	capacity, err := strconv.ParseInt(capacityStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
	if err != nil {
		return fmt.Errorf("failed to convert filesystem %s capacity, err: %w", c.params.Name, err)
	}

	c.capacity = capacity
	return nil
}

func (c *Querier) validateWorkloadType(fs map[string]interface{}) error {
	if c.params.WorkloadType == "" {
		return nil
	}

	fsWorkloadTypeID, ok := utils.GetValue[string](fs, "workloadTypeId")
	if !ok {
		return nil
	}

	queriedWorkloadTypeID, err := c.cli.GetApplicationTypeByName(c.ctx, c.params.WorkloadType)
	if err != nil {
		return err
	}

	if queriedWorkloadTypeID == "" {
		return fmt.Errorf("workload type %s does not exist", c.params.WorkloadType)
	}

	if queriedWorkloadTypeID != fsWorkloadTypeID {
		return fmt.Errorf("queried workload type is %s, but actually got %s", queriedWorkloadTypeID, fsWorkloadTypeID)
	}

	return nil
}
