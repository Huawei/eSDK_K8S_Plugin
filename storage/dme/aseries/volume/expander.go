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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// ExpandVolumeModel is used to expand a filesystem volume
type ExpandVolumeModel struct {
	Name     string
	Capacity int64 // The unit is byte.
}

// Expander is used to expand a filesystem volume
type Expander struct {
	ctx    context.Context
	cli    client.DMEASeriesClientInterface
	params *ExpandVolumeModel
}

// NewExpander inits a new filesystem volume expander
func NewExpander(ctx context.Context, cli client.DMEASeriesClientInterface, params *ExpandVolumeModel) *Expander {
	return &Expander{
		ctx:    ctx,
		cli:    cli,
		params: params,
	}
}

// Expand expands a filesystem resource capacity on storage
func (e *Expander) Expand() error {
	return e.expandFilesystem()
}

func (e *Expander) expandFilesystem() error {
	fs, err := e.cli.GetFileSystemByName(e.ctx, e.params.Name)
	if err != nil {
		return fmt.Errorf("get filesystem by name: %s failed: %w", e.params.Name, err)
	}

	if fs == nil {
		return fmt.Errorf("filesystem %s does not exist", e.params.Name)
	}

	if e.params.Capacity == fs.TotalCapacityInByte {
		log.AddContext(e.ctx).Infof("the size of filesystem %s has not changed and the current size is %d",
			e.params.Name, fs.TotalCapacityInByte)
		return nil
	} else if e.params.Capacity < fs.TotalCapacityInByte {
		return fmt.Errorf("failed to expand filesystem %s, new size %d is less than current size %d",
			e.params.Name, e.params.Capacity, fs.TotalCapacityInByte)
	}

	pool, err := e.cli.GetHyperScalePoolByName(e.ctx, fs.StoragePoolName)
	if err != nil {
		return fmt.Errorf("get storage pool by name: %s failed: %w", fs.StoragePoolName, err)
	}

	if pool == nil {
		return fmt.Errorf("failed to expand filesystem: %s, pool: %s does not exist", e.params.Name,
			fs.StoragePoolName)
	}

	params := &client.UpdateFileSystemParams{Capacity: transDmeCapacityFromByteIoGb(e.params.Capacity)}
	return e.cli.UpdateFileSystem(e.ctx, fs.ID, params)
}
