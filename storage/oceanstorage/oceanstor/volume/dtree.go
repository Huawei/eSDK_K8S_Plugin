/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	accessKrb5ReadOnly  = 0
	accessKrb5ReadWrite = 1
	accessKrb5None      = 5
	accessKrb5Default   = -1
)

// DTree provides base DTree client
type DTree struct {
	Base
}

// NewDTree inits a new DTree client
func NewDTree(cli client.OceanstorClientInterface) *DTree {
	return &DTree{
		Base: Base{
			cli:              cli,
			metroRemoteCli:   nil,
			replicaRemoteCli: nil,
			product:          "",
		},
	}
}

func (p *DTree) preCreate(ctx context.Context, params map[string]interface{}) error {
	_, flag := utils.ToStringWithFlag(params["authclient"])
	if !flag {
		msg := "authclient must be provided for filesystem"
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	err := p.setWorkLoadID(ctx, p.cli, params)
	if err != nil {
		return err
	}

	// all_squash  all_squash: 0  no_all_squash: 1
	val, exist := utils.ToStringWithFlag(params["allsquash"])
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
	val, exist = utils.ToStringWithFlag(params["rootsquash"])
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

	return nil
}

// Create creates DTree volume
func (p *DTree) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	taskFlow := flow.NewTaskFlow(ctx, "Create-FileSystem-DTree-Volume")
	taskFlow.AddTask("Check-FS", p.checkFSExist, nil)
	taskFlow.AddTask("Create-DTree", p.createDtree, p.revertDtree)
	taskFlow.AddTask("Create-Share", p.createShare, p.revertShare)
	taskFlow.AddTask("Allow-Share-Access", p.allowShareAccess, p.revertShareAccess)
	taskFlow.AddTask("Create-Quota", p.createQuota, p.revertQuota)

	_, err = taskFlow.Run(params)
	if err != nil {
		taskFlow.Revert()
		return nil, err
	}

	return p.prepareVolObj(ctx, params, nil)
}

// Delete deletes volume
func (p *DTree) Delete(ctx context.Context, params map[string]interface{}) error {
	var err error

	taskFlow := flow.NewTaskFlow(ctx, "Delete-FileSystem-DTree-Volume")
	taskFlow.AddTask("Check-DTree", p.checkDtreeExist, nil)
	taskFlow.AddTask("Delete-Quota", p.deleteQuota, nil)
	taskFlow.AddTask("Delete-Share", p.deleteShare, nil)
	taskFlow.AddTask("Delete-DTree", p.deleteDtree, nil)

	_, err = taskFlow.Run(params)
	if err != nil {
		taskFlow.Revert()
		return err
	}

	return nil
}

// Expand expands volume size
func (p *DTree) Expand(ctx context.Context, parentName, dTreeName, vstoreID string, spaceHardQuota int64) error {
	dTreeID, err := p.getDtreeID(ctx, parentName, vstoreID, dTreeName)
	if err != nil {
		return err
	}

	req := map[string]interface{}{
		"PARENTTYPE":    client.ParentTypeDTree,
		"PARENTID":      dTreeID,
		"range":         "[0-100]",
		"vstoreId":      vstoreID,
		"QUERYTYPE":     "2",
		"SPACEUNITTYPE": client.SpaceUnitTypeGB,
	}
	quotaInfos, err := p.cli.BatchGetQuota(ctx, req)
	if err != nil {
		log.AddContext(ctx).Errorf("get quota arrays failed, params: %+v, error: %v", req, err)
		return err
	}
	if len(quotaInfos) == 0 {
		log.AddContext(ctx).Infof("get empty quota arrays params: %+v", req)
		data := make(map[string]interface{})
		data["PARENTTYPE"] = client.ParentTypeDTree
		data["PARENTNAME"] = dTreeName
		data["QUOTATYPE"] = client.QuotaTypeDir
		data["SPACEUNITTYPE"] = client.SpaceUnitTypeGB
		data["SPACEHARDQUOTA"] = spaceHardQuota
		data["vstoreId"] = vstoreID
		_, err = p.cli.CreateQuota(ctx, data)
		if err != nil {
			log.AddContext(ctx).Errorf("create dtree quota failed, params: %+v, error: %v", data, err)
			return err
		}
		return nil
	}

	quotaInfo, ok := quotaInfos[0].(map[string]interface{})
	if !ok {
		log.AddContext(ctx).Errorf("quota arrays data is not valid, quotaInfos[0]: %+v", quotaInfos[0])
		return errors.New("data in response is not valid")
	}
	quotaID, _ := utils.ToStringWithFlag(quotaInfo["ID"])
	err = p.cli.UpdateQuota(ctx, quotaID, map[string]interface{}{
		"SPACEHARDQUOTA": spaceHardQuota,
		"vstoreId":       vstoreID,
	})
	if err != nil {
		log.AddContext(ctx).Errorf("update quota failed, SPACEHARDQUOTA :%v, SPACEUNITTYPE: %v vstoreId: %v, err: %v",
			spaceHardQuota, client.SpaceUnitTypeGB, vstoreID, err)
		return err
	}
	return nil
}

