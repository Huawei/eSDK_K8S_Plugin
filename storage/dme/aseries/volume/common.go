/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

package volume

import (
	"context"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
)

// ModeHandler abstracts the mode-specific operations that differ between
// local (zone-scoped KVCache) and global (NFS/DTFS filesystem) modes.
type ModeHandler interface {
	// GetPool resolves a pool name to a PoolRef.
	// Local: zone-scoped GetStoragePoolByName; Global: GetHyperScalePoolByName.
	GetPool(ctx context.Context, poolName string) (client.PoolRef, error)
	// Delete removes the volume from storage.
	// Local: KVCache one-stop delete; Global: NFS/DTFS share + filesystem delete.
	Delete(ctx context.Context) error
}

// LocalVolumeHandler handles operations for local (zone-scoped) mode with KVCache.
type LocalVolumeHandler struct {
	Cli            client.DMEASeriesClientInterface
	KvCacheStoreId string
}

// GetPool resolves a pool name to a zone-scoped StoragePool.
func (h *LocalVolumeHandler) GetPool(ctx context.Context, poolName string) (client.PoolRef, error) {
	pool, err := h.Cli.GetStoragePoolByName(ctx, poolName, h.Cli.GetZoneID())
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, fmt.Errorf("pool %s does not exist", poolName)
	}
	return pool, nil
}

// Delete removes the KVCache store from storage.
func (h *LocalVolumeHandler) Delete(ctx context.Context) error {
	queryParams := &client.QueryKVCacheParams{ID: h.KvCacheStoreId}
	existingKV, err := h.Cli.QueryKVCache(ctx, queryParams)
	if err != nil {
		return err
	}
	if existingKV == nil {
		return nil
	}
	return h.Cli.DeleteKVCache(ctx, h.KvCacheStoreId)
}

// GlobalVolumeHandler handles operations for global mode with NFS/DTFS filesystem.
type GlobalVolumeHandler struct {
	Cli      client.DMEASeriesClientInterface
	Name     string
	Protocol string
}

// GetPool resolves a pool name to a HyperScalePool.
func (h *GlobalVolumeHandler) GetPool(ctx context.Context, poolName string) (client.PoolRef, error) {
	pool, err := h.Cli.GetHyperScalePoolByName(ctx, poolName)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, fmt.Errorf("pool %s does not exist", poolName)
	}
	return pool, nil
}

// Delete removes the NFS/DTFS shares and filesystem from storage.
func (h *GlobalVolumeHandler) Delete(ctx context.Context) error {
	sharePath := "/" + h.Name + "/"
	nfsShare, err := h.Cli.GetNfsShareByPath(ctx, sharePath)
	if err != nil {
		return err
	}
	if nfsShare != nil {
		if err := h.Cli.DeleteNfsShare(ctx, nfsShare.ID); err != nil {
			return err
		}
	}
	dtShare, err := h.Cli.GetDataTurboShareByPath(ctx, sharePath)
	if err != nil {
		return err
	}
	if dtShare != nil {
		if err := h.Cli.DeleteDataTurboShare(ctx, dtShare.ID); err != nil {
			return err
		}
	}
	fs, err := h.Cli.GetFileSystemByName(ctx, h.Name)
	if err != nil {
		return err
	}
	if fs == nil {
		return nil
	}
	return h.Cli.DeleteFileSystem(ctx, fs.ID)
}
