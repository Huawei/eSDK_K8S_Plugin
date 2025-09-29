/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume/creator"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/flow"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
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

	queryNfsSharePerPage int64 = 100
)

// ErrLogicPortFailOver indicates an error that logic port is fail over.
var ErrLogicPortFailOver = errors.New("logic port is not running on it's own site")

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

	isRunningOnOwnSite bool
}

// NewNAS inits a new nas client
func NewNAS(cli, metroRemoteCli client.OceanstorClientInterface, product constants.OceanstorVersion,
	nasHyperMetro NASHyperMetro, isRunningOnOwnSite bool) *NAS {

	return &NAS{
		Base: Base{
			cli:            cli,
			metroRemoteCli: metroRemoteCli,
			product:        product,
		},
		NASHyperMetro:      nasHyperMetro,
		isRunningOnOwnSite: isRunningOnOwnSite,
	}
}

func (p *NAS) selectSnapshotParent(ctx context.Context, params map[string]interface{}) error {
	if params == nil {
		return errors.New("parameters is empty")
	}

	wrapper := creator.NewParameter(params)

	if !wrapper.IsSnapshot() {
		return nil
	}

	snapshotName := utils.GetFSSnapshotName(wrapper.SourceSnapshotName())
	existsCli, snapshot, err := p.tryGetSnapshotByName(ctx, wrapper.SnapshotParentId(), snapshotName)
	if err != nil {
		return fmt.Errorf("try get snapshot by name error: %w", err)
	}
	if snapshot == nil {
		return fmt.Errorf("snapshot %s of filesystem %s not exists", snapshotName, wrapper.SnapshotParentId())
	}

	activeCli := p.GetActiveHyperMetroCli()
	if activeCli == existsCli {
		params["snapshotID"] = snapshot["ID"]
		params["snapshotParentName"] = snapshot["PARENTNAME"]
		return nil
	}

	snapshotParentName, ok := snapshot["PARENTNAME"].(string)
	if !ok {
		return fmt.Errorf("convert snapshotParentName to string error: [%v]", snapshot["PARENTNAME"])
	}
	activeSnapshot, err := p.getActiveSnapshot(ctx, activeCli, snapshotName, snapshotParentName)
	if err != nil {
		return fmt.Errorf("get active snapshot %s error: %w", snapshotName, err)
	}
	if activeSnapshot == nil {
		return fmt.Errorf("active snapshot %s doesn't exists", snapshotName)
	}
	params["snapshotID"] = activeSnapshot["ID"]
	params["snapshotParentName"] = activeSnapshot["PARENTNAME"]
	return nil
}

func (p *NAS) preModify(ctx context.Context, params map[string]any) error {
	if params == nil {
		return fmt.Errorf("premodify param is nil")
	}

	err := p.commonPreModify(ctx, params)
	if err != nil {
		return err
	}

	name, exists := params["name"]
	if !exists {
		return fmt.Errorf("the key \"name\" does not exist")
	}
	if _, ok := name.(string); !ok {
		return fmt.Errorf("failed to convert filesystem name to string, data: %v", name)
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

	params["localVStoreID"] = p.LocVStoreID
	params["remoteVStoreID"] = p.RmtVStoreID
	params["product"] = p.product

	return nil
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
	pvName, _ := utils.GetValue[string](params, "pvName")
	if name == pvName {
		params["name"] = utils.GetFileSystemName(name)
	}

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

	params["localVStoreID"] = p.LocVStoreID
	params["remoteVStoreID"] = p.RmtVStoreID
	params["product"] = p.product

	return nil
}

// Create creates fs volume
func (p *NAS) Create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := p.selectSnapshotParent(ctx, params); err != nil {
		return nil, err
	}

	return p.create(ctx, params)
}

// Modify modify fs volume
func (p *NAS) Modify(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	err := p.preModify(ctx, params)
	if err != nil {
		return nil, err
	}

	volumeCreator, err := creator.NewFromParameters(ctx, params, p.cli, p.metroRemoteCli)
	if err != nil {
		return nil, err
	}

	return volumeCreator.CreateVolume(ctx)
}

