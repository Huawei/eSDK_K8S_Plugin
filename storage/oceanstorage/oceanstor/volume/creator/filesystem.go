/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgutils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// FsOptionFunc defines the function to change fields of FilesystemCreator
type FsOptionFunc func(*FilesystemCreator)

var _ VolumeCreator = (*FilesystemCreator)(nil)

// FilesystemCreator is the filesystem creator that implement VolumeCreator interface.
type FilesystemCreator struct {
	*BaseCreator

	fileSystemMode  string
	unixPermissions string
	workloadTypeID  string
	advancedOptions string

	createdFilesystem map[string]any
	standbyRequest    map[string]any
}

// NewFsCreatorFromParams returns an instance of FilesystemCreator
func NewFsCreatorFromParams(cli client.OceanstorClientInterface,
	params *Parameter, opts ...FsOptionFunc) *FilesystemCreator {
	baseCreator := &BaseCreator{cli: cli}
	baseCreator.Init(params)

	creator := &FilesystemCreator{
		BaseCreator:       baseCreator,
		fileSystemMode:    storage.LocalFilesystemMode,
		unixPermissions:   params.FsPermission(),
		workloadTypeID:    params.WorkloadTypeID(),
		advancedOptions:   params.AdvancedOptions(),
		createdFilesystem: make(map[string]any),
		standbyRequest:    make(map[string]any),
	}

	if params.IsHyperMetro() {
		creator.fileSystemMode = storage.HyperMetroFilesystemMode
	} else if params.FilesystemMode() != "" {
		creator.fileSystemMode = params.FilesystemMode()
	}

	if creator.fileSystemMode == storage.HyperMetroFilesystemMode {
		creator.vStoreId = creator.cli.GetvStoreID()
	}

	for _, opt := range opts {
		opt(creator)
	}

	return creator
}

// CreateVolume creates a filesystem volume on the storage backend.
func (creator *FilesystemCreator) CreateVolume(ctx context.Context) (utils.Volume, error) {
	volume := utils.NewVolume(creator.fsName)

	fsId, err := creator.createResources(ctx)
	if err != nil {
		return nil, err
	}
	volume.SetID(fsId)
	volume.SetSize(utils.TransK8SCapacity(creator.capacity, constants.AllocationUnitBytes))

	return volume, nil
}

func (creator *FilesystemCreator) rollback(ctx context.Context) {
	creator.transaction.Rollback()
}

func (creator *FilesystemCreator) getCreatedFilesystem() map[string]any {
	return creator.createdFilesystem
}

var standbySyncFields = []string{"ENABLEDEDUP", "ENABLECOMPRESSION"}

func (creator *FilesystemCreator) setStandbyParameters(req map[string]any) {
	for _, field := range standbySyncFields {
		if value, ok := req[field]; ok {
			creator.standbyRequest[field] = value
		}
	}
}

func (creator *FilesystemCreator) createResources(ctx context.Context) (string, error) {
	var fsId string
	var err error

	creator.transaction.
		Then(func() error {
			fsId, err = creator.createFilesystem(ctx)
			return err
		}, func() {
			req := map[string]any{"ID": fsId}
			if creator.vStoreId != "" {
				req["vstoreId"] = creator.vStoreId
			}
			if err := creator.cli.DeleteFileSystem(ctx, req); err != nil {
				log.AddContext(ctx).Errorf("delete filesystem %s error: %v", creator.fsName, err)
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

func (creator *FilesystemCreator) createFilesystem(ctx context.Context) (string, error) {
	poolId, err := creator.GetPoolID(ctx, creator.storagePoolName)
	if err != nil {
		return "", fmt.Errorf("create volume %s error: %w", creator.fsName, err)
	}
	fs, err := creator.cli.GetFileSystemByName(ctx, creator.fsName)
	if err != nil {
		return "", fmt.Errorf("create volume %s error: %w", creator.fsName, err)
	}

	if fs != nil {
		return utils.GetValueOrFallback(fs, "ID", ""), nil
	}

	req, err := creator.genCreateRequest(ctx, poolId)
	if err != nil {
		return "", err
	}

	fs, err = creator.cli.CreateFileSystem(ctx, req)

	if err != nil {
		return "", fmt.Errorf("create filesystem %s error: %w", creator.fsName, err)
	}

	creator.createdFilesystem = fs

	return utils.GetValueOrFallback(fs, "ID", ""), nil
}

func (creator *FilesystemCreator) genCreateRequest(ctx context.Context, poolId string) (map[string]any, error) {
	req := map[string]any{
		"NAME":           creator.fsName,
		"PARENTID":       poolId,
		"CAPACITY":       creator.capacity,
		"DESCRIPTION":    creator.description,
		"ALLOCTYPE":      creator.allocType,
		"fileSystemMode": creator.fileSystemMode,
	}

	if len(creator.standbyRequest) != 0 {
		for k, v := range creator.standbyRequest {
			req[k] = v
		}
	}

	if creator.fileSystemMode == storage.HyperMetroFilesystemMode {
		req["vstoreId"] = creator.vStoreId
	}

	if creator.unixPermissions != "" {
		req["unixPermissions"] = creator.unixPermissions
	}

	if creator.isShowSnapDir != nil {
		req["ISSHOWSNAPDIR"] = *creator.isShowSnapDir
	}

	if creator.snapshotReservePer != nil {
		req["SNAPSHOTRESERVEPER"] = *creator.snapshotReservePer
	}

	if creator.workloadTypeID != "" {
		id, err := strconv.ParseUint(creator.workloadTypeID, 0, 32)
		if err != nil {
			log.AddContext(ctx).Errorf("cannot convert workloadtype to int32: %v", err)
			return nil, err
		}

		req["workloadTypeId"] = uint32(id)
	}

	if creator.advancedOptions != "" {
		advancedOptions := make(map[string]any)
		err := json.Unmarshal([]byte(creator.advancedOptions), &advancedOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal advancedOptions parameters[%s] error: %v",
				creator.advancedOptions, err)
		}
		req = pkgutils.CombineMap(advancedOptions, req)
	}

	return req, nil
}
