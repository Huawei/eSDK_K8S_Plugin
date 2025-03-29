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

// Package dtree defines operations of fusion storage dtree
package dtree

import (
	"context"
	"errors"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	readWriteAccessValue = 1
	synchronize          = 0
)

// CreateDTreeModel is used to create a volume
type CreateDTreeModel struct {
	Protocol     string
	DTreeName    string
	ParentName   string
	AllSquash    int
	RootSquash   int
	Description  string
	FsPermission string
	Capacity     int64
	AuthClients  []string
}

func (model *CreateDTreeModel) sharePath() string {
	return "/" + model.ParentName + "/" + model.DTreeName
}

// Creator is used to create a pacific dtree volume
type Creator struct {
	accountId  string
	dtreeId    string
	quotaId    string
	nfsShareId string

	ctx    context.Context
	cli    client.IRestClient
	params *CreateDTreeModel
}

// NewCreator inits a new dtree client
func NewCreator(ctx context.Context, cli client.IRestClient, params *CreateDTreeModel) *Creator {
	return &Creator{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Create creates a dtree resource and returns a volume object
func (c *Creator) Create() (utils.Volume, error) {
	tr := flow.NewTransaction()
	tr.Then(c.createDTree, c.rollbackDTree).
		Then(c.createQuota, c.rollbackQuota)

	if c.params.Protocol == constants.ProtocolNfs {
		tr.Then(c.createNfsShare, c.rollbackNfsShare)
		tr.Then(c.createAuthClients, nil)
	}

	err := tr.Commit()
	if err != nil {
		tr.Rollback()
		return nil, err
	}

	vol := utils.NewVolume(c.params.DTreeName)
	vol.SetSize(c.params.Capacity)
	vol.SetID(c.dtreeId)
	vol.SetDTreeParentName(c.params.ParentName)

	return vol, nil
}

func (c *Creator) createDTree() error {
	dtree, err := c.cli.GetDTreeByName(c.ctx, c.params.ParentName, c.params.DTreeName)
	if err != nil {
		return err
	}
	if dtree != nil {
		c.dtreeId = dtree.Id
		return nil
	}

	dtree, err = c.cli.CreateDTree(c.ctx, c.params.ParentName, c.params.DTreeName, c.params.FsPermission)
	if err != nil {
		return err
	}
	if dtree == nil || dtree.Id == "" {
		return errors.New("failed to create dtree: new dtree is nil or with empty id")
	}
	c.dtreeId = dtree.Id

	return nil
}

func (c *Creator) rollbackDTree() {
	if c.dtreeId == "" {
		return
	}

	if err := c.cli.DeleteDTree(c.ctx, c.dtreeId); err != nil {
		log.AddContext(c.ctx).Errorf("Failed to rollback dtree: %v", err)
	}
}

func (c *Creator) createQuota() error {
	quota, err := c.cli.GetQuotaByDTreeId(c.ctx, c.dtreeId)
	if err != nil {
		return err
	}
	if quota != nil {
		if quota.SpaceHardQuota != c.params.Capacity {
			return fmt.Errorf("failed to create quota: quota already exists and capacity %d"+
				" is different from quota %d", c.params.Capacity, quota.SpaceHardQuota)
		}
		c.quotaId = quota.Id
		return nil
	}

	quota, err = c.cli.CreateDTreeQuota(c.ctx, c.dtreeId, c.params.Capacity)
	if err != nil {
		return err
	}
	if quota == nil || quota.Id == "" {
		return errors.New("failed to create quota: new quota is nil or with empty id")
	}
	c.quotaId = quota.Id

	return nil
}

func (c *Creator) rollbackQuota() {
	if c.quotaId == "" {
		return
	}

	if err := c.cli.DeleteDTreeQuota(c.ctx, c.quotaId); err != nil {
		log.AddContext(c.ctx).Errorf("failed to rollback dtree quota: %v", err)
	}
}

func (c *Creator) createNfsShare() error {
	nfsShare, err := c.cli.GetDTreeNfsShareByPath(c.ctx, c.params.sharePath())
	if err != nil {
		return err
	}
	if nfsShare != nil {
		log.AddContext(c.ctx).Infof("Delete duplicate nfs share, share id: %s", nfsShare.Id)
		if err := c.cli.DeleteDTreeNfsShare(c.ctx, nfsShare.Id); err != nil {
			return err
		}
	}

	newShare, err := c.cli.CreateDTreeNfsShare(c.ctx, &client.CreateDTreeNfsShareRequest{
		DtreeId:     c.dtreeId,
		Sharepath:   c.params.sharePath(),
		Description: c.params.Description,
	})
	if err != nil {
		return err
	}
	if newShare == nil || newShare.Id == "" {
		return errors.New("failed to create nfs share: new nfs share is nil or with empty id")
	}
	c.nfsShareId = newShare.Id

	return nil
}

func (c *Creator) rollbackNfsShare() {
	if c.nfsShareId == "" {
		return
	}

	if err := c.cli.DeleteDTreeNfsShare(c.ctx, c.nfsShareId); err != nil {
		log.AddContext(c.ctx).Errorf("Failed to rollback dtree nfs share: %v", err)
	}
}

func (c *Creator) createAuthClients() error {
	for _, authClient := range c.params.AuthClients {
		if err := c.cli.AddNfsShareAuthClient(c.ctx, &client.AddNfsShareAuthClientRequest{
			AccessName:  authClient,
			ShareId:     c.nfsShareId,
			AccessValue: readWriteAccessValue,
			Sync:        synchronize,
			AllSquash:   c.params.AllSquash,
			RootSquash:  c.params.RootSquash,
		}); err != nil {
			return err
		}
	}

	return nil
}