func (p *DTree) createDtree(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	// format request param
	data := make(map[string]interface{})
	if params["fspermission"] != nil && params["fspermission"] != "" {
		data["unixPermissions"] = params["fspermission"]
	}
	if _, ok := params["name"]; ok {
		data["NAME"] = params["name"]
	}
	if _, ok := params["parentname"]; ok {
		data["PARENTNAME"] = params["parentname"]
	}
	if _, ok := params["vstoreid"]; ok {
		data["vstoreId"] = params["vstoreid"]
	}
	data["PARENTTYPE"] = client.ParentTypeFS
	data["securityStyle"] = client.SecurityStyleUnix

	res, err := p.cli.CreateDTree(ctx, data)
	if err != nil {
		log.AddContext(ctx).Errorf("create dtree failed, params: %+v, err: %v", data, err)
		return nil, err
	}

	dTreeID, _ := utils.ToStringWithFlag(res["ID"])
	vStoreID, _ := utils.ToStringWithFlag(params["vstoreid"])

	log.AddContext(ctx).Infof("create dtree success, dtreeID: %v vstoreID: %v", dTreeID, vStoreID)

	return map[string]interface{}{
		"dTreeId":  dTreeID,
		"vstoreid": vStoreID,
	}, nil
}

func (p *DTree) checkDtreeExist(ctx context.Context, params,
	taskResult map[string]interface{}) (map[string]interface{}, error) {

	dTreeName, _ := utils.ToStringWithFlag(params["name"])
	fsName, _ := utils.ToStringWithFlag(params["parentname"])
	vStoreID, _ := utils.ToStringWithFlag(params["vstoreid"])

	dTreeInfo, err := p.cli.GetDTreeByName(ctx, "0", fsName, vStoreID, dTreeName)
	if err != nil {
		msg := fmt.Sprintf("get dtree failed, params: %+v, error:%v", params, err)
		log.AddContext(ctx).Errorf(msg)
		return nil, errors.New(msg)
	}
	if dTreeInfo == nil {
		log.AddContext(ctx).Infof("delete dtree finish, dtree not found. params :%+v", params)
		return nil, nil
	}

	return map[string]interface{}{
		"dTreeId": dTreeInfo["ID"],
	}, nil
}

func (p *DTree) checkFSExist(ctx context.Context, params,
	taskResult map[string]interface{}) (map[string]interface{}, error) {
	parentFS, _ := utils.ToStringWithFlag(params["parentname"])

	fs, err := p.cli.GetFileSystemByName(ctx, parentFS)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem by name %s error: %v", parentFS, err)
		return nil, err
	}
	if fs == nil {
		msg := fmt.Sprintf("Filesystem %s does not exist", parentFS)
		log.AddContext(ctx).Errorf(msg)
		return nil, errors.New(msg)
	}
	var fsID string
	if _, ok := fs["ID"]; ok {
		fsID, _ = utils.ToStringWithFlag(fs["ID"])
	}

	log.AddContext(ctx).Infof("parentName %s is exist", parentFS)

	return map[string]interface{}{
		"fsId": fsID,
	}, nil
}

func (p *DTree) revertDtree(ctx context.Context, taskResult map[string]interface{}) error {
	vStoreID, _ := utils.ToStringWithFlag(taskResult["vstoreid"])
	dTreeID, _ := utils.ToStringWithFlag(taskResult["dTreeId"])

	err := p.cli.DeleteDTreeByID(ctx, vStoreID, dTreeID)
	if err != nil {
		log.AddContext(ctx).Errorf("revert dtree failed, dTreeID: %v vStoreID: %v, error: %v", dTreeID, vStoreID, err)
		return err
	}

	log.AddContext(ctx).Infof("revert create dTree success,dTreeID: %s, vStoreID: %s", dTreeID, vStoreID)
	return nil
}

