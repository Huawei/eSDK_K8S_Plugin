package volume

import (
	"errors"
	"fmt"
	"storage/fusionstorage/client"
	"storage/fusionstorage/smartx"
	"strconv"
	"utils"
	"utils/log"
	"utils/taskflow"
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

func (p *SAN) getQoS(params map[string]interface{}) error {
	if v, exist := params["qos"].(string); exist && v != "" {
		qos, err := smartx.VerifyQos(v)
		if err != nil {
			log.Errorf("Verify qos %s error: %v", v, err)
			return err
		}
		params["qos"] = qos
	}

	return nil
}

func (p *SAN) preCreate(params map[string]interface{}) error {
	name := params["name"].(string)
	params["name"] = utils.GetFusionStorageLunName(name)

	if v, exist := params["storagepool"].(string); exist {
		pool, err := p.cli.GetPoolByName(v)
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

	err := p.getQoS(params)
	if err != nil {
		return err
	}
	log.Infof("params is %v", params)
	return nil
}

func (p *SAN) Create(params map[string]interface{}) error {
	err := p.preCreate(params)
	if err != nil {
		return err
	}

	taskflow := taskflow.NewTaskFlow("Create-FusionStorage-LUN-Volume")
	taskflow.AddTask("Create-LUN", p.createLun, p.revertLun)
	taskflow.AddTask("Create-QoS", p.createQoS, nil)

	_, err = taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return err
	}

	return nil
}

func (p *SAN) createLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name := params["name"].(string)

	vol, err := p.cli.GetVolumeByName(name)
	if err != nil {
		log.Errorf("Get LUN %s error: %v", name, err)
		return nil, err
	}

	if vol == nil {
		if _, exist := params["clonefrom"]; exist {
			err = p.clone(params)
		} else if _, exist := params["fromSnapshot"]; exist {
			err = p.createFromSnapshot(params)
		} else {
			err = p.cli.CreateVolume(params)
		}
	}

	if err != nil {
		log.Errorf("Create LUN %s error: %v", name, err)
		return nil, err
	}

	return map[string]interface{}{
		"volumeName": name,
	}, nil
}

func (p *SAN) clone(params map[string]interface{}) error {
	cloneFrom := params["clonefrom"].(string)

	srcVol, err := p.cli.GetVolumeByName(cloneFrom)
	if err != nil {
		log.Errorf("Get clone src vol %s error: %v", cloneFrom, err)
		return err
	}
	if srcVol == nil {
		msg := fmt.Sprintf("Clone src vol %s does not exist", cloneFrom)
		log.Errorln(msg)
		return errors.New(msg)
	}

	volCapacity := params["capacity"].(int64)
	if volCapacity < int64(srcVol["volSize"].(float64)) {
		msg := fmt.Sprintf("Clone vol capacity must be >= src %s", cloneFrom)
		log.Errorln(msg)
		return errors.New(msg)
	}

	snapshotName := fmt.Sprintf("k8s_vol_%s_snap_%d", cloneFrom, utils.RandomInt(10000000000))

	err = p.cli.CreateSnapshot(snapshotName, cloneFrom)
	if err != nil {
		log.Errorf("Create snapshot %s error: %v", snapshotName, err)
		return err
	}

	defer func() {
		p.cli.DeleteSnapshot(snapshotName)
	}()

	volName := params["name"].(string)

	err = p.cli.CreateVolumeFromSnapshot(volName, volCapacity, snapshotName)
	if err != nil {
		log.Errorf("Create volume %s from %s error: %v", volName, snapshotName, err)
		return err
	}

	return nil
}

func (p *SAN) createFromSnapshot(params map[string]interface{}) error {
	srcSnapshotName := params["fromSnapshot"].(string)

	srcSnapshot, err := p.cli.GetSnapshotByName(srcSnapshotName)
	if err != nil {
		log.Errorf("Get clone src snapshot %s error: %v", srcSnapshotName, err)
		return err
	}
	if srcSnapshot == nil {
		msg := fmt.Sprintf("Src snapshot %s does not exist", srcSnapshotName)
		log.Errorln(msg)
		return errors.New(msg)
	}

	volCapacity := params["capacity"].(int64)
	if volCapacity < int64(srcSnapshot["snapshotSize"].(float64)) {
		msg := fmt.Sprintf("Clone vol capacity must be >= src snapshot %s", srcSnapshotName)
		log.Errorln(msg)
		return errors.New(msg)
	}

	volName := params["name"].(string)

	err = p.cli.CreateVolumeFromSnapshot(volName, volCapacity, srcSnapshotName)
	if err != nil {
		log.Errorf("Clone snapshot %s to %s error: %v", srcSnapshotName, volName, err)
		return err
	}

	return nil
}

