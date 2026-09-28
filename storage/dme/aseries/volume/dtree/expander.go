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

package dtree

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	directoryQuota = "directory_quota"
	parentType     = "dtree"
)

// ExpandDTreeModel is used to expand a DME DTree volume
type ExpandDTreeModel struct {
	ParentName string
	DTreeName  string
	Capacity   int64
}

// Expander is used to expand a DME DTree volume
type Expander struct {
	ctx    context.Context
	cli    client.DMEASeriesClientInterface
	params *ExpandDTreeModel
}

// NewExpander inits a new DME DTree expander
func NewExpander(ctx context.Context, cli client.DMEASeriesClientInterface, params *ExpandDTreeModel) *Expander {
	return &Expander{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Expand expands the DME DTree volume quota capacity
func (e *Expander) Expand() error {
	// Get parent filesystem to find fsId
	fsInfo, err := e.cli.GetFileSystemByName(e.ctx, e.params.ParentName)
	if err != nil {
		return fmt.Errorf("query parent filesystem %s failed: %w", e.params.ParentName, err)
	}
	if fsInfo == nil {
		return fmt.Errorf("parent filesystem %s does not exist", e.params.ParentName)
	}

	// Get DTree info
	dtreeInfo, err := e.cli.GetDTreeByName(e.ctx, fsInfo.ID, e.params.DTreeName)
	if err != nil {
		return fmt.Errorf("query DTree %s failed: %w", e.params.DTreeName, err)
	}
	if dtreeInfo == nil {
		return fmt.Errorf("DTree %s does not exist", e.params.DTreeName)
	}

	// Check existing quota
	quota, err := e.cli.GetDTreeQuotaByRawID(e.ctx, dtreeInfo.RawID)
	if err != nil {
		return fmt.Errorf("query quota for DTree %s failed: %w", e.params.DTreeName, err)
	}

	if quota != nil {
		if err := e.cli.UpdateDTreeQuota(e.ctx, quota.ID, &client.UpdateQuotaParams{
			SpaceHardQuota: e.params.Capacity}); err != nil {
			return fmt.Errorf("update quota %s for DTree %s failed: %w", quota.ID, e.params.DTreeName, err)
		}
	} else {
		// Create new quota
		log.AddContext(e.ctx).Infof("Quota not found, creating new quota for DTree %s with %d bytes",
			e.params.DTreeName, e.params.Capacity)
		_, err := e.cli.CreateDTreeQuota(e.ctx, &client.CreateQuotaParams{
			ParentID:       dtreeInfo.ID,
			ParentType:     parentType,
			QuotaType:      directoryQuota,
			SpaceHardQuota: e.params.Capacity,
		})
		if err != nil {
			return fmt.Errorf("create quota for DTree %s failed: %w", e.params.DTreeName, err)
		}
	}

	return nil
}