func (p *NAS) create(ctx context.Context, params map[string]interface{}) (utils.Volume, error) {
	hyperMetro, err := isHyperMetroFromParams(params)
	if err != nil {
		return nil, err
	}

	activeCli := p.cli
	standbyCli := p.cli
	if hyperMetro {
		activeCli = p.GetActiveHyperMetroCli()
		standbyCli = p.GetStandbyHyperMetroCli()
		if err := p.matchStoragePool(params); err != nil {
			return nil, err
		}
	} else if !p.isRunningOnOwnSite {
		return nil, ErrLogicPortFailOver
	}

	volumeCreator, err := creator.NewFromParameters(ctx, params, activeCli, standbyCli)
	if err != nil {
		return nil, err
	}

	return volumeCreator.CreateVolume(ctx)
}

func (p *NAS) validateManage(ctx context.Context, params, fs map[string]interface{}) error {
	return p.validateManageWorkLoadType(ctx, params, fs)
}

func (p *NAS) matchStoragePool(params map[string]interface{}) error {
	if p.cli == p.GetActiveHyperMetroCli() {
		// If current client is active storage client, do not need to switch their storage pool.
		return nil
	}
	if params == nil {
		return fmt.Errorf("parameters is nil")
	}

	// If active storage and standby storage has been switched, we need switch their storage pool either.
	params["storagepool"], params["remotestoragepool"] = params["remotestoragepool"], params["storagepool"]
	params["poolID"], params["remotePoolID"] = params["remotePoolID"], params["poolID"]

	return nil
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
	if capacity, err := strconv.ParseInt(fs["CAPACITY"].(string),
		constants.DefaultIntBase, constants.DefaultIntBitSize); err == nil {
		volObj.SetSize(utils.TransK8SCapacity(capacity, constants.AllocationUnitBytes))
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
		return fmt.Errorf("failed to get the snapshot count of filesystem %s error: %v", fsID, err)
	}
	if fsSnapshotNum > 0 {
		return fmt.Errorf("there are %d snapshots exist in filesystem %s. "+
			"Please delete the snapshots firstly", fsSnapshotNum, fsName)
	}

	hyperMetroIDs, err := p.parseHyperMetroPairs(fs)
	if err != nil {
		return err
	}
	taskflow := flow.NewTaskFlow(ctx, "Delete-FileSystem-Volume")
	if len(hyperMetroIDs) > 0 {
		if p.metroRemoteCli == nil {
			return errors.New("hyper metro backend is not configured")
		}

		taskflow.AddTask("Set-HyperMetro-ActiveClient", p.setActiveClient, nil)
		taskflow.AddTask("Delete-HyperMetro-Share", p.deleteHyperMetroShare, nil)
		taskflow.AddTask("Delete-HyperMetro", p.DeleteHyperMetro, nil)
		taskflow.AddTask("Delete-HyperMetro-Remote-FileSystem", p.deleteHyperMetroRemoteFS, nil)
		taskflow.AddTask("Delete-Local-FileSystem", p.deleteHyperMetroLocalFS, nil)
	} else if len(hyperMetroIDs) == 0 {
		if !p.isRunningOnOwnSite {
			return ErrLogicPortFailOver
		}

		taskflow.AddTask("Delete-Local-FileSystem", p.deleteLocalFS, nil)
	}

	vStoreID, _ := fs["vstoreId"].(string)
	params := map[string]interface{}{
		"name":           fsName,
		"hypermetroIDs":  hyperMetroIDs,
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
	if err := p.assertExpandSize(ctx, fsName, curSize, newSize); err != nil {
		return err
	}

	hyperMetroIDs, err := p.parseHyperMetroPairs(fs)
	if err != nil {
		return err
	}
	expandTask := flow.NewTaskFlow(ctx, "Expand-FileSystem-Volume")
	expandTask.AddTask("Expand-PreCheck-Capacity", p.preExpandCheckCapacity, nil)

	if len(hyperMetroIDs) > 0 {
		if p.metroRemoteCli == nil {
			return errors.New("hypermetro backend is not configured")
		}
		expandTask.AddTask("Set-HyperMetro-ActiveClient", p.setActiveClient, nil)

		if p.product.IsDoradoV6OrV7() && !p.FsHyperMetroActiveSite {
			// If product is DoradoV6 and the filesystem is created by standby site, need to get remote filesystem id.
			expandTask.AddTask("Expand-Remote-PreCheck-Capacity", p.preExpandCheckRemoteCapacity, nil)
		}

		if !p.product.IsDoradoV6OrV7() {
			// The NAS hyper metro feature of Dorado V6 can automatically synchronize capacity from the
			// active storage to the standby storage, so we don't need to expand the capacity of standby filesystem.
			expandTask.AddTask("Expand-Remote-PreCheck-Capacity", p.preExpandCheckRemoteCapacity, nil)
			expandTask.AddTask("Expand-HyperMetro-Remote-FileSystem", p.expandHyperMetroRemoteFS, nil)
		}
	} else if !p.isRunningOnOwnSite {
		return ErrLogicPortFailOver
	}

	expandTask.AddTask("Expand-Local-FileSystem", p.expandLocalFS, nil)
	params := map[string]interface{}{
		"name":            fsName,
		"size":            newSize,
		"expandSize":      newSize - curSize,
		"localFSID":       fs["ID"].(string),
		"localParentName": fs["PARENTNAME"].(string),
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

func (p *NAS) createRemoteFS(ctx context.Context, params, taskResult map[string]any) (map[string]any, error) {
	fsName, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	remoteCli, ok := taskResult["remoteCli"].(client.OceanstorClientInterface)
	if !ok {
		return nil, pkgUtils.Errorf(ctx,
			"convert remoteCli to client.OceanstorClientInterface failed, data: %v", taskResult["remoteCli"])
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

	return map[string]any{
		"remoteFSID": fs["ID"].(string),
	}, nil
}

func (p *NAS) revertRemoteFS(ctx context.Context, taskResult map[string]interface{}) error {
	fsID, exist := taskResult["remoteFSID"].(string)
	if !exist || fsID == "" {
		return nil
	}
	remoteCli, ok := taskResult["remoteCli"].(client.OceanstorClientInterface)
	if !ok {
		return pkgUtils.Errorf(ctx,
			"convert remoteCli to client.OceanstorClientInterface failed, data: %v", taskResult["remoteCli"])
	}
	deleteParams := map[string]interface{}{
		"ID": fsID,
	}
	if vStoreID, _ := taskResult["remoteVStoreID"].(string); vStoreID != "" {
		deleteParams["vstoreId"] = vStoreID
	}
	return remoteCli.DeleteFileSystem(ctx, deleteParams)
}

func (p *NAS) setActiveClient(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if !p.product.IsDoradoV6OrV7() {
		return nil, nil
	}

	var activeClient client.OceanstorClientInterface
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
	err := p.DeleteShare(ctx, name, vStoreID, activeClient)

	return nil, err
}

// DeleteShare used to delete filesystem share
func (p *NAS) DeleteShare(ctx context.Context, name, vStoreID string, cli client.OceanstorClientInterface) error {
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

// SafeDeleteShare used to delete filesystem share
func (p *NAS) SafeDeleteShare(ctx context.Context, name, vStoreID string, cli client.OceanstorClientInterface) error {
	sharePath := utils.GetOriginSharePath(name)
	share, err := cli.GetNfsShareByPath(ctx, sharePath, vStoreID)
	if err != nil {
		return fmt.Errorf("get nfs share by path %s error: %w", sharePath, err)
	}

	if share == nil {
		log.AddContext(ctx).Infof("share [%s] not exist, return successful", name)
		return nil
	}

	shareID, ok := share["ID"].(string)
	if !ok {
		return fmt.Errorf("convert shareID to string failed, data: %v", share["ID"])
	}

	return cli.SafeDeleteNfsShare(ctx, shareID, vStoreID)
}

// DeleteFS used to delete filesystem by name
func (p *NAS) DeleteFS(ctx context.Context, fsName string, cli client.OceanstorClientInterface) error {
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
	err := p.DeleteShare(ctx, name, vStoreID, p.cli)
	if err != nil {
		return nil, err
	}

	return nil, p.DeleteFS(ctx, name, p.cli)
}

func (p *NAS) deleteHyperMetroLocalFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	return nil, p.DeleteFS(ctx, name, p.cli)
}

func (p *NAS) deleteHyperMetroRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name, ok := params["name"].(string)
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert fsName to string failed, data: %v", params["name"])
	}
	err := p.DeleteFS(ctx, name, p.metroRemoteCli)

	return nil, err
}

func (p *NAS) getHyperMetroParams(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.metroRemoteCli == nil {
		msg := "hypermetro backend is not configured"
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

// DeleteHyperMetro used to delete hyperMetro pair
func (p *NAS) DeleteHyperMetro(ctx context.Context,
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

func (p *NAS) waitHyperMetroPairDeleted(ctx context.Context,
	pairID string, activeClient client.OceanstorClientInterface) error {
	var err error
	if p.product.IsDoradoV6OrV7() {
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
	var cli client.OceanstorClientInterface
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
		msg := fmt.Sprintf("Remote Filesystem %s newSize %d must be greater than or equal to curSize %d",
			remoteFsName, newSize, curSize)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return map[string]interface{}{
		"remoteFSID": remoteFs["ID"].(string),
	}, nil
}

func (p *NAS) expandFS(ctx context.Context, objID string, newSize int64, cli client.OceanstorClientInterface) error {
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

func (p *NAS) expandHyperMetroRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.product.IsDoradoV6OrV7() {
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
	fs, err := p.getFilesystemByName(ctx, p.cli, fsName)
	if err != nil {
		return nil, err
	}
	// no matter client whether switched, the parent id of the creating snapshot is always get by original client.
	parentId := fs.ID

	activeCli := p.cli
	if len(fs.HyperMetroPairIds) > 0 {
		activeCli = p.GetActiveHyperMetroCli()
	} else if !p.isRunningOnOwnSite {
		return nil, ErrLogicPortFailOver
	}

	if activeCli != p.cli {
		// we need always get filesystem information from active client of the hyper metro filesystem pair.
		fs, err = p.getFilesystemByName(ctx, activeCli, fsName)
		if err != nil {
			return nil, err
		}
	}

	snapshot, err := activeCli.GetFSSnapshotByName(ctx, fs.ID, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	snapshotSize, err := strconv.ParseInt(fs.CAPACITY, constants.DefaultIntBase, constants.DefaultIntBitSize)
	if err != nil {
		log.AddContext(ctx).Errorf("parse filesystem failed. err:%v, CAPACITY: %v", err, fs.CAPACITY)
		return nil, err
	}
	if snapshot != nil {
		log.AddContext(ctx).Infof("The snapshot %s is already exist.", snapshotName)
		return p.getSnapshotReturnInfo(snapshot, snapshotSize), nil
	}

	snapshot, err = activeCli.CreateFSSnapshot(ctx, snapshotName, fs.ID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create snapshot %s for filesystem %s error: %v",
			snapshotName, fs.ID, err)
		return nil, err
	}

	res := p.getSnapshotReturnInfo(snapshot, snapshotSize)
	res["ParentID"] = parentId
	return res, nil
}

// DeleteSnapshot deletes fs snapshot
func (p *NAS) DeleteSnapshot(ctx context.Context, snapshotParentId, snapshotName string) error {
	existsCli, snapshot, err := p.tryGetSnapshotByName(ctx, snapshotParentId, snapshotName)
	if err != nil {
		return fmt.Errorf("try get snapshot by name %s error: %w", snapshotName, err)
	}
	if snapshot == nil {
		if p.metroRemoteCli != nil && (p.cli.GetCurrentSiteWwn() == p.metroRemoteCli.GetCurrentSiteWwn()) {
			return fmt.Errorf("failed to find snapshot %s,"+
				" error: logical ports are running on one site", snapshotName)
		}

		log.AddContext(ctx).Infof("Filesystem snapshot %s to delete does not exist", snapshotName)
		return nil
	}

	activeCli := p.GetActiveHyperMetroCli()
	if activeCli == existsCli {
		if err := p.deleteFsSnapshot(ctx, activeCli, snapshot); err != nil {
			return err
		}
		return nil
	}

	snapshotParentName, ok := snapshot["PARENTNAME"].(string)
	if !ok {
		return fmt.Errorf("convert snapshotParentName to string failed, data: [%v]", snapshot["PARENTNAME"])
	}
	activeSnapshot, err := p.getActiveSnapshot(ctx, activeCli, snapshotName, snapshotParentName)
	if err != nil {
		return fmt.Errorf("get active snapshot %s error: %w", snapshotName, err)
	}
	if activeSnapshot == nil {
		log.AddContext(ctx).Infof("Filesystem snapshot %s to delete does not exist", snapshotName)
		return nil
	}

	if err := p.deleteFsSnapshot(ctx, activeCli, activeSnapshot); err != nil {
		return err
	}
	return nil
}

func (p *NAS) tryGetSnapshotByName(ctx context.Context,
	snapshotParentId, snapshotName string) (client.OceanstorClientInterface, map[string]any, error) {
	snapshot, err := p.cli.GetFSSnapshotByName(ctx, snapshotParentId, snapshotName)
	if err != nil {
		return nil, nil, fmt.Errorf("get filesystem %s snapshot by name %s error: %v",
			snapshotParentId, snapshotName, err)
	}
	existsCli := p.cli
	if snapshot == nil && p.metroRemoteCli != nil {
		snapshot, err = p.metroRemoteCli.GetFSSnapshotByName(ctx, snapshotParentId, snapshotName)
		if err != nil {
			return nil, nil, fmt.Errorf("get filesystem %s snapshot by name %s from hyperMetro client error: %v",
				snapshotParentId, snapshotName, err)
		}
		existsCli = p.metroRemoteCli
	}
	return existsCli, snapshot, nil
}

func (p *NAS) deleteFsSnapshot(ctx context.Context,
	activeCli client.OceanstorClientInterface, snapshot map[string]any) error {
	snapshotId, ok := snapshot["ID"].(string)
	if !ok {
		return fmt.Errorf("convert snapshotId to string failed, data: [%v]", snapshot["ID"])
	}
	if err := activeCli.DeleteFSSnapshot(ctx, snapshotId); err != nil {
		return err
	}

	return nil
}

func (p *NAS) getActiveSnapshot(ctx context.Context,
	activeCli client.OceanstorClientInterface, snapshotName string, snapshotParentName string) (map[string]any, error) {
	filesystem, err := p.getFilesystemByName(ctx, activeCli, snapshotParentName)
	if err != nil {
		return nil, fmt.Errorf("get filesystem name %s error: %w", snapshotParentName, err)
	}
	snapshot, err := activeCli.GetFSSnapshotByName(ctx, filesystem.ID, snapshotName)
	if err != nil {
		return nil, fmt.Errorf("get filesystem %s snapshot by name %s error: %w",
			filesystem.ID, snapshotName, err)
	}

	return snapshot, nil
}

func (p *NAS) getActiveClient(taskResult map[string]interface{}) client.OceanstorClientInterface {
	activeClient, exist := taskResult["activeClient"].(client.OceanstorClientInterface)
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

// GetActiveHyperMetroCli used to get active cli
func (p *NAS) GetActiveHyperMetroCli() client.OceanstorClientInterface {
	if p.metroRemoteCli == nil {
		return p.cli
	}

	if p.FsHyperMetroActiveSite {
		return p.cli
	}

	return p.metroRemoteCli
}

// GetStandbyHyperMetroCli used to get standby cli
func (p *NAS) GetStandbyHyperMetroCli() client.OceanstorClientInterface {
	if p.metroRemoteCli == nil {
		return nil
	}

	if p.FsHyperMetroActiveSite {
		return p.metroRemoteCli
	}

	return p.cli
}

func (p *NAS) getFilesystemByName(ctx context.Context,
	cli client.OceanstorClientInterface, name string) (*base.FilesystemResponse, error) {
	fsMap, err := cli.GetFileSystemByName(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem by name %s error: %v", name, err)
		return nil, err
	}
	if fsMap == nil {
		msg := fmt.Sprintf("Filesystem %s to create snapshot does not exist", name)
		log.AddContext(ctx).Errorf(msg)
		return nil, errors.New(msg)
	}

	fs, err := utils.ConvertMapToStruct[base.FilesystemResponse](fsMap)
	if err != nil {
		return nil, fmt.Errorf("ConvertMapToStruct %v error: %w", fsMap, err)
	}
	if fs == nil {
		return nil, fmt.Errorf("filesystem %s to create snapshot does not exist", name)
	}

	fs.HyperMetroPairIds, err = p.parseHyperMetroPairs(fsMap)
	if err != nil {
		return nil, err
	}

	return fs, nil
}

func (p *NAS) getFilesystemByID(ctx context.Context,
	cli client.OceanstorClientInterface, id string) (*base.FilesystemResponse, error) {
	fsMap, err := cli.GetFileSystemByID(ctx, id)
	if err != nil {
		log.AddContext(ctx).Errorf("get filesystem by id %s error: %v", id, err)
		return nil, err
	}
	if fsMap == nil {
		msg := fmt.Sprintf("filesystem %s does not exist", id)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	fs, err := utils.ConvertMapToStruct[base.FilesystemResponse](fsMap)
	if err != nil {
		return nil, fmt.Errorf("convertMapToStruct %v error: %w", fsMap, err)
	}
	if fs == nil {
		return nil, fmt.Errorf("filesystem %s does not exist", id)
	}

	fs.HyperMetroPairIds, err = p.parseHyperMetroPairs(fsMap)
	if err != nil {
		return nil, err
	}

	return fs, nil
}
func (p *NAS) parseHyperMetroPairs(fsMap map[string]any) ([]string, error) {
	var hyperMetroIds []string
	if fsMap == nil {
		return hyperMetroIds, nil
	}

	rawPairIds, exists := fsMap["HYPERMETROPAIRIDS"]
	if !exists {
		return hyperMetroIds, nil
	}

	pairIdStr, ok := rawPairIds.(string)
	if !ok {
		return nil, fmt.Errorf("hyperMetroPairIds is not a string, data: %+v", rawPairIds)
	}

	pairIdBytes := []byte(pairIdStr)
	if err := json.Unmarshal(pairIdBytes, &hyperMetroIds); err != nil {
		return nil, fmt.Errorf("unmarshal hyperMetroIDBytes failed, error: %w", err)
	}

	return hyperMetroIds, nil
}

func (p *NAS) assertExpandSize(ctx context.Context, fsName string, curSize, newSize int64) error {
	if newSize == curSize {
		log.AddContext(ctx).Infof("the size of filesystem %s has not changed and the current size is %d",
			fsName, newSize)
		return nil
	} else if newSize < curSize {
		msg := fmt.Sprintf("Filesystem %s newSize %d must be greater than or equal to curSize %d",
			fsName, newSize, curSize)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

func isHyperMetroFromParams(params map[string]any) (bool, error) {
	val, exists := params["hypermetro"]
	if !exists {
		return false, nil
	}

	hyperMetro, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("parameter hyperMetro [%v] in sc must be bool", val)
	}

	return hyperMetro, nil
}
