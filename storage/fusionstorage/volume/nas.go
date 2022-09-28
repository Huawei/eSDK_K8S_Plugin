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
	"strings"
	"time"

	"huawei-csi-driver/storage/fusionstorage/client"
	fsUtils "huawei-csi-driver/storage/fusionstorage/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/taskflow"
)

const (
	notSupportSnapShotSpace = 0
	spaceQuotaUnitKB        = 1
	quotaTargetFilesystem   = 1
	quotaParentFileSystem   = "40"
	directoryQuotaType      = "1"
)

const (
	allSquashString    string = "all_squash"
	noAllSquashString  string = "no_all_squash"
	rootSquashString   string = "root_squash"
	noRootSquashString string = "no_root_squash"
	visibleString      string = "visible"
	invisibleString    string = "invisible"
	allSquash                 = 0
	noAllSquash               = 1
	rootSquash                = 0
	noRootSquash              = 1
)

type NAS struct {
	cli *client.Client
}

func NewNAS(cli *client.Client) *NAS {
	return &NAS{
		cli: cli,
	}
}

func (p *NAS) preCreate(ctx context.Context, params map[string]interface{}) error {
	var err error
	authclient, exist := params["authclient"].(string)
	if !exist || authclient == "" {
		return utils.Errorln(ctx, "authclient must be provided for filesystem")
	}

	params["accountname"], exist = params["accountname"].(string)
	if !exist || params["accountname"] == "" {
		params["accountname"] = "system"
		params["accountid"] = "0"
	} else {
		params["accountid"], err = p.cli.GetAccountIdByName(ctx, params["accountname"].(string))
		if err != nil {
			return utils.Errorf(ctx, "Get account name by id failed. account name:%s, error:%v",
				params["accountname"], err)
		}
	}

	if v, exist := params["storagepool"].(string); exist {
		pool, err := p.cli.GetPoolByName(ctx, v)
		if err != nil {
			return err
		}
		if pool == nil {
			return utils.Errorf(ctx, "Storage pool %s doesn't exist", v)
		}

		params["poolId"] = int64(pool["poolId"].(float64))
	}

	name := params["name"].(string)
	params["name"] = utils.GetFileSystemName(name)

	if v, exist := params["clonefrom"].(string); exist {
		params["clonefrom"] = utils.GetFileSystemName(v)
	}

	if v, exist := params["storagequota"].(string); exist {
		quotaParams, err := fsUtils.ExtractStorageQuotaParameters(ctx, v)
		if err != nil {
			return utils.Errorf(ctx, "extract storageQuota %s failed", v)
		}

		params["spaceQuota"] = quotaParams["spaceQuota"].(string)
		if v, exist := quotaParams["gracePeriod"]; exist {
			gracePeriod, err := utils.TransToIntStrict(ctx, v)
			if err != nil {
				return utils.Errorf(ctx, "trans %s to int type error", v)
			}
			params["gracePeriod"] = gracePeriod
		}
	}

	// all_squash  all_squash: 0  no_all_squash: 1
	val, exist := params["allsquash"].(string)
	if !exist || val == "" {
		params["allsquash"] = noAllSquash
	} else {
		if strings.EqualFold(val, noAllSquashString) {
			params["allsquash"] = noAllSquash
		} else if strings.EqualFold(val, allSquashString) {
			params["allsquash"] = allSquash
		} else {
			return utils.Errorf(ctx, "parameter allSquash [%v] in sc must be %s or %s.",
				val, allSquashString, noAllSquashString)
		}
	}

	// root_squash
	val, exist = params["rootsquash"].(string)
	if !exist || val == "" {
		params["rootsquash"] = noRootSquash
	} else {
		if strings.EqualFold(val, noRootSquashString) {
			params["rootsquash"] = noRootSquash
		} else if strings.EqualFold(val, rootSquashString) {
			params["rootsquash"] = rootSquash
		} else {
			return utils.Errorf(ctx, "parameter rootSquash [%v] in sc must be %s or %s.",
				val, rootSquashString, noRootSquashString)
		}
	}

	if val, ok := params["snapshotdirectoryvisibility"].(string); ok {
		if strings.EqualFold(val, visibleString) {
			params["isshowsnapdir"] = true
		} else if strings.EqualFold(val, invisibleString) {
			params["isshowsnapdir"] = false
		} else {
			return utils.Errorf(ctx, "parameter snapshotDirectoryVisibility [%v] in sc must be %s or %s.",
				params["snapshotdirectoryvisibility"], visibleString, invisibleString)
		}
	}

	return nil
}

