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
	"errors"
	"fmt"

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

var _ VolumeCreator = (*ModifyFsCreator)(nil)

// ModifyFsOptionFunc defines the function to change fields of ModifyFsCreator
type ModifyFsOptionFunc func(creator *ModifyFsCreator)

// NewModifyCreatorFromParams returns an instance of ModifyFsCreator
func NewModifyCreatorFromParams(
	activeCli client.BaseClientInterface,
	standbyCli client.BaseClientInterface,
	params *Parameter,
	opts ...ModifyFsOptionFunc,
) *ModifyFsCreator {
	base := &BaseCreator{cli: activeCli}
	base.Init(params)
	creator := &ModifyFsCreator{
		BaseCreator: base,
		activeCli:   activeCli,
		standbyCli:  standbyCli,
		params:      params,
	}
	creator.hyperMetro = params.IsHyperMetro()

	for _, opt := range opts {
		opt(creator)
	}

	return creator
}

// ModifyFsCreator is the filesystem creator that implement VolumeCreator interface.
type ModifyFsCreator struct {
	*BaseCreator
	activeCli  client.BaseClientInterface
	standbyCli client.BaseClientInterface
	params     *Parameter

	hyperMetro bool
}

// CreateVolume creates a hyper metro filesystem volume on the storage backend.
func (creator *ModifyFsCreator) CreateVolume(ctx context.Context) (utils.Volume, error) {
	log.AddContext(ctx).Infof("begin to modify filesystem volume: %s", creator.fsName)
	// query filesystem on active site, an error will be returned if not exists
	activeFs, err := creator.activeCli.GetFileSystemByName(ctx, creator.fsName)
	if err != nil {
		return nil, fmt.Errorf("query filesystem error during modify volume: %s, err:%w", creator.fsName, err)
	}
	if activeFs == nil || len(activeFs) == 0 {
		return nil, fmt.Errorf("file system not exists on active site during modify volume: %s", creator.fsName)
	}

	volume := utils.NewVolume(creator.fsName)

	if creator.hyperMetro {
		// modify local filesystem to hyper metro filesystem
		pairId, err := creator.getPairIdByFsId(ctx, activeFs)
		if err != nil {
			return nil, fmt.Errorf("failed to get pair id of active hypermetro pair, error: %v", err)
		}
		if pairId != "" {
			log.AddContext(ctx).Infof("hypermetro pair already exists, pair id: %s", pairId)
			return volume, nil
		}

		standbyCreator, err := creator.newStandbyCreatorForModify(ctx, activeFs, err)
		if err != nil {
			return nil, err
		}

		var standbyFsId string
		creator.transaction.Then(func() error {
			standbyFsId, err = standbyCreator.createFilesystem(ctx)
			return err
		}, func() {
			req := map[string]any{"ID": standbyFsId}
			if creator.vStoreId != "" {
				req["vstoreId"] = creator.vStoreId
			}
			if err := standbyCreator.cli.SafeDeleteFileSystem(ctx, req); err != nil {
				log.AddContext(ctx).Errorf("delete filesystem %s error: %v", creator.fsName, err)
			}
		}).Then(func() error {
			pairId, err = creator.createHyperMetroPair(ctx, getValueOrFallback(activeFs, "ID", ""), standbyFsId)
			return err
		}, func() {
			if err := creator.rollbackHyperMetroPair(ctx, pairId); err != nil {
				log.AddContext(ctx).Errorf("failed to rollback hypermetro pair %s, error: %v", pairId, err)
			}
		})
	}

	err = creator.transaction.Commit()
	if err != nil {
		creator.rollback(ctx)
		return nil, err
	}

	return volume, nil
}

func (creator *ModifyFsCreator) rollback(ctx context.Context) {
	creator.transaction.Rollback()
}

func (creator *ModifyFsCreator) newStandbyCreatorForModify(ctx context.Context,
	activeFs map[string]interface{}, err error) (*FilesystemCreator, error) {
	poolName := getValueOrFallback(activeFs, "PARENTNAME", "")
	if poolName == "" {
		return nil, errors.New("pool name cannot be empty")
	}

	// capacity
	capacity, err := utils.TransToInt(activeFs["CAPACITY"])
	if err != nil {
		errMsg := fmt.Sprintf("Convert filesystem capacity failed, CAPACITY: %v", activeFs["CAPACITY"])
		log.AddContext(ctx).Errorln(errMsg)
		return nil, err
	}
	log.AddContext(ctx).Infof("capacity: %d", capacity)

	standbyCreator := NewFsCreatorFromParams(creator.standbyCli, creator.params)
	standbyCreator.storagePoolName = creator.params.RemoteStoragePool()
	standbyCreator.description = getValueOrFallback(activeFs, "DESCRIPTION", "")
	standbyCreator.capacity = int64(capacity)
	return standbyCreator, nil
}
