package volume

import (
	"errors"
	"fmt"
	"storage/fusionstorage/client"
	"strconv"
	"time"
	"utils"
	"utils/log"
	"utils/taskflow"
)

type NAS struct {
	cli *client.Client
}

func NewNAS(cli *client.Client) *NAS {
	return &NAS{
		cli: cli,
	}
}

func (p *NAS) preCreate(params map[string]interface{}) error {
	authclient, exist := params["authclient"].(string)
	if !exist || authclient == "" {
		msg := fmt.Sprintf("authclient must be provided for filesystem")
		log.Errorln(msg)
		return errors.New(msg)
	}

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

	name := params["name"].(string)
	params["name"] = utils.GetFileSystemName(name)

	if v, exist := params["clonefrom"].(string); exist {
		params["clonefrom"] = utils.GetFileSystemName(v)
	}

	return nil
}

func (p *NAS) Create(params map[string]interface{}) error {
	err := p.preCreate(params)
	if err != nil {
		return err
	}

	createTask := taskflow.NewTaskFlow("Create-FileSystem-Volume")
	createTask.AddTask("Create-FS", p.createFS, p.revertFS)
	createTask.AddTask("Create-Quota", p.createQuota, p.revertQuota)
	createTask.AddTask("Create-Share", p.createShare, p.revertShare)
	createTask.AddTask("Allow-Share-Access", p.allowShareAccess, nil)
	_, err = createTask.Run(params)
	if err != nil {
		createTask.Revert()
		return err
	}

	return nil
}

func (p *NAS) createFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)
	fs, err := p.cli.GetFileSystemByName(fsName)
	if err != nil {
		log.Errorf("Get filesystem %s error: %v", fsName, err)
		return nil, err
	}

	if fs == nil {
		_, exist := params["clonefrom"]
		if exist {
			fs, err = p.clone(params)
		} else {
			fs, err = p.cli.CreateFileSystem(params)
		}
	}

	if err != nil {
		log.Errorf("Create filesystem %s error: %v", fsName, err)
		return nil, err
	}

	err = p.waitFilesystemCreated(fsName)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"fsID": strconv.FormatInt(int64(fs["id"].(float64)), 10),
	}, nil
}

func (p *NAS) clone(params map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *NAS) revertFS(taskResult map[string]interface{}) error {
	fsID, exist := taskResult["fsID"].(string)
	if !exist {
		return nil
	}

	return p.deleteFS(fsID)
}

func (p *NAS) deleteFS(fsID string) error {
	err := p.cli.DeleteFileSystem(fsID)
	if err != nil {
		log.Errorf("Delete filesystem %s error: %v", fsID, err)
	}

	return err
}

func (p *NAS) createQuota(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsID, _ := taskResult["fsID"].(string)
	quota, err := p.cli.GetQuotaByFileSystem(fsID)
	if err != nil {
		log.Errorf("Get filesystem %s quota error: %v", fsID, err)
		return nil, err
	}

	if quota == nil {
		quotaParams := map[string]interface{}{
			"parent_id":         fsID,
			"parent_type":       "40",
			"quota_type":        "1",
			"space_hard_quota":  params["capacity"].(int64),
			"snap_space_switch": 0,
			"space_unit_type":   2,
		}
		err := p.cli.CreateQuota(quotaParams)
		if err != nil {
			log.Errorf("Create filesystem quota %v error: %v", quotaParams, err)
			return nil, err
		}
	}

	return nil, nil
}

func (p *NAS) revertQuota(taskResult map[string]interface{}) error {
	fsID, exist := taskResult["fsID"].(string)
	if !exist {
		return nil
	}

	return p.deleteQuota(fsID)
}

func (p *NAS) deleteQuota(fsID string) error {
	quota, err := p.cli.GetQuotaByFileSystem(fsID)
	if err != nil {
		log.Errorf("Get filesystem %s quota error: %v", fsID, err)
		return err
	}

	if quota != nil {
		quotaId := quota["id"].(string)
		err := p.cli.DeleteQuota(quotaId)
		if err != nil {
			log.Errorf("Delete filesystem quota %s error: %v", quotaId, err)
			return err
		}
	}

	return nil
}

