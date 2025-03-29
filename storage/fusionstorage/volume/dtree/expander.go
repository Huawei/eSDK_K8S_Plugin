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

// Package dtree defines operations of fusion storage dTree
package dtree

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
)

// ExpandDTreeModel is used to expand the capacity of pacific dTree volume.
type ExpandDTreeModel struct {
	ParentName string
	DTreeName  string
	Capacity   int64
}

// NewExpander inits a new dTree expand client
func NewExpander(ctx context.Context, cli client.IRestClient, params *ExpandDTreeModel) *Expander {
	return &Expander{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Expander is used to expand the capacity of pacific dTree volume.
type Expander struct {
	ctx    context.Context
	cli    client.IRestClient
	params *ExpandDTreeModel
}

// Expand used to expand pacific dTree
func (e *Expander) Expand() error {
	tr := flow.NewTransaction()
	tr.Then(e.ExpandDTree, nil)

	err := tr.Commit()
	if err != nil {
		tr.Rollback()
		return err
	}

	return nil
}

// ExpandDTree used to expand pacific dTree
func (e *Expander) ExpandDTree() error {
	dTree, err := e.cli.GetDTreeByName(e.ctx, e.params.ParentName, e.params.DTreeName)
	if err != nil {
		return err
	}

	if dTree == nil {
		return fmt.Errorf("the volume: %s to be expanded does not exist", e.params.DTreeName)
	}

	quota, err := e.cli.GetQuotaByDTreeId(e.ctx, dTree.Id)
	if err != nil {
		return err
	}

	// If the quota exists, update the capacity.
	if quota != nil {
		if e.params.Capacity < int64(quota.SpaceHardQuota) {
			return fmt.Errorf("target capacity: %d must be greater than the current capacity: %d",
				e.params.Capacity, int64(quota.SpaceHardQuota))
		}

		err = e.cli.UpdateDTreeQuota(e.ctx, quota.Id, e.params.Capacity)
		if err != nil {
			return err
		}

		return nil
	}

	// If the quota does not exist, create one.
	quota, err = e.cli.CreateDTreeQuota(e.ctx, dTree.Id, e.params.Capacity)
	if err != nil {
		return err
	}
	if quota == nil || quota.Id == "" {
		return fmt.Errorf("create quota for pacifc dTree: %s failed, quota id is empty or quota is nil",
			dTree.Id)
	}

	return nil
}
