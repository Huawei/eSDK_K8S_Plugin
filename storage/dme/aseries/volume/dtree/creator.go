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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	nfsShareReadWriteStandalone = "read_and_write"
	nfsShareWriteModeSync       = "synchronization"
	dpcShareReadWrite           = "read_and_write"
)

// CreateDTreeVolumeModel is used to create a DME DTree volume
type CreateDTreeVolumeModel struct {
	Protocol     string
	DTreeName    string
	ParentName   string
	AllSquash    string
	RootSquash   string
	FsPermission string
	Capacity     int64
	AuthClients  []string
	AuthUsers    []string
	Description  string
}

func (model *CreateDTreeVolumeModel) sharePath() string {
	return "/" + model.ParentName + "/" + model.DTreeName
}

// Creator is used to create a DME DTree volume
type Creator struct {
	fsId         string
	dtreeId      string
	dtreeRawId   string
	quotaId      string
	nfsShareId   string
	dpcShareId   string
	dtreeCreated bool // true only if we created the DTree (not pre-existing)

	ctx    context.Context
	cli    client.DMEASeriesClientInterface
	params *CreateDTreeVolumeModel
}

// NewCreator inits a new DME DTree creator
func NewCreator(ctx context.Context, cli client.DMEASeriesClientInterface, params *CreateDTreeVolumeModel) *Creator {
	return &Creator{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Create creates a DTree volume using a task-flow approach with per-step rollback
func (c *Creator) Create() (utils.Volume, error) {
	tr := flow.NewTransaction()
	tr.Then(c.checkParentFS, nil)
	tr.Then(c.createDTree, c.rollbackDTree)
	tr.Then(c.createQuota, c.rollbackQuota)

	if len(c.params.AuthClients) > 0 {
		tr.Then(c.createNfsShare, c.rollbackNfsShare)
	}

	if len(c.params.AuthUsers) > 0 {
		tr.Then(c.createDpcShare, c.rollbackDpcShare)
	}

	err := tr.Commit()
	if err != nil {
		log.AddContext(c.ctx).Errorf("Failed to create DTree volume %s: %v", c.params.DTreeName, err)
		tr.Rollback()
		return nil, err
	}

	return c.newVolume(c.dtreeId), nil
}

// newVolume constructs a Volume from creator params with the given DTree ID
func (c *Creator) newVolume(dtreeID string) utils.Volume {
	vol := utils.NewVolume(c.params.DTreeName)
	vol.SetSize(c.params.Capacity)
	vol.SetID(dtreeID)
	vol.SetDTreeParentName(c.params.ParentName)
	return vol
}

// checkParentFS checks if the parent filesystem exists and sets fsId
func (c *Creator) checkParentFS() error {
	fsInfo, err := c.cli.GetFileSystemByName(c.ctx, c.params.ParentName)
	if err != nil {
		return fmt.Errorf("query parent filesystem %s failed: %w", c.params.ParentName, err)
	}
	if fsInfo == nil {
		return fmt.Errorf("parent filesystem %s does not exist", c.params.ParentName)
	}
	c.fsId = fsInfo.ID
	return nil
}

// createDTree checks if DTree already exists; creates one if missing
func (c *Creator) createDTree() error {
	dtreeInfo, err := c.cli.GetDTreeByName(c.ctx, c.fsId, c.params.DTreeName)
	if err != nil {
		return fmt.Errorf("query DTree %s in filesystem %s failed: %w", c.params.DTreeName, c.fsId, err)
	}

	if dtreeInfo != nil {
		c.dtreeId = dtreeInfo.ID
		c.dtreeRawId = dtreeInfo.RawID
		c.dtreeCreated = false
		return nil
	}

	params := &client.CreateDTreeParams{
		CreateDtreesParam: []*client.CreateDtreeParam{
			{DtreeName: c.params.DTreeName, Count: 1},
		},
		QuotaSwitch:     true,
		StorageID:       c.cli.GetStorageID(),
		FsID:            c.fsId,
		UnixPermissions: c.params.FsPermission,
	}

	resp, err := c.cli.CreateDTree(c.ctx, params)
	if err != nil {
		return fmt.Errorf("create DTree %s failed: %w", c.params.DTreeName, err)
	}

	c.dtreeId = resp.DtreeID
	c.dtreeRawId = resp.DtreeRawID
	c.dtreeCreated = true
	return nil
}

// rollbackDTree rolls back DTree creation; only deletes if we created it
func (c *Creator) rollbackDTree() {
	if c.dtreeId == "" || !c.dtreeCreated {
		return
	}

	if err := c.cli.DeleteDTreeByID(c.ctx, c.dtreeId); err != nil {
		log.AddContext(c.ctx).Warningf("Failed to rollback DTree %s: %v", c.dtreeId, err)
	}
}

// createQuota checks if quota exists for the DTree; creates one if missing, or validates capacity if present
func (c *Creator) createQuota() error {
	quotaInfo, err := c.cli.GetDTreeQuotaByRawID(c.ctx, c.dtreeRawId)
	if err != nil {
		return fmt.Errorf("query quota for DTree %s failed: %w", c.params.DTreeName, err)
	}

	if quotaInfo != nil {
		if quotaInfo.SpaceHardQuota != c.params.Capacity {
			return fmt.Errorf("quota capacity %d does not match expected %d for DTree %s",
				quotaInfo.SpaceHardQuota, c.params.Capacity, c.params.DTreeName)
		}
		// Quota already exists with matching capacity — no need to track for rollback
		return nil
	}

	quotaParams := &client.CreateQuotaParams{
		ParentID:       c.dtreeId,
		ParentType:     parentType,
		QuotaType:      directoryQuota,
		SpaceHardQuota: c.params.Capacity,
	}
	resp, err := c.cli.CreateDTreeQuota(c.ctx, quotaParams)
	if err != nil {
		return fmt.Errorf("create quota for DTree %s failed: %w", c.params.DTreeName, err)
	}
	c.quotaId = resp.ID
	return nil
}

// rollbackQuota rolls back quota creation
func (c *Creator) rollbackQuota() {
	if c.quotaId == "" {
		return
	}

	if err := c.cli.DeleteDTreeQuota(c.ctx, c.quotaId); err != nil {
		log.AddContext(c.ctx).Warningf("Failed to rollback quota %s: %v", c.quotaId, err)
	}
}

// createNfsShare checks if NFS share exists for the DTree; creates one if missing, or deletes and recreates if present
func (c *Creator) createNfsShare() error {
	sharePath := c.params.sharePath()
	nfsShare, err := c.cli.GetNfsShareByDTreePath(c.ctx, sharePath)
	if err != nil {
		return fmt.Errorf("query NFS share for DTree %s failed: %w", c.params.DTreeName, err)
	}

	if nfsShare != nil {
		if err := c.cli.DeleteDTreeNfsShare(c.ctx, nfsShare.ID); err != nil {
			return fmt.Errorf("delete existing NFS share %s for DTree %s failed: %w",
				nfsShare.ID, c.params.DTreeName, err)
		}
	}

	nfsParam := client.DTreeCreateNfsShareParam{
		SharePath:   sharePath,
		FsID:        c.fsId,
		Description: c.params.Description,
	}
	for _, authClient := range c.params.AuthClients {
		nfsParam.NfsClientAddition = append(nfsParam.NfsClientAddition, client.NfsClientAddition{
			Name:                     authClient,
			Permission:               nfsShareReadWriteStandalone,
			WriteMode:                nfsShareWriteModeSync,
			PermissionConstraint:     c.params.AllSquash,
			RootPermissionConstraint: c.params.RootSquash,
		})
	}

	resp, err := c.cli.CreateDTreeNfsShare(c.ctx, client.CreateNfsShareRequestBody{
		CreateNfsShareParam: nfsParam,
	})
	if err != nil {
		return fmt.Errorf("create NFS share for DTree %s failed: %w", c.params.DTreeName, err)
	}
	c.nfsShareId = resp.ID
	return nil
}

// rollbackNfsShare rolls back NFS share creation
func (c *Creator) rollbackNfsShare() {
	if c.nfsShareId == "" {
		return
	}

	if err := c.cli.DeleteDTreeNfsShare(c.ctx, c.nfsShareId); err != nil {
		log.AddContext(c.ctx).Warningf("Failed to rollback NFS share %s: %v", c.nfsShareId, err)
	}
}

// createDpcShare checks if DPC share exists for the DTree; creates one if missing, or deletes and recreates if present
func (c *Creator) createDpcShare() error {
	sharePath := c.params.sharePath()
	dpcShare, err := c.cli.GetDataTurboShareByDTreePath(c.ctx, sharePath)
	if err != nil {
		return fmt.Errorf("query DPC share for DTree %s failed: %w", c.params.DTreeName, err)
	}

	if dpcShare != nil {
		if err := c.cli.DeleteDTreeDataTurboShare(c.ctx, dpcShare.ID); err != nil {
			return fmt.Errorf("delete existing DPC share %s for DTree %s failed: %w",
				dpcShare.ID, c.params.DTreeName, err)
		}
	}

	dpcParams := client.CreateDpcShareParams{
		FsID:        c.fsId,
		DtreeID:     c.dtreeId,
		Description: c.params.Description,
		Charset:     storage.CharsetUtf8,
	}
	for _, user := range c.params.AuthUsers {
		adminInfo, err := c.cli.GetDataTurboUserByName(c.ctx, user)
		if err != nil {
			return fmt.Errorf("get DataTurbo user %s failed: %w", user, err)
		}
		if adminInfo == nil {
			return fmt.Errorf("dataTurbo user %s does not exist", user)
		}
		dpcParams.DpcShareAuth = append(dpcParams.DpcShareAuth, client.DpcAuth{
			DpcUserID:  adminInfo.ID,
			Permission: dpcShareReadWrite,
		})
	}

	resp, err := c.cli.CreateDTreeDpcShare(c.ctx, dpcParams)
	if err != nil {
		return fmt.Errorf("create DPC share for DTree %s failed: %w", c.params.DTreeName, err)
	}
	c.dpcShareId = resp.ID
	return nil
}

// rollbackDpcShare rolls back DPC share creation
func (c *Creator) rollbackDpcShare() {
	if c.dpcShareId == "" {
		return
	}

	if err := c.cli.DeleteDTreeDataTurboShare(c.ctx, c.dpcShareId); err != nil {
		log.AddContext(c.ctx).Warningf("Failed to rollback DPC share %s: %v", c.dpcShareId, err)
	}
}
