/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume/creator"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/concurrent"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Base defines the base storage client
type Base struct {
	cli              client.OceanstorClientInterface
	metroRemoteCli   client.OceanstorClientInterface
	replicaRemoteCli client.OceanstorClientInterface
	product          constants.OceanstorVersion
}

func (p *Base) commonPreModify(ctx context.Context, params map[string]interface{}) error {
	analyzers := [...]func(context.Context, map[string]interface{}) error{
		p.getAllocType,
		p.getQoS,
		p.getFileMode,
		p.getMetroPairSyncSpeed,
	}

	for _, analyzer := range analyzers {
		err := analyzer(ctx, params)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Base) commonPreCreate(ctx context.Context, params map[string]interface{}) error {
	analyzers := [...]func(context.Context, map[string]interface{}) error{
		p.getAllocType,
		p.getCloneSpeed,
		p.getPoolID,
		p.getQoS,
		p.getFileMode,
		p.getMetroPairSyncSpeed,
	}

	for _, analyzer := range analyzers {
		err := analyzer(ctx, params)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Base) getAllocType(_ context.Context, params map[string]interface{}) error {
	if v, exist := params["alloctype"].(string); exist && v == "thick" {
		params["alloctype"] = 0
	} else {
		params["alloctype"] = 1
	}

	return nil
}

func (p *Base) getCloneSpeed(_ context.Context, params map[string]interface{}) error {
	_, cloneExist := params["clonefrom"].(string)
	_, srcVolumeExist := params["sourcevolumename"].(string)
	_, srcSnapshotExist := params["sourcesnapshotname"].(string)
	if !(cloneExist || srcVolumeExist || srcSnapshotExist) {
		return nil
	}

	if v, exist := params["clonespeed"].(string); exist && v != "" {
		speed, err := strconv.Atoi(v)
		if err != nil || speed < constants.CloneSpeedLevel1 || speed > constants.CloneSpeedLevel4 {
			return fmt.Errorf("error config %s for clonespeed", v)
		}
		params["clonespeed"] = speed
	} else {
		params["clonespeed"] = constants.CloneSpeedLevel3
	}

	return nil
}

func (p *Base) getFileMode(_ context.Context, params map[string]interface{}) error {
	if params == nil || len(params) == 0 {
		return nil
	}
	if mode, exist := params["filesystemmode"].(string); exist {
		if mode == "HyperMetro" {
			params["filesystemmode"] = "1"
			params["skipNfsShareAndQos"] = true
		} else if mode == "local" {
			params["filesystemmode"] = "0"
		} else {
			return errors.New("don't support fileSystemMode, only HyperMetro and local can be set.")
		}
	}
	return nil
}

func (p *Base) getPoolID(ctx context.Context, params map[string]interface{}) error {
	poolName, exist := params["storagepool"].(string)
	if !exist || poolName == "" {
		return errors.New("must specify storage pool to create volume")
	}

	pool, err := p.cli.GetPoolByName(ctx, poolName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get storage pool %s info error: %v", poolName, err)
		return err
	}
	if pool == nil {
		return fmt.Errorf("storage pool %s doesn't exist", poolName)
	}

	params["poolID"] = pool["ID"]
	return nil
}

func (p *Base) getQoS(ctx context.Context, params map[string]interface{}) error {
	if v, exist := params["qos"].(string); exist && v != "" {
		qos, err := smartx.ExtractQoSParameters(ctx, p.product, v)
		if err != nil {
			return utils.Errorf(ctx, "qos parameter %s error: %v", v, err)
		}

		validatedQos, err := smartx.ValidateQoSParameters(p.product, qos)
		if err != nil {
			return utils.Errorf(ctx, "validate qos parameters failed, error %v", err)
		}
		params["qos"] = validatedQos
	}

	return nil
}

func (p *Base) getRemotePoolID(ctx context.Context,
	params map[string]interface{}, remoteCli client.OceanstorClientInterface) (string, error) {
	remotePool, exist := params["remotestoragepool"].(string)
	if !exist || len(remotePool) == 0 {
		msg := "no remote pool is specified"
		log.AddContext(ctx).Errorln(msg)
		return "", errors.New(msg)
	}

	pool, err := remoteCli.GetPoolByName(ctx, remotePool)
	if err != nil {
		log.AddContext(ctx).Errorf("Get remote storage pool %s info error: %v", remotePool, err)
		return "", err
	}
	if pool == nil {
		return "", fmt.Errorf("remote storage pool %s doesn't exist", remotePool)
	}

	return pool["ID"].(string), nil
}

func (p *Base) preExpandCheckCapacity(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	// check the local pool
	localParentName, ok := params["localParentName"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert localParentName to string failed, data: %v",
			params["localParentName"])
	}

	pool, err := p.cli.GetPoolByName(ctx, localParentName)
	if err != nil || pool == nil {
		msg := fmt.Sprintf("Get storage pool %s info error: %v", localParentName, err)
		log.AddContext(ctx).Errorf(msg)
		return nil, errors.New(msg)
	}

	return nil, nil
}

func (p *Base) getSnapshotReturnInfo(snapshot map[string]interface{}, snapshotSize int64) map[string]interface{} {
	snapshotCreated := utils.ParseIntWithDefault(snapshot["TIMESTAMP"].(string),
		constants.DefaultIntBase, constants.DefaultIntBitSize, 0)
	snapshotSizeBytes := snapshotSize * constants.AllocationUnitBytes
	return map[string]interface{}{
		"CreationTime": snapshotCreated,
		"SizeBytes":    snapshotSizeBytes,
		"ParentID":     snapshot["PARENTID"].(string),
	}
}

func (p *Base) getRemoteDeviceID(ctx context.Context, deviceSN string) (string, error) {
	remoteDevice, err := p.cli.GetRemoteDeviceBySN(ctx, deviceSN)
	if err != nil {
		log.AddContext(ctx).Errorf("Get remote device %s error: %v", deviceSN, err)
		return "", err
	}
	if remoteDevice == nil {
		msg := fmt.Sprintf("Remote device of SN %s does not exist", deviceSN)
		log.AddContext(ctx).Errorln(msg)
		return "", errors.New(msg)
	}

	if remoteDevice["HEALTHSTATUS"] != remoteDeviceHealthStatus ||
		remoteDevice["RUNNINGSTATUS"] != remoteDeviceRunningStatusLinkUp {
		msg := fmt.Sprintf("Remote device %s status is not normal", deviceSN)
		log.AddContext(ctx).Errorln(msg)
		return "", errors.New(msg)
	}

	return remoteDevice["ID"].(string), nil
}

func (p *Base) getWorkLoadIDByName(ctx context.Context,
	cli client.OceanstorClientInterface,
	workloadTypeName string) (string, error) {
	workloadTypeID, err := cli.GetApplicationTypeByName(ctx, workloadTypeName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get application types returned error: %v", err)
		return "", err
	}
	if workloadTypeID == "" {
		msg := fmt.Sprintf("The workloadType %s does not exist on storage", workloadTypeName)
		log.AddContext(ctx).Errorln(msg)
		return "", errors.New(msg)
	}
	return workloadTypeID, nil
}

func (p *Base) setWorkLoadID(ctx context.Context,
	cli client.OceanstorClientInterface, params map[string]interface{}) error {
	if val, ok := params["applicationtype"].(string); ok {
		workloadTypeID, err := p.getWorkLoadIDByName(ctx, cli, val)
		if err != nil {
			return err
		}
		params["workloadTypeID"] = workloadTypeID
	}
	return nil
}

func (p *Base) prepareVolObj(ctx context.Context, params, res map[string]interface{}) (utils.Volume, error) {
	volName, ok := params["name"].(string)
	if !ok {
		return nil, utils.Errorf(ctx, "expecting string for volume name, received type %T", params["name"])
	}

	volObj := utils.NewVolume(volName)
	if res != nil {
		if lunWWN, ok := res["lunWWN"].(string); ok {
			volObj.SetLunWWN(lunWWN)
		}
	}

	capacity := utils.GetValueOrFallback(params, "capacity", int64(0))
	volObj.SetSize(utils.TransK8SCapacity(capacity, constants.AllocationUnitBytes))
	return volObj, nil
}

func (p *Base) getMetroPairSyncSpeed(_ context.Context, params map[string]interface{}) error {
	if params == nil {
		return nil
	}

	hyper, exist := params["hypermetro"].(bool)
	if !exist || !hyper {
		return nil
	}

	if v, exist := params["metropairsyncspeed"].(string); exist && v != "" {
		speed, err := strconv.Atoi(v)
		if err != nil || speed < client.MetroPairSyncSpeedLow || speed > client.MetroPairSyncSpeedHighest {
			return fmt.Errorf("error config %s for metroPairSyncSpeed", v)
		}
		params["metropairsyncspeed"] = speed
	}

	return nil
}

func (p *Base) autoManageAuthClient(ctx context.Context, volume string, clients []string,
	accessVal constants.AuthClientAccessVal) error {
	sharePath := utils.GetOriginSharePath(volume)
	vstoreID := p.cli.GetvStoreID()
	share, err := p.cli.GetNfsShareByPath(ctx, sharePath, vstoreID)
	if err != nil {
		return fmt.Errorf("failed to get share %s NFS share by path: %w", sharePath, err)
	}
	if share == nil || share["ID"] == "" {
		if accessVal == constants.AuthClientNoAccess {
			log.AddContext(ctx).Infof("share %s does not exist, already no access permission", sharePath)
			return nil
		}
		return fmt.Errorf("nfs share %s does not exist", sharePath)
	}
	shareID, _ := utils.GetValue[string](share, "ID")

	for _, authClient := range clients {
		if err := p.createOrUpdateAuthClient(ctx, shareID, authClient, accessVal); err != nil {
			return fmt.Errorf("failed to create or update auth client of share %s: %w", sharePath, err)
		}
	}
	return nil
}

func (p *Base) getExistingAuthClientAttr(ctx context.Context, shareID, name string,
	accessVal int) (*base.AllowNfsShareAccessRequest, error) {
	// Get existing auth client to copy its attributes
	clients, err := p.cli.GetNfsShareAccessRange(ctx, shareID, p.cli.GetvStoreID(), 0, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth client")
	}

	defaultReq := &base.AllowNfsShareAccessRequest{
		Name:        name,
		ParentID:    shareID,
		VStoreID:    p.cli.GetvStoreID(),
		AccessVal:   accessVal,
		AllSquash:   constants.NoAllSquashValue,
		RootSquash:  constants.NoRootSquashValue,
		AccessKrb5:  creator.AccessKrb5ReadNoneInt,
		AccessKrb5i: creator.AccessKrb5ReadNoneInt,
		AccessKrb5p: creator.AccessKrb5ReadNoneInt,
	}

	if len(clients) == 0 {
		return defaultReq, nil
	}

	authClient, ok := clients[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("convert client %v to map[string]interface{} failed", clients[0])
	}
	allSquash, _ := utils.GetValue[string](authClient, "ALLSQUASH")
	defaultReq.AllSquash, err = strconv.Atoi(allSquash)
	if err != nil {
		return nil, fmt.Errorf("convert allSquash %v to int failed", allSquash)
	}

	rootSquash, _ := utils.GetValue[string](authClient, "ROOTSQUASH")
	defaultReq.RootSquash, err = strconv.Atoi(rootSquash)
	if err != nil {
		return nil, fmt.Errorf("convert rootSquash %v to int failed", rootSquash)
	}

	accessKrb5, _ := utils.GetValue[string](authClient, "ACCESSKRB5")
	defaultReq.AccessKrb5, err = strconv.Atoi(accessKrb5)
	if err != nil {
		return nil, fmt.Errorf("convert accessKrb5 %v to int failed", accessKrb5)
	}

	accessKrb5i, _ := utils.GetValue[string](authClient, "ACCESSKRB5I")
	defaultReq.AccessKrb5i, err = strconv.Atoi(accessKrb5i)
	if err != nil {
		return nil, fmt.Errorf("convert accessKrb5i %v to int failed", accessKrb5i)
	}

	accessKrb5p, _ := utils.GetValue[string](authClient, "ACCESSKRB5P")
	defaultReq.AccessKrb5p, err = strconv.Atoi(accessKrb5p)
	if err != nil {
		return nil, fmt.Errorf("convert accessKrb5p %v to int failed", accessKrb5p)
	}

	return defaultReq, nil
}

func (p *Base) createOrUpdateAuthClient(ctx context.Context, shareID, authClientName string,
	accessVal constants.AuthClientAccessVal) error {
	vstoreID := p.cli.GetvStoreID()
	authClient, err := p.cli.GetNfsShareAccess(ctx, shareID, authClientName, vstoreID)
	if err != nil {
		return fmt.Errorf("failed to get auth client access: %w", err)
	}

	if authClient == nil {
		req, err := p.getExistingAuthClientAttr(ctx, shareID, authClientName, int(accessVal))
		if err != nil {
			return fmt.Errorf("failed to get existing auth client attr: %w", err)
		}
		if err := p.cli.AllowNfsShareAccess(ctx, req); err != nil {
			return fmt.Errorf("failed to allow auth client access: %w", err)
		}
	} else {
		authClientID, ok := utils.GetValue[string](authClient, "ID")
		if !ok {
			return fmt.Errorf("failed to get ID field from auth client: %v", authClient)
		}
		if err := p.cli.ModifyNfsShareAccess(ctx, authClientID, vstoreID, accessVal); err != nil {
			return fmt.Errorf("failed to modify auth client access: %w", err)
		}
	}

	return nil
}

func (p *Base) checkAllClientsStatus(ctx context.Context, volume string, authClients []string, expectStats bool) error {
	sharePath := "/" + volume
	// Concurrently check each auth client.
	results := concurrent.ForEach(ctx, authClients, len(authClients), func(ctx context.Context,
		authClient string) (bool, error) {
		var res bool
		// Wait until the access status is allowed.
		err := utils.WaitUntil(func() (bool, error) {
			status, err := p.cli.CheckNfsShareAccessStatus(ctx, sharePath, authClient, p.cli.GetvStoreID(),
				constants.AuthClientReadWrite)
			if err != nil && strings.Contains(err.Error(), "invalid character 'S' looking for beginning of value") {
				log.AddContext(ctx).Warningf(
					"The current storage version does not support to check the client status.")
				res = true
				return res, nil
			}
			res = status == expectStats
			return res, err
		}, checkAccessStatusTimeout, time.Second)

		return res, err
	})

	// Collect results.
	var err error
	for _, result := range results {
		if result.Err != nil {
			err = errors.Join(err, result.Err)
			continue
		}

		if !result.Value {
			err = errors.Join(err, errors.New("check status of clients failed"))
		}
	}
	if err != nil {
		return fmt.Errorf("failed to check all clients status: %w", err)
	}
	return nil
}
