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
	"strings"
	"time"

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

type NASHyperMetro struct {
	FsHyperMetroActiveSite bool
	LocVStoreID            string
	RmtVStoreID            string
}

type NAS struct {
	Base
	NASHyperMetro
}

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

	name := params["name"].(string)
	params["name"] = utils.GetFileSystemName(name)
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

	return nil
}

func (p *NAS) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	taskflow := taskflow.NewTaskFlow(ctx, "Create-FileSystem-Volume")

	taskflow.AddTask("Create-Local-FS", p.createLocalFS, p.revertLocalFS)

	taskflow.AddTask("Create-Share", p.createShare, p.revertShare)
	taskflow.AddTask("Allow-Share-Access", p.allowShareAccess, p.revertShareAccess)
	taskflow.AddTask("Create-QoS", p.createLocalQoS, p.revertLocalQoS)

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

func (p *NAS) createLocalFS(ctx context.Context, params, taskResult map[string]interface{}) (
	map[string]interface{}, error) {

	fsName := params["name"].(string)
	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return nil, err
	}

	if fs == nil {
		params["parentid"] = params["poolID"].(string)
		params["vstoreId"] = params["localVStoreID"].(string)
		fs, err = p.cli.CreateFileSystem(ctx, params)
	} else {
		if fs["ISCLONEFS"].(string) == "false" {
			return map[string]interface{}{
				"localFSID": fs["ID"].(string),
			}, nil
		}

		fsID := fs["ID"].(string)
		err = p.waitFSSplitDone(ctx, fsID)
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Create filesystem %s error: %v", fsName, err)
		return nil, err
	}

	return map[string]interface{}{
		"localFSID": fs["ID"].(string),
	}, nil
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

		splitStatus := fs["SPLITSTATUS"].(string)
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

	fsID := taskResult["remoteFSID"].(string)
	remoteCli := taskResult["remoteCli"].(client.BaseClientInterface)

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
	remoteCli := taskResult["remoteCli"].(client.BaseClientInterface)
	smartX := smartx.NewSmartX(remoteCli)
	return smartX.DeleteQos(ctx, qosID, fsID, "fs", "")
}

func (p *NAS) createShare(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)
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
			"description": "Created from Kubernetes Provisioner",
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
			client := c.(map[string]interface{})
			name := client["NAME"].(string)
			accesses[name] = c
		}
	}

	return accesses, nil
}

func (p *NAS) allowShareAccess(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	shareID := taskResult["shareID"].(string)
	authClient := params["authclient"].(string)
	activeClient := p.getActiveClient(taskResult)
	vStoreID := p.getVStoreID(taskResult)
	accesses, err := p.getCurrentShareAccess(ctx, shareID, vStoreID, activeClient)
	if err != nil {
		log.AddContext(ctx).Errorf("Get current access of share %s error: %v", shareID, err)
		return nil, err
	}

	for _, i := range strings.Split(authClient, ";") {
		_, exist := accesses[i]
		delete(accesses, i)

		if exist {
			continue
		}

		req := &client.AllowNfsShareAccessRequest{
			Name:       i,
			ParentID:   shareID,
			AccessVal:  1,
			Sync:       0,
			AllSquash:  params["allsquash"].(int),
			RootSquash: params["rootsquash"].(int),
			VStoreID:   vStoreID,
		}
		err := activeClient.AllowNfsShareAccess(ctx, req)
		if err != nil {
			log.AddContext(ctx).Errorf("Allow nfs share access %v failed. error: %v", req, err)
			return nil, err
		}
	}

	// Remove all other extra access
	for _, i := range accesses {
		access := i.(map[string]interface{})
		accessID := access["ID"].(string)

		err := activeClient.DeleteNfsShareAccess(ctx, accessID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessID, err)
		}
	}

	return map[string]interface{}{
		"authClient": authClient,
	}, nil
}

func (p *NAS) revertShareAccess(ctx context.Context, taskResult map[string]interface{}) error {
	shareID := taskResult["shareID"].(string)
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
		access := accesses[i].(map[string]interface{})
		accessID := access["ID"].(string)
		err := p.cli.DeleteNfsShareAccess(ctx, accessID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessID, err)
		}
	}
	return nil
}

func (p *NAS) Delete(ctx context.Context, name string) error {
	fsName := utils.GetFileSystemName(name)
	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}
	if fs == nil {
		log.AddContext(ctx).Infof("Filesystem %s to delete does not exist", fsName)
		return nil
	}

	taskflow := taskflow.NewTaskFlow(ctx, "Delete-FileSystem-Volume")
	taskflow.AddTask("Delete-Local-FileSystem", p.deleteLocalFS, nil)

	vStoreID, _ := fs["vstoreId"].(string)
	params := map[string]interface{}{
		"name":           name,
		"localVStoreID":  vStoreID,
		"remoteVStoreID": p.RmtVStoreID,
	}

	_, err = taskflow.Run(params)
	return err
}

func (p *NAS) deleteShare(ctx context.Context, name, vStoreID string, cli client.BaseClientInterface) error {
	sharePath := utils.GetSharePath(name)
	share, err := cli.GetNfsShareByPath(ctx, sharePath, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return err
	}

	if share != nil {
		shareID := share["ID"].(string)
		err := cli.DeleteNfsShare(ctx, shareID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete share %s error: %v", shareID, err)
			return err
		}
	}

	return nil
}

func (p *NAS) deleteFS(ctx context.Context, name string, cli client.BaseClientInterface) error {
	fsName := utils.GetFileSystemName(name)
	fs, err := cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}

	if fs == nil {
		log.AddContext(ctx).Infof("Filesystem %s to delete does not exist", fsName)
		return nil
	}

	fsID := fs["ID"].(string)
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
	name := params["name"].(string)
	vStoreID := p.getVStoreID(taskResult)
	err := p.deleteShare(ctx, name, vStoreID, p.cli)
	if err != nil {
		return nil, err
	}

	return nil, p.deleteFS(ctx, name, p.cli)
}

func (p *NAS) deleteReplicationRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name := params["name"].(string)
	vStoreID, _ := taskResult["remoteVStoreID"].(string)
	err := p.deleteShare(ctx, name, vStoreID, p.replicaRemoteCli)
	if err != nil {
		return nil, err
	}

	return nil, p.deleteFS(ctx, name, p.replicaRemoteCli)
}

func (p *NAS) setLocalFSID(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"localFSID": params["localFSID"].(string),
	}, nil
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
