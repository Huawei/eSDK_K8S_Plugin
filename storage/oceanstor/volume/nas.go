package volume

import (
	"errors"
	"fmt"
	"storage/oceanstor/client"
	"storage/oceanstor/smartx"
	"strconv"
	"strings"
	"time"
	"utils"
	"utils/log"
	"utils/taskflow"
)

const (
	FILESYSTEM_HEALTH_STATUS_NORMAL   = "1"
	FILESYSTEM_SPLIT_STATUS_NOT_START = "1"
	FILESYSTEM_SPLIT_STATUS_SPLITTING = "2"
	FILESYSTEM_SPLIT_STATUS_QUEUING   = "3"
	FILESYSTEM_SPLIT_STATUS_ABNORMAL  = "4"
)

type NAS struct {
	Base
}

func NewNAS(cli *client.Client) *NAS {
	return &NAS{
		Base: Base{cli: cli},
	}
}

func (p *NAS) preCreate(params map[string]interface{}) error {
	if _, exist := params["authclient"].(string); !exist {
		msg := "authclient must be provided for filesystem"
		log.Errorln(msg)
		return errors.New(msg)
	}

	err := p.commonPreCreate(params)
	if err != nil {
		return err
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

	taskflow := taskflow.NewTaskFlow("Create-FileSystem-Volume")
	taskflow.AddTask("Create-FS", p.createFS, p.deleteFS)
	taskflow.AddTask("Create-QoS", p.createQoS, p.deleteQoS)
	taskflow.AddTask("Create-Share", p.createShare, p.deleteShare)
	taskflow.AddTask("Allow-Share-Access", p.allowShareAccess, nil)

	err = taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
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
		params["parentid"] = params["poolID"].(string)

		_, exist := params["clonefrom"]
		if exist {
			fs, err = p.clone(params)
		} else {
			fs, err = p.cli.CreateFileSystem(params)
		}
	} else {
		if fs["ISCLONEFS"].(string) == "false" {
			return map[string]interface{}{
				"fsID": fs["ID"].(string),
			}, nil
		}

		fsID := fs["ID"].(string)
		err = p.waitFSSplitDone(fsID)
	}

	if err != nil {
		log.Errorf("Create filesystem %s error: %v", fsName, err)
		return nil, err
	}

	return map[string]interface{}{
		"fsID": fs["ID"].(string),
	}, nil
}

func (p *NAS) clone(params map[string]interface{}) (map[string]interface{}, error) {
	clonefrom := params["clonefrom"].(string)
	cloneFromFS, err := p.cli.GetFileSystemByName(clonefrom)
	if err != nil {
		log.Errorf("Get clone src filesystem %s error: %v", clonefrom, err)
		return nil, err
	}
	if cloneFromFS == nil {
		msg := fmt.Errorf("Filesystem %s does not exist", clonefrom)
		log.Errorln(msg)
		return nil, msg
	}

	srcFSCapacity, err := strconv.ParseInt(cloneFromFS["CAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	cloneFSCapacity := params["capacity"].(int64)
	if cloneFSCapacity < srcFSCapacity {
		msg := fmt.Sprintf("Clone filesystem capacity must be >= src %s", clonefrom)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	parentID := cloneFromFS["ID"].(string)
	fsName := params["name"].(string)
	allocType := params["alloctype"].(int)

	cloneFS, err := p.cli.CloneFileSystem(fsName, allocType, parentID)
	if err != nil {
		return nil, err
	}

	cloneFSID := cloneFS["ID"].(string)

	if cloneFSCapacity > srcFSCapacity {
		err := p.cli.ExtendFileSystem(cloneFSID, cloneFSCapacity)
		if err != nil {
			log.Errorf("Extend filesystem %s to capacity %d error: %v", cloneFSID, cloneFSCapacity, err)
			p.cli.DeleteFileSystem(cloneFSID)
			return nil, err
		}
	}

	cloneSpeed := params["clonespeed"].(int)
	err = p.cli.SplitCloneFS(cloneFSID, cloneSpeed)
	if err != nil {
		log.Errorf("Split filesystem %s error: %v", fsName, err)
		p.cli.DeleteFileSystem(cloneFSID)
		return nil, err
	}

	err = p.waitFSSplitDone(cloneFSID)
	if err != nil {
		return nil, err
	}

	return cloneFS, nil
}

func (p *NAS) waitFSSplitDone(fsID string) error {
	err := utils.WaitUntil(func() (bool, error) {
		fs, err := p.cli.GetFileSystemByID(fsID)
		if err != nil {
			return false, err
		}

		if fs["ISCLONEFS"] == "false" {
			return true, nil
		}

		if fs["HEALTHSTATUS"].(string) != FILESYSTEM_HEALTH_STATUS_NORMAL {
			return false, fmt.Errorf("Filesystem %s the bad healthStatus code %s", fs["NAME"], fs["HEALTHSTATUS"].(string))
		}
		splitStatus := fs["SPLITSTATUS"].(string)
		if splitStatus == FILESYSTEM_SPLIT_STATUS_QUEUING ||
			splitStatus == FILESYSTEM_SPLIT_STATUS_SPLITTING ||
			splitStatus == FILESYSTEM_SPLIT_STATUS_NOT_START {
			return false, nil
		} else if splitStatus == FILESYSTEM_SPLIT_STATUS_ABNORMAL {
			return false, fmt.Errorf("Filesystem clone %s is stopped", fs["NAME"])
		} else {
			return true, nil
		}
	}, time.Hour*6, time.Second*5)

	return err
}

func (p *NAS) deleteFS(taskResult map[string]interface{}) error {
	fsID, exist := taskResult["fsID"].(string)
	if !exist || fsID == "" {
		return nil
	}

	return p.cli.DeleteFileSystem(fsID)
}

func (p *NAS) createQoS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	fsID := taskResult["fsID"].(string)

	smartX := smartx.NewSmartX(p.cli)
	qosID, err := smartX.CreateQos(fsID, "fs", qos)
	if err != nil {
		log.Errorf("Create qos %v for fs %s error: %v", qos, fsID, err)
		return nil, err
	}

	return map[string]interface{}{
		"qosID": qosID,
	}, nil
}

func (p *NAS) deleteQoS(taskResult map[string]interface{}) error {
	fsID, fsIDExist := taskResult["fsID"].(string)
	qosID, qosIDExist := taskResult["qosID"].(string)
	if !fsIDExist || !qosIDExist {
		return nil
	}

	smartX := smartx.NewSmartX(p.cli)
	err := smartX.DeleteQos(qosID, fsID, "fs")
	return err
}

func (p *NAS) createShare(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)
	sharePath := utils.GetSharePath(fsName)
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
		"shareID": share["ID"].(string),
	}, nil
}

