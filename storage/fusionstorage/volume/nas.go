/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

// Package volume defines operations of fusion storage
package volume

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/fusionstorage/client"
	"huawei-csi-driver/storage/fusionstorage/smartx"
	"huawei-csi-driver/storage/fusionstorage/types"
	fsUtils "huawei-csi-driver/storage/fusionstorage/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/flow"
	"huawei-csi-driver/utils/log"
)

const (
	notSupportSnapShotSpace = 0
	spaceQuotaUnitKB        = 1
	quotaTargetFilesystem   = 1
	quotaParentFileSystem   = "40"
	directoryQuotaType      = "1"
	quotaInvalidValue       = 18446744073709552000
	waitUntilTimeout        = 6 * time.Hour
	waitUntilInterval       = 5 * time.Second
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

// NAS provides nas storage client
type NAS struct {
	cli *client.RestClient
}

// NewNAS inits a new nas client
func NewNAS(cli *client.RestClient) *NAS {
	return &NAS{
		cli: cli,
	}
}

func (p *NAS) checkAuthclient(ctx context.Context, params map[string]interface{}) error {
	authclient, exist := params["authclient"].(string)
	if !exist || authclient == "" {
		return pkgUtils.Errorln(ctx, "authclient must be provided for filesystem")
	}

	return nil
}

func (p *NAS) preProcessAccountName(ctx context.Context, params map[string]interface{}) error {
	var exist bool
	var err error
	params["accountname"], exist = params["accountname"].(string)
	if !exist || params["accountname"] == "" {
		params["accountname"] = "system"
		params["accountid"] = "0"
	} else {
		params["accountid"], err = p.cli.GetAccountIdByName(ctx, params["accountname"].(string))
		if err != nil {
			msg := fmt.Sprintf("Get account id by name failed. account name:%s, error:%v",
				params["accountname"], err)
			return pkgUtils.Errorln(ctx, msg)
		}
	}

	return nil
}

func (p *NAS) checkStoragePool(ctx context.Context, params map[string]interface{}) error {
	if poolName, exist := params["storagepool"].(string); exist {
		pool, err := p.cli.GetPoolByName(ctx, poolName)
		if err != nil {
			return pkgUtils.Errorln(ctx, fmt.Sprintf("GetPoolByName failed. error: %v", err))
		}
		if pool == nil {
			return pkgUtils.Errorln(ctx, fmt.Sprintf("Storage pool %s doesn't exist", poolName))
		}

		params["poolId"] = int64(pool["poolId"].(float64))
	}

	return nil
}

func (p *NAS) preProcessName(ctx context.Context, params map[string]interface{}) error {
	name, ok := params["name"].(string)
	if !ok {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("preCreate failed, param does not contain name: [%v]",
			params["name"]))
	}
	params["name"] = utils.GetFileSystemName(name)

	return nil
}

func (p *NAS) preProcessQuota(ctx context.Context, params map[string]interface{}) error {
	if v, exist := params["storagequota"].(string); exist {
		quotaParams, err := fsUtils.ExtractStorageQuotaParameters(ctx, v)
		if err != nil {
			return pkgUtils.Errorln(ctx, fmt.Sprintf("extract storageQuota %s failed", v))
		}

		params["spaceQuota"] = quotaParams["spaceQuota"]
		if v, exist := quotaParams["gracePeriod"]; exist {
			gracePeriod, err := utils.TransToIntStrict(ctx, v)
			if err != nil {
				return pkgUtils.Errorln(ctx, fmt.Sprintf("trans %s to int type error", v))
			}
			params["gracePeriod"] = gracePeriod
		}
	}

	return nil
}

func (p *NAS) preProcessSquash(ctx context.Context, params map[string]interface{}) error {
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
			return pkgUtils.Errorln(ctx, fmt.Sprintf("parameter allSquash [%v] in sc must be %s or %s.",
				val, allSquashString, noAllSquashString))
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
			return pkgUtils.Errorln(ctx, fmt.Sprintf("parameter rootSquash [%v] in sc must be %s or %s.",
				val, rootSquashString, noRootSquashString))
		}
	}

	return nil
}

