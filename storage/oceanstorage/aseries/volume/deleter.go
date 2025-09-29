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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// DeleteFilesystemModel is used to delete a filesystem volume
type DeleteFilesystemModel struct {
	Protocol string
	Name     string
}

func (model *DeleteFilesystemModel) sharePath() string {
	return "/" + model.Name + "/"
}

// Deleter is used to delete a filesystem volume
type Deleter struct {
	vstoreId string
	ctx      context.Context
	cli      client.OceanASeriesClientInterface
	params   *DeleteFilesystemModel
}

// NewDeleter inits a new filesystem volume deleter
func NewDeleter(ctx context.Context, cli client.OceanASeriesClientInterface, params *DeleteFilesystemModel) *Deleter {
	return &Deleter{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Delete deletes a filesystem resource from storage
func (c *Deleter) Delete() error {
	tr := flow.NewTransaction()
	tr.Then(c.prepareParams, nil)
	if c.params.Protocol == constants.ProtocolNfs {
		tr.Then(c.deleteNfsShare, nil)
	}

	if c.params.Protocol == constants.ProtocolDtfs {
		tr.Then(c.deleteDataTurboShare, nil)
	}

	tr.Then(c.deleteFilesystem, nil)

	err := tr.Commit()
	if err != nil {
		tr.Rollback()
		return err
	}

	return nil
}

func (c *Deleter) prepareParams() error {
	c.vstoreId = c.cli.GetvStoreID()
	return nil
}

func (c *Deleter) deleteNfsShare() error {
	nfsShare, err := c.cli.GetNfsShareByPath(c.ctx, c.params.sharePath(), c.vstoreId)
	if err != nil {
		return err
	}

	if len(nfsShare) == 0 {
		log.AddContext(c.ctx).Infof("NFS share %s has been deleted", c.params.sharePath())
		return nil
	}

	shareID, ok := utils.GetValue[string](nfsShare, "ID")
	if !ok {
		return fmt.Errorf("failed to delete NFS share %s, get share info with empty ID", c.params.sharePath())
	}

	return c.cli.DeleteNfsShare(c.ctx, shareID, c.vstoreId)
}

func (c *Deleter) deleteDataTurboShare() error {
	dtShare, err := c.cli.GetDataTurboShareByPath(c.ctx, c.params.sharePath(), c.vstoreId)
	if err != nil {
		return err
	}

	if len(dtShare) == 0 {
		log.AddContext(c.ctx).Infof("DataTurbo share %s has been deleted", c.params.sharePath())
		return nil
	}

	shareID, ok := utils.GetValue[string](dtShare, "ID")
	if !ok {
		return fmt.Errorf("failed to delete DataTurbo share %s, get share info with empty ID", c.params.sharePath())
	}

	return c.cli.DeleteDataTurboShare(c.ctx, shareID, c.vstoreId)
}

func (c *Deleter) deleteFilesystem() error {
	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.Name, c.vstoreId)
	if err != nil {
		return err
	}

	if len(fs) == 0 {
		log.AddContext(c.ctx).Infof("Filesystem %s has been deleted", c.params.Name)
		return nil
	}

	fsId, ok := utils.GetValue[string](fs, "ID")
	if !ok {
		return fmt.Errorf("failed to delete filesystem %s, get filesystem info with empty ID", c.params.Name)
	}

	qosId, ok := utils.GetValue[string](fs, "IOCLASSID")
	if ok && qosId != "" {
		smartX := smartx.NewSmartX(c.cli)
		err = smartX.DeleteQos(c.ctx, qosId, fsId, c.vstoreId)
		if err != nil {
			return fmt.Errorf("remove filesystem %s from qos %s failed, err: %w", fsId, qosId, err)
		}
	}

	deleteParams := map[string]interface{}{"ID": fsId}
	return c.cli.DeleteFileSystem(c.ctx, deleteParams)
}
