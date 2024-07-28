/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

package creator

import (
	"context"

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
)

// SnapshotFsOptionFunc defines the function to change fields of SnapshotFsCreator
type SnapshotFsOptionFunc func(*SnapshotFsCreator)

var _ VolumeCreator = (*SnapshotFsCreator)(nil)

// SnapshotFsCreator provides the ability to create a snapshot file system.
type SnapshotFsCreator struct {
	*BaseCreator
	cloneCreator *CloneFsCreator
	params       *Parameter

	snapshotID         string
	snapshotName       string
	snapshotParentID   string
	snapshotParentName string
	cloneSpeed         int
}

// NewSnapshotFsFromParams returns an instance of NewSnapshotFsFromParams
func NewSnapshotFsFromParams(cli client.BaseClientInterface,
	params *Parameter, opts ...SnapshotFsOptionFunc) *SnapshotFsCreator {
	base := &BaseCreator{cli: cli}
	base.Init(params)

	creator := &SnapshotFsCreator{
		BaseCreator:        base,
		params:             params,
		snapshotID:         params.SnapshotID(),
		snapshotName:       utils.GetFSSnapshotName(params.SourceSnapshotName()),
		snapshotParentID:   params.SnapshotParentId(),
		snapshotParentName: params.SnapshotParentName(),
		cloneSpeed:         params.CloneSpeed(),
	}

	for _, opt := range opts {
		opt(creator)
	}

	return creator
}

// CreateVolume creates a snapshot filesystem volume on the storage backend.
func (creator *SnapshotFsCreator) CreateVolume(ctx context.Context) (utils.Volume, error) {
	creator.cloneCreator = NewCloneFsCreatorByParams(creator.cli,
		creator.params,
		WithParentSnapshotId(creator.snapshotID),
		WithIsDeleteParentSnapshot(false),
		WithCloneFrom(creator.snapshotParentName))

	creator.cloneCreator.BaseCreator = creator.BaseCreator
	return creator.cloneCreator.CreateVolume(ctx)
}

func (creator *SnapshotFsCreator) rollback(ctx context.Context) {
	creator.cloneCreator.rollback(ctx)
}
