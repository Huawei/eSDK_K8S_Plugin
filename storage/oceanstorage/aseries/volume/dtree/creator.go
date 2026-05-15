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
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	readWriteAccessValue = 1
	synchronize          = 0
	dtreeVolumeType      = 16445
	directoryQuotaType   = 1
	spaceUnitTypeBytes   = 0
	description          = "Created from Kubernetes CSI"
)

// CreateDTreeModel is used to create a DTree volume
type CreateDTreeModel struct {
	Protocol     string
	DTreeName    string
	ParentName   string
	AllSquash    int
	RootSquash   int
	FsPermission string
	Capacity     int64
	AuthClients  []string
	AuthUsers    []string
}

func (model *CreateDTreeModel) sharePath() string {
	return "/" + model.ParentName + "/" + model.DTreeName
}

// Creator is used to create an A-series DTree volume
type Creator struct {
	vstoreId   string
	fsId       string
	dtreeId    string
	quotaId    string
	nfsShareId string
	dtShareId  string

	ctx    context.Context
	cli    client.OceanASeriesClientInterface
	params *CreateDTreeModel
}

// NewCreator inits a new DTree creator
func NewCreator(ctx context.Context, cli client.OceanASeriesClientInterface, params *CreateDTreeModel) *Creator {
	return &Creator{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Create creates a DTree resource and returns a volume object
func (c *Creator) Create() (utils.Volume, error) {
	tr := flow.NewTransaction()
	tr.Then(c.checkParentFS, nil)
	tr.Then(c.createDTree, c.rollbackDTree)
	tr.Then(c.createQuota, c.rollbackQuota)

	if c.params.Protocol == constants.ProtocolNfs {
		tr.Then(c.createNfsShare, c.rollbackNfsShare)
		tr.Then(c.createAuthClients, nil)
	}

	if c.params.Protocol == constants.ProtocolDtfs {
		tr.Then(c.createDataTurboShare, c.rollbackDataTurboShare)
		tr.Then(c.addAuthUser, nil)
	}

	err := tr.Commit()
	if err != nil {
		log.AddContext(c.ctx).Errorf("Failed to create DTree volume %s: %v", c.params.DTreeName, err)
		tr.Rollback()
		return nil, err
	}

	vol := utils.NewVolume(c.params.DTreeName)
	vol.SetSize(c.params.Capacity)
	vol.SetID(c.dtreeId)
	vol.SetDTreeParentName(c.params.ParentName)

	return vol, nil
}

// checkParentFS checks if the parent filesystem exists
func (c *Creator) checkParentFS() error {
	c.vstoreId = c.cli.GetvStoreID()

	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.ParentName, c.vstoreId)
	if err != nil {
		return err
	}
	if len(fs) == 0 {
		return fmt.Errorf("parent filesystem %s does not exist", c.params.ParentName)
	}

	fsId, ok := utils.GetValue[string](fs, "ID")
	if !ok || fsId == "" {
		return fmt.Errorf("get parent filesystem ID failed, fs: %v", fs)
	}
	c.fsId = fsId
	return nil
}

// createDTree creates a DTree
func (c *Creator) createDTree() error {
	dtreeInfo, err := c.cli.GetDTreeByName(c.ctx, c.params.ParentName, c.params.DTreeName, c.vstoreId)
	if err != nil {
		return err
	}
	if dtreeInfo != nil {
		dtreeId, ok := utils.GetValue[string](dtreeInfo, "ID")
		if !ok || dtreeId == "" {
			return fmt.Errorf("get DTree ID failed, dtreeInfo: %v", dtreeInfo)
		}
		c.dtreeId = dtreeId
		return nil
	}

	req := &client.DTreeCreateRequest{
		Name:       c.params.DTreeName,
		ParentName: c.params.ParentName,
	}

	if c.params.FsPermission != "" {
		req.UnixPermissions = c.params.FsPermission
	}
	if c.vstoreId != "" {
		req.VStoreID = c.vstoreId
	}

	resp, err := c.cli.CreateDTree(c.ctx, req)

	if err != nil {
		return err
	}

	dtreeId, ok := utils.GetValue[string](resp, "ID")
	if !ok || dtreeId == "" {
		return fmt.Errorf("failed to create DTree: got empty ID from response %v", resp)
	}
	c.dtreeId = dtreeId

	return nil
}

// rollbackDTree rolls back DTree creation
func (c *Creator) rollbackDTree() {
	if c.dtreeId == "" {
		return
	}

	if err := c.cli.DeleteDTreeByID(c.ctx, c.vstoreId, c.dtreeId); err != nil {
		log.AddContext(c.ctx).Warningf("Failed to rollback DTree %s: %v", c.dtreeId, err)
	}
}

// createQuota creates a quota for the DTree
func (c *Creator) createQuota() error {
	quotaInfo, err := c.cli.GetDTreeQuota(c.ctx, c.dtreeId, c.vstoreId)
	if err != nil {
		return err
	}
	if quotaInfo != nil {
		if err := c.checkExistingQuotaAndValidate(quotaInfo); err != nil {
			return err
		}
		if c.quotaId != "" {
			return nil
		}
	}

	return c.createNewQuotaRecord()
}

// checkExistingQuotaAndValidate checks existing quota information and validates capacity
func (c *Creator) checkExistingQuotaAndValidate(quotaInfo map[string]interface{}) error {
	quotaId, ok := utils.GetValue[string](quotaInfo, "ID")
	if !ok || quotaId == "" {
		return nil
	}
	c.quotaId = quotaId

	spaceHardQuotaStr, ok := utils.GetValue[string](quotaInfo, "SPACEHARDQUOTA")
	if ok && spaceHardQuotaStr != "" {
		existingQuota, parseErr := parseQuotaValue(spaceHardQuotaStr)
		if parseErr != nil {
			log.AddContext(c.ctx).Errorf("Failed to parse quota value %s: %v", spaceHardQuotaStr, parseErr)
			return fmt.Errorf("quota already exists with different capacity: failed to parse existing quota value %s",
				spaceHardQuotaStr)
		}
		if existingQuota != c.params.Capacity {
			log.AddContext(c.ctx).Errorf("Quota capacity mismatch: existing %d, requested %d for quota string %s",
				existingQuota, c.params.Capacity, spaceHardQuotaStr)
			return fmt.Errorf("quota already exists with different capacity: existing %d, requested %d",
				existingQuota, c.params.Capacity)
		}
	}

	return nil
}

// createNewQuotaRecord creates a new quota record for the DTree
func (c *Creator) createNewQuotaRecord() error {
	req := &client.DTreeQuotaRequest{
		PARENTTYPE:     strconv.Itoa(dtreeVolumeType),
		PARENTID:       c.dtreeId,
		QUOTATYPE:      strconv.Itoa(directoryQuotaType),
		SPACEHARDQUOTA: strconv.FormatInt(c.params.Capacity, constants.DefaultIntBase),
		SPACEUNITTYPE:  strconv.Itoa(spaceUnitTypeBytes),
	}
	if c.vstoreId != "" {
		req.VStoreID = c.vstoreId
	}

	resp, err := c.cli.CreateDTreeQuota(c.ctx, req)
	if err != nil {
		return err
	}

	quotaId, ok := utils.GetValue[string](resp, "ID")
	if !ok || quotaId == "" {
		return fmt.Errorf("failed to create DTree quota: got empty ID from response %v", resp)
	}
	c.quotaId = quotaId

	return nil
}

// rollbackQuota rolls back DTree quota creation
func (c *Creator) rollbackQuota() {
	if c.quotaId == "" {
		return
	}

	if err := c.cli.DeleteDTreeQuota(c.ctx, c.quotaId, c.vstoreId); err != nil {
		log.AddContext(c.ctx).Warningf("Failed to rollback DTree quota %s: %v", c.quotaId, err)
	}
}

// createNfsShare creates an NFS share for the DTree
func (c *Creator) createNfsShare() error {
	nfsShare, err := c.cli.GetNfsShareByPath(c.ctx, c.params.sharePath(), c.vstoreId)
	if err != nil {
		return err
	}

	if nfsShare != nil {
		shareID, ok := utils.GetValue[string](nfsShare, "ID")
		if ok && shareID != "" {
			log.AddContext(c.ctx).Infof("Delete duplicate NFS share, share id: %s", shareID)
			if err = c.cli.DeleteNfsShare(c.ctx, shareID, c.vstoreId); err != nil {
				return err
			}
		}
	}

	createParams := map[string]interface{}{
		"sharepath":   c.params.sharePath(),
		"fsid":        c.fsId,
		"description": description,
		"dtreeid":     c.dtreeId,
	}
	if c.vstoreId != "" {
		createParams["vStoreID"] = c.vstoreId
	}

	newShare, err := c.cli.CreateNfsShare(c.ctx, createParams)
	if err != nil {
		return err
	}
	if newShare == nil {
		return fmt.Errorf("failed to create NFS share %s: response is nil", c.params.sharePath())
	}

	shareID, ok := utils.GetValue[string](newShare, "ID")
	if !ok || shareID == "" {
		return fmt.Errorf("failed to create NFS share %s: got empty ID", c.params.sharePath())
	}
	c.nfsShareId = shareID

	return nil
}

// rollbackNfsShare rolls back NFS share creation
func (c *Creator) rollbackNfsShare() {
	if c.nfsShareId == "" {
		return
	}

	if err := c.cli.DeleteNfsShare(c.ctx, c.nfsShareId, c.vstoreId); err != nil {
		log.AddContext(c.ctx).Warningf("Failed to rollback NFS share %s: %v", c.nfsShareId, err)
	}
}

// createAuthClients creates auth clients for the NFS share
func (c *Creator) createAuthClients() error {
	for _, authClient := range c.params.AuthClients {
		if err := c.cli.AllowNfsShareAccess(c.ctx, &base.AllowNfsShareAccessRequest{
			Name:       authClient,
			ParentID:   c.nfsShareId,
			VStoreID:   c.vstoreId,
			AccessVal:  readWriteAccessValue,
			Sync:       synchronize,
			AllSquash:  c.params.AllSquash,
			RootSquash: c.params.RootSquash,
		}); err != nil {
			return err
		}
	}

	return nil
}

// createDataTurboShare creates a DataTurbo share for the DTree
func (c *Creator) createDataTurboShare() error {
	dtShare, err := c.cli.GetDataTurboShareByPath(c.ctx, c.params.sharePath(), c.vstoreId)
	if err != nil {
		return err
	}

	if dtShare != nil {
		shareID, ok := utils.GetValue[string](dtShare, "ID")
		if ok && shareID != "" {
			log.AddContext(c.ctx).Infof("Delete duplicate DataTurbo share, share id: %s", shareID)
			if err = c.cli.DeleteDataTurboShare(c.ctx, shareID, c.vstoreId); err != nil {
				return err
			}
		}
	}

	newShare, err := c.cli.CreateDataTurboShare(c.ctx, &client.CreateDataTurboShareParams{
		SharePath:   c.params.sharePath(),
		FsId:        c.fsId,
		Description: description,
		VstoreId:    c.vstoreId,
	})
	if err != nil {
		return err
	}
	if newShare == nil {
		return fmt.Errorf("failed to create DataTurbo share %s: response is nil", c.params.sharePath())
	}

	shareID, ok := utils.GetValue[string](newShare, "ID")
	if !ok || shareID == "" {
		return fmt.Errorf("failed to create DataTurbo share %s: got empty ID", c.params.sharePath())
	}
	c.dtShareId = shareID

	return nil
}

// rollbackDataTurboShare rolls back DataTurbo share creation
func (c *Creator) rollbackDataTurboShare() {
	if c.dtShareId == "" {
		return
	}

	if err := c.cli.DeleteDataTurboShare(c.ctx, c.dtShareId, c.vstoreId); err != nil {
		log.AddContext(c.ctx).Warningf("Failed to rollback DataTurbo share %s: %v", c.dtShareId, err)
	}
}

// addAuthUser adds auth user for DataTurbo share
func (c *Creator) addAuthUser() error {
	for _, user := range c.params.AuthUsers {
		if err := c.cli.AddDataTurboShareUser(c.ctx, &client.AddDataTurboShareUserParams{
			UserName:   user,
			ShareId:    c.dtShareId,
			Permission: readWriteAccessValue,
			VstoreId:   c.vstoreId,
		}); err != nil {
			return err
		}
	}

	return nil
}

// parseQuotaValue parses quota string value to int64
func parseQuotaValue(quotaStr string) (int64, error) {
	if quotaStr == "" {
		return 0, nil
	}
	var val int64
	val, err := strconv.ParseInt(quotaStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
	if err != nil {
		return 0, fmt.Errorf("parse quota value %s failed: %w", quotaStr, err)
	}
	return val, nil
}
