/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.com/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

// Package dtree defines operations of A-series dTree
package dtree

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// ExpandDTreeModel is used to expand the capacity of A-series DTree volume
type ExpandDTreeModel struct {
	ParentName string
	DTreeName  string
	Capacity   int64
}

// Expander is used to expand the capacity of A-series DTree volume
type Expander struct {
	ctx      context.Context
	cli      client.OceanASeriesClientInterface
	params   *ExpandDTreeModel
	vstoreId string
}

// NewExpander inits a new DTree expander
func NewExpander(ctx context.Context, cli client.OceanASeriesClientInterface, params *ExpandDTreeModel) *Expander {
	return &Expander{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Expand expands the DTree volume capacity with transaction support
func (e *Expander) Expand() error {
	e.vstoreId = e.cli.GetvStoreID()

	err := e.expandDTree()
	if err != nil {
		log.AddContext(e.ctx).Errorf("failed to expand DTree volume %s: %v", e.params.DTreeName, err)
		return err
	}
	return nil
}

// expandDTree performs the actual DTree quota expansion
func (e *Expander) expandDTree() error {
	dTree, err := e.cli.GetDTreeByName(e.ctx, e.params.ParentName, e.params.DTreeName, e.vstoreId)
	if err != nil {
		return err
	}

	if dTree == nil {
		return fmt.Errorf("the volume: %s to be expanded does not exist", e.params.DTreeName)
	}

	dtreeID, ok := utils.GetValue[string](dTree, "ID")
	if !ok || dtreeID == "" {
		return fmt.Errorf("get DTree ID failed for expansion, dtreeInfo: %v", dTree)
	}

	newQuota, err := e.updateOrCreateQuota(dtreeID)
	if err != nil {
		return err
	}
	if newQuota == nil {
		return fmt.Errorf("create quota for A-series DTree: %s failed, quota is nil", dtreeID)
	}

	return nil
}

// updateOrCreateQuota checks if quota exists and updates it, or creates a new quota if it doesn't exist.
func (e *Expander) updateOrCreateQuota(dtreeID string) (map[string]interface{}, error) {
	quota, err := e.cli.GetDTreeQuota(e.ctx, dtreeID, e.vstoreId)
	if err != nil {
		return nil, err
	}

	if quota != nil {
		// Check and update existing quota
		quotaID, ok := utils.GetValue[string](quota, "ID")
		if !ok || quotaID == "" {
			log.AddContext(e.ctx).Errorf("Get quota ID failed for DTree %s", dtreeID)
			return nil, fmt.Errorf("get quota ID failed for DTree %s", dtreeID)
		}

		spaceHardQuotaStr, ok := utils.GetValue[string](quota, "SPACEHARDQUOTA")
		if ok && spaceHardQuotaStr != "" {
			existingQuota, parseErr := parseQuotaValue(spaceHardQuotaStr)
			if parseErr != nil {
				log.AddContext(e.ctx).Errorf("Failed to parse existing quota value %s for DTree %s: %v",
					spaceHardQuotaStr, dtreeID, parseErr)
				return nil, fmt.Errorf("failed to parse existing quota value %s: %w",
					spaceHardQuotaStr, parseErr)
			}
			if e.params.Capacity < existingQuota {
				log.AddContext(e.ctx).Errorf("Target capacity: %d must be greater than the current capacity: %d",
					e.params.Capacity, existingQuota)
				return nil, fmt.Errorf("target capacity: %d must be greater than the current capacity: %d",
					e.params.Capacity, existingQuota)
			}
		}

		// Update existing quota
		req := &client.DTreeQuotaUpdateRequest{
			VStoreID:       e.vstoreId,
			SPACEHARDQUOTA: strconv.FormatInt(e.params.Capacity, constants.DefaultIntBase),
			SPACEUNITTYPE:  strconv.Itoa(spaceUnitTypeBytes),
		}

		err = e.cli.UpdateDTreeQuota(e.ctx, quotaID, req)
		return quota, err
	}

	// Create new quota if it doesn't exist
	req := &client.DTreeQuotaRequest{
		PARENTTYPE:     strconv.Itoa(dtreeVolumeType),
		PARENTID:       dtreeID,
		QUOTATYPE:      strconv.Itoa(directoryQuotaType),
		SPACEHARDQUOTA: strconv.FormatInt(e.params.Capacity, constants.DefaultIntBase),
		SPACEUNITTYPE:  strconv.Itoa(spaceUnitTypeBytes),
	}
	if e.vstoreId != "" {
		req.VStoreID = e.vstoreId
	}

	return e.cli.CreateDTreeQuota(e.ctx, req)
}
