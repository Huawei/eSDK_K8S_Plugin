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

// Package volume defines operations of volumes
package volume

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	readWriteAccessValue = 1
	synchronize          = 0
)

// CreateFilesystemModel is used to create a volume
type CreateFilesystemModel struct {
	Protocol        string
	Name            string
	PoolName        string
	WorkloadType    string
	Capacity        int64
	Description     string
	UnixPermissions string
	AllSquash       int
	RootSquash      int
	Qos             string
	AuthClients     []string
	AuthUsers       []string
	AdvancedOptions map[string]interface{}
}

func (model *CreateFilesystemModel) sharePath() string {
	return "/" + model.Name + "/"
}

// Creator is used to create a filesystem volume
type Creator struct {
	vstoreId         string
	poolId           string
	workloadTypeId   string
	fsId             string
	nfsShareId       string
	dataTurboShareId string
	qosId            string

	ctx    context.Context
	cli    client.OceanASeriesClientInterface
	params *CreateFilesystemModel
}

// NewCreator inits a new filesystem volume creator
func NewCreator(ctx context.Context, cli client.OceanASeriesClientInterface, params *CreateFilesystemModel) *Creator {
	return &Creator{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Create creates a filesystem resource and returns a volume object
func (c *Creator) Create() (utils.Volume, error) {
	tr := flow.NewTransaction()
	tr.Then(c.validateAndPrepareParams, nil)
	tr.Then(c.createFilesystem, c.rollBackendFilesystem)

	if c.params.Protocol == constants.ProtocolNfs {
		tr.Then(c.createNfsShare, c.rollbackNfsShare)
		tr.Then(c.createAuthClients, nil)
	}

	if c.params.Protocol == constants.ProtocolDtfs {
		tr.Then(c.createDataTurboShare, c.rollbackDataTurboShare)
		tr.Then(c.addAuthUser, nil)
	}

	tr.Then(c.createQos, c.revertQos)

	err := tr.Commit()
	if err != nil {
		tr.Rollback()
		return nil, err
	}

	vol := utils.NewVolume(c.params.Name)
	vol.SetSize(utils.TransK8SCapacity(c.params.Capacity, constants.AllocationUnitBytes))
	vol.SetID(c.fsId)

	return vol, nil
}

func (c *Creator) validateAndPrepareParams() error {
	if c.params.Protocol == constants.ProtocolNfs && len(c.params.AuthClients) == 0 {
		return fmt.Errorf("authClient parameter must be provided in StorageClass for nfs protocol")
	}

	if c.params.Protocol == constants.ProtocolDtfs && len(c.params.AuthUsers) == 0 {
		return fmt.Errorf("authUser parameter must be provided in StorageClass for dtfs protocol")
	}

	c.vstoreId = c.cli.GetvStoreID()

	err := c.setPoolId()
	if err != nil {
		return err
	}

	err = c.setWorkloadId()
	if err != nil {
		return err
	}

	return nil
}

func (c *Creator) setPoolId() error {
	pool, err := c.cli.GetPoolByName(c.ctx, c.params.PoolName)
	if err != nil {
		return err
	}

	if pool == nil {
		return fmt.Errorf("pool %s does not exist", c.params.PoolName)
	}

	poolId, ok := utils.GetValue[string](pool, "ID")
	if !ok {
		return fmt.Errorf("get pool %s info with empty ID", c.params.PoolName)
	}

	c.poolId = poolId
	return nil
}

func (c *Creator) setWorkloadId() error {
	if c.params.WorkloadType == "" {
		c.workloadTypeId = ""
		return nil
	}

	workloadTypeId, err := c.cli.GetApplicationTypeByName(c.ctx, c.params.WorkloadType)
	if err != nil {
		return err
	}

	if workloadTypeId == "" {
		return fmt.Errorf("workload type %s does not exist", c.params.WorkloadType)
	}

	c.workloadTypeId = workloadTypeId
	return nil
}

func (c *Creator) createFilesystem() error {
	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.Name, c.vstoreId)
	if err != nil {
		return err
	}
	if len(fs) != 0 {
		if fsId, ok := utils.GetValue[string](fs, "ID"); ok {
			c.fsId = fsId
			c.qosId, _ = utils.GetValue[string](fs, "IOCLASSID")
			return nil
		}
	}

	fs, err = c.cli.CreateFileSystem(c.ctx, &client.CreateFilesystemParams{
		Name:            c.params.Name,
		ParentId:        c.poolId,
		Capacity:        c.params.Capacity,
		Description:     c.params.Description,
		WorkLoadTypeId:  c.workloadTypeId,
		UnixPermissions: c.params.UnixPermissions,
		VstoreId:        c.vstoreId,
	}, c.params.AdvancedOptions)
	if err != nil {
		return err
	}

	if fs == nil {
		return fmt.Errorf("failed to create filesystem %s, get filesystem info of nil", c.params.Name)
	}

	fsId, ok := utils.GetValue[string](fs, "ID")
	if !ok {
		return fmt.Errorf("failed to create filesystem %s, get filesystem info with empty ID", c.params.Name)
	}

	c.fsId = fsId
	return nil
}

func (c *Creator) rollBackendFilesystem() {
	if c.fsId == "" {
		return
	}

	deleteParams := map[string]interface{}{"ID": c.fsId}
	if err := c.cli.DeleteFileSystem(c.ctx, deleteParams); err != nil {
		log.AddContext(c.ctx).Errorf("Failed to rollback filesystem %s: %v", c.fsId, err)
	}
}

func (c *Creator) createNfsShare() error {
	nfsShare, err := c.cli.GetNfsShareByPath(c.ctx, c.params.sharePath(), c.vstoreId)
	if err != nil {
		return err
	}

	if len(nfsShare) != 0 {
		shareID, ok := utils.GetValue[string](nfsShare, "ID")
		if !ok {
			return fmt.Errorf("get NFS share info %v with empty ID", nfsShare)
		}

		log.AddContext(c.ctx).Warningf("Delete duplicate nfs share, share id: %s", shareID)
		if err = c.cli.DeleteNfsShare(c.ctx, shareID, c.vstoreId); err != nil {
			return err
		}
	}

	createParams := map[string]interface{}{
		"sharepath":   c.params.sharePath(),
		"fsid":        c.fsId,
		"description": c.params.Description,
		"vStoreID":    c.vstoreId,
	}
	newShare, err := c.cli.CreateNfsShare(c.ctx, createParams)
	if err != nil {
		return err
	}

	if newShare == nil {
		return fmt.Errorf("failed to create NFS share %s, get share info of nil", c.params.sharePath())
	}

	shareID, ok := utils.GetValue[string](newShare, "ID")
	if !ok {
		return fmt.Errorf("failed to create NFS share %s, get share info with empty ID", c.params.sharePath())
	}

	c.nfsShareId = shareID
	return nil
}

func (c *Creator) rollbackNfsShare() {
	if c.nfsShareId == "" {
		return
	}

	if err := c.cli.DeleteNfsShare(c.ctx, c.nfsShareId, c.vstoreId); err != nil {
		log.AddContext(c.ctx).Errorf("Failed to rollback nfs share %s: %v", c.nfsShareId, err)
	}
}

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

func (c *Creator) createDataTurboShare() error {
	dtShare, err := c.cli.GetDataTurboShareByPath(c.ctx, c.params.sharePath(), c.vstoreId)
	if err != nil {
		return err
	}

	if len(dtShare) != 0 {
		shareID, ok := utils.GetValue[string](dtShare, "ID")
		if !ok {
			return fmt.Errorf("get DataTurbo share info %v with empty ID", dtShare)
		}

		log.AddContext(c.ctx).Warningf("Delete duplicate DataTurbo share, share id: %s", shareID)
		if err = c.cli.DeleteDataTurboShare(c.ctx, shareID, c.vstoreId); err != nil {
			return err
		}
	}

	newShare, err := c.cli.CreateDataTurboShare(c.ctx, &client.CreateDataTurboShareParams{
		SharePath:   c.params.sharePath(),
		FsId:        c.fsId,
		Description: c.params.Description,
		VstoreId:    c.vstoreId,
	})
	if err != nil {
		return err
	}

	if newShare == nil {
		return fmt.Errorf("failed to create DataTurbo share %s, get share info of nil", c.params.sharePath())
	}

	shareID, ok := utils.GetValue[string](newShare, "ID")
	if !ok {
		return fmt.Errorf("failed to create DataTurbo share %s, get share info with empty ID", c.params.sharePath())
	}

	c.dataTurboShareId = shareID
	return nil
}

func (c *Creator) rollbackDataTurboShare() {
	if c.dataTurboShareId == "" {
		return
	}

	if err := c.cli.DeleteDataTurboShare(c.ctx, c.dataTurboShareId, c.vstoreId); err != nil {
		log.AddContext(c.ctx).Errorf("Failed to rollback DataTurbo share %s: %v", c.dataTurboShareId, err)
	}
}

func (c *Creator) addAuthUser() error {
	for _, user := range c.params.AuthUsers {
		if err := c.cli.AddDataTurboShareUser(c.ctx, &client.AddDataTurboShareUserParams{
			UserName:   user,
			ShareId:    c.dataTurboShareId,
			Permission: readWriteAccessValue,
			VstoreId:   c.vstoreId,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (c *Creator) createQos() error {
	if c.params.Qos == "" {
		return nil
	}

	if c.qosId != "" {
		qos, err := c.cli.GetQosByID(c.ctx, c.qosId, c.vstoreId)
		if err != nil {
			return fmt.Errorf("get qos %s failed, err: %w", c.qosId, err)
		}

		if len(qos) != 0 {
			return nil
		}
	}

	qosParams, err := smartx.ExtractQoSParameters(c.ctx, c.params.Qos)
	if err != nil {
		return fmt.Errorf("extarct qos parameter %s failed, err: %w", c.params.Qos, err)
	}

	formatedQos, err := smartx.ConvertQoSParametersValueToInt(qosParams)
	if err != nil {
		return fmt.Errorf("convert qos value %v to int failed, err: %w", qosParams, err)
	}

	smartX := smartx.NewSmartX(c.cli)
	qosId, err := smartX.CreateQos(c.ctx, c.fsId, c.vstoreId, formatedQos)
	if err != nil {
		return fmt.Errorf("create qos %v failed, err: %w", formatedQos, err)
	}

	c.qosId = qosId
	return nil
}

func (c *Creator) revertQos() {
	if c.fsId == "" || c.qosId == "" {
		return
	}
	smartX := smartx.NewSmartX(c.cli)
	if err := smartX.DeleteQos(c.ctx, c.qosId, c.fsId, c.vstoreId); err != nil {
		log.AddContext(c.ctx).Errorf("Failed to rollback qos %s: %v", c.qosId, err)
	}
}
