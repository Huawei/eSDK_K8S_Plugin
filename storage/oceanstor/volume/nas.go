/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"huawei-csi-driver/pkg/constants"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/storage/oceanstor/smartx"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/taskflow"
)

const (
	allSquashString    string = "all_squash"
	noAllSquashString  string = "no_all_squash"
	rootSquashString   string = "root_squash"
	noRootSquashString string = "no_root_squash"
	visibleString      string = "visible"
	invisibleString    string = "invisible"

	allSquash    = 0
	noAllSquash  = 1
	rootSquash   = 0
	noRootSquash = 1
)

// NASHyperMetro defines HyperMetro nas storage
type NASHyperMetro struct {
	FsHyperMetroActiveSite bool
	LocVStoreID            string
	RmtVStoreID            string
}

// NAS provides base nas client
type NAS struct {
	Base
	NASHyperMetro
}

type allowNfsShareAccessParam struct {
	shareID      string
	authClient   string
	vStoreID     string
	activeClient client.BaseClientInterface
	accesses     map[string]interface{}
}

// NewNAS inits a new nas client
func NewNAS(cli, metroRemoteCli, replicaRemoteCli client.BaseClientInterface, product string, nasHyperMetro NASHyperMetro) *NAS {
	return &NAS{
		Base: Base{
			cli:              cli,
			metroRemoteCli:   metroRemoteCli,
			replicaRemoteCli: replicaRemoteCli,
			product:          product,
		},
		NASHyperMetro: nasHyperMetro,
	}
}

