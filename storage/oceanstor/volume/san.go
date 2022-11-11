/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/storage/oceanstor/smartx"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/taskflow"
)

type SAN struct {
	Base
}

func NewSAN(cli, metroRemoteCli, replicaRemoteCli client.BaseClientInterface, product string) *SAN {
	return &SAN{
		Base: Base{
			cli:              cli,
			metroRemoteCli:   metroRemoteCli,
			replicaRemoteCli: replicaRemoteCli,
			product:          product,
		},
	}
}

func (p *SAN) preCreate(ctx context.Context, params map[string]interface{}) error {
	err := p.commonPreCreate(ctx, params)
	if err != nil {
		return err
	}
	name := params["name"].(string)
	params["name"] = utils.GetLunName(name)
	err = p.setWorkLoadID(ctx, p.cli, params)
	if err != nil {
		return err
	}
	return nil
}

func (p *SAN) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	taskflow := taskflow.NewTaskFlow(ctx, "Create-LUN-Volume")
	taskflow.AddTask("Create-Local-LUN", p.createLocalLun, p.revertLocalLun)
	taskflow.AddTask("Create-Local-QoS", p.createLocalQoS, p.revertLocalQoS)
	res, err := taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return nil, err
	}

	volObj := p.prepareVolObj(ctx, params, res)
	return volObj, nil
}

func (p *SAN) Delete(ctx context.Context, name string) error {
	lunName := utils.GetLunName(name)
	lun, err := p.cli.GetLunByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun by name %s error: %v", lunName, err)
		return err
	}
	if lun == nil {
		log.AddContext(ctx).Infof("Lun %s to delete does not exist", lunName)
		return nil
	}

	taskflow := taskflow.NewTaskFlow(ctx, "Delete-LUN-Volume")
	taskflow.AddTask("Delete-Local-LUN", p.deleteLocalLun, nil)

	params := map[string]interface{}{
		"lun":     lun,
		"lunID":   lun["ID"].(string),
		"lunName": lunName,
	}

	_, err = taskflow.Run(params)
	return err
}

func (p *SAN) createLocalLun(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["name"].(string)

	lun, err := p.cli.GetLunByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get LUN %s error: %v", lunName, err)
		return nil, err
	}

	if lun == nil {
		params["parentid"] = params["poolID"].(string)
		lun, err = p.cli.CreateLun(ctx, params)
		if err != nil {
			log.AddContext(ctx).Errorf("Create LUN %s error: %v", lunName, err)
			return nil, err
		}
	}
	return map[string]interface{}{
		"localLunID": lun["ID"].(string),
		"lunWWN":     lun["WWN"].(string),
	}, nil
}

func (p *SAN) revertLocalLun(ctx context.Context, taskResult map[string]interface{}) error {
	lunID, exist := taskResult["localLunID"].(string)
	if !exist || lunID == "" {
		return nil
	}
	err := p.cli.DeleteLun(ctx, lunID)
	return err
}

func (p *SAN) createLocalQoS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	lunID := taskResult["localLunID"].(string)
	lun, err := p.cli.GetLunByID(ctx, lunID)
	if err != nil {
		return nil, err
	}

	qosID, exist := lun["IOCLASSID"].(string)
	if !exist || qosID == "" {
		smartX := smartx.NewSmartX(p.cli)
		qosID, err = smartX.CreateQos(ctx, lunID, "lun", "", qos)
		if err != nil {
			log.AddContext(ctx).Errorf("Create qos %v for lun %s error: %v", qos, lunID, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"localQosID": qosID,
	}, nil
}

func (p *SAN) revertLocalQoS(ctx context.Context, taskResult map[string]interface{}) error {
	lunID, lunIDExist := taskResult["localLunID"].(string)
	qosID, qosIDExist := taskResult["localQosID"].(string)
	if !lunIDExist || !qosIDExist {
		return nil
	}
	smartX := smartx.NewSmartX(p.cli)
	err := smartX.DeleteQos(ctx, qosID, lunID, "lun", "")
	return err
}

func (p *SAN) revertRemoteLun(ctx context.Context, taskResult map[string]interface{}) error {
	lunID, exist := taskResult["remoteLunID"].(string)
	if !exist {
		return nil
	}
	remoteCli := taskResult["remoteCli"].(client.BaseClientInterface)
	return remoteCli.DeleteLun(ctx, lunID)
}

func (p *SAN) createRemoteQoS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	lunID := taskResult["remoteLunID"].(string)
	remoteCli := taskResult["remoteCli"].(client.BaseClientInterface)

	lun, err := remoteCli.GetLunByID(ctx, lunID)
	if err != nil {
		return nil, err
	}

	qosID, exist := lun["IOCLASSID"].(string)
	if !exist || qosID == "" {
		smartX := smartx.NewSmartX(remoteCli)
		qosID, err = smartX.CreateQos(ctx, lunID, "lun", "", qos)
		if err != nil {
			log.AddContext(ctx).Errorf("Create qos %v for lun %s error: %v", qos, lunID, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"remoteQosID": qosID,
	}, nil
}

func (p *SAN) revertRemoteQoS(ctx context.Context, taskResult map[string]interface{}) error {
	lunID, lunIDExist := taskResult["remoteLunID"].(string)
	qosID, qosIDExist := taskResult["remoteQosID"].(string)
	if !lunIDExist || !qosIDExist {
		return nil
	}
	remoteCli := taskResult["remoteCli"].(client.BaseClientInterface)
	smartX := smartx.NewSmartX(remoteCli)
	return smartX.DeleteQos(ctx, qosID, lunID, "lun", "")
}

func (p *SAN) deleteLocalLun(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["lunName"].(string)
	err := p.deleteLun(ctx, lunName, p.cli)
	return nil, err
}

func (p *SAN) deleteLun(ctx context.Context, name string, cli client.BaseClientInterface) error {
	lun, err := cli.GetLunByName(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun by name %s error: %v", name, err)
		return err
	}
	if lun == nil {
		log.AddContext(ctx).Infof("Lun %s to delete does not exist", name)
		return nil
	}

	lunID := lun["ID"].(string)

	qosID, exist := lun["IOCLASSID"].(string)
	if exist && qosID != "" {
		smartX := smartx.NewSmartX(cli)
		err := smartX.DeleteQos(ctx, qosID, lunID, "lun", "")
		if err != nil {
			log.AddContext(ctx).Errorf("Remove lun %s from qos %s error: %v", lunID, qosID, err)
			return err
		}
	}

	err = cli.DeleteLun(ctx, lunID)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete lun %s error: %v", lunID, err)
		return err
	}

	return nil
}