func (p *NAS) deleteShare(taskResult map[string]interface{}) error {
	shareID, exist := taskResult["shareID"].(string)
	if !exist || len(shareID) == 0 {
		return nil
	}

	return p.cli.DeleteNfsShare(shareID)
}

func (p *NAS) allowShareAccess(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	shareID := taskResult["shareID"].(string)
	authClient := params["authclient"].(string)

	for _, i := range strings.Split(authClient, ";") {
		access, err := p.cli.GetNfsShareAccess(shareID, i)
		if err != nil {
			log.Errorf("Get access %s of share %s error: %v", i, shareID, err)
			return nil, err
		}

		if access == nil {
			params := map[string]interface{}{
				"name":      i,
				"parentid":  shareID,
				"accessval": 1,
			}

			err := p.cli.AllowNfsShareAccess(params)
			if err != nil {
				log.Errorf("Allow nfs share access %v error: %v", params, err)
				return nil, err
			}
		}
	}

	return nil, nil
}

func (p *NAS) Delete(name string) error {
	sharePath := utils.GetSharePath(name)
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
		}
	} else {
		shareID := share["ID"].(string)
		err = p.cli.DeleteNfsShare(shareID)
		if err != nil {
			log.Errorf("Delete nfs share %s error: %v", shareID, err)
			return err
		}

		fsID := share["FSID"].(string)
		fs, err = p.cli.GetFileSystemByID(fsID)
		if err != nil {
			log.Errorf("Get filesystem by ID %s error: %v", fsID, err)
			return err
		}
	}

	fsID := fs["ID"].(string)
	qosID := fs["IOCLASSID"].(string)
	if qosID != "" {
		smartX := smartx.NewSmartX(p.cli)
		err := smartX.DeleteQos(qosID, fsID, "fs")
		if err != nil {
			log.Errorf("Remove filesystem %s from qos %s error: %v", fsID, qosID, err)
			return err
		}
	}

	isCloneFS := fs["ISCLONEFS"].(string)
	if isCloneFS == "true" {
		splitStatus := fs["SPLITSTATUS"].(string)
		if splitStatus == FILESYSTEM_SPLIT_STATUS_SPLITTING ||
			splitStatus == FILESYSTEM_SPLIT_STATUS_QUEUING {
			p.cli.StopCloneFSSplit(fsID)
		}
	}

	err = p.cli.DeleteFileSystem(fsID)
	if err != nil {
		log.Errorf("Delete filesystem %s error: %v", fsID, err)
		return err
	}

	return nil
}