func (p *NAS) preCreate(ctx context.Context, params map[string]interface{}) error {
	if _, exist := params["authclient"].(string); !exist {
		msg := "authclient must be provided for filesystem"
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	err := p.commonPreCreate(ctx, params)
	if err != nil {
		return err
	}

	name, ok := params["name"].(string)
	if !ok {
		return pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	params["name"] = utils.GetFileSystemName(name)

	if v, exist := params["sourcevolumename"].(string); exist {
		params["clonefrom"] = v
	} else if v, exist := params["sourcesnapshotname"].(string); exist {
		params["fromSnapshot"] = utils.GetFSSnapshotName(v)
	} else if v, exist := params["clonefrom"].(string); exist {
		params["clonefrom"] = v
	}

	err = p.setWorkLoadID(ctx, p.cli, params)
	if err != nil {
		return err
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

	// convert reservedsnapshotspaceratio to int
	if val, exist := params["reservedsnapshotspaceratio"].(string); exist {
		intVal, err := strconv.Atoi(val)
		if err != nil {
			return err
		}
		params["reservedsnapshotspaceratio"] = intVal
	}

	return nil
}

// Create creates fs volume
func (p *NAS) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	taskflow := taskflow.NewTaskFlow(ctx, "Create-FileSystem-Volume")

	replication, replicationOK := params["replication"].(bool)
	hyperMetro, hyperMetroOK := params["hypermetro"].(bool)

	if (replicationOK && replication) && (hyperMetroOK && hyperMetro) {
		msg := "cannot create replication and hypermetro for a volume at the same time"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	} else if replicationOK && replication {
		taskflow.AddTask("Get-Replication-Params", p.getReplicationParams, nil)
	} else if hyperMetroOK && hyperMetro {
		taskflow.AddTask("Get-HyperMetro-Params", p.getHyperMetroParams, nil)
	}

	taskflow.AddTask("Create-Local-FS", p.createLocalFS, p.revertLocalFS)
	if replicationOK && replication {
		taskflow.AddTask("Create-Remote-FS", p.createRemoteFS, p.revertRemoteFS)
		taskflow.AddTask("Create-Remote-QoS", p.createRemoteQoS, p.revertRemoteQoS)
		taskflow.AddTask("Create-Replication-Pair", p.createReplicationPair, nil)
	} else if hyperMetroOK && hyperMetro {
		taskflow.AddTask("Set-HyperMetro-ActiveClient", p.setActiveClient, nil)
		taskflow.AddTask("Create-Remote-FS", p.createRemoteFS, p.revertRemoteFS)
		taskflow.AddTask("Create-Remote-QoS", p.createRemoteQoS, p.revertRemoteQoS)
		taskflow.AddTask("Create-HyperMetro", p.createHyperMetro, p.revertHyperMetro)
	}

	if skipShare, exist := params["skipNfsShareAndQos"].(bool); !exist || !skipShare {
		taskflow.AddTask("Create-Share", p.createShare, p.revertShare)
		taskflow.AddTask("Allow-Share-Access", p.allowShareAccess, p.revertShareAccess)
		taskflow.AddTask("Create-QoS", p.createLocalQoS, p.revertLocalQoS)
	}

	params["localVStoreID"] = p.LocVStoreID
	params["remoteVStoreID"] = p.RmtVStoreID
	_, err = taskflow.Run(params)
	if err != nil {
		// In order to prevent residue from being left in the event of a creation failure (If the deletion
		// operation fails for the first time and the deletion operation is delivered for the second time,
		// the CSI does not receive the deletion request, but the storage create)
		taskflow.Revert()
		return nil, err
	}

	volObj := p.prepareVolObj(ctx, params, nil)
	return volObj, nil
}

func (p *NAS) validateManage(ctx context.Context, params, fs map[string]interface{}) error {
	return p.validateManageWorkLoadType(ctx, params, fs)
}

func (p *NAS) validateManageWorkLoadType(ctx context.Context, params, fs map[string]interface{}) error {
	err := p.setWorkLoadID(ctx, p.cli, params)
	if err != nil {
		return err
	}

	newWorkloadTypeID, ok := params["workloadTypeID"].(string)
	if !ok {
		return nil
	}

	oldWorkloadTypeID, ok := fs["workloadTypeId"].(string)
	if !ok {
		return nil
	}

	if newWorkloadTypeID != oldWorkloadTypeID {
		return fmt.Errorf(" the workload type is different between new [%s] and old [%s].",
			newWorkloadTypeID, oldWorkloadTypeID)
	}

	return nil
}

func (p *NAS) createLocalFS(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {

	fsName, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return nil, err
	}

	var isClone bool
	if fs == nil {
		params["parentid"] = params["poolID"]
		params["vstoreId"] = params["localVStoreID"]

		if _, exist := params["clonefrom"]; exist {
			fs, err = p.clone(ctx, params)
			if err != nil {
				log.AddContext(ctx).Warningf("p.clone() failed, param:%+v", params)
			}
			isClone = true
		} else if _, exist := params["fromSnapshot"]; exist {
			fs, err = p.createFromSnapshot(ctx, params)
			if err != nil {
				log.AddContext(ctx).Warningf("p.createFromSnapshot() failed, param:%+v", params)
			}
			isClone = true
		} else {
			fs, err = p.cli.CreateFileSystem(ctx, params)
			if err != nil {
				log.AddContext(ctx).Warningf("CreateFileSystem() failed, param:%+v", params)
			}
		}
	} else {
		if fs["ISCLONEFS"].(string) != "false" {
			fsID, ok := fs["ID"].(string)
			if !ok {
				log.AddContext(ctx).Warningf("convert fsID to string failed, data: %v", fs["ID"])
			}
			err = p.waitFSSplitDone(ctx, fsID)
		}
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Create filesystem %s error: %v", fsName, err)
		return nil, err
	}

	localFSID, ok := fs["ID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert localFSID to string failed, data: %v", fs["ID"])
	}
	if err = p.updateFileSystem(ctx, isClone, localFSID, params); err != nil {
		log.AddContext(ctx).Errorf("Update filesystem %s error: %v", fsName, err)
		return nil, err
	}

	return map[string]interface{}{
		"localFSID": localFSID,
	}, nil
}

func (p *NAS) updateFileSystem(ctx context.Context, isClone bool, objID string, params map[string]interface{}) error {
	if !isClone {
		return nil
	}

	log.AddContext(ctx).Infof("The fileSystem %s is cloned, now to update some fields.",
		params["name"].(string))
	data := make(map[string]interface{})
	if val, exist := params["reservedsnapshotspaceratio"].(int); exist {
		data["SNAPSHOTRESERVEPER"] = val
	}

	if val, exist := params["isshowsnapdir"].(bool); exist {
		data["ISSHOWSNAPDIR"] = val
	}

	if val, exist := params["description"].(string); exist {
		data["DESCRIPTION"] = val
	}

	if data == nil {
		log.AddContext(ctx).Infof("The fileSystem %s is cloned, but no field need to update.",
			params["name"].(string))
		return nil
	}

	// Only update the local FS, the remote FS is created separately, no need to update
	err := p.cli.UpdateFileSystem(ctx, objID, data)
	if err != nil {
		log.AddContext(ctx).Errorf("Update FileSystem %s field [%v], error: %v", objID, data, err)
		return err
	}
	return nil
}

func (p *NAS) clone(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	clonefrom, ok := params["clonefrom"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert clonefrom to string failed, data: %v", params["clonefrom"])
	}
	cloneFromFS, err := p.cli.GetFileSystemByName(ctx, clonefrom)
	if err != nil {
		log.AddContext(ctx).Errorf("Get clone src filesystem %s error: %v", clonefrom, err)
		return nil, err
	}
	if cloneFromFS == nil {
		msg := fmt.Errorf("Filesystem %s does not exist", clonefrom)
		log.AddContext(ctx).Errorln(msg)
		return nil, msg
	}

	srcFSCapacity, err := strconv.ParseInt(cloneFromFS["CAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	cloneFSCapacity, ok := params["capacity"].(int64)
	if !ok {
		log.AddContext(ctx).Warningf("convert cloneFSCapacity to int64 failed, data: %v", params["capacity"])
	}
	if cloneFSCapacity < srcFSCapacity {
		msg := fmt.Sprintf("Clone filesystem capacity must be >= src %s", clonefrom)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	cloneFilesystemReq := &CloneFilesystemRequest{
		FsName:               params["name"].(string),
		ParentID:             cloneFromFS["ID"].(string),
		ParentSnapshotID:     "",
		AllocType:            params["alloctype"].(int),
		CloneSpeed:           params["clonespeed"].(int),
		CloneFsCapacity:      cloneFSCapacity,
		SrcCapacity:          srcFSCapacity,
		DeleteParentSnapshot: true,
		VStoreId:             systemVStore,
	}
	cloneFS, err := p.cloneFilesystem(ctx, cloneFilesystemReq)
	if err != nil {
		log.AddContext(ctx).Errorf("Clone filesystem %s from source filesystem %s error: %s",
			cloneFilesystemReq.FsName, cloneFilesystemReq.ParentID, err)
		return nil, err
	}

	return cloneFS, nil
}

func (p *NAS) createFromSnapshot(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	srcSnapshotName, ok := params["fromSnapshot"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert srcSnapshotName to string failed, data: %v", params["fromSnapshot"])
	}
	snapshotParentId, ok := params["snapshotparentid"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert snapshotParentId to string failed, data: %v", params["snapshotparentid"])
	}
	srcSnapshot, err := p.cli.GetFSSnapshotByName(ctx, snapshotParentId, srcSnapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get src filesystem snapshot %s error: %v", srcSnapshotName, err)
		return nil, err
	}
	if srcSnapshot == nil {
		msg := fmt.Errorf("src snapshot %s does not exist", srcSnapshotName)
		log.AddContext(ctx).Errorln(msg)
		return nil, msg
	}

	parentName, ok := srcSnapshot["PARENTNAME"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert parentName to string failed, data: %v", srcSnapshot["PARENTNAME"])
	}
	parentFS, err := p.cli.GetFileSystemByName(ctx, parentName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get clone src filesystem %s error: %v", parentName, err)
		return nil, err
	}
	if parentFS == nil {
		msg := fmt.Errorf("Filesystem %s does not exist", parentName)
		log.AddContext(ctx).Errorln(msg)
		return nil, msg
	}

	srcSnapshotCapacity, err := strconv.ParseInt(parentFS["CAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	cloneFilesystemReq := &CloneFilesystemRequest{
		FsName:               params["name"].(string),
		ParentID:             srcSnapshot["PARENTID"].(string),
		ParentSnapshotID:     srcSnapshot["ID"].(string),
		AllocType:            params["alloctype"].(int),
		CloneSpeed:           params["clonespeed"].(int),
		CloneFsCapacity:      params["capacity"].(int64),
		SrcCapacity:          srcSnapshotCapacity,
		DeleteParentSnapshot: false,
		VStoreId:             systemVStore,
	}
	cloneFS, err := p.cloneFilesystem(ctx, cloneFilesystemReq)
	if err != nil {
		log.AddContext(ctx).Errorf("Clone filesystem %s from source snapshot %s error: %s",
			cloneFilesystemReq.FsName, cloneFilesystemReq.ParentSnapshotID, err)
		return nil, err
	}

	return cloneFS, nil
}

func (p *NAS) cloneFilesystem(ctx context.Context, req *CloneFilesystemRequest) (map[string]interface{}, error) {
	cloneFS, err := p.cli.CloneFileSystem(ctx, req.FsName, req.AllocType, req.ParentID, req.ParentSnapshotID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create cloneFilesystem failed. source filesystem ID [%s], error: [%v]",
			req.ParentID, err)
		return nil, err
	}

	cloneFSID, ok := cloneFS["ID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert cloneFSID to string failed, data: %v", cloneFS["ID"])
	}
	if req.CloneFsCapacity > req.SrcCapacity {
		err := p.cli.ExtendFileSystem(ctx, cloneFSID, req.CloneFsCapacity)
		if err != nil {
			log.AddContext(ctx).Errorf("Extend filesystem %s to capacity %d error: %v",
				cloneFSID, req.CloneFsCapacity, err)
			_ = p.cli.DeleteFileSystem(ctx, map[string]interface{}{"ID": cloneFSID})
			return nil, err
		}
	}

	vStoreId, ok := cloneFS["vstoreId"].(string)
	if ok {
		req.VStoreId = vStoreId
	}

	err = p.splitClone(ctx, cloneFSID, req)
	if err != nil {
		log.AddContext(ctx).Errorf("split clone failed. err: %v", err)
	}

	return cloneFS, nil
}

func (p *NAS) splitClone(ctx context.Context, cloneFSID string, req *CloneFilesystemRequest) error {
	err := p.cli.SplitCloneFS(ctx, cloneFSID, req.VStoreId, req.CloneSpeed, req.DeleteParentSnapshot)
	if err != nil {
		log.AddContext(ctx).Errorf("Split filesystem [%s] error: %v", req.FsName, err)
		delErr := p.cli.DeleteFileSystem(ctx, map[string]interface{}{"ID": cloneFSID})
		if delErr != nil {
			log.AddContext(ctx).Errorf("Delete filesystem [%s] error: %v", cloneFSID, err)
		}
		return err
	}

	return p.waitFSSplitDone(ctx, cloneFSID)
}

func (p *NAS) waitFSSplitDone(ctx context.Context, fsID string) error {
	return utils.WaitUntil(func() (bool, error) {
		fs, err := p.cli.GetFileSystemByID(ctx, fsID)
		if err != nil {
			return false, err
		}

		if fs["ISCLONEFS"] == "false" {
			return true, nil
		}

		if fs["HEALTHSTATUS"].(string) != filesystemHealthStatusNormal {
			return false, fmt.Errorf("filesystem %s has the bad healthStatus code %s", fs["NAME"], fs["HEALTHSTATUS"].(string))
		}

		splitStatus, ok := fs["SPLITSTATUS"].(string)
		if !ok {
			return false, pkgUtils.Errorf(ctx, "convert splitStatus to string failed, data: %v", fs["SPLITSTATUS"])
		}
		if splitStatus == filesystemSplitStatusQueuing ||
			splitStatus == filesystemSplitStatusSplitting ||
			splitStatus == filesystemSplitStatusNotStart {
			return false, nil
		} else if splitStatus == filesystemSplitStatusAbnormal {
			return false, fmt.Errorf("filesystem clone [%s] split status is interrupted, SPLITSTATUS: [%s]",
				fs["NAME"], splitStatus)
		} else {
			return true, nil
		}
	}, time.Hour*6, time.Second*5)
}

func (p *NAS) revertLocalFS(ctx context.Context, taskResult map[string]interface{}) error {
	fsID, exist := taskResult["localFSID"].(string)
	if !exist || fsID == "" {
		return nil
	}
	deleteParams := map[string]interface{}{
		"ID": fsID,
	}
	if vStoreID, _ := taskResult["localVStoreID"].(string); vStoreID != "" {
		deleteParams["vstoreId"] = vStoreID
	}
	return p.cli.DeleteFileSystem(ctx, deleteParams)
}

func (p *NAS) createLocalQoS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	activeClient := p.getActiveClient(taskResult)
	smartX := smartx.NewSmartX(activeClient)
	vStoreID := p.getVStoreID(taskResult)
	fsID := p.getActiveFsID(taskResult)
	qosID, err := smartX.CreateQos(ctx, fsID, "fs", vStoreID, qos)
	if err != nil {
		log.AddContext(ctx).Errorf("Create qos %v for fs %s error: %v", qos, fsID, err)
		return nil, err
	}

	return map[string]interface{}{
		"localQoSID": qosID,
	}, nil
}

func (p *NAS) revertLocalQoS(ctx context.Context, taskResult map[string]interface{}) error {
	fsID, fsIDExist := taskResult["localFSID"].(string)
	qosID, qosIDExist := taskResult["localQoSID"].(string)
	if !fsIDExist || !qosIDExist {
		return nil
	}

	activeClient := p.getActiveClient(taskResult)
	smartX := smartx.NewSmartX(activeClient)
	vStoreID := p.getVStoreID(taskResult)
	fsID = p.getActiveFsID(taskResult)
	return smartX.DeleteQos(ctx, qosID, fsID, "fs", vStoreID)
}

func (p *NAS) createRemoteQoS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.product == "DoradoV6" {
		return nil, nil
	}

	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	fsID, ok := taskResult["remoteFSID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsID to string failed, data: %v", taskResult["remoteFSID"])
	}
	remoteCli, ok := taskResult["remoteCli"].(client.BaseClientInterface)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert remoteCli to BaseClientInterface failed, data: %v", taskResult["remoteCli"])
	}

	smartX := smartx.NewSmartX(remoteCli)
	qosID, err := smartX.CreateQos(ctx, fsID, "fs", "", qos)
	if err != nil {
		log.AddContext(ctx).Errorf("Create qos %v for fs %s error: %v", qos, fsID, err)
		return nil, err
	}

	return map[string]interface{}{
		"remoteQoSID": qosID,
	}, nil
}

