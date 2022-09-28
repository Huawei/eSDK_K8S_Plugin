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
	"errors"
	"fmt"
	"strconv"

	"huawei-csi-driver/storage/fusionstorage/client"
	"huawei-csi-driver/storage/fusionstorage/smartx"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/taskflow"
)

const (
	SCSITYPE  = 0
	ISCSITYPE = 1
)

type SAN struct {
	cli *client.Client
}

func NewSAN(cli *client.Client) *SAN {
	return &SAN{
		cli: cli,
	}
}

func (p *SAN) getQoS(ctx context.Context, params map[string]interface{}) error {
	if v, exist := params["qos"].(string); exist && v != "" {
		qos, err := smartx.VerifyQos(ctx, v)
		if err != nil {
			log.AddContext(ctx).Errorf("Verify qos %s error: %v", v, err)
			return err
		}
		params["qos"] = qos
	}

	return nil
}

func (p *SAN) preCreate(ctx context.Context, params map[string]interface{}) error {
	name := params["name"].(string)
	params["name"] = utils.GetFusionStorageLunName(name)

	if v, exist := params["storagepool"].(string); exist {
		pool, err := p.cli.GetPoolByName(ctx, v)
		if err != nil {
			return err
		}
		if pool == nil {
			return fmt.Errorf("Storage pool %s doesn't exist", v)
		}

		params["poolId"] = int64(pool["poolId"].(float64))
	}

	if v, exist := params["sourcevolumename"].(string); exist && v != "" {
		params["clonefrom"] = utils.GetFusionStorageLunName(v)
	} else if v, exist := params["sourcesnapshotname"].(string); exist && v != "" {
		params["fromSnapshot"] = utils.GetFusionStorageSnapshotName(v)
	} else if v, exist := params["clonefrom"].(string); exist && v != "" {
		params["clonefrom"] = utils.GetFusionStorageLunName(v)
	}

	err := p.getQoS(ctx, params)
	if err != nil {
		return err
	}
	log.AddContext(ctx).Infof("params is %v", params)
	return nil
}

func (p *SAN) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	taskflow := taskflow.NewTaskFlow(ctx, "Create-FusionStorage-LUN-Volume")
	taskflow.AddTask("Create-LUN", p.createLun, p.revertLun)
	taskflow.AddTask("Create-QoS", p.createQoS, nil)

	res, err := taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return nil, err
	}
	volObj := p.prepareVolObj(ctx, params, res)
	return volObj, nil
}

func (p *SAN) prepareVolObj(ctx context.Context, params, res map[string]interface{}) utils.Volume {
	volName, isStr := params["name"].(string)
	if !isStr {
		// Not expecting this error to happen
		log.AddContext(ctx).Warningf("Expecting string for volume name, received type %T", params["name"])
	}
	volObj := utils.NewVolume(volName)
	if lunWWN, ok := res["lunWWN"].(string); ok {
		volObj.SetLunWWN(lunWWN)
	}
	return volObj
}

func (p *SAN) createLun(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name := params["name"].(string)

	vol, err := p.cli.GetVolumeByName(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get LUN %s error: %v", name, err)
		return nil, err
	}

	if vol == nil {
		if _, exist := params["clonefrom"]; exist {
			err = p.clone(ctx, params)
		} else if _, exist := params["fromSnapshot"]; exist {
			err = p.createFromSnapshot(ctx, params)
		} else {
			err = p.cli.CreateVolume(ctx, params)
		}
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Create LUN %s error: %v", name, err)
		return nil, err
	}

	return map[string]interface{}{
		"volumeName": name,
	}, nil
}

