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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	// ErrVolumeTypeConflict is used for create a conflict volume.
	ErrVolumeTypeConflict = errors.New("cannot create replication and hypermetro for a volume at the same time")
	// ErrNotFoundCli is used for get empty client interface.
	ErrNotFoundCli = errors.New("not found client")
)

// StandbyVolumeCreator is the interface that defines methods for standby volume creator.
type StandbyVolumeCreator interface {
	SingleVolumeCreator
	setStandbyParameters(map[string]any)
}

// SingleVolumeCreator is the interface that defines methods for single volume creator.
type SingleVolumeCreator interface {
	VolumeCreator
	rollback(context.Context)
	getCreatedFilesystem() map[string]any
}

// VolumeCreator is the interface that wraps the CreateVolume method.
type VolumeCreator interface {
	CreateVolume(context.Context) (utils.Volume, error)
}

// NewFromParameters generates a volume creator instance from parameters.
func NewFromParameters(
	ctx context.Context,
	parameters map[string]any,
	activeCli client.OceanstorClientInterface,
	standbyCli client.OceanstorClientInterface,
) (VolumeCreator, error) {
	params := NewParameter(parameters)

	if err := validateParameters(ctx, params, activeCli, standbyCli); err != nil {
		return nil, err
	}

	if params.IsModifyVolume() {
		return NewModifyCreatorFromParams(activeCli, standbyCli, params), nil
	} else if params.IsHyperMetro() {
		return NewHyperMetroCreatorFromParams(activeCli, standbyCli, params), nil
	}

	return newSingle(params, activeCli), nil
}

func validateParameters(
	ctx context.Context,
	params *Parameter,
	activeCli client.OceanstorClientInterface,
	standbyCli client.OceanstorClientInterface,
) error {
	if activeCli == nil {
		log.AddContext(ctx).Errorln(ErrNotFoundCli)
		return ErrNotFoundCli
	}

	if params.IsHyperMetro() && standbyCli == nil {
		return ErrNotFoundCli
	}

	if params.IsHyperMetro() && params.IsReplication() {
		log.AddContext(ctx).Errorln(ErrVolumeTypeConflict)
		return ErrVolumeTypeConflict
	}

	return nil
}

func newSingle(params *Parameter, cli client.OceanstorClientInterface) SingleVolumeCreator {
	if params.IsClone() {
		return NewCloneFsCreatorByParams(cli, params)
	} else if params.IsSnapshot() {
		return NewSnapshotFsFromParams(cli, params)
	}

	return NewFsCreatorFromParams(cli, params)
}