func (p *NAS) preProcessSnapDir(ctx context.Context, params map[string]interface{}) error {
	if val, ok := params["snapshotdirectoryvisibility"].(string); ok {
		if strings.EqualFold(val, visibleString) {
			params["isshowsnapdir"] = true
		} else if strings.EqualFold(val, invisibleString) {
			params["isshowsnapdir"] = false
		} else {
			return pkgUtils.Errorln(ctx, fmt.Sprintf("parameter snapshotDirectoryVisibility [%v] in sc must be %s or %s.",
				params["snapshotdirectoryvisibility"], visibleString, invisibleString))
		}
	}

	return nil
}

var (
	funcSetForQoSValidity = map[string]func(int) bool{
		"maxMBPS": func(value int) bool {
			return value > 0 && value <= types.MaxMbpsOfConvergedQoS
		},
		"maxIOPS": func(value int) bool {
			return value > 0 && value <= types.MaxIopsOfConvergedQoS
		},
	}
)

func (p *NAS) preProcessConvergedQoS(ctx context.Context, params map[string]interface{}) error {
	if params == nil {
		log.AddContext(ctx).Infof("preProcessConvergedQoS params is nil.")
		return nil
	}

	qosStr, ok := params["qos"].(string)
	if !ok || qosStr == "" {
		delete(params, "qos")
		return nil
	}

	var qos map[string]int
	err := json.Unmarshal([]byte(qosStr), &qos)
	if err != nil {
		msg := fmt.Sprintf("Unmarshal qosStr: [%s] failed, error: %v", qosStr, err)
		return pkgUtils.Errorln(ctx, msg)
	}

	for qosKey, qosVal := range qos {
		f, exist := funcSetForQoSValidity[qosKey]
		if !exist {
			msg := fmt.Sprintf("QoS key: [%s] is invalid.", qosKey)
			return pkgUtils.Errorln(ctx, msg)
		}

		if !f(qosVal) {
			msg := fmt.Sprintf("QoS value: [%d] is invalid, QoS key: [%s].", qosVal, qosKey)
			return pkgUtils.Errorln(ctx, msg)
		}
	}

	params["qos"] = qos
	return nil
}

func (p *NAS) preCreate(ctx context.Context, params map[string]interface{}) error {
	if err := p.checkAuthclient(ctx, params); err != nil {
		return err
	}

	if err := p.preProcessAccountName(ctx, params); err != nil {
		return err
	}

	if err := p.checkStoragePool(ctx, params); err != nil {
		return err
	}

	if err := p.preProcessName(ctx, params); err != nil {
		return err
	}

	if v, exist := params["clonefrom"].(string); exist {
		params["clonefrom"] = v
	}

	if err := p.preProcessQuota(ctx, params); err != nil {
		return err
	}

	if err := p.preProcessSquash(ctx, params); err != nil {
		return err
	}

	if err := p.preProcessSnapDir(ctx, params); err != nil {
		return err
	}

	if err := p.preProcessConvergedQoS(ctx, params); err != nil {
		return err
	}

	return nil
}