func (p *SAN) clone(ctx context.Context, params map[string]interface{}) error {
	cloneFrom := params["clonefrom"].(string)

	srcVol, err := p.cli.GetVolumeByName(ctx, cloneFrom)
	if err != nil {
		log.AddContext(ctx).Errorf("Get clone src vol %s error: %v", cloneFrom, err)
		return err
	}
	if srcVol == nil {
		msg := fmt.Sprintf("Clone src vol %s does not exist", cloneFrom)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	volCapacity := params["capacity"].(int64)
	if volCapacity < int64(srcVol["volSize"].(float64)) {
		msg := fmt.Sprintf("Clone vol capacity must be >= src %s", cloneFrom)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	snapshotName := fmt.Sprintf("k8s_vol_%s_snap_%d", cloneFrom, utils.RandomInt(10000000000))

	err = p.cli.CreateSnapshot(ctx, snapshotName, cloneFrom)
	if err != nil {
		log.AddContext(ctx).Errorf("Create snapshot %s error: %v", snapshotName, err)
		return err
	}

	defer func() {
		p.cli.DeleteSnapshot(ctx, snapshotName)
	}()

	volName := params["name"].(string)

	err = p.cli.CreateVolumeFromSnapshot(ctx, volName, volCapacity, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Create volume %s from %s error: %v", volName, snapshotName, err)
		return err
	}

	return nil
}

func (p *SAN) createFromSnapshot(ctx context.Context, params map[string]interface{}) error {
	srcSnapshotName := params["fromSnapshot"].(string)

	srcSnapshot, err := p.cli.GetSnapshotByName(ctx, srcSnapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get clone src snapshot %s error: %v", srcSnapshotName, err)
		return err
	}
	if srcSnapshot == nil {
		msg := fmt.Sprintf("Src snapshot %s does not exist", srcSnapshotName)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	volCapacity := params["capacity"].(int64)
	if volCapacity < int64(srcSnapshot["snapshotSize"].(float64)) {
		msg := fmt.Sprintf("Clone vol capacity must be >= src snapshot %s", srcSnapshotName)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	volName := params["name"].(string)

	err = p.cli.CreateVolumeFromSnapshot(ctx, volName, volCapacity, srcSnapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Clone snapshot %s to %s error: %v", srcSnapshotName, volName, err)
		return err
	}

	return nil
}

func (p *SAN) revertLun(ctx context.Context, taskResult map[string]interface{}) error {
	volName, exist := taskResult["volumeName"].(string)
	if !exist || volName == "" {
		return nil
	}
	err := p.cli.DeleteVolume(ctx, volName)
	return err
}

func (p *SAN) createQoS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	volName := taskResult["volumeName"].(string)
	qosName, err := p.cli.GetQoSNameByVolume(ctx, volName)
	if err != nil {
		return nil, err
	}

	if qosName == "" {
		smartQos := smartx.NewQoS(p.cli)
		qosName, err = smartQos.AddQoS(ctx, volName, qos)
		if err != nil {
			log.AddContext(ctx).Errorf("Create qos %v for lun %s error: %v", qos, volName, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"QosName": qosName,
	}, nil
}

func (p *SAN) Delete(ctx context.Context, name string) error {
	vol, err := p.cli.GetVolumeByName(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get volume by name %s error: %v", name, err)
		return err
	}
	if vol == nil {
		log.AddContext(ctx).Warningf("Volume %s doesn't exist while trying to delete it", name)
		return nil
	}

	smartQos := smartx.NewQoS(p.cli)
	err = smartQos.RemoveQoS(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Remove QoS of volume %s error: %v", name, err)
		return err
	}

	return p.cli.DeleteVolume(ctx, name)
}

func (p *SAN) Expand(ctx context.Context, name string, newSize int64) (bool, error) {
	lun, err := p.cli.GetVolumeByName(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun by name %s error: %v", name, err)
		return false, err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s to expand does not exist", name)
		log.AddContext(ctx).Errorf(msg)
		return false, errors.New(msg)
	}

	isAttached := int64(lun["volType"].(float64)) == SCSITYPE || int64(lun["volType"].(float64)) == ISCSITYPE
	curSize := int64(lun["volSize"].(float64))
	if newSize <= curSize {
		msg := fmt.Sprintf("Lun %s newSize %d must be greater than curSize %d", name, newSize, curSize)
		log.AddContext(ctx).Errorln(msg)
		return false, errors.New(msg)
	}

	expandTask := taskflow.NewTaskFlow(ctx, "Expand-LUN-Volume")
	expandTask.AddTask("Expand-PreCheck-Capacity", p.preExpandCheckCapacity, nil)
	expandTask.AddTask("Expand-Local-Lun", p.expandLocalLun, nil)

	params := map[string]interface{}{
		"lunName":       lun["volName"].(string),
		"size":          newSize,
		"expandSize":    newSize - curSize,
		"localParentId": int64(lun["poolId"].(float64)),
	}
	_, err = expandTask.Run(params)
	return isAttached, err
}

func (p *SAN) preExpandCheckCapacity(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	// check the local pool
	localParentId := params["localParentId"].(int64)
	pool, err := p.cli.GetPoolById(ctx, localParentId)
	if err != nil || pool == nil {
		log.AddContext(ctx).Errorf("Get storage pool %s info error: %v", localParentId, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) expandLocalLun(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["lunName"].(string)
	newSize := params["size"].(int64)

	err := p.cli.ExtendVolume(ctx, lunName, newSize)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand volume %s error: %v", lunName, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) CreateSnapshot(ctx context.Context,
	lunName, snapshotName string) (map[string]interface{}, error) {
	lun, err := p.cli.GetVolumeByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun by name %s error: %v", lunName, err)
		return nil, err
	} else if lun == nil {
		msg := fmt.Sprintf("Create snapshot from Lun %s does not exist", lunName)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	snapshot, err := p.cli.GetSnapshotByName(ctx, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	if snapshot != nil {
		if snapshot["fatherName"].(string) != lunName {
			msg := fmt.Sprintf("Snapshot %s is already exist, but the parent LUN %s is incompatible",
				snapshotName, lunName)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		} else {
			snapshotCreated, _ := strconv.ParseInt(snapshot["createTime"].(string), 10, 64)
			snapshotSize := int64(snapshot["snapshotSize"].(float64)) * 1024 * 1024
			return map[string]interface{}{
				"CreationTime": snapshotCreated,
				"SizeBytes":    snapshotSize,
				"ParentID":     strconv.FormatInt(int64(lun["volId"].(float64)), 10),
			}, nil
		}
	}

	taskflow := taskflow.NewTaskFlow(ctx, "Create-LUN-Snapshot")
	taskflow.AddTask("Create-Snapshot", p.createSnapshot, nil)

	_, err = taskflow.Run(map[string]interface{}{
		"lunName":      lunName,
		"snapshotName": snapshotName,
	})
	if err != nil {
		taskflow.Revert()
		return nil, err
	}

	snapshot, err = p.cli.GetSnapshotByName(ctx, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	snapshotCreated, _ := strconv.ParseInt(snapshot["createTime"].(string), 10, 64)
	snapshotSize := int64(snapshot["snapshotSize"].(float64)) * 1024 * 1024
	return map[string]interface{}{
		"CreationTime": snapshotCreated,
		"SizeBytes":    snapshotSize,
		"ParentID":     strconv.FormatInt(int64(lun["volId"].(float64)), 10),
	}, nil
}

func (p *SAN) DeleteSnapshot(ctx context.Context, snapshotName string) error {
	snapshot, err := p.cli.GetSnapshotByName(ctx, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
		return err
	}

	if snapshot == nil {
		log.AddContext(ctx).Infof("Lun snapshot %s to delete does not exist", snapshotName)
		return nil
	}

	err = p.cli.DeleteSnapshot(ctx, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete snapshot %s error: %v", snapshotName, err)
		return err
	}

	return nil
}

func (p *SAN) createSnapshot(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["lunName"].(string)
	snapshotName := params["snapshotName"].(string)

	err := p.cli.CreateSnapshot(ctx, snapshotName, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Create snapshot %s for lun %s error: %v", snapshotName, lunName, err)
		return nil, err
	}

	return map[string]interface{}{
		"snapshotName": params["snapshotName"].(string),
	}, nil
}
