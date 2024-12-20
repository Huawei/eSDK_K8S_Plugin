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

// Package volume is used for OceanDisk san
package volume

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// SAN provides base san client
type SAN struct {
	Base
}

// NewSAN inits a new san client
func NewSAN(cli client.OceandiskClientInterface) *SAN {
	return &SAN{
		Base: Base{
			cli: cli,
		},
	}
}

func (p *SAN) preCreate(ctx context.Context, params map[string]interface{}) error {
	err := p.commonPreCreate(ctx, params)
	if err != nil {
		return err
	}

	err = p.setWorkLoadID(ctx, p.cli, params)
	if err != nil {
		return err
	}

	return nil
}

// Create creates lun volume
func (p *SAN) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	taskFlow := flow.NewTaskFlow(ctx, "Create-Namespace-Volume")

	taskFlow.AddTask("Create-Local-Namespace", p.createLocalNamespace, p.revertLocalNamespace)
	taskFlow.AddTask("Create-Local-QoS", p.createLocalQoS, p.revertLocalQoS)

	res, err := taskFlow.Run(params)
	if err != nil {
		taskFlow.Revert()
		return nil, err
	}

	return p.prepareVolObj(ctx, params, res)
}

func (p *SAN) createLocalNamespace(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {
	// 1. Check whether the namespace exists.
	namespaceName, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("assert namespace name: [%v] to string failed", params["name"])
	}

	namespace, err := p.cli.GetNamespaceByName(ctx, namespaceName)
	if err != nil {
		return nil, err
	}

	// 2. Do create.
	if len(namespace) == 0 {
		createNamespaceParams, err := client.MakeCreateNamespaceParams(params)
		if err != nil {
			return nil, err
		}

		namespace, err = p.cli.CreateNamespace(ctx, *createNamespaceParams)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"localNamespaceID": namespace["ID"],
		"namespaceWWN":     namespace["WWN"],
	}, nil
}

func (p *SAN) revertLocalNamespace(ctx context.Context, taskResult map[string]interface{}) error {
	namespaceID, ok := utils.GetValue[string](taskResult, "localNamespaceID")
	if !ok || namespaceID == "" {
		return nil
	}

	return p.cli.DeleteNamespace(ctx, namespaceID)
}

func (p *SAN) createLocalQoS(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	// 1. Get namespace by id, check whether the namespace exists.
	namespaceId, ok := taskResult["localNamespaceID"].(string)
	if !ok {
		return nil, fmt.Errorf("namespaceId: [%v] convert to string failed", taskResult["localNamespaceID"])
	}
	namespace, err := p.cli.GetNamespaceByID(ctx, namespaceId)
	if err != nil {
		return nil, err
	}

	// 2. Do create qos.
	qosID, exist := namespace["IOCLASSID"].(string)
	if !exist || qosID == "" {
		smartX := smartx.NewSmartX(p.cli)
		qosID, err = smartX.CreateQos(ctx, namespaceId, qos)
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"localQosID": qosID,
	}, nil
}

func (p *SAN) revertLocalQoS(ctx context.Context, taskResult map[string]interface{}) error {
	namespaceID, namespaceIdOk := utils.GetValue[string](taskResult, "localNamespaceID")
	qosID, qosIdOk := utils.GetValue[string](taskResult, "localQosID")
	if !namespaceIdOk || !qosIdOk {
		return nil
	}
	smartX := smartx.NewSmartX(p.cli)
	return smartX.DeleteQos(ctx, qosID, namespaceID)
}

