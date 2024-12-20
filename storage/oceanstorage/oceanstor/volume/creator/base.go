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

// Package creator provides creator of volume
package creator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	// FilesystemObjectType filesystem object identifier
	FilesystemObjectType = "fs"
)

// BaseCreator provides some common methods for volume creation.
type BaseCreator struct {
	cli         client.OceanstorClientInterface
	transaction *flow.Transaction

	// common fields of the filesystem
	vStoreId           string
	fsName             string
	storagePoolName    string
	storagePoolId      string
	description        string
	capacity           int64
	allocType          int
	isShowSnapDir      *bool
	snapshotReservePer *int

	qos map[string]int

	// fields about shares
	isCreateNfsShare bool
	isCreateQoS      bool
	authClient       string
	allSquash        int
	rootSquash       int
	accessKrb5       int
	accessKrb5i      int
	accessKrb5p      int

	vStorePairId       string
	domainId           string
	isPairOnlineDelete bool
	isSyncPair         bool
	metroPairSyncSpeed int
}

// Init initiates fields of BaseCreator
func (c *BaseCreator) Init(params *Parameter) {
	c.transaction = flow.NewTransaction()
	c.fsName = params.PvcName()
	c.storagePoolName = params.StoragePool()
	c.storagePoolId = params.PoolID()
	c.description = params.Description()
	c.capacity = params.Capacity()
	c.allocType = params.AllocType()
	c.qos = params.QoS()
	c.authClient = params.AuthClient()
	c.allSquash = params.AllSquash()
	c.rootSquash = params.RootSquash()
	c.accessKrb5 = params.AccessKrb5()
	c.accessKrb5i = params.AccessKrb5i()
	c.accessKrb5p = params.AccessKrb5p()
	c.domainId = params.MetroDomainID()
	c.vStorePairId = params.VStorePairId()
	c.metroPairSyncSpeed = params.SyncMetroPairSpeed()

	if !params.IsSkipNfsShareAndQos() {
		c.isCreateNfsShare = true
		c.isCreateQoS = true
	}

	if val, ok := params.IsShowSnapDir(); ok {
		c.isShowSnapDir = &val
	}

	if val, ok := params.SnapshotReservePer(); ok {
		c.snapshotReservePer = &val
	}

	if params.Product().IsDoradoV6OrV7() && params.IsHyperMetro() {
		c.vStoreId = c.cli.GetvStoreID()
	}

	if params.Product().IsDoradoV6OrV7() {
		c.isSyncPair = false
		c.isPairOnlineDelete = false
	} else {
		c.isSyncPair = true
		c.isPairOnlineDelete = true
	}
}

// GetPoolID gets the id of pool by its fsName from storage.
func (c *BaseCreator) GetPoolID(ctx context.Context, storagePoolName string) (string, error) {
	if c.storagePoolId != "" {
		return c.storagePoolId, nil
	}

	pool, err := c.cli.GetPoolByName(ctx, storagePoolName)
	if err != nil {
		return "", fmt.Errorf("get storage pool %s info error: %w", storagePoolName, err)
	}
	if pool == nil || utils.GetValueOrFallback(pool, "ID", "") == "" {
		return "", fmt.Errorf("storage pool %s doesn't exist", storagePoolName)
	}

	return utils.GetValueOrFallback(pool, "ID", ""), nil
}

// CreateNfsShare creates nfs share for the filesystem.
func (c *BaseCreator) CreateNfsShare(ctx context.Context, fsName, fsId, desc, vStoreId string) (string, error) {
	if !c.isCreateNfsShare {
		return "", nil
	}

	sharePath := utils.GetSharePath(fsName)
	share, err := c.cli.GetNfsShareByPath(ctx, sharePath, vStoreId)
	if err != nil {
		return "", fmt.Errorf("get nfs share by path %s error: %w", sharePath, err)
	}

	if share != nil {
		return utils.GetValueOrFallback(share, "ID", ""), nil
	}

	req := map[string]any{
		"sharepath":   sharePath,
		"fsid":        fsId,
		"description": desc,
		"vStoreID":    vStoreId,
	}

	share, err = c.cli.CreateNfsShare(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create nfs share %v error: %w", req, err)
	}

	return utils.GetValueOrFallback(share, "ID", ""), nil
}

// RollbackShare rollbacks nfs share resource.
func (c *BaseCreator) RollbackShare(ctx context.Context, shareId, vStoreId string) error {
	if !c.isCreateNfsShare {
		return nil
	}

	err := c.cli.DeleteNfsShare(ctx, shareId, vStoreId)
	if err != nil {
		return fmt.Errorf("delete nfs share %v error: %w", shareId, err)
	}

	return nil
}

// ShareAccessParams is parameters for creating share access
type ShareAccessParams struct {
	shareId     string
	vStoreId    string
	authClient  string
	allSquash   int
	rootSquash  int
	accessKrb5  int
	accessKrb5i int
	accessKrb5p int
}