func (p *SAN) revertLun(taskResult map[string]interface{}) error {
	volName, exist := taskResult["volumeName"].(string)
	if !exist || volName == "" {
		return nil
	}
	err := p.cli.DeleteVolume(volName)
	return err
}

func (p *SAN) createQoS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	volName := taskResult["volumeName"].(string)
	qosName, err := p.cli.GetQoSNameByVolume(volName)
	if err != nil {
		return nil, err
	}

	if qosName == "" {
		smartQos := smartx.NewQoS(p.cli)
		qosName, err = smartQos.AddQoS(volName, qos)
		if err != nil {
			log.Errorf("Create qos %v for lun %s error: %v", qos, volName, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"QosName": qosName,
	}, nil
}

func (p *SAN) Delete(name string) error {
	vol, err := p.cli.GetVolumeByName(name)
	if err != nil {
		log.Errorf("Get volume by name %s error: %v", name, err)
		return err
	}
	if vol == nil {
		log.Warningf("Volume %s doesn't exist while trying to delete it", name)
		return nil
	}

	smartQos := smartx.NewQoS(p.cli)
	err = smartQos.RemoveQoS(name)
	if err != nil {
		log.Errorf("Remove QoS of volume %s error: %v", name, err)
		return err
	}

	return p.cli.DeleteVolume(name)
}

func (p *SAN) Expand(name string, newSize int64) (bool, error) {
	lun, err := p.cli.GetVolumeByName(name)
	if err != nil {
		log.Errorf("Get lun by name %s error: %v", name, err)
		return false, err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s to expand does not exist", name)
		log.Errorf(msg)
		return false, errors.New(msg)
	}

	isAttached := int64(lun["volType"].(float64)) == SCSITYPE || int64(lun["volType"].(float64)) == ISCSITYPE
	curSize := int64(lun["volSize"].(float64))
	if newSize <= curSize {
		msg := fmt.Sprintf("Lun %s newSize %d must be greater than curSize %d", name, newSize, curSize)
		log.Errorln(msg)
		return false, errors.New(msg)
	}

	expandTask := taskflow.NewTaskFlow("Expand-LUN-Volume")
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

func (p *SAN) preExpandCheckCapacity(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	// check the local pool
	localParentId := params["localParentId"].(int64)
	pool, err := p.cli.GetPoolById(localParentId)
	if err != nil || pool == nil {
		log.Errorf("Get storage pool %s info error: %v", localParentId, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) expandLocalLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["lunName"].(string)
	newSize := params["size"].(int64)

	err := p.cli.ExtendVolume(lunName, newSize)
	if err != nil {
		log.Errorf("Expand volume %s error: %v", lunName, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) CreateSnapshot(lunName, snapshotName string) (map[string]interface{}, error) {
	lun, err := p.cli.GetVolumeByName(lunName)
	if err != nil {
		log.Errorf("Get lun by name %s error: %v", lunName, err)
		return nil, err
	}
	if lun == nil {
		msg := fmt.Sprintf("Create snapshot from Lun %s does not exist", lunName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	snapshot, err := p.cli.GetSnapshotByName(snapshotName)
	if err != nil {
		log.Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	if snapshot != nil {
		snapshotParentName := snapshot["fatherName"].(string)
		if snapshotParentName != lunName {
			msg := fmt.Sprintf("Snapshot %s is already exist, but the parent LUN %s is incompatible", snapshotName, lunName)
			log.Errorln(msg)
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

	taskflow := taskflow.NewTaskFlow("Create-LUN-Snapshot")
	taskflow.AddTask("Create-Snapshot", p.createSnapshot, nil)

	params := map[string]interface{}{
		"lunName":      lunName,
		"snapshotName": snapshotName,
	}

	_, err = taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return nil, err
	}

	snapshot, err = p.cli.GetSnapshotByName(snapshotName)
	if err != nil {
		log.Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
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

func (p *SAN) DeleteSnapshot(snapshotName string) error {
	snapshot, err := p.cli.GetSnapshotByName(snapshotName)
	if err != nil {
		log.Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
		return err
	}

	if snapshot == nil {
		log.Infof("Lun snapshot %s to delete does not exist", snapshotName)
		return nil
	}

	err = p.cli.DeleteSnapshot(snapshotName)
	if err != nil {
		log.Errorf("Delete snapshot %s error: %v", snapshotName, err)
		return err
	}

	return nil
}

func (p *SAN) createSnapshot(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["lunName"].(string)
	snapshotName := params["snapshotName"].(string)

	err := p.cli.CreateSnapshot(snapshotName, lunName)
	if err != nil {
		log.Errorf("Create snapshot %s for lun %s error: %v", snapshotName, lunName, err)
		return nil, err
	}

	return map[string]interface{}{
		"snapshotName": params["snapshotName"].(string),
	}, nil
}
