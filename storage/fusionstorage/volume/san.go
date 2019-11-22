package volume

import (
	"errors"
	"fmt"
	"storage/fusionstorage/client"
	"utils"
	"utils/log"
	"utils/taskflow"
)

type SAN struct {
	cli *client.Client
}

var taskStatusCache = make(map[string]bool)

func NewSAN(cli *client.Client) *SAN {
	return &SAN{
		cli: cli,
	}
}
func (p *SAN) Create(params map[string]interface{}) error {

	name := params["name"].(string)
	if taskStatusCache[name] {
		delete(taskStatusCache, name)
		return nil
	}
	taskflow := taskflow.NewTaskFlow("Create-FusionStorage-LUN-Volume")
	taskflow.AddTask("Create-LUN", p.createLun, nil)
	err := taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return err
	}
	taskStatusCache[name] = true // 任务完成

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
		_, exist := params["cloneFrom"]
		if exist {
			err = p.clone(params)
		} else {
			err = p.cli.CreateVolume(params)
		}
	}

	if err != nil {
		log.Errorf("Create LUN %s error: %v", name, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) clone(params map[string]interface{}) error {

	cloneFrom := params["cloneFrom"].(string)

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
	volName := params["name"].(string)

	err = p.cli.CreateVolumeFromSnapshot(volName, volCapacity, snapshotName)
	if err != nil {
		log.Errorf("Create volume %s from %s error: %v", volName, snapshotName, err)
		return err
	}
	err = p.cli.DeleteSnapshot(snapshotName)
	if err != nil {
		log.Errorf("Delete snapshot %s of %s error: %v", snapshotName, srcVol, err)
		return err
	}

	return nil

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
	err = p.cli.DeleteVolume(name)
	if err != nil {
		return err
	}
	return nil
}
