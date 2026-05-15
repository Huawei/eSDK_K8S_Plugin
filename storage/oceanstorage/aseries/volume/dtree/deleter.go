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

// Package dtree defines operations of A-series dtree
package dtree

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Deleter is used to delete an A-series DTree volume
type Deleter struct {
	ctx        context.Context
	cli        client.OceanASeriesClientInterface
	vstoreId   string
	parentName string
	dtreeName  string
	nfsShareId string
	dtShareId  string
	protocol   string
}

// NewDeleter inits a new DTree deleter
func NewDeleter(ctx context.Context, cli client.OceanASeriesClientInterface, parentName, dtreeName,
	protocol string) *Deleter {
	return &Deleter{
		ctx:        ctx,
		cli:        cli,
		parentName: parentName,
		dtreeName:  dtreeName,
		protocol:   protocol,
	}
}

// Delete deletes a DTree volume with proper cleanup sequence
func (d *Deleter) Delete() error {
	d.vstoreId = d.cli.GetvStoreID()

	if d.protocol == constants.ProtocolDtfs {
		err := d.deleteDataTurboShare()
		if err != nil {
			log.AddContext(d.ctx).Errorf("Failed to delete DataTurbo share for DTree %s under parent %s, error: %v",
				d.dtreeName, d.parentName, err)
			return err
		}
	}

	if d.protocol == constants.ProtocolNfs {
		err := d.deleteNfsShare()
		if err != nil {
			log.AddContext(d.ctx).Errorf("Failed to delete NFS share for DTree %s under parent %s, error: %v",
				d.dtreeName, d.parentName, err)
			return err
		}
	}

	err := d.deleteDTree()
	if err != nil {
		log.AddContext(d.ctx).Errorf("Failed to delete DTree %s under parent %s, vStoreID: %s, error: %v", d.dtreeName,
			d.parentName, d.vstoreId, err)
		return err
	}

	return nil
}

// deleteNfsShare deletes the NFS share associated with the DTree
func (d *Deleter) deleteNfsShare() error {
	sharePath := d.sharePath()
	nfsShare, err := d.cli.GetNfsShareByPath(d.ctx, sharePath, d.vstoreId)
	if err != nil {
		return err
	}
	if nfsShare == nil {
		log.AddContext(d.ctx).Infof("The NFS share %q does not exist, skip deleting", sharePath)
		return nil
	}

	shareID, ok := utils.GetValue[string](nfsShare, "ID")
	if !ok || shareID == "" {
		log.AddContext(d.ctx).Errorf("NFS share %q exists but has invalid empty ID, deletion aborted due to data issue",
			sharePath)
		return fmt.Errorf("failed to delete NFS share %s: share has invalid empty ID", sharePath)
	}

	if err := d.cli.DeleteNfsShare(d.ctx, shareID, d.vstoreId); err != nil {
		return err
	}

	return nil
}

// deleteDataTurboShare deletes the DataTurbo share associated with the DTree
func (d *Deleter) deleteDataTurboShare() error {
	sharePath := d.sharePath()
	dtShare, err := d.cli.GetDataTurboShareByPath(d.ctx, sharePath, d.vstoreId)
	if err != nil {
		return err
	}
	if len(dtShare) == 0 {
		log.AddContext(d.ctx).Infof("The DataTurbo share %q does not exist, skip deleting", sharePath)
		return nil
	}

	shareID, ok := utils.GetValue[string](dtShare, "ID")
	if !ok || shareID == "" {
		log.AddContext(d.ctx).Errorf("DataTurbo share %q exists but has invalid empty ID, deletion aborted due to data issue",
			sharePath)
		return fmt.Errorf("failed to delete DataTurbo share %s: share has invalid empty ID", sharePath)
	}

	if err := d.cli.DeleteDataTurboShare(d.ctx, shareID, d.vstoreId); err != nil {
		return err
	}

	return nil
}

// deleteDTree deletes the DTree itself
func (d *Deleter) deleteDTree() error {
	dtreeInfo, err := d.cli.GetDTreeByName(d.ctx, d.parentName, d.dtreeName, d.vstoreId)
	if err != nil {
		return fmt.Errorf("failed to check DTree %s existence: %w", d.dtreeName, err)
	}
	if dtreeInfo == nil {
		log.AddContext(d.ctx).Infof("DTree %q under namespace %q has already been deleted",
			d.dtreeName, d.parentName)
		return nil
	}

	dtreeID, ok := utils.GetValue[string](dtreeInfo, "ID")
	if !ok || dtreeID == "" {
		return fmt.Errorf("invalid DTree ID for %s, dtreeInfo: %v", d.dtreeName, dtreeInfo)
	}

	err = d.cli.DeleteDTreeByID(d.ctx, d.vstoreId, dtreeID)
	if err != nil {
		return fmt.Errorf("failed to delete DTree %s (ID: %s): %w", d.dtreeName, dtreeID, err)
	}
	return nil
}

// sharePath generates the full path for the share
func (d *Deleter) sharePath() string {
	if d.parentName == "" || d.dtreeName == "" {
		log.AddContext(d.ctx).Errorf("Invalid parentName or dtreeName for share path")
		return ""
	}
	return "/" + d.parentName + "/" + d.dtreeName
}
