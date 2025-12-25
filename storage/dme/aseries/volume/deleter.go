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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
)

// DeleteVolumeModel is used to delete a filesystem volume
type DeleteVolumeModel struct {
	Protocol string
	Name     string
}

func (model *DeleteVolumeModel) sharePath() string {
	return "/" + model.Name + "/"
}

// Deleter is used to delete a filesystem volume
type Deleter struct {
	ctx    context.Context
	cli    client.DMEASeriesClientInterface
	params *DeleteVolumeModel
}

// NewDeleter inits a new filesystem volume deleter
func NewDeleter(ctx context.Context, cli client.DMEASeriesClientInterface, params *DeleteVolumeModel) *Deleter {
	return &Deleter{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Delete deletes a filesystem resource from storage
func (c *Deleter) Delete() error {
	if err := c.deleteNfsShare(); err != nil {
		return err
	}

	if err := c.deleteDataTurboShare(); err != nil {
		return err
	}

	if err := c.deleteFilesystem(); err != nil {
		return err
	}

	return nil
}

func (c *Deleter) deleteNfsShare() error {
	nfsShare, err := c.cli.GetNfsShareByPath(c.ctx, c.params.sharePath())
	if err != nil {
		return err
	}
	if nfsShare == nil {
		return nil
	}
	return c.cli.DeleteNfsShare(c.ctx, nfsShare.ID)
}

func (c *Deleter) deleteDataTurboShare() error {
	dtShare, err := c.cli.GetDataTurboShareByPath(c.ctx, c.params.sharePath())
	if err != nil {
		return err
	}
	if dtShare == nil {
		return nil
	}
	return c.cli.DeleteDataTurboShare(c.ctx, dtShare.ID)
}

func (c *Deleter) deleteFilesystem() error {
	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.Name)
	if err != nil {
		return err
	}

	if fs == nil {
		return nil
	}
	return c.cli.DeleteFileSystem(c.ctx, fs.ID)
}