// Create creates fs volume
func (p *NAS) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	createTask := flow.NewTaskFlow(ctx, "Create-FileSystem-Volume")
	createTask.AddTask("Create-FS", p.createFS, p.revertFS)
	createTask.AddTask("Create-Quota", p.createQuota, p.revertQuota)
	createTask.AddTask("Create-Converged-QoS", p.createConvergedQoS, p.revertConvergedQoS)
	if params["protocol"] != "dpc" {
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
		"fsID":   strconv.FormatInt(int64(fs["id"].(float64)), 10),
		"fsName": fsName,
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

func (p *NAS) createConvergedQoS(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {

	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	fsName, ok := taskResult["fsName"].(string)
	if !ok {
		msg := fmt.Sprintf("convert fsName: [%v] to string failed", taskResult["fsName"])
		return nil, pkgUtils.Errorln(ctx, msg)
	}
	existQosPolicyId, err := p.cli.GetQoSPolicyIdByFsName(ctx, fsName)
	if err != nil {
		return nil, err
	}

	if existQosPolicyId != types.NoQoSPolicyId {
		return map[string]interface{}{"QosPolicyId": existQosPolicyId}, nil
	}

	qosName := smartx.ConstructQosNameByCurrentTime("fs")
	req := &types.CreateConvergedQoSReq{
		QosScale: types.QosScaleNamespace,
		Name:     qosName,
		QosMode:  types.QosModeManual,
		MaxMbps:  qos["maxMBPS"],
		MaxIops:  qos["maxIOPS"],
	}
	qosID, err := p.cli.CreateConvergedQoS(ctx, req)
	if err != nil {
		return nil, err
	}

	associateReq := &types.AssociateConvergedQoSWithVolumeReq{
		QosScale:    types.QosScaleNamespace,
		ObjectName:  fsName,
		QoSPolicyID: qosID,
	}
	err = p.cli.AssociateConvergedQoSWithVolume(ctx, associateReq)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{"QosName": qosName}, nil
}

func (p *NAS) createQuota(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsID, ok := taskResult["fsID"].(string)
	if !ok {
		msg := fmt.Sprintf("Task %v does not contain fsID field.", taskResult)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	quota, err := p.cli.GetQuotaByFileSystemById(ctx, fsID)
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

func (p *NAS) revertConvergedQoS(ctx context.Context, taskResult map[string]interface{}) error {
	fsName, exist := taskResult["fsName"].(string)
	if !exist {
		return nil
	}
	return p.deleteConvergedQoSByFsName(ctx, fsName)
}

func (p *NAS) deleteConvergedQoSByFsName(ctx context.Context, fsName string) error {
	// 1. Obtains the QoS associated with the file system.
	// 2. Disassociate
	// 3. Check the number of QoS associations.
	// 4. Delete target QoS
	qosId, err := p.cli.GetQoSPolicyIdByFsName(ctx, fsName)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("GetQoSPolicyIdByFsName failed, error: %v", err))
	}

	if qosId == types.NoQoSPolicyId {
		log.AddContext(ctx).Infof("No qos associated with fs: %v", fsName)
		return nil
	}

	err = p.cli.DisassociateConvergedQoSWithVolume(ctx, fsName)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("DisassociateConvergedQoSWithVolume failed, error: %v", err))
	}

	count, err := p.cli.GetQoSPolicyAssociationCount(ctx, qosId)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("GetQoSPolicyAssociationCount failed, error: %v", err))
	}
	if count != 0 {
		log.AddContext(ctx).Warningf("The Converged Qos %d associate objs count: %d. Please delete QoS manually",
			qosId, count)
		return nil
	}

	qosName, err := p.cli.GetConvergedQoSNameByID(ctx, qosId)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("GetConvergedQoSNameByID failed, error: %v", err))
	}
	err = p.cli.DeleteConvergedQoS(ctx, qosName)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("DeleteConvergedQoS failed, error: %v", err))
	}

	return nil
}

func (p *NAS) revertQuota(ctx context.Context, taskResult map[string]interface{}) error {
	fsID, exist := taskResult["fsID"].(string)
	if !exist {
		return nil
	}
	return p.deleteQuota(ctx, fsID)
}

func (p *NAS) deleteQuota(ctx context.Context, fsID string) error {
	quota, err := p.cli.GetQuotaByFileSystemById(ctx, fsID)
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
		if fs["running_status"].(float64) == 0 { // filesystem is ok
			return true, nil
		} else {
			return false, nil
		}
	}, waitUntilTimeout, waitUntilInterval)
	return err
}

func (p *NAS) revertShare(ctx context.Context, taskResult map[string]interface{}) error {
	shareID, exist := taskResult["shareID"].(string)
	if !exist {
		log.AddContext(ctx).Warningf("convert shareID to string failed, data: %v", taskResult["shareID"])
		return nil
	}
	accountId, exist := taskResult["accountId"].(string)
	if !exist {
		log.AddContext(ctx).Warningf("convert accountID to string failed, data: %v", taskResult["accountId"])
		return nil
	}
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

// Query queries volume by name
func (p *NAS) Query(ctx context.Context, fsName string) (utils.Volume, error) {
	quota, err := p.cli.GetQuotaByFileSystemName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return nil, err
	}

	return p.setSize(ctx, fsName, quota)
}

