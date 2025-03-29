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

package dtree

import (
	"context"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

type deleterModel struct {
	ParentName string
	DTreeName  string
}

func (model *deleterModel) sharePath() string {
	return "/" + model.ParentName + "/" + model.DTreeName
}

// Deleter is used to delete a dtree resource
type Deleter struct {
	ctx context.Context
	cli client.IRestClient

	params *deleterModel
}

// NewDeleter returns a new Deleter
func NewDeleter(ctx context.Context, cli client.IRestClient, parentName, dtreeName string) *Deleter {
	return &Deleter{
		ctx: ctx,
		cli: cli,
		params: &deleterModel{
			ParentName: parentName,
			DTreeName:  dtreeName,
		},
	}
}

// Delete deletes a dtree resource
func (d *Deleter) Delete() error {
	if err := d.deleteNfsShare(); err != nil {
		return err
	}

	if err := d.deleteDTree(); err != nil {
		return err
	}

	return nil
}

func (d *Deleter) deleteNfsShare() error {
	nfsShare, err := d.cli.GetDTreeNfsShareByPath(d.ctx, d.params.sharePath())
	if err != nil {
		return err
	}
	if nfsShare == nil {
		log.AddContext(d.ctx).Infof("The share %q has been deleted", d.params.sharePath())
		return nil
	}

	return d.cli.DeleteDTreeNfsShare(d.ctx, nfsShare.Id)
}

func (d *Deleter) deleteDTree() error {
	fs, err := d.cli.GetFileSystemByName(d.ctx, d.params.ParentName)
	if err != nil {
		return err
	}
	if fs == nil {
		log.AddContext(d.ctx).Infof("The parent of dtree %q has been deleted", d.params.ParentName)
		return nil
	}

	dtree, err := d.cli.GetDTreeByName(d.ctx, d.params.ParentName, d.params.DTreeName)
	if err != nil {
		return err
	}
	if dtree == nil {
		log.AddContext(d.ctx).Infof("The dtree %q of namespace %q has been deleted",
			d.params.DTreeName, d.params.ParentName)
		return nil
	}

	return d.cli.DeleteDTree(d.ctx, dtree.Id)
}
