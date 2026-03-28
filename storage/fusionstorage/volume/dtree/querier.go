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

// Package dtree defines operations of fusion storage dtree
package dtree

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

type querierModel struct {
	DTreeName  string
	ParentName string
}

// Querier is used to query dtree volume
type Querier struct {
	ctx    context.Context
	cli    client.IRestClient
	params *querierModel
}

// NewQuerier inits a new dtree querier
func NewQuerier(ctx context.Context, cli client.IRestClient, name, parentName string) *Querier {
	return &Querier{
		ctx: ctx,
		cli: cli,
		params: &querierModel{
			DTreeName:  name,
			ParentName: parentName,
		},
	}
}

// Query query a dtree and return a volume object
func (q *Querier) Query() (utils.Volume, error) {
	dtree, err := q.cli.GetDTreeByName(q.ctx, q.params.ParentName, q.params.DTreeName)
	if err != nil {
		return nil, err
	}
	if dtree == nil || dtree.Id == "" {
		return nil, fmt.Errorf("the dtree %q of parent %q is not exist", q.params.DTreeName, q.params.ParentName)
	}

	quota, err := q.cli.GetQuotaByDTreeId(q.ctx, dtree.Id)
	if err != nil {
		return nil, err
	}
	if quota == nil || quota.Id == "" {
		return nil, fmt.Errorf("the quota of dtree %q of parent %q is not exist", q.params.DTreeName, q.params.ParentName)
	}
	if quota.SpaceHardQuota == 0 {
		return nil, fmt.Errorf("the SpaceHardQuota of dtree %q of parent %q is 0",
			q.params.DTreeName, q.params.ParentName)
	}

	vol := utils.NewVolume(q.params.DTreeName)
	vol.SetSize(quota.SpaceHardQuota)
	vol.SetID(dtree.Id)
	vol.SetDTreeParentName(q.params.ParentName)

	return vol, nil
}
