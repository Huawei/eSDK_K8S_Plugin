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

// Package dtree defines operations of A-series dtree
package dtree

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

// Querier is used to query an A-series DTree volume
type Querier struct {
	ctx        context.Context
	cli        client.OceanASeriesClientInterface
	dtreeName  string
	parentName string
	vstoreId   string
}

// NewQuerier inits a new DTree querier
func NewQuerier(ctx context.Context, cli client.OceanASeriesClientInterface, name, parentName string) *Querier {
	return &Querier{
		ctx:        ctx,
		cli:        cli,
		dtreeName:  name,
		parentName: parentName,
	}
}

// Query queries a DTree and returns a volume object
func (q *Querier) Query() (utils.Volume, error) {
	q.vstoreId = q.cli.GetvStoreID()

	dtree, err := q.cli.GetDTreeByName(q.ctx, q.parentName, q.dtreeName, q.vstoreId)
	if err != nil {
		return nil, err
	}
	if dtree == nil {
		return nil, fmt.Errorf("the dtree %s of parent %s is not exist", q.dtreeName, q.parentName)
	}

	dtreeID, ok := utils.GetValue[string](dtree, "ID")
	if !ok || dtreeID == "" {
		return nil, fmt.Errorf("get DTree ID failed, dtreeInfo: %v", dtree)
	}

	quota, err := q.cli.GetDTreeQuota(q.ctx, dtreeID, q.vstoreId)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return nil, fmt.Errorf("the quota of dtree %s of parent %s is not exist", q.dtreeName, q.parentName)
	}

	spaceHardQuotaStr, ok := utils.GetValue[string](quota, "SPACEHARDQUOTA")
	if !ok || spaceHardQuotaStr == "" {
		return nil, fmt.Errorf("SPACEHARDQUOTA is not exist or empty for dtree %s of parent %s",
			q.dtreeName, q.parentName)
	}

	spaceHardQuota, parseErr := parseQuotaValue(spaceHardQuotaStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parse SpaceHardQuota of dtree %s failed: %w", q.dtreeName, parseErr)
	}
	if spaceHardQuota == 0 {
		return nil, fmt.Errorf("the SpaceHardQuota of dtree %s of parent %s is 0",
			q.dtreeName, q.parentName)
	}

	vol := utils.NewVolume(q.dtreeName)
	vol.SetSize(spaceHardQuota)
	vol.SetID(dtreeID)
	vol.SetDTreeParentName(q.parentName)

	return vol, nil
}