func (p *NAS) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	createTask := taskflow.NewTaskFlow(ctx, "Create-FileSystem-Volume")
	createTask.AddTask("Create-FS", p.createFS, p.revertFS)
	if params["protocol"] == "dpc" {
		createTask.AddTask("Create-Quota", p.createQuota, nil)
	} else {
		createTask.AddTask("Create-Quota", p.createQuota, p.revertQuota)
		createTask.AddTask("Create-Share", p.createShare, p.revertShare)
		createTask.AddTask("Allow-Share-Access", p.allowShareAccess, nil)
	}
	_, err = createTask.Run(params)
	if err != nil {
		createTask.Revert()
		return nil, err
	}

	volObj := p.prepareVolObj(ctx, params)
	return volObj, nil
}

func (p *NAS) prepareVolObj(ctx context.Context, params map[string]interface{}) utils.Volume {
	volName, isStr := params["name"].(string)
	if !isStr {
		// Not expecting this error to happen
		log.AddContext(ctx).Warningf("Expecting string for volume name, received type %T", params["name"])
	}
	return utils.NewVolume(volName)
}

func (p *NAS) createFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName, ok := params["name"].(string)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain name field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return nil, err
	}

	if fs == nil {
		_, exist := params["clonefrom"]
		if exist {
			fs, err = p.clone(params)
		} else {
			fs, err = p.cli.CreateFileSystem(ctx, params)
		}
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Create filesystem %s error: %v", fsName, err)
		return nil, err
	}

	err = p.waitFilesystemCreated(ctx, fsName)
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

func (p *NAS) revertFS(ctx context.Context, taskResult map[string]interface{}) error {
	fsID, exist := taskResult["fsID"].(string)
	if !exist {
		return nil
	}
	return p.deleteFS(ctx, fsID)
}

func (p *NAS) deleteFS(ctx context.Context, fsID string) error {
	err := p.cli.DeleteFileSystem(ctx, fsID)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete filesystem %s error: %v", fsID, err)
	}

	return err
}

