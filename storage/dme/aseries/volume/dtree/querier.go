/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

// Package dtree defines operations of DME DTree volumes
package dtree

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

// Querier is used to query a DME DTree volume
type Querier struct {
	ctx        context.Context
	cli        client.DMEASeriesClientInterface
	dtreeName  string
	parentName string
}

// NewQuerier inits a new DME DTree querier
func NewQuerier(ctx context.Context, cli client.DMEASeriesClientInterface, dtreeName, parentName string) *Querier {
	return &Querier{
		ctx:        ctx,
		cli:        cli,
		dtreeName:  dtreeName,
		parentName: parentName,
	}
}

// Query queries a DME DTree volume and returns a volume object
func (q *Querier) Query() (utils.Volume, error) {
	// Get parent filesystem
	fsInfo, err := q.cli.GetFileSystemByName(q.ctx, q.parentName)
	if err != nil {
		return nil, fmt.Errorf("query parent filesystem %s failed: %w", q.parentName, err)
	}
	if fsInfo == nil {
		return nil, fmt.Errorf("parent filesystem %s does not exist", q.parentName)
	}

	// Get DTree info
	dtreeInfo, err := q.cli.GetDTreeByName(q.ctx, fsInfo.ID, q.dtreeName)
	if err != nil {
		return nil, fmt.Errorf("query DTree %s failed: %w", q.dtreeName, err)
	}
	if dtreeInfo == nil {
		return nil, fmt.Errorf("dtree %s of parent %s does not exist", q.dtreeName, q.parentName)
	}

	// Get quota info for capacity
	quota, err := q.cli.GetDTreeQuotaByRawID(q.ctx, dtreeInfo.RawID)
	if err != nil {
		return nil, fmt.Errorf("query quota for DTree %s failed: %w", q.dtreeName, err)
	}
	if quota == nil {
		return nil, fmt.Errorf("the quota of dtree %s of parent %s does not exist", q.dtreeName, q.parentName)
	}

	capacity := quota.SpaceHardQuota
	vol := utils.NewVolume(q.dtreeName)
	vol.SetSize(capacity)
	vol.SetID(dtreeInfo.ID)
	vol.SetDTreeParentName(q.parentName)
	return vol, nil
}