func (p *DTree) deleteDtree(ctx context.Context, params,
	taskResult map[string]interface{}) (map[string]interface{}, error) {

	parentName, _ := utils.ToStringWithFlag(params["parentname"])
	dTreeName, _ := utils.ToStringWithFlag(params["name"])
	vStoreID, _ := utils.ToStringWithFlag(params["vstoreid"])

	err := p.cli.DeleteDTreeByName(ctx, parentName, dTreeName, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("delete dTree failed, parentName: %s, dTreeName: %s, vStoreID: %s",
			parentName, dTreeName, vStoreID)
	}

	log.AddContext(ctx).Infof("delete create dTree success, parentName: %s, dTreeName: %s, vStoreID: %s",
		parentName, dTreeName, vStoreID)
	return nil, err
}

func (p *DTree) allowShareAccess(ctx context.Context, params, taskResult map[string]any) (map[string]any, error) {
	shareID, _ := utils.ToStringWithFlag(taskResult["shareId"])
	authClient, _ := utils.ToStringWithFlag(params["authclient"])
	vStoreID, _ := utils.ToStringWithFlag(params["vstoreid"])

	accesses, err := p.getCurrentShareAccess(ctx, shareID, vStoreID, p.cli)
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
			Name:        i,
			ParentID:    shareID,
			AccessVal:   1,
			Sync:        0,
			AllSquash:   params["allsquash"].(int),
			RootSquash:  params["rootsquash"].(int),
			VStoreID:    vStoreID,
			AccessKrb5:  formatKerberosParam(params["accesskrb5"]),
			AccessKrb5i: formatKerberosParam(params["accesskrb5i"]),
			AccessKrb5p: formatKerberosParam(params["accesskrb5p"]),
		}
		err = p.cli.AllowNfsShareAccess(ctx, req)
		if err != nil {
			log.AddContext(ctx).Errorf("Allow nfs share access %v failed. error: %v", req, err)
			return nil, err
		}
	}

	// Remove all other extra access
	for _, i := range accesses {
		access, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("allowShareAccess convert access to map failed, data: %v", i)
			continue
		}
		accessID, _ := utils.ToStringWithFlag(access["ID"])

		err = p.cli.DeleteNfsShareAccess(ctx, accessID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessID, err)
		}
	}

	return map[string]interface{}{
		"authClient": authClient,
	}, nil
}

func (p *DTree) getCurrentShareAccess(ctx context.Context, shareID, vStoreID string,
	cli client.OceanstorClientInterface) (map[string]interface{}, error) {
	count, err := cli.GetNfsShareAccessCount(ctx, shareID, vStoreID)
	if err != nil {
		return nil, err
	}

	accesses := make(map[string]interface{})

	var i int64 = 0
	for ; i < count; i += queryNfsSharePerPage {
		clients, err := cli.GetNfsShareAccessRange(ctx, shareID, vStoreID, i, i+queryNfsSharePerPage)
		if err != nil {
			return nil, err
		}
		if clients == nil {
			break
		}

		for _, c := range clients {
			clientTemp, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := utils.ToStringWithFlag(clientTemp["NAME"])
			accesses[name] = c
		}
	}

	return accesses, nil
}

func (p *DTree) revertShareAccess(ctx context.Context, taskResult map[string]interface{}) error {
	shareID, _ := utils.ToStringWithFlag(taskResult["shareId"])
	authClient, exist := utils.ToStringWithFlag(taskResult["authClient"])
	if !exist {
		return nil
	}

	vStoreID, _ := utils.ToStringWithFlag(taskResult["vstoreid"])
	accesses, err := p.getCurrentShareAccess(ctx, shareID, vStoreID, p.cli)
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
			log.AddContext(ctx).Warningf("revertShareAccess convert access to map failed, data: %v", accesses[i])
			continue
		}
		accessID, _ := utils.ToStringWithFlag(access["ID"])
		err := p.cli.DeleteNfsShareAccess(ctx, accessID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessID, err)
		}
	}
	return nil
}