// Query queries volume by name
func (p *SAN) Query(ctx context.Context, name string) (utils.Volume, error) {
	namespace, err := p.cli.GetNamespaceByName(ctx, name)
	if err != nil {
		return nil, utils.Errorf(ctx, "get lun by name %s error: %v", name, err)
	}

	if len(namespace) == 0 {
		return nil, utils.Errorf(ctx, "lun [%s] to query does not exist", name)
	}

	volObj := utils.NewVolume(name)
	if lunWWN, ok := namespace["WWN"].(string); ok {
		volObj.SetLunWWN(lunWWN)
	}

	// set the size, need to trans Sectors to Bytes
	capacityStr, ok := namespace["CAPACITY"].(string)
	if !ok {
		return nil, utils.Errorf(ctx, "convert capacity: %v to string failed", namespace["CAPACITY"])
	}

	capacity, err := strconv.ParseInt(capacityStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
	if err == nil {
		volObj.SetSize(utils.TransK8SCapacity(capacity, constants.AllocationUnitBytes))
	}

	return volObj, nil
}

// Delete deletes volume by volume name
func (p *SAN) Delete(ctx context.Context, name string) error {
	namespace, err := p.cli.GetNamespaceByName(ctx, name)
	if err != nil {
		return err
	}
	if len(namespace) == 0 {
		log.AddContext(ctx).Infof("Namespace: %s to be deleted does not exist.", name)
		return nil
	}

	taskFlow := flow.NewTaskFlow(ctx, "Delete-Namespace-Volume")
	taskFlow.AddTask("Delete-Local-Namespace", p.deleteNamespace, nil)

	params := map[string]interface{}{
		"namespaceName": name,
	}

	_, err = taskFlow.Run(params)
	return err
}

func (p *SAN) deleteNamespace(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {
	namespaceName, ok := params["namespaceName"].(string)
	if !ok {
		return nil, fmt.Errorf("assert namespaceName: [%v] to string failed", params["namespaceName"])
	}

	namespace, err := p.cli.GetNamespaceByName(ctx, namespaceName)
	if err != nil {
		return nil, err
	}
	if len(namespace) == 0 {
		log.AddContext(ctx).Infof("Namespace: [%s] to be deleted does not exist.", namespaceName)
		return nil, nil
	}

	namespaceID, ok := namespace["ID"].(string)
	if !ok {
		return nil, fmt.Errorf("assert namespaceID: [%v] to string failed", namespace["ID"])
	}

	// Oceandisk storage doesn't provide the IOCLASSID field, we need to traverse all ioclass to delete related QoS.
	smartX := smartx.NewSmartX(p.cli)
	err = smartX.DeleteQosByNamespaceId(ctx, namespaceID)
	if err != nil {
		return nil, fmt.Errorf("delete namespace %s related qos failed, error: %v", namespaceID, err)
	}

	return nil, p.cli.DeleteNamespace(ctx, namespaceID)
}

// Expand expands volume to target size
func (p *SAN) Expand(ctx context.Context, name string, newSize int64) (bool, error) {
	namespace, err := p.cli.GetNamespaceByName(ctx, name)
	if err != nil {
		return false, err
	} else if len(namespace) == 0 {
		return false, fmt.Errorf("namespace %s to be expanded does not exist", name)
	}

	exposedToInitiator, ok := namespace["EXPOSEDTOINITIATOR"].(string)
	if !ok {
		return false, fmt.Errorf("assert exposedToInitiator: [%v] to string failed", namespace["EXPOSEDTOINITIATOR"])
	}
	isAttached, err := strconv.ParseBool(exposedToInitiator)
	if err != nil {
		return isAttached, fmt.Errorf("parse exposedToInitiator: [%v] to bool failed, error: %w",
			exposedToInitiator, err)
	}

	capacityStr, ok := namespace["CAPACITY"].(string)
	if !ok {
		return false, fmt.Errorf("assert capacity: [%v] to string failed", namespace["CAPACITY"])
	}
	curSize := utils.ParseIntWithDefault(capacityStr, constants.DefaultIntBase, constants.DefaultIntBitSize, 0)
	if newSize == curSize {
		log.AddContext(ctx).Infof("Target capacity of namespace: [%v] is the same as the current capacity: [%d].",
			name, newSize)
		return isAttached, nil
	} else if newSize < curSize {
		return false, fmt.Errorf("target capacity: [%d] of namespace: [%v] must be greater than or equal to "+
			"current capacity: [%d]", newSize, name, curSize)
	}

	expandTask := flow.NewTaskFlow(ctx, "Expand-Namespace-Volume")
	expandTask.AddTask("Expand-Local-Namespace", p.expandLocalNamespace, nil)

	params := map[string]interface{}{
		"name":            name,
		"size":            newSize,
		"namespaceID":     namespace["ID"],
		"localParentName": namespace["PARENTNAME"],
	}
	_, err = expandTask.Run(params)
	return isAttached, err
}

func (p *SAN) expandLocalNamespace(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {
	// 1. Check whether the storage pool exists.
	localParentName, ok := params["localParentName"].(string)
	if !ok {
		return nil, fmt.Errorf("assert localParentName: [%v] to string failed", params["localParentName"])
	}

	pool, err := p.cli.GetPoolByName(ctx, localParentName)
	if err != nil {
		return nil, err
	} else if len(pool) == 0 {
		return nil, fmt.Errorf("storage pool: [%s] dose not exist", localParentName)
	}

	// 2. do expand
	namespaceID, ok := params["namespaceID"].(string)
	if !ok {
		return nil, fmt.Errorf("assert namespaceID: [%v] to string failed", params["namespaceID"])
	}

	newSize, ok := params["size"].(int64)
	if !ok {
		return nil, fmt.Errorf("assert size: [%v] to int64 failed", params["size"])
	}

	return nil, p.cli.ExtendNamespace(ctx, namespaceID, newSize)
}