// AllowShareAccess allows nfs share access.
func (c *BaseCreator) AllowShareAccess(ctx context.Context, params ShareAccessParams) error {
	if !c.isCreateNfsShare {
		return nil
	}

	removeShareAccess := func(authClient map[string]any) {
		accessId := utils.GetValueOrFallback(authClient, "ID", "")
		if accessId == "" {
			return
		}

		if err := c.cli.DeleteNfsShareAccess(ctx, accessId, params.vStoreId); err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessId, err)
		}
	}

	if err := c.traverseShareAccesses(ctx, params.shareId, params.vStoreId, removeShareAccess); err != nil {
		return err
	}

	return c.createAuthClients(ctx, params)
}

// RollbackShareAccess rollbacks nfs share access.
func (c *BaseCreator) RollbackShareAccess(ctx context.Context, shareId, vStoreId, authClient string) error {
	if !c.isCreateNfsShare {
		return nil
	}

	removeShareAccessInAuthClient := func(authClientMap map[string]any) {
		authClients := strings.Split(authClient, ";")
		authClientName := utils.GetValueOrFallback(authClientMap, "NAME", "")
		if !utils.Contains(authClients, authClientName) {
			return
		}

		accessId := utils.GetValueOrFallback(authClientMap, "ID", "")
		if accessId == "" {
			return
		}

		if err := c.cli.DeleteNfsShareAccess(ctx, accessId, vStoreId); err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessId, err)
		}
	}

	if err := c.traverseShareAccesses(ctx, shareId, vStoreId, removeShareAccessInAuthClient); err != nil {
		return fmt.Errorf("delete extra nfs share access %s error: %w", shareId, err)
	}

	return nil
}

// CreateQoS creates qos for filesystem.
func (c *BaseCreator) CreateQoS(ctx context.Context, fsID, vStoreId string) (string, error) {
	if !c.isCreateQoS || c.qos == nil {
		return "", nil
	}

	smartX := smartx.NewSmartX(c.cli)
	qosID, err := smartX.CreateQos(ctx, fsID, FilesystemObjectType, vStoreId, c.qos)
	if err != nil {
		return "", fmt.Errorf("create qos %v for fs %s error: %w", c.qos, fsID, err)
	}

	return qosID, nil
}

// RollbackQoS rollbacks qos resource.
func (c *BaseCreator) RollbackQoS(ctx context.Context, qosId, fsId, vStoreId string) error {
	if !c.isCreateQoS || c.qos == nil {
		return nil
	}

	smartX := smartx.NewSmartX(c.cli)
	if err := smartX.DeleteQos(ctx, qosId, fsId, FilesystemObjectType, vStoreId); err != nil {
		return fmt.Errorf("delete qos %v for fs %s error: %w", qosId, fsId, err)
	}

	return nil
}

func (c *BaseCreator) addNfsShareTransactionStep(
	ctx context.Context,
	fsId *string,
	fsName, description, vStoreId string,
) {
	if !c.isCreateNfsShare {
		return
	}

	var shareId string
	c.transaction.
		Then(
			func() error {
				if fsId == nil {
					return fmt.Errorf("create nfs share failed. filesystem id is nil")
				}
				var err error
				shareId, err = c.CreateNfsShare(ctx, fsName, *fsId, description, vStoreId)
				return err
			},
			func() {
				if err := c.RollbackShare(ctx, shareId, vStoreId); err != nil {
					log.AddContext(ctx).Errorln(err)
				}
			}).
		Then(
			func() error { return c.AllowShareAccess(ctx, c.getShareAccessParams(shareId)) },
			func() {
				if err := c.RollbackShareAccess(ctx, shareId, c.vStoreId, c.authClient); err != nil {
					log.AddContext(ctx).Errorln(err)
				}
			},
		)
}

func (c *BaseCreator) addQoSTransactionStep(ctx context.Context, fsId *string, vStoreId string) {
	if !c.isCreateQoS || c.qos == nil {
		return
	}

	var qosId string
	var err error
	c.transaction.
		Then(
			func() error {
				if fsId == nil {
					return fmt.Errorf("create qos share failed. filesystem id is nil")
				}
				qosId, err = c.CreateQoS(ctx, *fsId, vStoreId)
				return err
			},
			func() {
				if err := c.RollbackQoS(ctx, qosId, *fsId, vStoreId); err != nil {
					log.AddContext(ctx).Errorln(err)
				}
			})
}

func (c *BaseCreator) createAuthClients(ctx context.Context, params ShareAccessParams) error {
	for _, i := range strings.Split(params.authClient, ";") {
		req := &client.AllowNfsShareAccessRequest{
			Name:        i,
			ParentID:    params.shareId,
			AccessVal:   1,
			Sync:        0,
			AllSquash:   params.allSquash,
			RootSquash:  params.rootSquash,
			VStoreID:    params.vStoreId,
			AccessKrb5:  params.accessKrb5,
			AccessKrb5i: params.accessKrb5i,
			AccessKrb5p: params.accessKrb5p,
		}
		if err := c.cli.AllowNfsShareAccess(ctx, req); err != nil {
			return fmt.Errorf("Allow nfs share access %v failed. error: %w", req, err)
		}
	}

	return nil
}