func (p *NAS) revertRemoteQoS(ctx context.Context, taskResult map[string]interface{}) error {
	if p.product == "DoradoV6" {
		return nil
	}

	fsID, fsIDExist := taskResult["remoteFSID"].(string)
	qosID, qosIDExist := taskResult["remoteQoSID"].(string)
	if !fsIDExist || !qosIDExist {
		return nil
	}
	remoteCli, ok := taskResult["remoteCli"].(client.BaseClientInterface)
	if !ok {
		return pkgUtils.Errorf(ctx, "convert remoteCli to client.BaseClientInterface failed, data: %v", taskResult["remoteCli"])
	}
	smartX := smartx.NewSmartX(remoteCli)
	return smartX.DeleteQos(ctx, qosID, fsID, "fs", "")
}

func (p *NAS) createShare(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	sharePath := utils.GetSharePath(fsName)
	activeClient := p.getActiveClient(taskResult)
	vStoreID := p.getVStoreID(taskResult)
	share, err := activeClient.GetNfsShareByPath(ctx, sharePath, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return nil, err
	}

	if share == nil {
		fsID := p.getActiveFsID(taskResult)
		shareParams := map[string]interface{}{
			"sharepath":   sharePath,
			"fsid":        fsID,
			"description": params["description"].(string),
			"vStoreID":    vStoreID,
		}

		share, err = activeClient.CreateNfsShare(ctx, shareParams)
		if err != nil {
			log.AddContext(ctx).Errorf("Create nfs share %v error: %v", shareParams, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"shareID": share["ID"].(string),
	}, nil
}

func (p *NAS) revertShare(ctx context.Context, taskResult map[string]interface{}) error {
	shareID, exist := taskResult["shareID"].(string)
	if !exist || len(shareID) == 0 {
		return nil
	}
	activeClient := p.getActiveClient(taskResult)
	vStoreID := p.getVStoreID(taskResult)
	return activeClient.DeleteNfsShare(ctx, shareID, vStoreID)
}

func (p *NAS) getCurrentShareAccess(ctx context.Context, shareID, vStoreID string, cli client.BaseClientInterface) (map[string]interface{}, error) {
	count, err := cli.GetNfsShareAccessCount(ctx, shareID, vStoreID)
	if err != nil {
		return nil, err
	}

	accesses := make(map[string]interface{})

	var i int64 = 0
	for ; i < count; i += 100 { // Query per page 100
		clients, err := cli.GetNfsShareAccessRange(ctx, shareID, vStoreID, i, i+100)
		if err != nil {
			return nil, err
		}
		if clients == nil {
			break
		}

		for _, c := range clients {
			client, ok := c.(map[string]interface{})
			if !ok {
				log.AddContext(ctx).Warningf("convert client to map failed, data: %v", c)
				continue
			}
			name, ok := client["NAME"].(string)
			if !ok {
				log.AddContext(ctx).Warningf("convert client name to string failed, data: %v", client["NAME"])
				continue
			}
			accesses[name] = c
		}
	}

	return accesses, nil
}

func (p *NAS) allowShareAccess(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	allowShareAccessParam, err := p.preShareAccessParam(ctx, params, taskResult)
	if err != nil {
		return nil, err
	}

	for _, i := range strings.Split(allowShareAccessParam.authClient, ";") {
		if _, exist := allowShareAccessParam.accesses[i]; exist {
			delete(allowShareAccessParam.accesses, i)
			continue
		}

		req := &client.AllowNfsShareAccessRequest{
			Name:        i,
			ParentID:    allowShareAccessParam.shareID,
			AccessVal:   1,
			Sync:        0,
			AllSquash:   params["allsquash"].(int),
			RootSquash:  params["rootsquash"].(int),
			VStoreID:    allowShareAccessParam.vStoreID,
			AccessKrb5:  formatKerberosParam(params["accesskrb5"]),
			AccessKrb5i: formatKerberosParam(params["accesskrb5i"]),
			AccessKrb5p: formatKerberosParam(params["accesskrb5p"]),
		}
		if err = allowShareAccessParam.activeClient.AllowNfsShareAccess(ctx, req); err != nil {
			log.AddContext(ctx).Errorf("Allow nfs share access %v failed. error: %v", req, err)
			return nil, err
		}
	}

	// Remove all other extra access
	for _, i := range allowShareAccessParam.accesses {
		access, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert access to map failed, data: %v", i)
			continue
		}
		accessID, ok := access["ID"].(string)
		if !ok {
			log.AddContext(ctx).Warningf("convert accessID to string failed, data: %v", access["ID"])
			continue
		}
		if err = allowShareAccessParam.activeClient.DeleteNfsShareAccess(ctx, accessID,
			allowShareAccessParam.vStoreID); err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessID, err)
		}
	}

	return map[string]interface{}{
		"authClient": allowShareAccessParam.authClient,
	}, nil
}

