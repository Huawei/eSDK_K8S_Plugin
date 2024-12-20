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

// Package volume is used for OceanDisk base
package volume

import (
	"context"
	"errors"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

// Base defines the base storage client
type Base struct {
	// The current product does not support the HyperMetro and replication capabilities.
	// The corresponding CLI will be added after the capabilities are supplemented.
	cli     client.OceandiskClientInterface
	product constants.OceanDiskVersion
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

func (p *Base) commonPreCreate(ctx context.Context, params map[string]interface{}) error {
	analyzers := [...]func(context.Context, map[string]interface{}) error{
		p.getPoolID,
		p.getQoS,
	}

	for _, analyzer := range analyzers {
		err := analyzer(ctx, params)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Base) getPoolID(ctx context.Context, params map[string]interface{}) error {
	if params == nil {
		return errors.New("getPoolID params is nil")
	}

	poolName, ok := params["storagepool"].(string)
	if !ok || poolName == "" {
		return errors.New("must specify storage pool to create volume")
	}

	pool, err := p.cli.GetPoolByName(ctx, poolName)
	if err != nil {
		return err
	}
	if len(pool) == 0 {
		return fmt.Errorf("storage pool %s doesn't exist", poolName)
	}

	params["poolID"] = pool["ID"]
	return nil
}

func (p *Base) getQoS(ctx context.Context, params map[string]interface{}) error {
	if params == nil {
		return errors.New("getQoS params is nil")
	}

	if v, exist := params["qos"].(string); exist && v != "" {
		qos, err := smartx.ExtractQoSParameters(ctx, v)
		if err != nil {
			return fmt.Errorf("extract qos parameter [%s] failed, error: %v", v, err)
		}

		validatedQos, err := smartx.ValidateQoSParameters(qos)
		if err != nil {
			return fmt.Errorf("validate qos parameters [%v] failed, error: %v", qos, err)
		}
		params["qos"] = validatedQos
	}

	return nil
}

func (p *Base) setWorkLoadID(ctx context.Context, cli client.OceandiskClientInterface,
	params map[string]interface{}) error {
	if params == nil {
		return errors.New("setWorkLoadID params is nil")
	}

	if val, ok := params["applicationtype"].(string); ok {
		workloadTypeID, err := p.getWorkLoadIDByName(ctx, cli, val)
		if err != nil {
			return err
		}
		params["workloadTypeID"] = workloadTypeID
	}

	return nil
}

func (p *Base) getWorkLoadIDByName(ctx context.Context, cli client.OceandiskClientInterface, workloadTypeName string) (
	string, error) {
	workloadTypeID, err := cli.GetApplicationTypeByName(ctx, workloadTypeName)
	if err != nil {
		return "", err
	}

	if workloadTypeID == "" {
		return "", fmt.Errorf("workloadType does not exist on storage")
	}

	return workloadTypeID, nil
}
