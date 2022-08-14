package volume

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/storage/oceanstor/smartx"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
	"huawei-csi-driver/utils/taskflow"
)

const (
	FILESYSTEM_HEALTH_STATUS_NORMAL   = "1"
	FILESYSTEM_SPLIT_STATUS_NOT_START = "1"
	FILESYSTEM_SPLIT_STATUS_SPLITTING = "2"
	FILESYSTEM_SPLIT_STATUS_QUEUING   = "3"
	FILESYSTEM_SPLIT_STATUS_ABNORMAL  = "4"

	REMOTE_DEVICE_HEALTH_STATUS          = "1"
	REMOTE_DEVICE_RUNNING_STATUS_LINK_UP = "10"

	REPLICATION_PAIR_RUNNING_STATUS_NORMAL = "1"
	REPLICATION_PAIR_RUNNING_STATUS_SYNC   = "23"

	REPLICATION_VSTORE_PAIR_RUNNING_STATUS_NORMAL = "1"
	REPLICATION_VSTORE_PAIR_RUNNING_STATUS_SYNC   = "23"

	REPLICATION_ROLE_PRIMARY = "0"
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

func NewNAS(cli, metroRemoteCli, replicaRemoteCli *client.Client, product string, nasHyperMetro NASHyperMetro) *NAS {
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

	if v, exist := params["sourcevolumename"].(string); exist {
		params["clonefrom"] = utils.GetFileSystemName(v)
	} else if v, exist := params["sourcesnapshotname"].(string); exist {
		params["fromSnapshot"] = utils.GetFSSnapshotName(v)
	} else if v, exist := params["clonefrom"].(string); exist {
		params["clonefrom"] = utils.GetFileSystemName(v)
	}

	err = p.setWorkLoadID(ctx, p.cli, params)
	if err != nil {
		return err
	}
	return nil
}

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

func (p *NAS) createLocalFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)

	fs, err := p.cli.GetFileSystemByName(ctx, fsName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem %s error: %v", fsName, err)
		return nil, err
	}

	if fs == nil {
		params["parentid"] = params["poolID"].(string)
		params["vstoreId"] = params["localVStoreID"].(string)

		if _, exist := params["clonefrom"]; exist {
			fs, err = p.clone(ctx, params)
		} else if _, exist := params["fromSnapshot"]; exist {
			fs, err = p.createFromSnapshot(ctx, params)
		} else {
			fs, err = p.cli.CreateFileSystem(ctx, params)
		}
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

func (p *NAS) clone(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	clonefrom := params["clonefrom"].(string)
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

	cloneFSCapacity := params["capacity"].(int64)
	if cloneFSCapacity < srcFSCapacity {
		msg := fmt.Sprintf("Clone filesystem capacity must be >= src %s", clonefrom)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	fsName := params["name"].(string)
	parentID := cloneFromFS["ID"].(string)
	cloneParams := map[string]interface{}{
		"fsName":           fsName,
		"parentID":         parentID,
		"parentSnapshotID": "",
		"allocType":        params["alloctype"].(int),
		"cloneSpeed":       params["clonespeed"].(int),
		"cloneFSCapacity":  cloneFSCapacity,
		"srcCapacity":      srcFSCapacity,
	}
	cloneFS, err := p.cloneFilesystem(ctx, cloneParams)
	if err != nil {
		log.AddContext(ctx).Errorf("Clone filesystem %s from source filesystem %s error: %s",
			fsName, parentID, err)
		return nil, err
	}

	return cloneFS, nil
}

func (p *NAS) createFromSnapshot(ctx context.Context,
	params map[string]interface{}) (map[string]interface{}, error) {
	srcSnapshotName := params["fromSnapshot"].(string)
	snapshotParentId := params["snapshotparentid"].(string)
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

	parentName := srcSnapshot["PARENTNAME"].(string)
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

	fsName := params["name"].(string)
	srcSnapshotID := srcSnapshot["ID"].(string)
	cloneParams := map[string]interface{}{
		"fsName":           fsName,
		"parentID":         srcSnapshot["PARENTID"].(string),
		"parentSnapshotID": srcSnapshotID,
		"allocType":        params["alloctype"].(int),
		"cloneSpeed":       params["clonespeed"].(int),
		"cloneFSCapacity":  params["capacity"].(int64),
		"srcCapacity":      srcSnapshotCapacity,
	}

	cloneFS, err := p.cloneFilesystem(ctx, cloneParams)
	if err != nil {
		log.AddContext(ctx).Errorf("Clone filesystem %s from source snapshot %s error: %s",
			fsName, srcSnapshotID, err)
		return nil, err
	}

	return cloneFS, nil
}

func (p *NAS) cloneFilesystem(ctx context.Context,
	cloneParams map[string]interface{}) (map[string]interface{}, error) {
	fsName := cloneParams["fsName"].(string)
	parentID := cloneParams["parentID"].(string)
	parentSnapshotID := cloneParams["parentSnapshotID"].(string)
	allocType := cloneParams["allocType"].(int)
	cloneSpeed := cloneParams["cloneSpeed"].(int)
	cloneFSCapacity := cloneParams["cloneFSCapacity"].(int64)
	srcCapacity := cloneParams["srcCapacity"].(int64)

	cloneFS, err := p.cli.CloneFileSystem(ctx, fsName, allocType, parentID, parentSnapshotID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create cloneFilesystem, source filesystem ID %s error: %s", parentID, err)
		return nil, err
	}

	cloneFSID := cloneFS["ID"].(string)
	deleteParams := map[string]interface{}{
		"ID": cloneFSID,
	}
	if cloneFSCapacity > srcCapacity {
		err := p.cli.ExtendFileSystem(ctx, cloneFSID, cloneFSCapacity)
		if err != nil {
			log.AddContext(ctx).Errorf("Extend filesystem %s to capacity %d error: %v",
				cloneFSID, cloneFSCapacity, err)
			_ = p.cli.DeleteFileSystem(ctx, deleteParams)
			return nil, err
		}
	}

	var isDeleteParentSnapshot = false
	if parentSnapshotID == "" {
		isDeleteParentSnapshot = true
	}

	err = p.cli.SplitCloneFS(ctx, cloneFSID, cloneSpeed, isDeleteParentSnapshot)
	if err != nil {
		log.AddContext(ctx).Errorf("Split filesystem %s error: %v", fsName, err)
		_ = p.cli.DeleteFileSystem(ctx, deleteParams)
		return nil, err
	}

	err = p.waitFSSplitDone(ctx, cloneFSID)
	if err != nil {
		return nil, err
	}

	return cloneFS, nil
}

func (p *NAS) waitFSSplitDone(ctx context.Context, fsID string) error {
	err := utils.WaitUntil(func() (bool, error) {
		fs, err := p.cli.GetFileSystemByID(ctx, fsID)
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
	remoteCli := taskResult["remoteCli"].(*client.Client)

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
	remoteCli := taskResult["remoteCli"].(*client.Client)
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

func (p *NAS) getCurrentShareAccess(ctx context.Context, shareID, vStoreID string, cli *client.Client) (map[string]interface{}, error) {
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

	var allSquash int
	var exist bool
	params["allsquash"], exist = params["allsquash"].(string)
	if !exist || params["allsquash"] == "" {
		allSquash = 1
	} else {
		allSquash, err = strconv.Atoi(params["allsquash"].(string))
		if err != nil {
			return nil, utils.Errorf(ctx, "parameter allSquash [%v] in sc needs to be a number.", params["allsquash"])
		}
	}

	var rootSquash int
	params["rootsquash"], exist = params["rootsquash"].(string)
	if !exist || params["rootsquash"] == "" {
		rootSquash = 1
	} else {
		rootSquash, err = strconv.Atoi(params["rootsquash"].(string))
		if err != nil {
			return nil, utils.Errorf(ctx, "parameter rootSquash [%v] in sc needs to be a number.", params["allsquash"])
		}
	}

	for _, i := range strings.Split(authClient, ";") {
		_, exist := accesses[i]
		delete(accesses, i)

		if exist {
			continue
		}

		params := map[string]interface{}{
			"NAME":       i,
			"PARENTID":   shareID,
			"ACCESSVAL":  1,
			"SYNC":       0,
			"ALLSQUASH":  allSquash,
			"ROOTSQUASH": rootSquash,
		}
		if vStoreID != "" {
			params["vstoreId"] = vStoreID
		}

		err := activeClient.AllowNfsShareAccess(ctx, params)
		if err != nil {
			log.AddContext(ctx).Errorf("Allow nfs share access %v error: %v", params, err)
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

	fsID := fs["ID"].(string)
	fsSnapshotNum, err := p.cli.GetFSSnapshotCountByParentId(ctx, fsID)
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to get the snapshot count of filesystem %s error: %v", fsID, err)
		return err
	}

	var replicationIDs []string
	replicationIDBytes := []byte(fs["REMOTEREPLICATIONIDS"].(string))
	json.Unmarshal(replicationIDBytes, &replicationIDs)

	var hypermetroIDs []string
	hypermetroIDBytes := []byte(fs["HYPERMETROPAIRIDS"].(string))
	json.Unmarshal(hypermetroIDBytes, &hypermetroIDs)

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
		"name":           name,
		"replicationIDs": replicationIDs,
		"hypermetroIDs":  hypermetroIDs,
		"localVStoreID":  vStoreID,
		"remoteVStoreID": p.RmtVStoreID,
	}

	_, err = taskflow.Run(params)
	return err
}

func (p *NAS) Expand(ctx context.Context, name string, newSize int64) error {
	fsName := utils.GetFileSystemName(name)
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

	curSize, _ := strconv.ParseInt(fs["CAPACITY"].(string), 10, 64)
	if newSize <= curSize {
		msg := fmt.Sprintf("Filesystem %s newSize %d must be greater than curSize %d", fsName, newSize, curSize)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	var replicationIDs []string
	replicationIDBytes := []byte(fs["REMOTEREPLICATIONIDS"].(string))
	_ = json.Unmarshal(replicationIDBytes, &replicationIDs)

	var hyperMetroIDs []string
	hyperMetroIDBytes := []byte(fs["HYPERMETROPAIRIDS"].(string))
	_ = json.Unmarshal(hyperMetroIDBytes, &hyperMetroIDs)

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
		"name":            name,
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

	vStoreID := vStore["ID"].(string)

	vStorePair, err := p.cli.GetReplicationvStorePairByvStore(ctx, vStoreID)
	if err != nil {
		return nil, err
	}
	if vStorePair == nil {
		return nil, nil
	}

	if vStorePair["ROLE"] != REPLICATION_ROLE_PRIMARY {
		msg := "Local role of vstore pair is not primary"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if vStorePair["RUNNINGSTATUS"] != REPLICATION_VSTORE_PAIR_RUNNING_STATUS_NORMAL &&
		vStorePair["RUNNINGSTATUS"] != REPLICATION_VSTORE_PAIR_RUNNING_STATUS_SYNC {
		msg := "Running status of vstore pair is abnormal"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	remotevStore := vStorePair["REMOTEVSTORENAME"].(string)
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

	if vStorePair != nil {
		vStorePairID = vStorePair["ID"].(string)
		remoteDeviceID = vStorePair["REMOTEDEVICEID"].(string)
		remoteDeviceSN = vStorePair["REMOTEDEVICESN"].(string)
	}

	remoteSystem, err := p.replicaRemoteCli.GetSystem(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Remote device is abnormal: %v", err)
		return nil, err
	}

	if remoteDeviceID == "" {
		sn := remoteSystem["ID"].(string)
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
	fsName := params["name"].(string)
	remoteCli := taskResult["remoteCli"].(*client.Client)

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

		params["parentid"] = taskResult["remotePoolID"].(string)
		params["vstoreId"] = params["remoteVStoreID"].(string)
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
	remoteCli := taskResult["remoteCli"].(*client.Client)
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
	replicationIDs := params["replicationIDs"].([]string)

	for _, pairID := range replicationIDs {
		pair, err := p.cli.GetReplicationPairByID(ctx, pairID)
		if err != nil {
			return nil, err
		}

		runningStatus := pair["RUNNINGSTATUS"].(string)
		if runningStatus == REPLICATION_PAIR_RUNNING_STATUS_NORMAL ||
			runningStatus == REPLICATION_PAIR_RUNNING_STATUS_SYNC {
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

	var activeClient *client.Client
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
	name := params["name"].(string)
	activeClient := p.getActiveClient(taskResult)
	vStoreID := p.getVStoreID(taskResult)
	err := p.deleteShare(ctx, name, vStoreID, activeClient)

	return nil, err
}

func (p *NAS) deleteShare(ctx context.Context, name, vStoreID string, cli *client.Client) error {
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

func (p *NAS) deleteFS(ctx context.Context, name string, cli *client.Client) error {
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

func (p *NAS) deleteHyperMetroLocalFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name := params["name"].(string)
	return nil, p.deleteFS(ctx, name, p.cli)
}

func (p *NAS) deleteHyperMetroRemoteFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name := params["name"].(string)
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
	vStorePairID := params["vStorePairID"].(string)

	localFSID := taskResult["localFSID"].(string)
	remoteFSID := taskResult["remoteFSID"].(string)
	activeClient := p.getActiveClient(taskResult)
	if activeClient != p.cli {
		localFSID = taskResult["remoteFSID"].(string)
		remoteFSID = taskResult["localFSID"].(string)
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

	pairID := pair["ID"].(string)
	err = activeClient.SyncHyperMetroPair(ctx, pairID)
	if err != nil {
		log.AddContext(ctx).Errorf("Sync nas hypermetro pair %s error: %v", pairID, err)
		if p.product == "DoradoV6" {
			activeClient.DeleteHyperMetroPair(ctx, pairID, false)
		} else {
			activeClient.DeleteHyperMetroPair(ctx, pairID, true)
		}
		return nil, err
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

	status := pair["RUNNINGSTATUS"].(string)
	if status == HYPERMETROPAIR_RUNNING_STATUS_NORMAL ||
		status == HYPERMETROPAIR_RUNNING_STATUS_TO_SYNC ||
		status == HYPERMETROPAIR_RUNNING_STATUS_SYNCING {
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
	hypermetroIDs := params["hypermetroIDs"].([]string)
	activeClient := p.getActiveClient(taskResult)
	for _, pairID := range hypermetroIDs {
		pair, err := activeClient.GetHyperMetroPair(ctx, pairID)
		if err != nil {
			return nil, err
		}

		if pair == nil {
			continue
		}

		status := pair["RUNNINGSTATUS"].(string)
		if status == HYPERMETROPAIR_RUNNING_STATUS_NORMAL ||
			status == HYPERMETROPAIR_RUNNING_STATUS_TO_SYNC ||
			status == HYPERMETROPAIR_RUNNING_STATUS_SYNCING {
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

func (p *NAS) waitHyperMetroPairDeleted(ctx context.Context, pairID string, activeClient *client.Client) error {
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
	var cli *client.Client
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
	name := params["name"].(string)
	remoteFsName := utils.GetFileSystemName(name)
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

	newSize := params["size"].(int64)
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

func (p *NAS) expandFS(ctx context.Context, objID string, newSize int64, cli *client.Client) error {
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
	fsID := taskResult["remoteFSID"].(string)
	newSize := params["size"].(int64)
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

	fsID := taskResult["remoteFSID"].(string)
	newSize := params["size"].(int64)
	err := p.expandFS(ctx, fsID, newSize, p.metroRemoteCli)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand hyperMetro filesystem %s error: %v", fsID, err)
		return nil, err
	}

	return nil, err
}

func (p *NAS) expandLocalFS(ctx context.Context,
	params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	newSize := params["size"].(int64)
	activeClient := p.getActiveClient(taskResult)
	fsID := p.getActiveFsID(taskResult)
	err := p.expandFS(ctx, fsID, newSize, activeClient)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand filesystem %s error: %v", fsID, err)
		return nil, err
	}
	return nil, err
}

func (p *NAS) CreateSnapshot(ctx context.Context, name, snapshotName string) (map[string]interface{}, error) {
	fsName := utils.GetFileSystemName(name)
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

	fsId := fs["ID"].(string)
	snapshot, err := p.cli.GetFSSnapshotByName(ctx, fsId, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get filesystem snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	snapshotSize, _ := strconv.ParseInt(fs["CAPACITY"].(string), 10, 64)
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

	snapshotId := snapshot["ID"].(string)
	err = p.cli.DeleteFSSnapshot(ctx, snapshotId)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete filesystem snapshot %s error: %v", snapshotId, err)
		return err
	}

	return nil
}

func (p *NAS) getActiveClient(taskResult map[string]interface{}) *client.Client {
	activeClient, exist := taskResult["activeClient"].(*client.Client)
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
