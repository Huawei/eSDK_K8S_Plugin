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
	"fmt"
	"strconv"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	capacityBitSize = 64
	capacityBase    = 10

	waitSplitTimeout  = 6 * time.Hour
	waitSplitInterval = 5 * time.Second

	filesystemHealthStatusNormal   = "1"
	filesystemSplitStatusNotStart  = "1"
	filesystemSplitStatusSplitting = "2"
	filesystemSplitStatusQueuing   = "3"
	filesystemSplitStatusAbnormal  = "4"

	systemVStore = "0"
)

// CloneFsOptionFunc defines the function to change fields of CloneFsCreator
type CloneFsOptionFunc func(*CloneFsCreator)

var _ VolumeCreator = (*CloneFsCreator)(nil)

// CloneFsCreator provides the ability to create a clone file system.
type CloneFsCreator struct {
	*BaseCreator

	cloneFrom              string
	cloneSpeed             int
	parentSnapshotId       string
	isDeleteParentSnapshot bool

	createdFilesystem map[string]any
}

// WithParentSnapshotId sets parentSnapshotId field of CloneFsCreator
func WithParentSnapshotId(snapshotId string) CloneFsOptionFunc {
	return func(creator *CloneFsCreator) {
		creator.parentSnapshotId = snapshotId
	}
}

// WithIsDeleteParentSnapshot sets isDeleteParentSnapshot field of CloneFsCreator
func WithIsDeleteParentSnapshot(isDeleteParentSnapshot bool) CloneFsOptionFunc {
	return func(creator *CloneFsCreator) {
		creator.isDeleteParentSnapshot = isDeleteParentSnapshot
	}
}

// WithCloneFrom sets cloneFrom field of CloneFsCreator
func WithCloneFrom(cloneFrom string) CloneFsOptionFunc {
	return func(creator *CloneFsCreator) {
		creator.cloneFrom = cloneFrom
	}
}

// NewCloneFsCreatorByParams returns an instance of CloneFsCreator
func NewCloneFsCreatorByParams(cli client.OceanstorClientInterface,
	params *Parameter, opts ...CloneFsOptionFunc) *CloneFsCreator {
	base := &BaseCreator{cli: cli}
	base.Init(params)

	creator := &CloneFsCreator{
		BaseCreator:            base,
		cloneFrom:              params.CloneFrom(),
		cloneSpeed:             params.CloneSpeed(),
		parentSnapshotId:       params.SnapshotParentId(),
		isDeleteParentSnapshot: true,
	}

	for _, opt := range opts {
		opt(creator)
	}

	return creator
}

// CreateVolume creates a clone filesystem volume on the storage backend.
func (creator *CloneFsCreator) CreateVolume(ctx context.Context) (utils.Volume, error) {
	volume := utils.NewVolume(creator.fsName)
	fs, err := creator.cli.GetFileSystemByName(ctx, creator.fsName)
	if err != nil {
		return nil, fmt.Errorf("get filesystem %s error: %v", creator.fsName, err)
	}

	if fs != nil {
		if err := creator.waitFsSplit(ctx, utils.GetValueOrFallback(fs, "ID", "")); err != nil {
			return nil, err
		}

		volume.SetID(utils.GetValueOrFallback(fs, "ID", ""))
		return volume, nil
	}

	fsId, err := creator.createResources(ctx)
	if err != nil {
		return nil, err
	}

	volume.SetID(fsId)
	volume.SetSize(utils.TransK8SCapacity(creator.capacity, constants.AllocationUnitBytes))

	return volume, nil
}

func (creator *CloneFsCreator) rollback(ctx context.Context) {
	creator.transaction.Rollback()
}

func (creator *CloneFsCreator) getCreatedFilesystem() map[string]any {
	return creator.createdFilesystem
}

func (creator *CloneFsCreator) createResources(ctx context.Context) (string, error) {
	var fsId string
	var err error
	creator.transaction.
		Then(
			func() error {
				fsId, err = creator.clone(ctx)
				return err
			}, func() {
				req := map[string]interface{}{"ID": fsId, "vstoreId": creator.vStoreId}
				if err := creator.cli.DeleteFileSystem(ctx, req); err != nil {
					log.AddContext(ctx).Errorf("Delete filesystem [%s] error: %v", fsId, err)
				}
			})

	creator.addNfsShareTransactionStep(ctx, &fsId, creator.fsName, creator.description, creator.vStoreId)

	creator.addQoSTransactionStep(ctx, &fsId, creator.vStoreId)

	if err := creator.transaction.Commit(); err != nil {
		creator.transaction.Rollback()
		return "", err
	}

	return fsId, nil
}

type cloneFilesystemRequest struct {
	fsName               string
	vStoreId             string
	parentID             string
	parentSnapshotID     string
	allocType            int
	cloneSpeed           int
	cloneFsCapacity      int64
	srcCapacity          int64
	deleteParentSnapshot bool
}

