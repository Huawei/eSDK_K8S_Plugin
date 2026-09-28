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
	"errors"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/dme/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	quotaControlCode      = "429"
	nfsShareReadWrite     = "read/write"
	nfsShareWriteModeSync = "synchronization"
	dpcShareReadWrite     = "read_and_write"
	sectorSize            = 512
)

var (
	errFilesystemNotFound = errors.New("filesystem not found")
	errNfsShareNotFound   = errors.New("nfs share not found")
	errDtfsShareNotFound  = errors.New("dtfs share not found")

	allSquashMap = map[int]string{
		constants.AllSquashValue:   constants.AllSquash,
		constants.NoAllSquashValue: constants.NoAllSquash,
	}
	rootSquashMap = map[int]string{
		constants.RootSquashValue:   constants.RootSquash,
		constants.NoRootSquashValue: constants.NoRootSquash,
	}
)

// Creator is used to create a filesystem volume
type Creator struct {
	fsId           string
	pool           client.PoolRef
	kvcacheStoreId string
	vstoreID       string
	ctx            context.Context
	cli            client.DMEASeriesClientInterface
	params         *CreateVolumeModel
	handler        ModeHandler
}

// CreateVolumeModel is used to create a volume
type CreateVolumeModel struct {
	SnapshotDirVisible bool
	Protocol           string
	Name               string
	PoolName           string
	Capacity           int64 // The unit is byte.
	Description        string
	AllSquash          int
	RootSquash         int
	AllocationType     string
	AuthClients        []string
	AuthUsers          []string
	EnableKVCache      bool
	EnableTimeAwareGC  bool
	GCTimeThreshold    int64
	VstoreName         string // zoneVstoreName from SC
}

func (model *CreateVolumeModel) sharePath() string {
	return "/" + model.Name + "/"
}

// NewCreator inits a new filesystem volume creator
func NewCreator(ctx context.Context, cli client.DMEASeriesClientInterface, params *CreateVolumeModel,
	handler ModeHandler) *Creator {
	return &Creator{
		ctx:     ctx,
		cli:     cli,
		params:  params,
		handler: handler,
	}
}

// Create creates a filesystem resource and returns a volume object
func (c *Creator) Create() (utils.Volume, error) {
	tr := flow.NewTransaction()

	if c.params.EnableKVCache {
		tr.Then(c.validateAndPrepareParams, nil)
		tr.Then(nil, c.cleanupKVCacheFilesystem)
		tr.Then(c.createKVCache, c.rollbackKVCache)
	} else {
		tr.Then(c.validateAndPrepareParams, nil)
		tr.Then(c.createFilesystem, c.rollBackendFilesystem)
	}

	err := tr.Commit()
	if err != nil {
		// If err is quota control err, return the error directly without rollback.
		var errResp client.AuthError
		if errors.As(err, &errResp) && errResp.Code == quotaControlCode {
			return nil, fmt.Errorf("create filesystem failed with restful quota control: %w", err)
		}

		tr.Rollback()
		return nil, err
	}

	vol := utils.NewVolume(c.params.Name)
	vol.SetSize(c.params.Capacity)
	if c.fsId != "" {
		vol.SetID(c.fsId)
	}
	if c.kvcacheStoreId != "" {
		vol.SetKvCacheStoreId(c.kvcacheStoreId)
	}

	return vol, nil
}

func (c *Creator) validateAndPrepareParams() error {
	if c.params.Protocol == constants.ProtocolNfs && len(c.params.AuthClients) == 0 && !c.params.EnableKVCache {
		return fmt.Errorf("authClient parameter must be provided in StorageClass for nfs protocol")
	}

	if c.params.Protocol == constants.ProtocolDtfs && len(c.params.AuthUsers) == 0 {
		return fmt.Errorf("authUser parameter must be provided in StorageClass for dtfs protocol")
	}

	if err := c.resolveVstoreID(); err != nil {
		return err
	}

	return c.setPool()
}

func (c *Creator) resolveVstoreID() error {
	if c.params.VstoreName == "" {
		return nil
	}
	vstores, err := c.cli.QueryVstores(c.ctx, &client.VstoreQueryParams{
		Name:      c.params.VstoreName,
		StorageID: c.cli.GetStorageID(),
		ZoneID:    c.cli.GetZoneID(),
	})
	if err != nil {
		return fmt.Errorf("query vstore by name %s failed: %w", c.params.VstoreName, err)
	}
	if len(vstores) == 0 {
		return fmt.Errorf("vstore %s not found", c.params.VstoreName)
	}
	c.vstoreID = vstores[0].ID
	return nil
}