func (p *NAS) preShareAccessParam(ctx context.Context, params,
	taskResult map[string]interface{}) (*allowNfsShareAccessParam, error) {
	var res allowNfsShareAccessParam
	var err error
	var b bool
	res.shareID, b = taskResult["shareID"].(string)
	if !b {
		return nil, pkgUtils.Errorf(ctx, "convert shareID to string failed, data: %v",
			taskResult["shareID"])
	}
	res.authClient, b = params["authclient"].(string)
	if !b {
		return nil, pkgUtils.Errorf(ctx, "convert authClient to string failed, data: %v",
			params["authclient"])
	}
	res.activeClient = p.getActiveClient(taskResult)
	res.vStoreID = p.getVStoreID(taskResult)
	res.accesses, err = p.getCurrentShareAccess(ctx, res.shareID, res.vStoreID, res.activeClient)
	if err != nil {
		return nil, pkgUtils.Errorf(ctx, "Get current access of share %s error: %v", res.shareID, err)
	}
	return &res, nil
}

func (p *NAS) revertShareAccess(ctx context.Context, taskResult map[string]interface{}) error {
	shareID, ok := taskResult["shareID"].(string)
	if !ok {
		return pkgUtils.Errorf(ctx, "convert shareID to string failed, data: %v", taskResult["shareID"])
	}
	authClient, exist := taskResult["authClient"].(string)
	if !exist {
		return nil
	}

	activeClient := p.getActiveClient(taskResult)
	vStoreID := p.getVStoreID(taskResult)
	accesses, err := p.getCurrentShareAccess(ctx, shareID, vStoreID, activeClient)
	if err != nil {
		log.AddContext(ctx).Errorf("Get current access of share %s error: %v", shareID, err)
		return err
	}

	for _, i := range strings.Split(authClient, ";") {
		if _, exist := accesses[i]; !exist {
			continue
		}
		access, ok := accesses[i].(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert access to map failed, data: %v", accesses[i])
			continue
		}
		accessID, ok := access["ID"].(string)
		if !ok {
			log.AddContext(ctx).Warningf("convert accessID to string failed, data: %v", access["ID"])
			continue
		}
		err := p.cli.DeleteNfsShareAccess(ctx, accessID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessID, err)
		}
	}
	return nil
}

// Query queries volume by name
func (p *NAS) Query(ctx context.Context, fsName string, params map[string]interface{}) (utils.Volume, error) {
	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Query filesystem %s error: %v", fsName, err)
		return nil, err
	}

	if fs == nil {
		return nil, utils.Errorf(ctx, "Filesystem [%s] to query does not exist", fsName)
	}

	if err = p.validateManage(ctx, params, fs); err != nil {
		return nil, err
	}

	volObj := utils.NewVolume(fsName)

	// set the size, need to trans Sectors to Bytes
	if capacity, err := strconv.ParseInt(fs["CAPACITY"].(string), 10, 64); err == nil {
		volObj.SetSize(utils.TransK8SCapacity(capacity, 512))
	}
	if fileSystemMode, ok := fs["fileSystemMode"].(string); ok {
		volObj.SetFilesystemMode(fileSystemMode)
	}

	return volObj, nil
}