func (p *DTree) deleteShareAccess(ctx context.Context, params,
	taskResult map[string]interface{}) (map[string]interface{}, error) {
	// get nfs share id

	parentName, _ := utils.ToStringWithFlag(params["parentname"])
	dTreaName, _ := utils.ToStringWithFlag(params["name"])
	vStoreID, _ := utils.ToStringWithFlag(params["vstoreid"])
	sharePath := fmt.Sprintf("/%s/%s", parentName, dTreaName)

	nfsInfo, err := p.cli.GetNfsShareByPath(ctx, sharePath, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("get nfs share failed, path: %v, vstoreID: %v,  error: %v", sharePath, vStoreID, err)
		return nil, err
	}
	if nfsInfo == nil {
		log.AddContext(ctx).Infof("delete share access finish, nfs not exist. path: %v, vstoreID: %v", sharePath, vStoreID)
		return nil, nil
	}

	shareID, _ := utils.ToStringWithFlag(nfsInfo["ID"])
	authClient, exist := utils.ToStringWithFlag(params["authclient"])
	if !exist {
		log.AddContext(ctx).Infof("delete share access finish, authClient not exists")
		return nil, nil
	}

	accesses, err := p.getCurrentShareAccess(ctx, shareID, vStoreID, p.cli)
	if err != nil {
		log.AddContext(ctx).Errorf("Get current access of share %s error: %v", shareID, err)
		return map[string]interface{}{
			"shareID": shareID,
		}, err
	}

	for _, i := range strings.Split(authClient, ";") {
		if _, exist := accesses[i]; !exist {
			continue
		}
		access, ok := accesses[i].(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("deleteShareAccess convert access to map failed, data: %v", accesses[i])
			continue
		}
		accessID, _ := utils.ToStringWithFlag(access["ID"])
		err := p.cli.DeleteNfsShareAccess(ctx, accessID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Warningf("Delete extra nfs share access %s error: %v", accessID, err)
		}
	}

	return map[string]interface{}{
		"shareID": shareID,
	}, nil
}