func (p *NAS) createQuota(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsID, ok := taskResult["fsID"].(string)
	if !ok {
		msg := fmt.Sprintf("Task %v does not contain fsID field.", taskResult)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	quota, err := p.cli.GetQuotaByFileSystem(ctx, fsID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s quota error: %v", fsID, err)
		return nil, err
	}

	if quota == nil {
		quotaParams := map[string]interface{}{
			"parent_id":              fsID,
			"parent_type":            quotaParentFileSystem,
			"quota_type":             directoryQuotaType,
			"snap_space_switch":      notSupportSnapShotSpace,
			"space_unit_type":        spaceQuotaUnitKB,
			"directory_quota_target": quotaTargetFilesystem,
		}

		capacity, ok := params["capacity"].(int64)
		if !ok {
			return nil, utils.Errorf(ctx, "The params %v does not contain capacity.", params)
		}

		if v, exist := params["spaceQuota"].(string); exist && v == "softQuota" {
			quotaParams["space_soft_quota"] = capacity
		} else {
			quotaParams["space_hard_quota"] = capacity
		}

		if v, exist := params["gracePeriod"].(int); exist {
			quotaParams["soft_grace_time"] = v
		}

		err := p.cli.CreateQuota(ctx, quotaParams)
		if err != nil {
			log.AddContext(ctx).Errorf("Create filesystem quota %v error: %v", quotaParams, err)
			return nil, err
		}
	}

	return nil, nil
}

func (p *NAS) revertQuota(ctx context.Context, taskResult map[string]interface{}) error {
	fsID, exist := taskResult["fsID"].(string)
	if !exist {
		return nil
	}
	return p.deleteQuota(ctx, fsID)
}

func (p *NAS) deleteQuota(ctx context.Context, fsID string) error {
	quota, err := p.cli.GetQuotaByFileSystem(ctx, fsID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s quota error: %v", fsID, err)
		return err
	}

	if quota != nil {
		quotaId, ok := quota["id"].(string)
		if !ok {
			msg := fmt.Sprintf("Quota %v does not contain id field.", quota)
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		err := p.cli.DeleteQuota(ctx, quotaId)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete filesystem quota %s error: %v", quotaId, err)
			return err
		}
	}

	return nil
}

func (p *NAS) createShare(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName, ok := params["name"].(string)
	if !ok {
		msg := fmt.Sprintf("Parameter %v does not contain name field.", params)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	accountId, ok := params["accountid"].(string)
	if !ok {
		return nil, utils.Errorf(ctx, "Parameter %v does not contain accountId.", params)
	}

	sharePath := utils.GetFSSharePath(fsName)
	share, err := p.cli.GetNfsShareByPath(ctx, sharePath, accountId)

	if err != nil {
		log.AddContext(ctx).Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return nil, err
	}

	if share == nil {
		shareParams := map[string]interface{}{
			"sharepath":   sharePath,
			"fsid":        taskResult["fsID"].(string),
			"description": "Created from Kubernetes Provisioner",
			"accountid":   params["accountid"].(string),
		}

		share, err = p.cli.CreateNfsShare(ctx, shareParams)
		if err != nil {
			log.AddContext(ctx).Errorf("Create nfs share %v error: %v", shareParams, err)
			return nil, err
		}
	}
	return map[string]interface{}{
		"shareID":   share["id"].(string),
		"accountId": accountId,
	}, nil
}

func (p *NAS) waitFilesystemCreated(ctx context.Context, fsName string) error {
	err := utils.WaitUntil(func() (bool, error) {
		fs, err := p.cli.GetFileSystemByName(ctx, fsName)
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

func (p *NAS) revertShare(ctx context.Context, taskResult map[string]interface{}) error {
	shareID, exist := taskResult["shareID"].(string)
	if !exist {
		return nil
	}
	accountId, exist := taskResult["accountId"].(string)
	return p.deleteShare(ctx, shareID, accountId)
}

func (p *NAS) deleteShare(ctx context.Context, shareID, accountId string) error {
	err := p.cli.DeleteNfsShare(ctx, shareID, accountId)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete share %s error: %v", shareID, err)
		return err
	}

	return nil
}

func (p *NAS) allowShareAccess(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {

	allowNfsShareAccessReq := &client.AllowNfsShareAccessRequest{
		AccessName:  params["authclient"].(string),
		ShareId:     taskResult["shareID"].(string),
		AccessValue: 1,
		AllSquash:   params["allsquash"].(int),
		RootSquash:  params["rootsquash"].(int),
		AccountId:   params["accountid"].(string),
	}

	err := p.cli.AllowNfsShareAccess(ctx, allowNfsShareAccessReq)
	if err != nil {
		log.AddContext(ctx).Errorf("Allow nfs share access %v error: %v", allowNfsShareAccessReq, err)
		return nil, err
	}

	return nil, nil
}

func (p *NAS) Delete(ctx context.Context, name string) error {
	fsName := utils.GetFileSystemName(name)
	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}
	// if filesystem not exist, it means quota, share have been deleted.
	if fs == nil {
		log.AddContext(ctx).Infof("Filesystem %s to delete does not exist", fsName)
		return nil
	}

	fsID := strconv.FormatInt(int64(fs["id"].(float64)), 10)
	accountId := fs["account_id"].(string)
	sharePath := utils.GetFSSharePath(name)
	share, err := p.cli.GetNfsShareByPath(ctx, sharePath, accountId)
	if err != nil {
		log.AddContext(ctx).Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return err
	}
	if share == nil {
		log.AddContext(ctx).Infof("Share %s to delete does not exist, continue to delete filesystem", sharePath)
		err = p.deleteQuota(ctx, fsID)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete filesystem %s quota error: %v", fsID, err)
			return err
		}

		err = p.deleteFS(ctx, fsID)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete filesystem %s error: %v", fsID, err)
			return err
		}
	} else {
		shareID := share["id"].(string)
		err = p.cli.DeleteNfsShare(ctx, shareID, accountId)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete nfs share %s error: %v", shareID, err)
			return err
		}

		fsID := share["file_system_id"].(string)
		err = p.deleteQuota(ctx, fsID)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete filesystem %s quota error: %v", fsID, err)
			return err
		}

		err = p.deleteFS(ctx, fsID)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete filesystem %s error: %v", fsID, err)
			return err
		}
	}
	return nil
}
