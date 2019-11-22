package volume

import (
	"errors"
	"fmt"
	"storage/oceanstor/client"
	"storage/oceanstor/smartx"
	"strconv"
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
	cli *client.Client
}

var taskStatusCache = make(map[string]bool)

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
	return nil
}

func (p *NAS) Create(params map[string]interface{}) error {
	name := params["name"].(string)
	if taskStatusCache[name] {
		delete(taskStatusCache, name)
		return nil
	}
	taskflow := taskflow.NewTaskFlow("Create-FileSystem-Volume")
	if product, exist := params["product"]; exist && product == "9000" {
		taskflow.AddTask("Create-FS", p.createFS9000, p.deleteFS9000)
		taskflow.AddTask("Create-Share", p.createShare9000, p.deleteShare9000)
		taskflow.AddTask("Create-Quota", p.createQuota9000, p.deleteQuota9000)
		taskflow.AddTask("Allow-Share-Access", p.allowShareAccess9000, nil)
	} else {
		taskflow.AddTask("Create-FS", p.createFS, p.deleteFS)
		taskflow.AddTask("Create-QoS", p.createQoS, p.deleteQoS)
		taskflow.AddTask("Create-Share", p.createShare, p.deleteShare)
		taskflow.AddTask("Allow-Share-Access", p.allowShareAccess, nil)
	}
	err := taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return err
	}
	taskStatusCache[name] = true
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

	return map[string]interface{}{
		"fsID": fs["ID"].(string),
	}, nil
}