// Delete deletes volume by name
func (p *NAS) Delete(ctx context.Context, fsName string) error {
	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}
	if fs == nil {
		log.AddContext(ctx).Infof("Filesystem %s to delete does not exist", fsName)
		return nil
	}

	fsID, ok := fs["ID"].(string)
	if !ok {
		log.AddContext(ctx).Warningf("convert fsID to string failed, data: %v", fs["ID"])
	}
	fsSnapshotNum, err := p.cli.GetFSSnapshotCountByParentId(ctx, fsID)
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to get the snapshot count of filesystem %s error: %v", fsID, err)
		return err
	}

	var replicationIDs []string
	replicationIDBytes := []byte(fs["REMOTEREPLICATIONIDS"].(string))
	err = json.Unmarshal(replicationIDBytes, &replicationIDs)
	if err != nil {
		return pkgUtils.Errorf(ctx, "Unmarshal replicationIDBytes failed, err: %v", err)
	}
	var hypermetroIDs []string
	hypermetroIDBytes := []byte(fs["HYPERMETROPAIRIDS"].(string))
	err = json.Unmarshal(hypermetroIDBytes, &hypermetroIDs)
	if err != nil {
		return pkgUtils.Errorf(ctx, "Unmarshal hypermetroIDBytes failed, error: %v", err)
	}
	taskflow := taskflow.NewTaskFlow(ctx, "Delete-FileSystem-Volume")
	if len(replicationIDs) > 0 {
		if p.replicaRemoteCli == nil {
			msg := "remote client for replication is nil"
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		if fsSnapshotNum > 1 {
			msg := fmt.Sprintf("There are %d snapshots exist in filesystem %s. "+
				"Please delete the snapshots firstly", fsSnapshotNum-1, fsName)
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		taskflow.AddTask("Delete-Replication-Pair", p.deleteReplicationPair, nil)
		taskflow.AddTask("Delete-Replication-Remote-FileSystem", p.deleteReplicationRemoteFS, nil)
		taskflow.AddTask("Delete-Local-FileSystem", p.deleteLocalFS, nil)
	}

	if len(hypermetroIDs) > 0 {
		if p.metroRemoteCli == nil {
			msg := "remote client for hypermetro is nil"
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		if fsSnapshotNum > 0 {
			msg := fmt.Sprintf("There are %d snapshots exist in filesystem %s. "+
				"Please delete the snapshots firstly", fsSnapshotNum, fsName)
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		taskflow.AddTask("Set-HyperMetro-ActiveClient", p.setActiveClient, nil)
		taskflow.AddTask("Delete-HyperMetro-Share", p.deleteHyperMetroShare, nil)
		taskflow.AddTask("Delete-HyperMetro", p.deleteHyperMetro, nil)
		taskflow.AddTask("Delete-HyperMetro-Remote-FileSystem", p.deleteHyperMetroRemoteFS, nil)
		taskflow.AddTask("Delete-Local-FileSystem", p.deleteHyperMetroLocalFS, nil)
	}

	if len(replicationIDs) == 0 && len(hypermetroIDs) == 0 {
		if fsSnapshotNum > 0 {
			msg := fmt.Sprintf("There are %d snapshots exist in filesystem %s. "+
				"Please delete the snapshots firstly", fsSnapshotNum, fsName)
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}
		taskflow.AddTask("Delete-Local-FileSystem", p.deleteLocalFS, nil)
	}

	vStoreID, _ := fs["vstoreId"].(string)
	params := map[string]interface{}{
		"name":           fsName,
		"replicationIDs": replicationIDs,
		"hypermetroIDs":  hypermetroIDs,
		"localVStoreID":  vStoreID,
		"remoteVStoreID": p.RmtVStoreID,
	}

	_, err = taskflow.Run(params)
	return err
}

// Expand expands volume size
func (p *NAS) Expand(ctx context.Context, fsName string, newSize int64) error {
	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}

	if fs == nil {
		msg := fmt.Sprintf("Filesystem %s to expand does not exist", fsName)
		log.AddContext(ctx).Errorf(msg)
		return errors.New(msg)
	}

	curSize := utils.ParseIntWithDefault(fs["CAPACITY"].(string), 10, 64, 0)
	if newSize <= curSize {
		msg := fmt.Sprintf("Filesystem %s newSize %d must be greater than curSize %d", fsName, newSize, curSize)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	var replicationIDs []string
	replicationIDBytes := []byte(fs["REMOTEREPLICATIONIDS"].(string))
	err = json.Unmarshal(replicationIDBytes, &replicationIDs)
	if err != nil {
		return pkgUtils.Errorf(ctx, "Unmarshal replicationIDBytes failed, error: %v", err)
	}
	var hyperMetroIDs []string
	hyperMetroIDBytes := []byte(fs["HYPERMETROPAIRIDS"].(string))
	err = json.Unmarshal(hyperMetroIDBytes, &hyperMetroIDs)
	if err != nil {
		return pkgUtils.Errorf(ctx, "Unmarshal hyperMetroIDBytes failed, error: %v", err)
	}
	expandTask := taskflow.NewTaskFlow(ctx, "Expand-FileSystem-Volume")
	expandTask.AddTask("Expand-PreCheck-Capacity", p.preExpandCheckCapacity, nil)

	if len(replicationIDs) > 0 {
		if p.replicaRemoteCli == nil {
			msg := "remote client for replication is nil"
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}
		expandTask.AddTask("Expand-Remote-PreCheck-Capacity", p.preExpandCheckRemoteCapacity, nil)
		expandTask.AddTask("Expand-Replication-Remote-FileSystem", p.expandReplicationRemoteFS, nil)
	}

	if len(hyperMetroIDs) > 0 {
		if p.metroRemoteCli == nil {
			msg := "remote client for hypermetro is nil"
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		expandTask.AddTask("Expand-Remote-PreCheck-Capacity", p.preExpandCheckRemoteCapacity, nil)
		expandTask.AddTask("Set-HyperMetro-ActiveClient", p.setActiveClient, nil)
		expandTask.AddTask("Expand-HyperMetro-Remote-FileSystem", p.expandHyperMetroRemoteFS, nil)
	}

	expandTask.AddTask("Expand-Local-FileSystem", p.expandLocalFS, nil)
	params := map[string]interface{}{
		"name":            fsName,
		"size":            newSize,
		"expandSize":      newSize - curSize,
		"localFSID":       fs["ID"].(string),
		"localParentName": fs["PARENTNAME"].(string),
		"replicationIDs":  replicationIDs,
		"hyperMetroIDs":   hyperMetroIDs,
	}
	_, err = expandTask.Run(params)
	return err
}

func (p *NAS) preExpandCheckCapacity(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	_, err := p.Base.preExpandCheckCapacity(ctx, params, taskResult)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"localFSID": params["localFSID"].(string),
	}, nil
}

func (p *NAS) getvStorePair(ctx context.Context) (map[string]interface{}, error) {
	localvStore := p.cli.GetvStoreName()
	if localvStore == "" {
		return nil, nil
	}

	vStore, err := p.cli.GetvStoreByName(ctx, localvStore)
	if err != nil {
		return nil, err
	}
	if vStore == nil {
		msg := fmt.Sprintf("Cannot find vstore of name %s", localvStore)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	vStoreID, ok := vStore["ID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert vStoreID to string failed, data: %v", vStore["ID"])
	}

	vStorePair, err := p.cli.GetReplicationvStorePairByvStore(ctx, vStoreID)
	if err != nil {
		return nil, err
	}
	if vStorePair == nil {
		return nil, nil
	}

	if vStorePair["ROLE"] != replicationRolePrimary {
		msg := "Local role of vstore pair is not primary"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if vStorePair["RUNNINGSTATUS"] != replicationVStorePairRunningStatusNormal &&
		vStorePair["RUNNINGSTATUS"] != replicationVStorePairRunningStatusSync {
		msg := "Running status of vstore pair is abnormal"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	remotevStore, ok := vStorePair["REMOTEVSTORENAME"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert remotevStore to string failed, data: %v", vStorePair["REMOTEVSTORENAME"])
	}
	if remotevStore != p.replicaRemoteCli.GetvStoreName() {
		msg := fmt.Sprintf("Remote vstore %s does not correspond with configuration", remotevStore)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return vStorePair, nil
}

func (p *NAS) getReplicationParams(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	var vStorePairID string
	var remoteDeviceID string
	var remoteDeviceSN string

	if p.replicaRemoteCli == nil {
		msg := "remote client for replication is nil"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	remotePoolID, err := p.getRemotePoolID(ctx, params, p.replicaRemoteCli)
	if err != nil {
		return nil, err
	}

	vStorePair, err := p.getvStorePair(ctx)
	if err != nil {
		return nil, err
	}

	var ok bool
	if vStorePair != nil {
		vStorePairID, ok = vStorePair["ID"].(string)
		if !ok {
			return nil, pkgUtils.Errorf(ctx, "convert vStorePairID to string failed, data: %v", vStorePair["ID"])
		}
		remoteDeviceID, ok = vStorePair["REMOTEDEVICEID"].(string)
		if !ok {
			return nil, pkgUtils.Errorf(ctx, "convert remoteDeviceID to string failed, data: %v", vStorePair["REMOTEDEVICEID"])
		}
		remoteDeviceSN, ok = vStorePair["REMOTEDEVICESN"].(string)
		if !ok {
			return nil, pkgUtils.Errorf(ctx, "convert remoteDeviceSN to string failed, data: %v", vStorePair["REMOTEDEVICESN"])
		}
	}

	remoteSystem, err := p.replicaRemoteCli.GetSystem(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Remote device is abnormal: %v", err)
		return nil, err
	}

	if remoteDeviceID == "" {
		sn, ok := remoteSystem["ID"].(string)
		if !ok {
			return nil, pkgUtils.Errorf(ctx, "convert sn to string failed, data: %v", remoteSystem["ID"])
		}
		remoteDeviceID, err = p.getRemoteDeviceID(ctx, sn)
		if err != nil {
			return nil, err
		}
	} else if remoteDeviceSN != remoteSystem["ID"] {
		msg := fmt.Sprintf("Remote device %s of replication vstore pair is not the same as configured one %s",
			remoteDeviceSN, remoteSystem["ID"])
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	res := map[string]interface{}{
		"remotePoolID":   remotePoolID,
		"remoteCli":      p.replicaRemoteCli,
		"remoteDeviceID": remoteDeviceID,
		"resType":        40,
	}

	if vStorePairID != "" {
		res["vStorePairID"] = vStorePairID
	}

	return res, nil
}

func (p *NAS) createRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	remoteCli, ok := taskResult["remoteCli"].(client.BaseClientInterface)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert remoteCli to client.BaseClientInterface failed, data: %v", taskResult["remoteCli"])
	}

	fs, err := remoteCli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get remote filesystem %s error: %v", fsName, err)
		return nil, err
	}

	if fs == nil {
		err = p.setWorkLoadID(ctx, remoteCli, params)
		if err != nil {
			return nil, err
		}

		params["parentid"] = taskResult["remotePoolID"]
		params["vstoreId"] = params["remoteVStoreID"]
		fs, err = remoteCli.CreateFileSystem(ctx, params)
		if err != nil {
			log.AddContext(ctx).Errorf("Create remote filesystem %s error: %v", fsName, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"remoteFSID": fs["ID"].(string),
	}, nil
}

func (p *NAS) revertRemoteFS(ctx context.Context, taskResult map[string]interface{}) error {
	fsID, exist := taskResult["remoteFSID"].(string)
	if !exist || fsID == "" {
		return nil
	}
	remoteCli, ok := taskResult["remoteCli"].(client.BaseClientInterface)
	if !ok {
		return pkgUtils.Errorf(ctx, "convert remoteCli to client.BaseClientInterface failed, data: %v", taskResult["remoteCli"])
	}
	deleteParams := map[string]interface{}{
		"ID": fsID,
	}
	if vStoreID, _ := taskResult["remoteVStoreID"].(string); vStoreID != "" {
		deleteParams["vstoreId"] = vStoreID
	}
	return remoteCli.DeleteFileSystem(ctx, deleteParams)
}

func (p *NAS) deleteReplicationPair(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	replicationIDs, ok := params["replicationIDs"].([]string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert replicationIDs to []string failed, data: %v", params["replicationIDs"])
	}
	for _, pairID := range replicationIDs {
		pair, err := p.cli.GetReplicationPairByID(ctx, pairID)
		if err != nil {
			return nil, err
		}

		runningStatus, ok := pair["RUNNINGSTATUS"].(string)
		if !ok {
			return nil, pkgUtils.Errorf(ctx, "convert runningStatus to string failed, data: %v", pair["RUNNINGSTATUS"])
		}
		if runningStatus == replicationPairRunningStatusNormal ||
			runningStatus == replicationPairRunningStatusSync {
			p.cli.SplitReplicationPair(ctx, pairID)
		}

		err = p.cli.DeleteReplicationPair(ctx, pairID)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete replication pair %s error: %v", pairID, err)
			return nil, err
		}
	}

	return nil, nil
}

func (p *NAS) setActiveClient(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.product != "DoradoV6" {
		return nil, nil
	}

	var activeClient client.BaseClientInterface
	if p.FsHyperMetroActiveSite {
		activeClient = p.cli
	} else {
		activeClient = p.metroRemoteCli
	}

	res := map[string]interface{}{
		"activeClient":   activeClient,
		"localVStoreID":  p.LocVStoreID,
		"remoteVStoreID": p.RmtVStoreID,
	}
	return res, nil
}

func (p *NAS) deleteHyperMetroShare(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	activeClient := p.getActiveClient(taskResult)
	vStoreID := p.getVStoreID(taskResult)
	err := p.deleteShare(ctx, name, vStoreID, activeClient)

	return nil, err
}

func (p *NAS) deleteShare(ctx context.Context, name, vStoreID string, cli client.BaseClientInterface) error {
	sharePath := utils.GetOriginSharePath(name)
	share, err := cli.GetNfsShareByPath(ctx, sharePath, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return err
	}

	if share != nil {
		shareID, ok := share["ID"].(string)
		if !ok {
			return pkgUtils.Errorf(ctx, "convert shareID to string failed, data: %v", share["ID"])
		}
		err := cli.DeleteNfsShare(ctx, shareID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete share %s error: %v", shareID, err)
			return err
		}
	}

	return nil
}

func (p *NAS) deleteFS(ctx context.Context, fsName string, cli client.BaseClientInterface) error {
	fs, err := cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}

	if fs == nil {
		log.AddContext(ctx).Infof("Filesystem %s to delete does not exist", fsName)
		return nil
	}

	fsID, ok := fs["ID"].(string)
	if !ok {
		log.AddContext(ctx).Warningf("convert fsID to string failed, data: %v", fs["ID"])
	}
	vStoreID, _ := fs["vstoreId"].(string)
	qosID, ok := fs["IOCLASSID"].(string)
	if ok && qosID != "" {
		smartX := smartx.NewSmartX(cli)
		err := smartX.DeleteQos(ctx, qosID, fsID, "fs", vStoreID)
		if err != nil {
			log.AddContext(ctx).Errorf("Remove filesystem %s from qos %s error: %v", fsID, qosID, err)
			return err
		}
	}
	deleteParams := map[string]interface{}{
		"ID": fsID,
	}
	err = cli.DeleteFileSystem(ctx, deleteParams)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete filesystem %s error: %v", fsID, err)
		return err
	}

	return nil
}

func (p *NAS) deleteLocalFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	vStoreID := p.getVStoreID(taskResult)
	err := p.deleteShare(ctx, name, vStoreID, p.cli)
	if err != nil {
		return nil, err
	}

	return nil, p.deleteFS(ctx, name, p.cli)
}

func (p *NAS) deleteReplicationRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	vStoreID, _ := taskResult["remoteVStoreID"].(string)
	err := p.deleteShare(ctx, name, vStoreID, p.replicaRemoteCli)
	if err != nil {
		return nil, err
	}

	return nil, p.deleteFS(ctx, name, p.replicaRemoteCli)
}

func (p *NAS) deleteHyperMetroLocalFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	return nil, p.deleteFS(ctx, name, p.cli)
}

func (p *NAS) deleteHyperMetroRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	err := p.deleteFS(ctx, name, p.metroRemoteCli)

	return nil, err
}