func (p *NAS) setSize(ctx context.Context, fsName string, quota map[string]interface{}) (utils.Volume, error) {
	volObj := utils.NewVolume(fsName)
	var capacity int64
	if hardSize, exits := quota["space_hard_quota"].(float64); exits && hardSize != quotaInvalidValue {
		capacity = int64(hardSize)
	} else if softSize, exits := quota["space_soft_quota"].(float64); exits && softSize != quotaInvalidValue {
		capacity = int64(hardSize)
	} else {
		msg := fmt.Sprintf("Quota %v does not contain space_hard_quota or space_soft_quota.", quota)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	spaceUnitType, exist := quota["space_unit_type"].(float64)
	if !exist {
		return nil, utils.Errorln(ctx, "Quota %v does not contain space_unit_type.")
	}
	volObj.SetSize(utils.TransK8SCapacity(capacity, int64(math.Pow(1024, spaceUnitType))))
	return volObj, nil
}

// DeleteNfsShare deletes nfs share
func (p *NAS) DeleteNfsShare(ctx context.Context, fsName, accountId string) (string, error) {
	sharePath := utils.GetOriginSharePath(fsName)
	share, err := p.cli.GetNfsShareByPath(ctx, sharePath, accountId)
	if err != nil {
		log.AddContext(ctx).Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return "", err
	}
	if share == nil {
		log.AddContext(ctx).Infof("Share %s to delete does not exist, continue to delete filesystem", sharePath)
		return "", nil
	}

	shareID, ok := share["id"].(string)
	if !ok {
		return "", pkgUtils.Errorln(ctx, fmt.Sprintf("convert id: [%v] to string failed.", share["id"]))
	}
	err = p.cli.DeleteNfsShare(ctx, shareID, accountId)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete nfs share %s error: %v", shareID, err)
		return "", err
	}

	fsIdInShare, ok := share["file_system_id"].(string)
	if !ok {
		return "", pkgUtils.Errorln(ctx, fmt.Sprintf("convert share[\"file_system_id\"] to string failed, val: %v",
			share["file_system_id"]))
	}

	return fsIdInShare, nil
}

// Delete deletes volume by name
func (p *NAS) Delete(ctx context.Context, fsName string) error {
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
	accountId, ok := fs["account_id"].(string)
	if !ok {
		return pkgUtils.Errorf(ctx, "convert accountID to string failed, data: %v", fs["account_id"])
	}
	fsIdInShare, err := p.DeleteNfsShare(ctx, fsName, accountId)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("DeleteNfsShare failed, err: %v", err))
	}
	if fsIdInShare != "" {
		fsID = fsIdInShare
	}

	err = p.deleteQuota(ctx, fsID)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("Delete filesystem %s quota error: %v", fsID, err))
	}

	err = p.deleteConvergedQoSByFsName(ctx, fsName)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("Delete filesystem %s qos error: %v", fsID, err))
	}

	err = p.deleteFS(ctx, fsID)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("Delete filesystem %s error: %v", fsID, err))
	}

	return nil
}

// Expand expands volume size
func (p *NAS) Expand(ctx context.Context, fsName string, newSize int64) error {
	quota, err := p.cli.GetQuotaByFileSystemName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("query quota error: %v", err)
		return err
	}
	quotaId, ok := quota["id"].(string)
	if !ok {
		msg := fmt.Sprintf("Quota %v does not contain id field.", quota)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	params := map[string]interface{}{
		"id": quotaId,
	}
	if oldHardSize, exits := quota["space_hard_quota"].(float64); exits && oldHardSize != quotaInvalidValue {
		params["space_hard_quota"] = newSize
	} else if oldSoftSize, exits := quota["space_soft_quota"].(float64); exits && oldSoftSize != quotaInvalidValue {
		params["space_soft_quota"] = newSize
	} else {
		msg := fmt.Sprintf("Quota %v does not contain space_hard_quota or space_soft_quota.", quota)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	err = p.cli.UpdateQuota(ctx, params)
	if err != nil {
		log.AddContext(ctx).Errorf("Update quota  error: %v", err)
		return err
	}
	return nil
}