func (creator *CloneFsCreator) clone(ctx context.Context) (string, error) {
	cloneFromFS, err := creator.cli.GetFileSystemByName(ctx, creator.cloneFrom)
	if err != nil {
		return "", fmt.Errorf("get clone src filesystem %s error: %v", creator.cloneFrom, err)
	}
	if cloneFromFS == nil {
		return "", fmt.Errorf("filesystem %s which clone from does not exist", creator.cloneFrom)
	}

	srcFsCapacity, err := strconv.ParseInt(
		utils.GetValueOrFallback(cloneFromFS, "CAPACITY", ""),
		capacityBase,
		capacityBitSize,
	)
	if err != nil {
		return "", err
	}

	if creator.capacity < srcFsCapacity {
		return "", fmt.Errorf("clone filesystem capacity %d must be >= source filesystem %s capacity %d",
			creator.capacity, creator.cloneFrom, srcFsCapacity)
	}

	cloneFilesystemReq := &cloneFilesystemRequest{
		fsName:               creator.fsName,
		parentID:             utils.GetValueOrFallback(cloneFromFS, "ID", ""),
		parentSnapshotID:     creator.parentSnapshotId,
		allocType:            creator.allocType,
		cloneSpeed:           creator.cloneSpeed,
		cloneFsCapacity:      creator.capacity,
		srcCapacity:          srcFsCapacity,
		deleteParentSnapshot: creator.isDeleteParentSnapshot,
		vStoreId:             systemVStore,
	}
	cloneFS, err := creator.cloneFilesystem(ctx, cloneFilesystemReq)
	if err != nil {
		log.AddContext(ctx).Errorf("Clone filesystem %s from source filesystem %s error: %s",
			cloneFilesystemReq.fsName, cloneFilesystemReq.parentID, err)
		return "", err
	}

	if err := creator.updateFilesystem(ctx, cloneFS); err != nil {
		log.AddContext(ctx).Errorf("Update filesystem %s error: %v", creator.fsName, err)
		return "", err
	}

	return cloneFS, err
}

func (creator *CloneFsCreator) cloneFilesystem(ctx context.Context, req *cloneFilesystemRequest) (string, error) {
	cloneFS, err := creator.cli.CloneFileSystem(ctx, req.fsName, req.allocType, req.parentID, req.parentSnapshotID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create cloneFilesystem failed. source filesystem ID [%s], error: [%v]",
			req.parentID, err)
		return "", err
	}
	cloneFSID := utils.GetValueOrFallback(cloneFS, "ID", "")
	if req.cloneFsCapacity > req.srcCapacity {
		err := creator.cli.ExtendFileSystem(ctx, cloneFSID, req.cloneFsCapacity)
		if err != nil {
			log.AddContext(ctx).Errorf("Extend filesystem %s to capacity %d error: %v",
				cloneFSID, req.cloneFsCapacity, err)
			_ = creator.cli.DeleteFileSystem(ctx, map[string]interface{}{"ID": cloneFSID})
			return "", err
		}
	}
	req.vStoreId = utils.GetValueOrFallback(cloneFS, "vstoreId", systemVStore)

	err = creator.splitClone(ctx, cloneFSID, req)
	if err != nil {
		log.AddContext(ctx).Errorf("split clone failed. err: %v", err)
	}

	creator.createdFilesystem = cloneFS

	return cloneFSID, nil
}

func (creator *CloneFsCreator) splitClone(ctx context.Context, cloneFSID string, req *cloneFilesystemRequest) error {
	err := creator.cli.SplitCloneFS(ctx, cloneFSID, req.vStoreId, req.cloneSpeed, req.deleteParentSnapshot)
	if err != nil {
		log.AddContext(ctx).Errorf("Split filesystem [%s] error: %v", req.fsName, err)
		delErr := creator.cli.DeleteFileSystem(ctx, map[string]interface{}{"ID": cloneFSID})
		if delErr != nil {
			log.AddContext(ctx).Errorf("Delete filesystem [%s] error: %v", cloneFSID, err)
		}
		return err
	}

	return creator.waitFsSplit(ctx, cloneFSID)
}

func (creator *CloneFsCreator) waitFsSplit(ctx context.Context, fsID string) error {
	return utils.WaitUntil(func() (bool, error) {
		fs, err := creator.cli.GetFileSystemByID(ctx, fsID)
		if err != nil {
			return false, err
		}

		if fs["ISCLONEFS"] == "false" {
			return true, nil
		}

		if fs["HEALTHSTATUS"].(string) != filesystemHealthStatusNormal {
			return false, fmt.Errorf("filesystem %s has the bad healthStatus code %s", fs["NAME"], fs["HEALTHSTATUS"].(string))
		}

		splitStatus, ok := fs["SPLITSTATUS"].(string)
		if !ok {
			return false, pkgUtils.Errorf(ctx, "convert splitStatus to string failed, data: %v", fs["SPLITSTATUS"])
		}
		if splitStatus == filesystemSplitStatusQueuing ||
			splitStatus == filesystemSplitStatusSplitting ||
			splitStatus == filesystemSplitStatusNotStart {
			return false, nil
		} else if splitStatus == filesystemSplitStatusAbnormal {
			return false, fmt.Errorf("filesystem clone [%s] split status is interrupted, SPLITSTATUS: [%s]",
				fs["NAME"], splitStatus)
		} else {
			return true, nil
		}
	}, waitSplitTimeout, waitSplitInterval)
}

func (creator *CloneFsCreator) updateFilesystem(ctx context.Context, fsId string) error {
	log.AddContext(ctx).Infof("The fileSystem %s is cloned, now to update some fields.", creator.fsName)

	req := map[string]any{
		"DESCRIPTION": creator.description,
	}

	if creator.isShowSnapDir != nil {
		req["ISSHOWSNAPDIR"] = *creator.isShowSnapDir
	}

	if creator.snapshotReservePer != nil {
		req["SNAPSHOTRESERVEPER"] = *creator.snapshotReservePer
	}

	return creator.cli.UpdateFileSystem(ctx, fsId, req)
}