func (p *NAS) getHyperMetroParams(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.metroRemoteCli == nil {
		msg := "remote client for hypermetro is nil"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	remotePoolID, err := p.getRemotePoolID(ctx, params, p.metroRemoteCli)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"remotePoolID": remotePoolID,
		"remoteCli":    p.metroRemoteCli,
	}, nil
}

func (p *NAS) createHyperMetro(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	vStorePairID, ok := params["vstorepairid"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert vStorePairID to string failed, data: %v", params["vstorepairid"])
	}
	localFSID, ok := taskResult["localFSID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert localFSID to string failed, data: %v", taskResult["localFSID"])
	}
	remoteFSID, ok := taskResult["remoteFSID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert remoteFSID to string failed, data: %v", taskResult["remoteFSID"])
	}
	activeClient := p.getActiveClient(taskResult)
	if activeClient != p.cli {
		localFSID, ok = taskResult["remoteFSID"].(string)
		if !ok {
			return nil, pkgUtils.Errorf(ctx, "convert localFSID to string failed, data: %v", taskResult["remoteFSID"])
		}

		remoteFSID, ok = taskResult["localFSID"].(string)
		if !ok {
			return nil, pkgUtils.Errorf(ctx, "convert remoteFSID to string failed, data: %v", taskResult["localFSID"])
		}
	}

	data := map[string]interface{}{
		"HCRESOURCETYPE": 2, // 2: file system
		"LOCALOBJID":     localFSID,
		"REMOTEOBJID":    remoteFSID,
		"SPEED":          4, // 4: highest speed
		"VSTOREPAIRID":   vStorePairID,
	}

	metroDomainID, exist := params["metroDomainID"].(string)
	if exist && metroDomainID != "" {
		data["DOMAINID"] = metroDomainID
	}

	pair, err := activeClient.CreateHyperMetroPair(ctx, data)
	if err != nil {
		log.AddContext(ctx).Errorf("Create nas hypermetro pair error: %v", err)
		return nil, err
	}

	pairID, ok := pair["ID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert pairID to string failed, data: %v", pair["ID"])
	}
	// There is no need to synchronize when use NAS Dorado V6 or OceanStor V6 HyperMetro Volume
	if p.product != constants.OceanStorDoradoV6 {
		err = activeClient.SyncHyperMetroPair(ctx, pairID)
		if err != nil {
			log.AddContext(ctx).Errorf("Sync nas hypermetro pair %s error: %v", pairID, err)
			delErr := activeClient.DeleteHyperMetroPair(ctx, pairID, true)
			if delErr != nil {
				log.AddContext(ctx).Errorf("delete hypermetro pair %s error: %v", pairID, err)
			}
			return nil, err
		}
	}

	return map[string]interface{}{
		"hyperMetroPairID": pairID,
	}, nil
}