func (p *NAS) createShare(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)
	sharePath := utils.GetFSSharePath(fsName)
	share, err := p.cli.GetNfsShareByPath(sharePath)

	if err != nil {
		log.Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return nil, err
	}

	if share == nil {
		shareParams := map[string]interface{}{
			"sharepath":   sharePath,
			"fsid":        taskResult["fsID"].(string),
			"description": "Created from Kubernetes Provisioner",
		}

		share, err = p.cli.CreateNfsShare(shareParams)
		if err != nil {
			log.Errorf("Create nfs share %v error: %v", shareParams, err)
			return nil, err
		}
	}
	return map[string]interface{}{
		"shareID": share["id"].(string),
	}, nil
}

func (p *NAS) waitFilesystemCreated(fsName string) error {
	err := utils.WaitUntil(func() (bool, error) {
		fs, err := p.cli.GetFileSystemByName(fsName)
		if err != nil {
			return false, err
		}
		if fs["running_status"].(float64) == 0 { //filesystem is ok
			return true, nil
		} else {
			return false, nil
		}
	}, time.Hour*6, time.Second*5)
	return err
}

func (p *NAS) revertShare(taskResult map[string]interface{}) error {
	shareID, exist := taskResult["shareID"].(string)
	if !exist {
		return nil
	}

	return p.deleteShare(shareID)
}

func (p *NAS) deleteShare(shareID string) error {
	err := p.cli.DeleteNfsShare(shareID)
	if err != nil {
		log.Errorf("Delete share %s error: %v", shareID, err)
		return err
	}

	return nil
}

func (p *NAS) allowShareAccess(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	createParams := map[string]interface{}{
		"name":      params["authclient"].(string),
		"shareid":   taskResult["shareID"].(string),
		"accessval": 1,
	}

	err := p.cli.AllowNfsShareAccess(createParams)
	if err != nil {
		log.Errorf("Allow nfs share access %v error: %v", createParams, err)
		return nil, err
	}

	return nil, nil
}

func (p *NAS) Delete(name string) error {
	sharePath := utils.GetFSSharePath(name)
	share, err := p.cli.GetNfsShareByPath(sharePath)
	if err != nil {
		log.Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return err
	}

	var fs map[string]interface{}
	if share == nil {
		log.Infof("Share %s to delete does not exist, continue to delete filesystem", sharePath)
		fsName := utils.GetFileSystemName(name)
		fs, err = p.cli.GetFileSystemByName(fsName)
		if err != nil {
			log.Errorf("Get filesystem %s error: %v", fsName, err)
			return err
		}

		if fs == nil {
			log.Infof("Filesystem %s to delete does not exist", fsName)
			return nil
		} else {
			fsID := strconv.FormatInt(int64(fs["id"].(float64)), 10)
			err = p.deleteQuota(fsID)
			if err != nil {
				log.Errorf("Delete filesystem %s quota error: %v", fsID, err)
				return err
			}

			err = p.deleteFS(fsID)
			if err != nil {
				log.Errorf("Delete filesystem %s error: %v", fsID, err)
				return err
			}
		}
	} else {
		shareID := share["id"].(string)
		err = p.cli.DeleteNfsShare(shareID)
		if err != nil {
			log.Errorf("Delete nfs share %s error: %v", shareID, err)
			return err
		}

		fsID := share["file_system_id"].(string)
		err = p.deleteQuota(fsID)
		if err != nil {
			log.Errorf("Delete filesystem %s quota error: %v", fsID, err)
			return err
		}

		err = p.deleteFS(fsID)
		if err != nil {
			log.Errorf("Delete filesystem %s error: %v", fsID, err)
			return err
		}
	}

	return nil
}