func (p *DTree) createQuota(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {

	var spaceHardQuota int64
	if _, ok := params["capacity"]; ok {
		spaceHardQuota, ok = params["capacity"].(int64)
		if !ok {
			log.AddContext(ctx).Errorf("get quota capacity failed, capacity: %v", params["capacity"])
			return nil, errors.New("capacity is invalid")
		}
	}

	data := make(map[string]interface{})
	data["PARENTTYPE"] = client.ParentTypeDTree
	data["PARENTID"] = taskResult["dTreeId"]
	data["QUOTATYPE"] = client.QuotaTypeDir
	data["SPACEHARDQUOTA"] = spaceHardQuota * 512
	data["vstoreId"] = params["vstoreid"]

	quota, err := p.cli.CreateQuota(ctx, data)
	if err != nil {
		log.AddContext(ctx).Errorf("create quota failed, data: %+v, err: %v", data, err)
		return nil, err
	}

	quotaID, _ := utils.ToStringWithFlag(quota["ID"])
	return map[string]interface{}{
		"quotaId": quotaID,
	}, nil

}

func (p *DTree) revertQuota(ctx context.Context, taskResult map[string]interface{}) error {
	quotaID, _ := utils.ToStringWithFlag(taskResult["quotaId"])
	vStoreID, _ := utils.ToStringWithFlag(taskResult["vstoreid"])

	err := p.cli.DeleteQuota(ctx, quotaID, vStoreID, client.ForceFlagFalse)
	if err != nil {
		log.AddContext(ctx).Errorf("revert quota failed, quotaID: %v, vStoreID: %v, err: %v", quotaID, vStoreID, err)
		return err
	}

	log.AddContext(ctx).Errorf("revert quota success, quotaID: %v, vStoreID: %v", quotaID, vStoreID)
	return nil
}

func (p *DTree) deleteQuota(ctx context.Context, params, taskResult map[string]any) (map[string]any, error) {
	req := map[string]interface{}{
		"PARENTTYPE":    client.ParentTypeDTree,
		"PARENTID":      taskResult["dTreeId"],
		"range":         "[0-100]",
		"vstoreId":      params["vstoreid"],
		"QUERYTYPE":     "2",
		"SPACEUNITTYPE": client.SpaceUnitTypeGB,
	}
	quotaInfos, err := p.cli.BatchGetQuota(ctx, req)
	if err != nil {
		log.AddContext(ctx).Errorf("get quota arrays failed, params: %+v, error: %v", req, err)
		return nil, err
	}
	if len(quotaInfos) == 0 {
		log.AddContext(ctx).Infof("get empty quota arrays params: %+v", req)
		return nil, nil
	}

	quotaInfo, ok := quotaInfos[0].(map[string]interface{})
	if !ok {
		log.AddContext(ctx).Errorf("quota arrays data is not valid, quotaInfos[0]: %+v", quotaInfos[0])
		return nil, errors.New("data in response is not valid")
	}

	vStoreID, _ := utils.ToStringWithFlag(params["vstoreid"])
	quotaID, _ := utils.ToStringWithFlag(quotaInfo["ID"])
	err = p.cli.DeleteQuota(ctx, quotaID, vStoreID, client.ForceFlagFalse)
	if err != nil {
		log.AddContext(ctx).Errorf("delete quota failed, quotaID: %v, vStoreID: %v, err: %v", quotaID, vStoreID, err)
	}
	return nil, err
}

func (p *DTree) createShare(ctx context.Context, params,
	taskResult map[string]interface{}) (map[string]interface{}, error) {

	parentName, _ := utils.ToStringWithFlag(params["parentname"])
	dTreeName, _ := utils.ToStringWithFlag(params["name"])
	vStoreID, _ := utils.ToStringWithFlag(params["vstoreid"])
	sharePath := fmt.Sprintf("/%s/%s", parentName, dTreeName)

	share, err := p.cli.GetNfsShareByPath(ctx, sharePath, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get dTree nfs share by path %s error: %v", sharePath, err)
		return nil, err
	}

	if share == nil {
		fsID, _ := utils.ToStringWithFlag(taskResult["fsId"])
		description, _ := utils.ToStringWithFlag(params["description"])
		shareParams := map[string]interface{}{
			"sharepath":   sharePath,
			"fsid":        fsID,
			"description": description,
			"vStoreID":    vStoreID,
			"DTREEID":     taskResult["dTreeId"],
		}

		share, err = p.cli.CreateNfsShare(ctx, shareParams)
		if err != nil {
			log.AddContext(ctx).Errorf("Create dTree nfs share %v error: %v", shareParams, err)
			return nil, err
		}
	}

	var shareID string
	if _, ok := share["ID"]; ok {
		shareID, _ = utils.ToStringWithFlag(share["ID"])
	}

	log.AddContext(ctx).Infof("create nfs share success, shareID: %v", shareID)

	return map[string]interface{}{
		"shareId": shareID,
	}, nil
}

func (p *DTree) revertShare(ctx context.Context, taskResult map[string]interface{}) error {
	shareID, exist := utils.ToStringWithFlag(taskResult["shareId"])
	if !exist || len(shareID) == 0 {
		log.AddContext(ctx).Errorf("revert nfs share failed, shareID not exists, shareID: %v", shareID)
		return nil
	}
	vStoreID, _ := utils.ToStringWithFlag(taskResult["vstoreid"])

	err := p.cli.DeleteNfsShare(ctx, shareID, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("revert nfs share failed, vStoreID: %v shareID: %v err: %v", vStoreID, shareID, err)
		return err
	}
	log.AddContext(ctx).Infof("revert nfs share success, vStoreID: %v shareID: %v", vStoreID, shareID)
	return nil
}

func (p *DTree) deleteShare(ctx context.Context, params,
	taskResult map[string]interface{}) (map[string]interface{}, error) {

	parentName, _ := utils.ToStringWithFlag(params["parentname"])
	dTreeName, _ := utils.ToStringWithFlag(params["name"])
	vStoreID, _ := utils.ToStringWithFlag(params["vstoreid"])
	sharePath := fmt.Sprintf("/%s/%s", parentName, dTreeName)

	share, err := p.cli.GetNfsShareByPath(ctx, sharePath, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return nil, err
	}

	if share != nil {
		shareID, _ := utils.ToStringWithFlag(share["ID"])
		err = p.cli.DeleteNfsShare(ctx, shareID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Errorf("Delete share %s error: %v", shareID, err)
			return nil, err
		}
	}
	log.AddContext(ctx).Infof("delete share success, shareID: %v, err: %v", share["ID"], err)
	return nil, nil
}

func (p *DTree) getDtreeID(ctx context.Context, parentName, vstoreID, dTreeName string) (string, error) {
	// get dtree id
	dTreeInfo, err := p.cli.GetDTreeByName(ctx, "", parentName, vstoreID, dTreeName)
	if err != nil {
		log.AddContext(ctx).Errorf("get dTree by name failed, parentName :%s, vstoreID: %s, dTreeName: %s err: %v",
			parentName, vstoreID, dTreeInfo, err)
		return "", err
	}
	if dTreeInfo == nil {
		log.AddContext(ctx).Errorf("get empty dtree finish,parentName :%s, vstoreID: %s, dTreeName: %s",
			parentName, vstoreID, dTreeInfo)
		return "", errors.New("empty dTree")
	}
	dTreeID, _ := utils.ToStringWithFlag(dTreeInfo["ID"])

	return dTreeID, nil
}

func formatKerberosParam(data interface{}) int {
	if data == nil {
		return accessKrb5Default
	}
	str, ok := data.(string)
	if !ok {
		return accessKrb5Default
	}

	switch str {
	case "read_only":
		return accessKrb5ReadOnly
	case "read_write":
		return accessKrb5ReadWrite
	case "none":
		return accessKrb5None
	default:
		return accessKrb5Default
	}
}