func (p *NAS) revertHyperMetro(ctx context.Context, taskResult map[string]interface{}) error {
	pairID, exist := taskResult["hyperMetroPairID"].(string)
	if !exist {
		return nil
	}

	activeClient := p.getActiveClient(taskResult)
	pair, err := activeClient.GetHyperMetroPair(ctx, pairID)
	if err != nil {
		return err
	}

	if pair == nil {
		return nil
	}

	status, ok := pair["RUNNINGSTATUS"].(string)
	if !ok {
		log.AddContext(ctx).Warningf("convert RUNNINGSTATUS to string failed, data: %v", pair["RUNNINGSTATUS"])
	}
	if status == hyperMetroPairRunningStatusNormal ||
		status == hyperMetroPairRunningStatusToSync ||
		status == hyperMetroPairRunningStatusSyncing {
		_ = activeClient.StopHyperMetroPair(ctx, pairID)
	}

	err = p.waitHyperMetroPairDeleted(ctx, pairID, activeClient)
	if err != nil {
		log.AddContext(ctx).Errorf("Revert nas hypermetro pair %s error: %v", pairID, err)
		return err
	}
	return nil
}

func (p *NAS) deleteHyperMetro(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	hypermetroIDs, ok := params["hypermetroIDs"].([]string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert hypermetroIDs to []string failed, data: %v", params["hypermetroIDs"])
	}
	activeClient := p.getActiveClient(taskResult)
	for _, pairID := range hypermetroIDs {
		pair, err := activeClient.GetHyperMetroPair(ctx, pairID)
		if err != nil {
			return nil, err
		}

		if pair == nil {
			continue
		}

		status, ok := pair["RUNNINGSTATUS"].(string)
		if !ok {
			log.AddContext(ctx).Warningf("convert RUNNINGSTATUS to string failed, data: %v", pair["RUNNINGSTATUS"])
		}

		if status == hyperMetroPairRunningStatusNormal ||
			status == hyperMetroPairRunningStatusToSync ||
			status == hyperMetroPairRunningStatusSyncing {
			activeClient.StopHyperMetroPair(ctx, pairID)
		}

		err = p.waitHyperMetroPairDeleted(ctx, pairID, activeClient)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete nas hypermetro pair %s error: %v", pairID, err)
			return nil, err
		}
	}

	return nil, nil
}