func (c *BaseCreator) traverseShareAccesses(ctx context.Context, shareId, vStoreId string,
	do func(map[string]any)) error {
	count, err := c.cli.GetNfsShareAccessCount(ctx, shareId, vStoreId)
	if err != nil {
		return err
	}

	const perPage = 100
	var num int64
	for ; num < count; num += perPage {
		clients, err := c.cli.GetNfsShareAccessRange(ctx, shareId, vStoreId, num, num+perPage)
		if err != nil {
			return err
		}
		if len(clients) == 0 {
			break
		}

		for _, item := range clients {
			authClient, ok := item.(map[string]any)
			if !ok {
				log.AddContext(ctx).Warningf("convert client to map failed, data: %v", item)
				continue
			}
			do(authClient)
		}
	}

	return nil
}

func (c *BaseCreator) getShareAccessParams(shareId string) ShareAccessParams {
	return ShareAccessParams{
		shareId:     shareId,
		vStoreId:    c.vStoreId,
		authClient:  c.authClient,
		allSquash:   c.allSquash,
		rootSquash:  c.rootSquash,
		accessKrb5:  c.accessKrb5,
		accessKrb5i: c.accessKrb5i,
		accessKrb5p: c.accessKrb5p,
	}
}

func (c *BaseCreator) createHyperMetroPair(ctx context.Context,
	activeFsId string, standbyFsId string) (string, error) {
	req := map[string]any{
		"HCRESOURCETYPE": filesystemHCRESourceType,
		"LOCALOBJID":     activeFsId,
		"REMOTEOBJID":    standbyFsId,
		"VSTOREPAIRID":   c.vStorePairId,
	}
	if c.domainId != "" {
		req["DOMAINID"] = c.domainId
	}
	if c.metroPairSyncSpeed != 0 {
		req["SPEED"] = c.metroPairSyncSpeed
	}
	pair, err := c.cli.CreateHyperMetroPair(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create nas hypermetro pair error: %w", err)
	}

	// There is no need to synchronize when use NAS Dorado V6 or OceanStor V6 HyperMetro Volume
	if c.isSyncPair {
		pairId := utils.GetValueOrFallback(pair, "ID", "")
		err = c.cli.SyncHyperMetroPair(ctx, pairId)
		if err != nil {
			log.AddContext(ctx).Errorf("Sync nas hypermetro pair %s error: %v", pairId, err)
			delErr := c.cli.DeleteHyperMetroPair(ctx, pairId, true)
			if delErr != nil {
				log.AddContext(ctx).Errorf("delete hypermetro pair %s error: %v", pairId, err)
			}
			return "", err
		}
	}

	return utils.GetValueOrFallback(pair, "ID", ""), nil
}

func (c *BaseCreator) rollbackHyperMetroPair(ctx context.Context, pairId string) error {
	pair, err := c.cli.GetHyperMetroPair(ctx, pairId)
	if err != nil {
		return err
	}
	if pair == nil {
		return nil
	}
	pairStatus := utils.GetValueOrFallback(pair, "RUNNINGSTATUS", "")
	if utils.Contains(pairRationalStatus, pairStatus) {
		if err := c.cli.StopHyperMetroPair(ctx, pairId); err != nil {
			return err
		}
	}

	return c.deleteHyperMetroPair(ctx, pairId)
}

func (c *BaseCreator) deleteHyperMetroPair(ctx context.Context, pairId string) error {
	err := c.cli.DeleteHyperMetroPair(ctx, pairId, c.isPairOnlineDelete)
	if err != nil {
		return utils.Errorf(ctx, "Delete hyperMetro Pair failed, err: %v", err)
	}

	return utils.WaitUntil(func() (bool, error) {
		pair, err := c.cli.GetHyperMetroPair(ctx, pairId)
		if err != nil {
			return false, err
		}

		if pair == nil {
			return true, nil
		}

		return false, nil
	}, time.Minute, time.Second)
}

func (c *BaseCreator) getPairIdByFsId(ctx context.Context, fs map[string]any) (string, error) {
	var hyperMetroIds []string
	hyperMetroIdBytes := []byte(fs["HYPERMETROPAIRIDS"].(string))
	err := json.Unmarshal(hyperMetroIdBytes, &hyperMetroIds)
	if err != nil {
		return "", fmt.Errorf("unmarshal hypermetroIDBytes failed, error: %w", err)
	}

	if len(hyperMetroIds) > 0 {
		return hyperMetroIds[0], nil
	}

	return "", nil
}
