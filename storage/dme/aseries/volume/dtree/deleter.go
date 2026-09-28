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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Deleter is used to delete a DME DTree volume
type Deleter struct {
	ctx        context.Context
	cli        client.DMEASeriesClientInterface
	parentName string
	dtreeName  string
}

// NewDeleter inits a new DME DTree deleter
func NewDeleter(ctx context.Context, cli client.DMEASeriesClientInterface, parentName, dtreeName string) *Deleter {
	return &Deleter{
		ctx:        ctx,
		cli:        cli,
		parentName: parentName,
		dtreeName:  dtreeName,
	}
}

// Delete deletes a DME DTree volume with proper cleanup sequence
func (d *Deleter) Delete() error {
	// Step 1: Delete NFS shares (query all associated NFS shares and delete them)
	if err := d.deleteNfsShares(); err != nil {
		return err
	}

	// Step 2: Delete DataTurbo shares (query all associated DPC shares and delete them)
	if err := d.deleteDataTurboShares(); err != nil {
		return err
	}

	// Step 3: Delete DTree (DME auto-cleans quotas)
	if err := d.deleteDTree(); err != nil {
		return err
	}

	return nil
}

func (d *Deleter) sharePath() string {
	return "/" + d.parentName + "/" + d.dtreeName
}

// deleteNfsShares queries and deletes all NFS shares for the DTree
func (d *Deleter) deleteNfsShares() error {
	sharePath := d.sharePath()
	nfsShare, err := d.cli.GetNfsShareByDTreePath(d.ctx, sharePath)
	if err != nil {
		return fmt.Errorf("query NFS share for DTree %s failed: %w", d.dtreeName, err)
	}
	if nfsShare == nil {
		log.AddContext(d.ctx).Infof("NFS share %q does not exist, skip deleting", sharePath)
		return nil
	}
	if err := d.cli.DeleteDTreeNfsShare(d.ctx, nfsShare.ID); err != nil {
		return fmt.Errorf("delete NFS share %s for DTree %s failed: %w", nfsShare.ID, d.dtreeName, err)
	}
	return nil
}

// deleteDataTurboShares queries and deletes all DataTurbo shares for the DTree
func (d *Deleter) deleteDataTurboShares() error {
	sharePath := d.sharePath()
	dpcShare, err := d.cli.GetDataTurboShareByDTreePath(d.ctx, sharePath)
	if err != nil {
		return fmt.Errorf("query DataTurbo share for DTree %s failed: %w", d.dtreeName, err)
	}
	if dpcShare == nil {
		log.AddContext(d.ctx).Infof("DataTurbo share %q does not exist, skip deleting", sharePath)
		return nil
	}
	if err := d.cli.DeleteDTreeDataTurboShare(d.ctx, dpcShare.ID); err != nil {
		return fmt.Errorf("delete DataTurbo share %s for DTree %s failed: %w", dpcShare.ID, d.dtreeName, err)
	}
	return nil
}

// deleteDTree queries and deletes the DTree; quota is auto-cleaned by DME
func (d *Deleter) deleteDTree() error {
	// Find parent FS to get fsId for DTree query
	fsInfo, err := d.cli.GetFileSystemByName(d.ctx, d.parentName)
	if err != nil {
		return fmt.Errorf("query parent filesystem %s failed: %w", d.parentName, err)
	}
	if fsInfo == nil {
		log.AddContext(d.ctx).Infof("Parent filesystem %s does not exist, DTree already cleaned up", d.parentName)
		return nil
	}

	dtreeInfo, err := d.cli.GetDTreeByName(d.ctx, fsInfo.ID, d.dtreeName)
	if err != nil {
		return fmt.Errorf("query DTree %s failed: %w", d.dtreeName, err)
	}
	if dtreeInfo == nil {
		log.AddContext(d.ctx).Infof("DTree %q under parent %q has already been deleted", d.dtreeName, d.parentName)
		return nil
	}

	if err := d.cli.DeleteDTreeByID(d.ctx, dtreeInfo.ID); err != nil {
		return fmt.Errorf("delete DTree %s (ID: %s) failed: %w", d.dtreeName, dtreeInfo.ID, err)
	}
	return nil
}