func (p *NAS) waitHyperMetroPairDeleted(ctx context.Context, pairID string, activeClient client.BaseClientInterface) error {
	var err error
	if p.product == "DoradoV6" {
		err = activeClient.DeleteHyperMetroPair(ctx, pairID, false)
	} else {
		err = activeClient.DeleteHyperMetroPair(ctx, pairID, true)
	}
	if err != nil {
		return utils.Errorf(ctx, "Delete hyperMetro Pair failed, err: %v", err)
	}

	err = utils.WaitUntil(func() (bool, error) {
		pair, err := activeClient.GetHyperMetroPair(ctx, pairID)
		if err != nil {
			return false, err
		}

		if pair == nil {
			return true, nil
		}

		return false, nil
	}, time.Minute, time.Second)
	return err
}

func (p *NAS) setLocalFSID(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"localFSID": params["localFSID"].(string),
	}, nil
}

func (p *NAS) preExpandCheckRemoteCapacity(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	// define the client
	var cli client.BaseClientInterface
	if p.replicaRemoteCli != nil {
		cli = p.replicaRemoteCli
	} else if p.metroRemoteCli != nil {
		cli = p.metroRemoteCli
	} else {
		msg := fmt.Sprintf("remote client for replication and hypermetro are nil")
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	// check the remote pool
	remoteFsName, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert remoteFsName to string failed, data: %v", params["name"])
	}
	remoteFs, err := cli.GetFileSystemByName(ctx, remoteFsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", remoteFsName, err)
		return nil, err
	}

	if remoteFs == nil {
		msg := fmt.Sprintf("remote filesystem %s to extend does not exist", remoteFsName)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	newSize, ok := params["size"].(int64)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert newSize to int64 failed, data: %v", params["size"])
	}
	curSize, err := strconv.ParseInt(remoteFs["CAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	if newSize < curSize {
		msg := fmt.Sprintf("Remote Filesystem %s newSize %d must be greater than curSize %d",
			remoteFsName, newSize, curSize)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return map[string]interface{}{
		"remoteFSID": remoteFs["ID"].(string),
	}, nil
}

func (p *NAS) expandFS(ctx context.Context, objID string, newSize int64, cli client.BaseClientInterface) error {
	params := map[string]interface{}{
		"CAPACITY": newSize,
	}
	err := cli.UpdateFileSystem(ctx, objID, params)
	if err != nil {
		log.AddContext(ctx).Errorf("Extend FileSystem %s CAPACITY %d, error: %v", objID, newSize, err)
		return err
	}
	return nil
}

func (p *NAS) expandReplicationRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsID, ok := taskResult["remoteFSID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert remoteFSID to string failed, data: %v", taskResult["remoteFSID"])
	}
	newSize, ok := params["size"].(int64)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert newSize to int64 failed, data: %v", params["size"])
	}
	err := p.expandFS(ctx, fsID, newSize, p.replicaRemoteCli)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand replica filesystem %s error: %v", fsID, err)
		return nil, err
	}

	return nil, err
}

func (p *NAS) expandHyperMetroRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.product == "DoradoV6" {
		return nil, nil
	}

	fsID, ok := taskResult["remoteFSID"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsID to string failed, data: %v", taskResult["remoteFSID"])
	}
	newSize, ok := params["size"].(int64)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert newSize to int64 failed, data: %v", params["size"])
	}
	err := p.expandFS(ctx, fsID, newSize, p.metroRemoteCli)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand hyperMetro filesystem %s error: %v", fsID, err)
		return nil, err
	}

	return nil, err
}

func (p *NAS) expandLocalFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	newSize, ok := params["size"].(int64)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert newSize to int64 failed, data: %v", params["size"])
	}
	activeClient := p.getActiveClient(taskResult)
	fsID := p.getActiveFsID(taskResult)
	err := p.expandFS(ctx, fsID, newSize, activeClient)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand filesystem %s error: %v", fsID, err)
		return nil, err
	}
	return nil, err
}

// CreateSnapshot creates fs snapshot
func (p *NAS) CreateSnapshot(ctx context.Context, fsName, snapshotName string) (map[string]interface{}, error) {
	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem by name %s error: %v", fsName, err)
		return nil, err
	}
	if fs == nil {
		msg := fmt.Sprintf("Filesystem %s to create snapshot does not exist", fsName)
		log.AddContext(ctx).Errorf(msg)
		return nil, errors.New(msg)
	}

	fsId, ok := fs["ID"].(string)
	if !ok {
		log.AddContext(ctx).Warningf("convert fsID to string failed, data: %v", fs["ID"])
	}
	snapshot, err := p.cli.GetFSSnapshotByName(ctx, fsId, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	snapshotSize, err := strconv.ParseInt(fs["CAPACITY"].(string), 10, 64)
	if err != nil {
		log.AddContext(ctx).Errorf("parse filesystem failed. err:%v, CAPACITY: %v", err, fs["CAPACITY"])
		return nil, err
	}
	if snapshot != nil {
		log.AddContext(ctx).Infof("The snapshot %s is already exist.", snapshotName)
		return p.getSnapshotReturnInfo(snapshot, snapshotSize), nil
	}

	snapshot, err = p.cli.CreateFSSnapshot(ctx, snapshotName, fsId)
	if err != nil {
		log.AddContext(ctx).Errorf("Create snapshot %s for filesystem %s error: %v",
			snapshotName, fsId, err)
		return nil, err
	}

	return p.getSnapshotReturnInfo(snapshot, snapshotSize), nil
}

// DeleteSnapshot deletes fs snapshot
func (p *NAS) DeleteSnapshot(ctx context.Context, snapshotParentId, snapshotName string) error {
	snapshot, err := p.cli.GetFSSnapshotByName(ctx, snapshotParentId, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem snapshot by name %s error: %v", snapshotName, err)
		return err
	}

	if snapshot == nil {
		log.AddContext(ctx).Infof("Filesystem snapshot %s to delete does not exist", snapshotName)
		return nil
	}

	snapshotId, ok := snapshot["ID"].(string)
	if !ok {
		return pkgUtils.Errorf(ctx, "convert snapshotId to string failed, data: %v", snapshot["ID"])
	}
	err = p.cli.DeleteFSSnapshot(ctx, snapshotId)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete filesystem snapshot %s error: %v", snapshotId, err)
		return err
	}

	return nil
}

func (p *NAS) getActiveClient(taskResult map[string]interface{}) client.BaseClientInterface {
	activeClient, exist := taskResult["activeClient"].(client.BaseClientInterface)
	if !exist {
		activeClient = p.cli
	}
	return activeClient
}

func (p *NAS) getActiveFsID(taskResult map[string]interface{}) string {
	fsID, _ := taskResult["localFSID"].(string)
	activeClient := p.getActiveClient(taskResult)
	if activeClient != p.cli {
		fsID, _ = taskResult["remoteFSID"].(string)
	}
	return fsID
}

func (p *NAS) getVStoreID(taskResult map[string]interface{}) string {
	vStoreID, _ := taskResult["localVStoreID"].(string)
	activeClient := p.getActiveClient(taskResult)
	if activeClient != p.cli {
		vStoreID, _ = taskResult["remoteVStoreID"].(string)
	}
	return vStoreID
}