func (p *NAS) clone(params map[string]interface{}) (map[string]interface{}, error) {
	cloneFrom := params["clonefrom"].(string)
	cloneFrom = utils.GetFileSystemName(cloneFrom)
	cloneFromFS, err := p.cli.GetFileSystemByName(cloneFrom)
	if err != nil {
		log.Errorf("Get clone src filesystem %s error: %v", cloneFrom, err)
		return nil, err
	}
	if cloneFromFS == nil {
		msg := fmt.Errorf("Filesystem %s does not exist", cloneFrom)
		log.Errorln(msg)
		return nil, msg
	}

	srcFSCapacity, err := strconv.ParseInt(cloneFromFS["CAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	cloneFSCapacity := params["capacity"].(int64)
	if cloneFSCapacity < srcFSCapacity {
		msg := fmt.Sprintf("Clone filesystem capacity must be >= src %s", cloneFrom)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	parentID := cloneFromFS["ID"].(string)
	fsName := params["name"].(string)
	allocType := params["alloctype"].(int)
	cloneFS, err := p.cli.GetFileSystemByName(fsName)
	if err != nil {
		return nil, err
	}
	if cloneFS == nil {
		cloneFS, err = p.cli.CloneFileSystem(fsName, allocType, parentID)
		if err != nil {
			return nil, err
		}
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
	cloneSpeed := params["clonespeed"].(string)
	err = p.cli.SplitCloneFS(cloneFSID, cloneSpeed)
	if err != nil {
		log.Errorf("Split filesystem %s error: %v", fsName, err)
		p.cli.DeleteFileSystem(cloneFSID)
		return nil, err
	}
	err = p.loopFsStatus(cloneFSID)
	if err != nil {
		return nil, err
	}
	return cloneFS, nil
}

func (p *NAS) loopFsStatus(fsID string) error {
	err := utils.WaitUntil(func() (bool, error) {
		fs, err := p.cli.GetFileSystemByID(fsID)
		if err != nil {
			return false, err
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
	fsID := taskResult["fsID"]
	if fsID == nil {
		return nil
	}
	err := p.cli.DeleteFileSystem(fsID.(string))
	return err
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

func (p *NAS) fsIsOk(fsName string) error {
	err := utils.WaitUntil(func() (bool, error) {
		fs, err := p.cli.GetFileSystemByName(fsName)
		if err != nil {
			return false, err
		}
		if fs["RUNNINGSTATUS"] == "27" { //filesystem is ok
			return true, nil
		} else {
			return false, nil
		}
	}, time.Hour*6, time.Second*5)
	return err
}

func (p *NAS) deleteQoS(taskResult map[string]interface{}) error {
	fsID, fsIDExist := taskResult["fsID"].(string)
	qosID, qosIDExist := taskResult["qosID"].(string)
	if fsIDExist && qosIDExist {
		smartX := smartx.NewSmartX(p.cli)
		err := smartX.DeleteQos(qosID, fsID, "fs")
		return err
	}
	return nil

}

func (p *NAS) createShare(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)
	err := p.fsIsOk(fsName)
	if err !=nil {
		return nil, err
	}
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
	if exist {
		err := p.cli.DeleteNfsShare9000(shareID)
		return err
	}
	return nil

}

func (p *NAS) allowShareAccess(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	shareID := taskResult["shareID"].(string)
	authClient := params["authclient"].(string)

	count, err := p.cli.GetNfsShareAccessCount(shareID)
	if err != nil {
		log.Errorf("Get access count of share %s error: %v", shareID, err)
		return nil, err
	}

	var existAccess map[string]interface{}

	for i := int64(0); i < count; i += 100 {
		start, end := i, i+100
		accesses, err := p.cli.GetNfsShareAccessRange(shareID, start, end)
		if err != nil {
			log.Errorf("Get accesses of share %s range %d:%d error: %v", shareID, start, end, err)
			return nil, err
		}

		for _, j := range accesses {
			access := j.(map[string]interface{})
			name := access["NAME"].(string)

			if name == authClient {
				existAccess = access
			} else {
				accessID := access["ID"].(string)
				err := p.cli.DeleteNfsShareAccess(accessID)
				if err != nil {
					log.Warningf("Delete access %s of share %s error: %v", accessID, shareID, err)
				}
			}
		}
	}

	if existAccess == nil {
		params := map[string]interface{}{
			"name":      authClient,
			"parentid":  shareID,
			"accessval": 1,
		}

		err := p.cli.AllowNfsShareAccess(params)
		if err != nil {
			log.Errorf("Allow nfs share access %v error: %v", params, err)
			return nil, err
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
func (p *NAS) createFS9000(params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	var result map[string]interface{}
	fsName := params["name"].(string)
	fs, err := p.cli.GetFilesystemByName9000(fsName)
	if err != nil {
		log.Errorf("Query directory %s error: %v", fsName, err)
		return nil, err
	}
	if fs == nil {
		fs, err = p.cli.CreateFileSystem9000(fsName)
	}
	if err != nil {
		log.Errorf("Create filesystem %s error: %v", fsName, err)
		return nil, err
	}
	result["fsName"] = fs["name"]
	return result, nil
}

func (p *NAS) deleteFS9000(taskResult map[string]interface{}) error {

	if fsName, exist := taskResult["fsName"].(string); exist {
		err := p.cli.DeleteFileSystem9000(fsName)
		return err
	}
	return nil
}

func (p *NAS) createShare9000(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)
	sharePath := fmt.Sprint("/" + fsName)
	description := fmt.Sprint("Created from Kubernetes Provisioner")
	shareExist, err := p.cli.GetNfsShareByPath9000(sharePath)
	if err != nil {
		return nil, err
	}
	if shareExist != nil {
		return shareExist, nil
	}

	share, err := p.cli.CreateNfsShare9000(sharePath, description)
	if err != nil {
		log.Errorf("Create nfs share %s error: %v", sharePath, err)
		return nil, err
	}
	shareId := share["id"].(string)
	return map[string]interface{}{
		"shareId": shareId,
	}, nil
}

func (p *NAS) deleteShare9000(taskResult map[string]interface{}) error {
	shareId, exist := taskResult["shareId"].(string)
	if exist {
		err := p.cli.DeleteNfsShare9000(shareId)
		return err
	}
	return nil

}

func (p *NAS) createQuota9000(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)
	treeName := fmt.Sprint("/" + fsName)
	quotaInfo, err := p.cli.GetFsQuotaByPath(treeName)
	if err != nil {
		return nil, err
	}
	if quotaInfo != nil {
		return map[string]interface{}{
			"quotaId": quotaInfo["id"],
		}, nil
	}
	capacity := params["capacity"].(int64)
	quotaInfo, err = p.cli.CreatFsQuota(treeName, capacity)
	if err != nil {
		log.Errorf("Create the quota of treeName %s error: %v", treeName, err)
		return nil, err
	}
	return map[string]interface{}{
		"quotaId": quotaInfo["id"],
	}, nil

}

func (p *NAS) deleteQuota9000(taskResult map[string]interface{}) error {
	quotaId, exist := taskResult["quotaId"].(string)
	if exist {
		err := p.cli.DeleteFsQuota(quotaId)
		return err
	}
	return nil

}
func (p *NAS) allowShareAccess9000(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	sharePath := fmt.Sprint("/" + params["name"].(string))
	err := p.cli.AllowNfsShareAccess9000(sharePath)
	return nil, err
}

func (p *NAS) Delete9000(name string) error {
	fsDirName := fmt.Sprint("/" + name)
	fsExist, err := p.cli.GetFilesystemByName9000(name)
	if err != nil {
		return err
	}
	if fsExist == nil {
		return nil
	}
	quota, err := p.cli.GetFsQuotaByPath(fsDirName)
	if err != nil {
		return err
	}
	if quotaId, exist := quota["id"].(string); exist {
		err = p.cli.DeleteFsQuota(quotaId)
		if err != nil {
			return err
		}
	}
	share, err := p.cli.GetNfsShareByPath9000(fsDirName)
	if err != nil {
		return err
	}
	if shareId, exist := share["id"].(string); exist {
		err = p.cli.DeleteNfsShare9000(shareId)
		if err != nil {
			return err
		}
	}
	return p.cli.DeleteFileSystem9000(name)
}
