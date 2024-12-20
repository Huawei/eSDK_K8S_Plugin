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
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

// SnapshotFsOptionFunc defines the function to change fields of SnapshotFsCreator
type SnapshotFsOptionFunc func(*SnapshotFsCreator)

var _ VolumeCreator = (*SnapshotFsCreator)(nil)

// SnapshotFsCreator provides the ability to create a snapshot file system.
type SnapshotFsCreator struct {
	*CloneFsCreator
}

// NewSnapshotFsFromParams returns an instance of NewSnapshotFsFromParams
func NewSnapshotFsFromParams(cli client.OceanstorClientInterface,
	params *Parameter, opts ...SnapshotFsOptionFunc) *SnapshotFsCreator {

	creator := &SnapshotFsCreator{
		CloneFsCreator: NewCloneFsCreatorByParams(cli,
			params,
			WithParentSnapshotId(params.SnapshotID()),
			WithIsDeleteParentSnapshot(false),
			WithCloneFrom(params.SnapshotParentName())),
	}

	return creator
}