func (c *Creator) setPool() error {
	pool, err := c.handler.GetPool(c.ctx, c.params.PoolName)
	if err != nil {
		return err
	}
	c.pool = pool
	return nil
}

func (c *Creator) getCreateNfsShareParam() *client.CreateNfsShareParam {
	nfsShareParam := &client.CreateNfsShareParam{
		StorageId:   c.cli.GetStorageID(),
		SharePath:   c.params.sharePath(),
		Description: c.params.Description,
	}

	for _, authClient := range c.params.AuthClients {
		nfsShareParam.NfsClientAddition = append(nfsShareParam.NfsClientAddition, &client.NfsClientAddition{
			Name:                     authClient,
			Permission:               nfsShareReadWrite,
			WriteMode:                nfsShareWriteModeSync,
			PermissionConstraint:     allSquashMap[c.params.AllSquash],
			RootPermissionConstraint: rootSquashMap[c.params.RootSquash],
		})
	}
	return nfsShareParam
}

func (c *Creator) getCreateDpcShareParam() (*client.CreateDpcShareParam, error) {
	dtShareParam := &client.CreateDpcShareParam{
		Charset:     storage.CharsetUtf8,
		Description: c.params.Description,
	}

	for _, username := range c.params.AuthUsers {
		adminInfo, err := c.cli.GetDataTurboUserByName(c.ctx, username)
		if err != nil {
			return nil, err
		}
		if adminInfo == nil {
			continue
		}
		dtShareParam.DpcAuth = append(dtShareParam.DpcAuth, &client.DpcAuth{
			DpcUserID:  adminInfo.ID,
			Permission: dpcShareReadWrite,
		})
	}
	if len(c.params.AuthUsers) != 0 && len(dtShareParam.DpcAuth) == 0 {
		return nil, errors.New("create filesystem failed: no valid users found in AuthUsers")
	}
	return dtShareParam, nil
}

