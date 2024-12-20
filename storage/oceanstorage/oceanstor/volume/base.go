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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
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
		return nil, pkgUtils.Errorf(ctx, "convert localParentName to string failed, data: %v", params["localParentName"])
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
