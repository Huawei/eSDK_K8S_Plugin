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

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	filesystemHCRESourceType = 2

	// hyper metro pair running status
	hyperMetroPairRunningStatusNormal  = "1"
	hyperMetroPairRunningStatusSyncing = "23"
	hyperMetroPairRunningStatusToSync  = "100"
)

var (
	pairRationalStatus = []string{
		hyperMetroPairRunningStatusNormal,
		hyperMetroPairRunningStatusToSync,
		hyperMetroPairRunningStatusSyncing,
	}
)

var _ VolumeCreator = (*HyperMetroFsCreator)(nil)

// HyperMetroFsOptionFunc defines the function to change fields of ModifyFsCreator
type HyperMetroFsOptionFunc func(creator *HyperMetroFsCreator)

// HyperMetroFsRequiredOptions defines the required options of ModifyFsCreator
type HyperMetroFsRequiredOptions struct {
	activeCli      client.BaseClientInterface
	standbyCli     client.BaseClientInterface
	activeCreator  VolumeCreator
	standbyCreator VolumeCreator

	// Name is the name of clone filesystem
	Name string
	// StoragePoolName is the name of storage pool that the filesystem belong to
	StoragePoolName string
	// StoragePoolId is the id of storage pool that the filesystem belong to
	StoragePoolId string
	// Description is the description of the filesystem
	Description string
	// Capacity is the capacity of the filesystem
	Capacity int64
	// AllocType is the allocation type of the filesystem
	AllocType int
}

// HyperMetroFsCreator is the filesystem creator that implement VolumeCreator interface.
type HyperMetroFsCreator struct {
	*BaseCreator
	active  VolumeCreator
	standby VolumeCreator
}

// NewHyperMetroCreatorFromParams returns an instance of HyperMetroFsCreator
func NewHyperMetroCreatorFromParams(
	activeCli client.BaseClientInterface,
	standbyCli client.BaseClientInterface,
	params *Parameter,
	opts ...HyperMetroFsOptionFunc,
) *HyperMetroFsCreator {
	// the nfs share and qos of HyperMetro filesystem must be created after creating hyper metro pair,
	// so, it'll skip nfs share and qos creation when create the filesystem on active and standby storage.
	params.SetIsSkipNfsShare(true)
	qos := params.QoS()
	params.SetQos(nil)
	activeCreator := newSingle(params, activeCli)
	standbyCreator := NewFsCreatorFromParams(standbyCli, params)
	standbyCreator.storagePoolName = params.RemoteStoragePool()
	standbyCreator.storagePoolId = params.RemotePoolId()
	// after creating hyper metro pair, enable the nfs and qos share creation.
	params.SetIsSkipNfsShare(false)
	params.SetQos(qos)

	base := &BaseCreator{cli: activeCli}
	base.Init(params)
	creator := &HyperMetroFsCreator{
		BaseCreator: base,
		active:      activeCreator,
		standby:     standbyCreator,
	}

	for _, opt := range opts {
		opt(creator)
	}

	return creator
}

// CreateVolume creates a hyper metro filesystem volume on the storage backend.
func (creator *HyperMetroFsCreator) CreateVolume(ctx context.Context) (utils.Volume, error) {
	var activeFs utils.Volume
	var activeFsId string
	var standbyFs utils.Volume
	creator.transaction.Then(func() error {
		var err error
		activeFs, err = creator.active.CreateVolume(ctx)
		if err != nil {
			return err
		}
		activeFsId = activeFs.GetID()
		return nil
	}, func() {
		creator.active.rollback(ctx)
	})
	creator.transaction.Then(func() error {
		var err error
		standbyFs, err = creator.standby.CreateVolume(ctx)
		if err != nil {
			return err
		}
		if activeFs.GetVolumeName() != standbyFs.GetVolumeName() {
			return fmt.Errorf("the volume of active end and that of the standby end not match")
		}
		return nil
	}, func() {
		creator.standby.rollback(ctx)
	})

	var pairId string
	creator.transaction.Then(func() error {
		var err error
		pairId, err = creator.createHyperMetroPair(ctx, activeFs.GetID(), standbyFs.GetID())
		return err
	}, func() {
		if err := creator.rollbackHyperMetroPair(ctx, pairId); err != nil {
			log.AddContext(ctx).Errorf("failed to rollback hypermetro pair %s, error: %v", pairId, err)
		}
	})
	creator.addNfsShareTransactionStep(ctx, &activeFsId, creator.fsName, creator.description, creator.vStoreId)
	creator.addQoSTransactionStep(ctx, &activeFsId, creator.vStoreId)
	err := creator.transaction.Commit()
	if err != nil {
		creator.rollback(ctx)
		return nil, err
	}

	volume := utils.NewVolume(activeFs.GetVolumeName())
	return volume, nil
}

func (creator *HyperMetroFsCreator) rollback(ctx context.Context) {
	creator.transaction.Rollback()
}