func (c *Creator) deleteNfsShare() error {
	nfsShare, err := c.cli.GetNfsShareByPath(c.ctx, c.params.sharePath())
	if err != nil {
		return err
	}
	if nfsShare != nil {
		if err := c.cli.SyncDeleteNfsShare(c.ctx, nfsShare.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Creator) deleteDpcShare() error {
	dtShare, err := c.cli.GetDataTurboShareByPath(c.ctx, c.params.sharePath())
	if err != nil {
		return err
	}
	if dtShare != nil {
		if err := c.cli.DeleteDataTurboShare(c.ctx, dtShare.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Creator) getCreateFilesystemParams() (*client.CreateFilesystemParams, error) {
	poolRawID := c.pool.GetRawID()

	param := &client.CreateFilesystemParams{
		SnapshotDirVisible: c.params.SnapshotDirVisible,
		StorageID:          c.cli.GetStorageID(),
		PoolRawID:          poolRawID,
		ZoneID:             c.cli.GetZoneID(),
		FilesystemSpecs: []*client.FilesystemSpec{
			{
				Name:        c.params.Name,
				Capacity:    transDmeCapacityFromByteIoGb(c.params.Capacity),
				Description: c.params.Description,
				Count:       1,
			},
		},
	}

	if (c.params.AllocationType == "thin") || (c.params.AllocationType == "thick") {
		param.Tuning = &client.Tuning{AllocationType: c.params.AllocationType}
	}

	// Get nfs share params.
	if len(c.params.AuthClients) != 0 {
		if err := c.deleteNfsShare(); err != nil {
			return nil, err
		}
		param.CreateNfsShareParam = c.getCreateNfsShareParam()
	}

	// Get data turbo share params at the same time.
	if len(c.params.AuthUsers) != 0 {
		if err := c.deleteDpcShare(); err != nil {
			return nil, err
		}
		dtShareParam, err := c.getCreateDpcShareParam()
		if err != nil {
			return nil, err
		}
		param.CreateDpcShareParam = dtShareParam
	}

	return param, nil
}

func (c *Creator) setFsId() error {
	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.Name)
	if err != nil {
		return err
	}
	if fs == nil {
		return errFilesystemNotFound
	}
	if c.params.Protocol == constants.ProtocolNfs {
		nfsShare, err := c.cli.GetNfsShareByPath(c.ctx, c.params.sharePath())
		if err != nil {
			return err
		}
		if nfsShare == nil {
			return errNfsShareNotFound
		}
	} else if c.params.Protocol == constants.ProtocolDtfs {
		dtShare, err := c.cli.GetDataTurboShareByPath(c.ctx, c.params.sharePath())
		if err != nil {
			return err
		}
		if dtShare == nil {
			return errDtfsShareNotFound
		}
	}
	c.fsId = fs.ID
	return nil
}

func (c *Creator) createFilesystem() error {
	err := c.setFsId()
	if err == nil {
		return nil
	}
	if !errors.Is(err, errFilesystemNotFound) {
		return err
	}
	param, err := c.getCreateFilesystemParams()
	if err != nil {
		return err
	}
	err = c.cli.CreateFileSystem(c.ctx, param)
	if err != nil {
		return err
	}
	return c.setFsId()
}

func (c *Creator) rollBackendFilesystem() {
	if err := c.handler.Delete(c.ctx); err != nil {
		log.AddContext(c.ctx).Warningf("Delete filesystem %s failed: %v", c.params.Name, err)
	}
}

func (c *Creator) createKVCache() error {
	// Idempotency: check if KVCache already exists
	queryParams := &client.QueryKVCacheParams{
		Name:     c.params.Name,
		VstoreID: c.vstoreID,
		ZoneID:   c.cli.GetZoneID(),
	}
	existingKV, err := c.cli.QueryKVCache(c.ctx, queryParams)
	if err != nil {
		return fmt.Errorf("query KVCache for idempotency check failed: %w", err)
	}
	if existingKV != nil {
		c.kvcacheStoreId = existingKV.ID
		return nil
	}

	// Get pool raw_id
	poolRawID := c.pool.GetRawID()

	capacitySectors := utils.TransVolumeCapacity(c.params.Capacity, sectorSize)
	params := &client.CreateKVCacheParams{
		StorageID: c.cli.GetStorageID(),
		ZoneID:    c.cli.GetZoneID(),
		PoolRawID: poolRawID,
		VstoreID:  c.vstoreID,
		KVCacheStores: []client.KVCacheStoreBaseInfo{
			{
				Name:        c.params.Name,
				Capacity:    capacitySectors,
				Description: c.params.Description,
			},
		},
	}

	if c.params.EnableTimeAwareGC {
		params.DataCleanupSwitch = "on"
		params.MaxSurvivalTime = int32(c.params.GCTimeThreshold)
	} else {
		params.DataCleanupSwitch = "off"
	}

	result, err := c.cli.CreateKVCache(c.ctx, params)
	if err != nil {
		return fmt.Errorf("create KVCache failed: %w", err)
	}
	c.kvcacheStoreId = result.ID
	return nil
}

func (c *Creator) rollbackKVCache() {
	if c.kvcacheStoreId == "" {
		return
	}
	if err := c.cli.DeleteKVCache(c.ctx, c.kvcacheStoreId); err != nil {
		log.AddContext(c.ctx).Errorf("rollback KVCache %s failed: %v", c.kvcacheStoreId, err)
	}
}

// cleanupKVCacheFilesystem deletes the filesystem and NFS share created by DME
// when creating KVCache. This is called during rollback to ensure no residual resources remain.
func (c *Creator) cleanupKVCacheFilesystem() {
	fs, err := c.cli.GetFileSystemByName(c.ctx, c.params.Name)
	if err != nil {
		log.AddContext(c.ctx).Warningf("query filesystem %s for KVCache rollback cleanup failed: %v", c.params.Name,
			err)
		return
	}
	if fs == nil {
		return
	}
	sharePath := c.params.sharePath()
	nfsShare, err := c.cli.GetNfsShareByPath(c.ctx, sharePath)
	if err != nil {
		log.AddContext(c.ctx).Warningf("query NFS share for filesystem %s rollback cleanup failed: %v", c.params.Name,
			err)
	} else if nfsShare != nil {
		// Delete KvCache NFS share must use private delete mode
		if err := c.cli.DeleteNfsPrivateShare(c.ctx, nfsShare.ID); err != nil {
			log.AddContext(c.ctx).Warningf("sync delete NFS private share %s for filesystem %s "+
				"rollback cleanup failed: %v", nfsShare.ID, c.params.Name, err)
		}
	}
	if err := c.cli.SyncDeleteFileSystem(c.ctx, fs.ID); err != nil {
		log.AddContext(c.ctx).Warningf("sync delete filesystem %s for KVCache rollback cleanup failed: %v", c.params.Name,
			err)
	}
}

func transDmeCapacityFromByteIoGb(capacity int64) float64 {
	return float64(capacity) / float64(constants.DmeCapacityUnitGb)
}
