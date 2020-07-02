package volume

import (
	"encoding/json"
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

	REMOTE_DEVICE_HEALTH_STATUS          = "1"
	REMOTE_DEVICE_RUNNING_STATUS_LINK_UP = "10"

	REPLICATION_PAIR_RUNNING_STATUS_NORMAL = "1"
	REPLICATION_PAIR_RUNNING_STATUS_SYNC   = "23"

	REPLICATION_VSTORE_PAIR_RUNNING_STATUS_NORMAL = "1"
	REPLICATION_VSTORE_PAIR_RUNNING_STATUS_SYNC   = "23"

	REPLICATION_ROLE_PRIMARY = "0"
)

type NAS struct {
	Base
}

func NewNAS(cli, metroRemoteCli, replicaRemoteCli *client.Client) *NAS {
	return &NAS{
		Base: Base{
			cli:              cli,
			metroRemoteCli:   metroRemoteCli,
			replicaRemoteCli: replicaRemoteCli,
		},
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

	if v, exist := params["sourcevolumename"].(string); exist {
		params["clonefrom"] = utils.GetFileSystemName(v)
	} else if v, exist := params["sourcesnapshotname"].(string); exist {
		params["fromSnapshot"] = utils.GetFSSnapshotName(v)
	} else if v, exist := params["clonefrom"].(string); exist {
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

	replication, replicationOK := params["replication"].(bool)
	hyperMetro, hyperMetroOK := params["hypermetro"].(bool)

	if (replicationOK && replication) && (hyperMetroOK && hyperMetro) {
		msg := "cannot create replication and hypermetro for a volume at the same time"
		log.Errorln(msg)
		return errors.New(msg)
	} else if replicationOK && replication {
		taskflow.AddTask("Get-Replication-Params", p.getReplicationParams, nil)
	} else if hyperMetroOK && hyperMetro {
		taskflow.AddTask("Get-HyperMetro-Params", p.getHyperMetroParams, nil)
	}

	taskflow.AddTask("Create-Local-FS", p.createLocalFS, p.revertLocalFS)
	taskflow.AddTask("Create-Share", p.createShare, p.revertShare)
	taskflow.AddTask("Allow-Share-Access", p.allowShareAccess, nil)
	taskflow.AddTask("Create-QoS", p.createLocalQoS, p.revertLocalQoS)

	if replication, ok := params["replication"].(bool); ok && replication {
		taskflow.AddTask("Create-Remote-FS", p.createRemoteFS, p.revertRemoteFS)
		taskflow.AddTask("Create-Remote-QoS", p.createRemoteQoS, p.revertRemoteQoS)
		taskflow.AddTask("Create-Replication-Pair", p.createReplicationPair, nil)
	} else if hyperMetro, ok := params["hypermetro"].(bool); ok && hyperMetro {
		taskflow.AddTask("Create-Remote-FS", p.createRemoteFS, p.revertRemoteFS)
		taskflow.AddTask("Create-Remote-QoS", p.createRemoteQoS, p.revertRemoteQoS)
		taskflow.AddTask("Create-HyperMetro", p.createHyperMetro, nil)
	}

	_, err = taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return err
	}

	return nil
}

func (p *NAS) createLocalFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)

	fs, err := p.cli.GetFileSystemByName(fsName)
	if err != nil {
		log.Errorf("Get filesystem %s error: %v", fsName, err)
		return nil, err
	}

	if fs == nil {
		params["parentid"] = params["poolID"].(string)

		if _, exist := params["clonefrom"]; exist {
			fs, err = p.clone(params)
		} else if _, exist := params["fromSnapshot"]; exist {
			fs, err = p.createFromSnapshot(params)
		} else {
			fs, err = p.cli.CreateFileSystem(params)
		}
	} else {
		if fs["ISCLONEFS"].(string) == "false" {
			return map[string]interface{}{
				"localFSID": fs["ID"].(string),
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
		"localFSID": fs["ID"].(string),
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
	cloneFS, err := p.cloneFilesystem(cloneParams)
	if err != nil {
		log.Errorf("Clone filesystem %s from source filesystem %s error: %s", fsName, parentID, err)
		return nil, err
	}

	return cloneFS, nil
}

func (p *NAS) createFromSnapshot(params map[string]interface{}) (map[string]interface{}, error) {
	srcSnapshotName := params["fromSnapshot"].(string)
	snapshotParentId := params["snapshotparentid"].(string)
	srcSnapshot, err := p.cli.GetFSSnapshotByName(snapshotParentId, srcSnapshotName)
	if err != nil {
		log.Errorf("Get src filesystem snapshot %s error: %v", srcSnapshotName, err)
		return nil, err
	}
	if srcSnapshot == nil {
		msg := fmt.Errorf("src snapshot %s does not exist", srcSnapshotName)
		log.Errorln(msg)
		return nil, msg
	}

	parentName := srcSnapshot["PARENTNAME"].(string)
	parentFS, err := p.cli.GetFileSystemByName(parentName)
	if err != nil {
		log.Errorf("Get clone src filesystem %s error: %v", parentName, err)
		return nil, err
	}
	if parentFS == nil {
		msg := fmt.Errorf("Filesystem %s does not exist", parentName)
		log.Errorln(msg)
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

	cloneFS, err := p.cloneFilesystem(cloneParams)
	if err != nil {
		log.Errorf("Clone filesystem %s from source snapshot %s error: %s", fsName, srcSnapshotID, err)
		return nil, err
	}

	return cloneFS, nil
}

func (p *NAS) cloneFilesystem(cloneParams map[string]interface{}) (map[string]interface{}, error) {
	fsName := cloneParams["fsName"].(string)
	parentID := cloneParams["parentID"].(string)
	parentSnapshotID := cloneParams["parentSnapshotID"].(string)
	allocType := cloneParams["allocType"].(int)
	cloneSpeed := cloneParams["cloneSpeed"].(int)
	cloneFSCapacity := cloneParams["cloneFSCapacity"].(int64)
	srcCapacity := cloneParams["srcCapacity"].(int64)

	cloneFS, err := p.cli.CloneFileSystem(fsName, allocType, parentID, parentSnapshotID)
	if err != nil {
		log.Errorf("Create cloneFilesystem, source filesystem ID %s error: %s", parentID, err)
		return nil, err
	}

	cloneFSID := cloneFS["ID"].(string)
	if cloneFSCapacity > srcCapacity {
		err := p.cli.ExtendFileSystem(cloneFSID, cloneFSCapacity)
		if err != nil {
			log.Errorf("Extend filesystem %s to capacity %d error: %v", cloneFSID, cloneFSCapacity, err)
			p.cli.DeleteFileSystem(cloneFSID)
			return nil, err
		}
	}

	var isDeleteParentSnapshot = false
	if parentSnapshotID == "" {
		isDeleteParentSnapshot = true
	}

	err = p.cli.SplitCloneFS(cloneFSID, cloneSpeed, isDeleteParentSnapshot)
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

func (p *NAS) revertLocalFS(taskResult map[string]interface{}) error {
	fsID, exist := taskResult["localFSID"].(string)
	if !exist || fsID == "" {
		return nil
	}

	return p.cli.DeleteFileSystem(fsID)
}

func (p *NAS) createLocalQoS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	fsID := taskResult["localFSID"].(string)

	smartX := smartx.NewSmartX(p.cli)
	qosID, err := smartX.CreateQos(fsID, "fs", qos)
	if err != nil {
		log.Errorf("Create qos %v for fs %s error: %v", qos, fsID, err)
		return nil, err
	}

	return map[string]interface{}{
		"localQoSID": qosID,
	}, nil
}

func (p *NAS) revertLocalQoS(taskResult map[string]interface{}) error {
	fsID, fsIDExist := taskResult["localFSID"].(string)
	qosID, qosIDExist := taskResult["localQoSID"].(string)
	if !fsIDExist || !qosIDExist {
		return nil
	}

	smartX := smartx.NewSmartX(p.cli)
	return smartX.DeleteQos(qosID, fsID, "fs")
}

func (p *NAS) createRemoteQoS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	fsID := taskResult["remoteFSID"].(string)
	remoteCli := taskResult["remoteCli"].(*client.Client)

	smartX := smartx.NewSmartX(remoteCli)
	qosID, err := smartX.CreateQos(fsID, "fs", qos)
	if err != nil {
		log.Errorf("Create qos %v for fs %s error: %v", qos, fsID, err)
		return nil, err
	}

	return map[string]interface{}{
		"remoteQoSID": qosID,
	}, nil
}

func (p *NAS) revertRemoteQoS(taskResult map[string]interface{}) error {
	fsID, fsIDExist := taskResult["remoteFSID"].(string)
	qosID, qosIDExist := taskResult["remoteQoSID"].(string)
	if !fsIDExist || !qosIDExist {
		return nil
	}

	remoteCli := taskResult["remoteCli"].(*client.Client)
	smartX := smartx.NewSmartX(remoteCli)
	return smartX.DeleteQos(qosID, fsID, "fs")
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
			"fsid":        taskResult["localFSID"].(string),
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

func (p *NAS) revertShare(taskResult map[string]interface{}) error {
	shareID, exist := taskResult["shareID"].(string)
	if !exist || len(shareID) == 0 {
		return nil
	}

	return p.cli.DeleteNfsShare(shareID)
}

func (p *NAS) getCurrentShareAccess(shareID string) (map[string]interface{}, error) {
	count, err := p.cli.GetNfsShareAccessCount(shareID)
	if err != nil {
		return nil, err
	}

	accesses := make(map[string]interface{})

	var i int64 = 0
	for ; i < count; i += 100 { // Query per page 100
		clients, err := p.cli.GetNfsShareAccessRange(shareID, i, i+100)
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

func (p *NAS) allowShareAccess(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	shareID := taskResult["shareID"].(string)
	authClient := params["authclient"].(string)

	accesses, err := p.getCurrentShareAccess(shareID)
	if err != nil {
		log.Errorf("Get current access of share %s error: %v", shareID, err)
		return nil, err
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
			"ALLSQUASH":  1,
			"ROOTSQUASH": 1,
		}

		err := p.cli.AllowNfsShareAccess(params)
		if err != nil {
			log.Errorf("Allow nfs share access %v error: %v", params, err)
			return nil, err
		}
	}

	// Remove all other extra access
	for _, i := range accesses {
		access := i.(map[string]interface{})
		accessID := access["ID"].(string)

		err := p.cli.DeleteNfsShareAccess(accessID)
		if err != nil {
			log.Warningf("Delete extra nfs share access %s error: %v", accessID, err)
		}
	}

	return nil, nil
}

func (p *NAS) Delete(name string) error {
	fsName := utils.GetFileSystemName(name)
	fs, err := p.cli.GetFileSystemByName(fsName)
	if err != nil {
		log.Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}
	if fs == nil {
		log.Infof("Filesystem %s to delete does not exist", fsName)
		return nil
	}

	fsID := fs["ID"].(string)
	fsSnapshotNum, err := p.cli.GetFSSnapshotCountByParentId(fsID)
	if err != nil {
		log.Errorf("Failed to get the snapshot count of filesystem %s error: %v", fsID, err)
		return err
	}

	var replicationIDs []string
	replicationIDBytes := []byte(fs["REMOTEREPLICATIONIDS"].(string))
	json.Unmarshal(replicationIDBytes, &replicationIDs)

	var hypermetroIDs []string
	hypermetroIDBytes := []byte(fs["HYPERMETROPAIRIDS"].(string))
	json.Unmarshal(hypermetroIDBytes, &hypermetroIDs)

	taskflow := taskflow.NewTaskFlow("Delete-FileSystem-Volume")

	if len(replicationIDs) > 0 {
		if p.replicaRemoteCli == nil {
			msg := "remote client for replication is nil"
			log.Errorln(msg)
			return errors.New(msg)
		}

		if fsSnapshotNum > 1 {
			msg := fmt.Sprintf("There are %d snapshots exist in filesystem %s. "+
				"Please delete the snapshots firstly", fsSnapshotNum-1, fsName)
			log.Errorln(msg)
			return errors.New(msg)
		}

		taskflow.AddTask("Delete-Replication-Pair", p.deleteReplicationPair, nil)
		taskflow.AddTask("Delete-Replication-Remote-FileSystem", p.deleteReplicationRemoteFS, nil)
		taskflow.AddTask("Delete-Local-FileSystem", p.deleteLocalFS, nil)
	}

	if len(hypermetroIDs) > 0 {
		if p.metroRemoteCli == nil {
			msg := "remote client for hypermetro is nil"
			log.Errorln(msg)
			return errors.New(msg)
		}

		if fsSnapshotNum > 0 {
			msg := fmt.Sprintf("There are %d snapshots exist in filesystem %s. "+
				"Please delete the snapshots firstly", fsSnapshotNum, fsName)
			log.Errorln(msg)
			return errors.New(msg)
		}

		taskflow.AddTask("Delete-HyperMetro", p.deleteHyperMetro, nil)
		taskflow.AddTask("Delete-HyperMetro-Remote-FileSystem", p.deleteHyperMetroRemoteFS, nil)
		taskflow.AddTask("Delete-Local-FileSystem", p.deleteLocalFS, nil)
	}

	if len(replicationIDs) == 0 && len(hypermetroIDs) == 0 {
		if fsSnapshotNum > 0 {
			msg := fmt.Sprintf("There are %d snapshots exist in filesystem %s. "+
				"Please delete the snapshots firstly", fsSnapshotNum, fsName)
			log.Errorln(msg)
			return errors.New(msg)
		}
		taskflow.AddTask("Delete-Local-FileSystem", p.deleteLocalFS, nil)
	}

	params := map[string]interface{}{
		"name":           name,
		"replicationIDs": replicationIDs,
		"hypermetroIDs":  hypermetroIDs,
	}

	_, err = taskflow.Run(params)
	return err
}

func (p *NAS) Expand(name string, newSize int64) error {
	fsName := utils.GetFileSystemName(name)
	fs, err := p.cli.GetFileSystemByName(fsName)
	if err != nil {
		log.Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}

	if fs == nil {
		msg := fmt.Sprintf("Filesystem %s to expand does not exist", fsName)
		log.Errorf(msg)
		return errors.New(msg)
	}

	curSize, _ := strconv.ParseInt(fs["CAPACITY"].(string), 10, 64)
	if newSize <= curSize {
		log.Warningf("Filesystem %s newSize %d must be greater than curSize %d", fsName, newSize, curSize)
		return nil
	}

	var replicationIDs []string
	replicationIDBytes := []byte(fs["REMOTEREPLICATIONIDS"].(string))
	_ = json.Unmarshal(replicationIDBytes, &replicationIDs)

	var hyperMetroIDs []string
	hyperMetroIDBytes := []byte(fs["HYPERMETROPAIRIDS"].(string))
	_ = json.Unmarshal(hyperMetroIDBytes, &hyperMetroIDs)

	expandTask := taskflow.NewTaskFlow("Expand-FileSystem-Volume")
	expandTask.AddTask("Expand-PreCheck-Capacity", p.preExpandCheckCapacity, nil)

	if len(replicationIDs) > 0 {
		if p.replicaRemoteCli == nil {
			msg := "remote client for replication is nil"
			log.Errorln(msg)
			return errors.New(msg)
		}
		expandTask.AddTask("Expand-Remote-PreCheck-Capacity", p.preExpandCheckRemoteCapacity, nil)
		expandTask.AddTask("Expand-Replication-Remote-FileSystem", p.expandReplicationRemoteFS, nil)
	}

	if len(hyperMetroIDs) > 0 {
		if p.metroRemoteCli == nil {
			msg := "remote client for hypermetro is nil"
			log.Errorln(msg)
			return errors.New(msg)
		}
		expandTask.AddTask("Expand-Remote-PreCheck-Capacity", p.preExpandCheckRemoteCapacity, nil)
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

func (p *NAS) getvStorePair() (map[string]interface{}, error) {
	localvStore := p.cli.GetvStoreName()
	if localvStore == "" {
		return nil, nil
	}

	vStore, err := p.cli.GetvStoreByName(localvStore)
	if err != nil {
		return nil, err
	}
	if vStore == nil {
		msg := fmt.Sprintf("Cannot find vstore of name %s", localvStore)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	vStoreID := vStore["ID"].(string)

	vStorePair, err := p.cli.GetReplicationvStorePairByvStore(vStoreID)
	if err != nil {
		return nil, err
	}
	if vStorePair == nil {
		return nil, nil
	}

	if vStorePair["ROLE"] != REPLICATION_ROLE_PRIMARY {
		msg := "Local role of vstore pair is not primary"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if vStorePair["RUNNINGSTATUS"] != REPLICATION_VSTORE_PAIR_RUNNING_STATUS_NORMAL &&
		vStorePair["RUNNINGSTATUS"] != REPLICATION_VSTORE_PAIR_RUNNING_STATUS_SYNC {
		msg := "Running status of vstore pair is abnormal"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	remotevStore := vStorePair["REMOTEVSTORENAME"].(string)
	if remotevStore != p.replicaRemoteCli.GetvStoreName() {
		msg := fmt.Sprintf("Remote vstore %s does not correspond with configuration", remotevStore)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return vStorePair, nil
}

func (p *NAS) getRemoteDeviceID(deviceSN string) (string, error) {
	remoteDevice, err := p.cli.GetRemoteDeviceBySN(deviceSN)
	if err != nil {
		log.Errorf("Get remote device %s error: %v", deviceSN, err)
		return "", err
	}
	if remoteDevice == nil {
		msg := fmt.Sprintf("Remote device of SN %s does not exist", deviceSN)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	if remoteDevice["HEALTHSTATUS"] != REMOTE_DEVICE_HEALTH_STATUS ||
		remoteDevice["RUNNINGSTATUS"] != REMOTE_DEVICE_RUNNING_STATUS_LINK_UP {
		msg := fmt.Sprintf("Remote device %s status is not normal", deviceSN)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	return remoteDevice["ID"].(string), nil
}

func (p *NAS) getReplicationParams(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	var vStorePairID string
	var remoteDeviceID string
	var remoteDeviceSN string

	if p.replicaRemoteCli == nil {
		msg := "remote client for replication is nil"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	remotePoolID, err := p.getRemotePoolID(params, p.replicaRemoteCli)
	if err != nil {
		return nil, err
	}

	vStorePair, err := p.getvStorePair()
	if err != nil {
		return nil, err
	}

	if vStorePair != nil {
		vStorePairID = vStorePair["ID"].(string)
		remoteDeviceID = vStorePair["REMOTEDEVICEID"].(string)
		remoteDeviceSN = vStorePair["REMOTEDEVICESN"].(string)
	}

	remoteSystem, err := p.replicaRemoteCli.GetSystem()
	if err != nil {
		log.Errorf("Remote device is abnormal: %v", err)
		return nil, err
	}

	if remoteDeviceID == "" {
		sn := remoteSystem["ID"].(string)
		remoteDeviceID, err = p.getRemoteDeviceID(sn)
		if err != nil {
			log.Errorf("Get remote device ID of SN %s error: %v", sn, err)
			return nil, err
		}
	} else if remoteDeviceSN != remoteSystem["ID"] {
		msg := fmt.Sprintf("Remote device %s of replication vstore pair is not the same as configured one %s",
			remoteDeviceSN, remoteSystem["ID"])
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	res := map[string]interface{}{
		"remotePoolID":   remotePoolID,
		"remoteCli":      p.replicaRemoteCli,
		"remoteDeviceID": remoteDeviceID,
	}

	if vStorePairID != "" {
		res["vStorePairID"] = vStorePairID
	}

	return res, nil
}

func (p *NAS) createRemoteFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsName := params["name"].(string)
	remoteCli := taskResult["remoteCli"].(*client.Client)

	fs, err := remoteCli.GetFileSystemByName(fsName)
	if err != nil {
		log.Errorf("Get remote filesystem %s error: %v", fsName, err)
		return nil, err
	}

	if fs == nil {
		params["parentid"] = taskResult["remotePoolID"].(string)
		fs, err = remoteCli.CreateFileSystem(params)
		if err != nil {
			log.Errorf("Create remote filesystem %s error: %v", fsName, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"remoteFSID": fs["ID"].(string),
	}, nil
}

func (p *NAS) revertRemoteFS(taskResult map[string]interface{}) error {
	fsID, exist := taskResult["remoteFSID"].(string)
	if !exist || fsID == "" {
		return nil
	}

	remoteCli := taskResult["remoteCli"].(*client.Client)
	return remoteCli.DeleteFileSystem(fsID)
}

func (p *NAS) createReplicationPair(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	localFSID := taskResult["localFSID"].(string)
	remoteFSID := taskResult["remoteFSID"].(string)
	remoteDeviceID := taskResult["remoteDeviceID"].(string)

	data := map[string]interface{}{
		"LOCALRESID":       localFSID,
		"LOCALRESTYPE":     40, // filesystem
		"REMOTEDEVICEID":   remoteDeviceID,
		"REMOTERESID":      remoteFSID,
		"REPLICATIONMODEL": 2, // asynchronous replication
		"SYNCHRONIZETYPE":  2, // timed wait after synchronization begins
		"SPEED":            4, // highest speed
	}

	replicationSyncPeriod, exist := params["replicationSyncPeriod"]
	if exist {
		data["TIMINGVAL"] = replicationSyncPeriod
	}

	vStorePairID, exist := taskResult["vStorePairID"]
	if exist {
		data["VSTOREPAIRID"] = vStorePairID
	}

	pair, err := p.cli.CreateReplicationPair(data)
	if err != nil {
		log.Errorf("Create replication pair error: %v", err)
		return nil, err
	}

	pairID := pair["ID"].(string)
	err = p.cli.SyncReplicationPair(pairID)
	if err != nil {
		log.Errorf("Sync replication pair %s error: %v", pairID, err)
		p.cli.DeleteReplicationPair(pairID)
		return nil, err
	}

	return nil, nil
}

func (p *NAS) deleteReplicationPair(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	replicationIDs := params["replicationIDs"].([]string)

	for _, pairID := range replicationIDs {
		pair, err := p.cli.GetReplicationPairByID(pairID)
		if err != nil {
			return nil, err
		}

		runningStatus := pair["RUNNINGSTATUS"].(string)
		if runningStatus == REPLICATION_PAIR_RUNNING_STATUS_NORMAL ||
			runningStatus == REPLICATION_PAIR_RUNNING_STATUS_SYNC {
			p.cli.SplitReplicationPair(pairID)
		}

		err = p.cli.DeleteReplicationPair(pairID)
		if err != nil {
			log.Errorf("Delete replication pair %s error: %v", pairID, err)
			return nil, err
		}
	}

	return nil, nil
}

func (p *NAS) deleteFS(name string, cli *client.Client) error {
	sharePath := utils.GetSharePath(name)
	share, err := cli.GetNfsShareByPath(sharePath)
	if err != nil {
		log.Errorf("Get nfs share by path %s error: %v", sharePath, err)
		return err
	}

	if share != nil {
		shareID := share["ID"].(string)
		err := cli.DeleteNfsShare(shareID)
		if err != nil {
			log.Errorf("Delete share %s error: %v", shareID, err)
			return err
		}
	}

	fsName := utils.GetFileSystemName(name)
	fs, err := cli.GetFileSystemByName(fsName)
	if err != nil {
		log.Errorf("Get filesystem %s error: %v", fsName, err)
		return err
	}

	if fs == nil {
		log.Infof("Filesystem %s to delete does not exist", fsName)
		return nil
	}

	fsID := fs["ID"].(string)

	qosID, ok := fs["IOCLASSID"].(string)
	if ok && qosID != "" {
		smartX := smartx.NewSmartX(cli)
		err := smartX.DeleteQos(qosID, fsID, "fs")
		if err != nil {
			log.Errorf("Remove filesystem %s from qos %s error: %v", fsID, qosID, err)
			return err
		}
	}

	err = cli.DeleteFileSystem(fsID)
	if err != nil {
		log.Errorf("Delete filesystem %s error: %v", fsID, err)
		return err
	}

	return nil
}

func (p *NAS) deleteLocalFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name := params["name"].(string)
	err := p.deleteFS(name, p.cli)

	return nil, err
}

func (p *NAS) deleteReplicationRemoteFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name := params["name"].(string)
	err := p.deleteFS(name, p.replicaRemoteCli)

	return nil, err
}

func (p *NAS) deleteHyperMetroRemoteFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	name := params["name"].(string)
	err := p.deleteFS(name, p.metroRemoteCli)

	return nil, err
}

func (p *NAS) getHyperMetroParams(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.metroRemoteCli == nil {
		msg := "remote client for hypermetro is nil"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	remotePoolID, err := p.getRemotePoolID(params, p.metroRemoteCli)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"remotePoolID": remotePoolID,
		"remoteCli":    p.metroRemoteCli,
	}, nil
}

func (p *NAS) createHyperMetro(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	vStorePairID := params["vStorePairID"].(string)

	localFSID := taskResult["localFSID"].(string)
	remoteFSID := taskResult["remoteFSID"].(string)

	data := map[string]interface{}{
		"HCRESOURCETYPE": 2, // 2: file system
		"LOCALOBJID":     localFSID,
		"REMOTEOBJID":    remoteFSID,
		"SPEED":          4, // 4: highest speed
		"VSTOREPAIRID":   vStorePairID,
	}

	pair, err := p.cli.CreateHyperMetroPair(data)
	if err != nil {
		log.Errorf("Create nas hypermetro pair error: %v", err)
		return nil, err
	}

	pairID := pair["ID"].(string)
	err = p.cli.SyncHyperMetroPair(pairID)
	if err != nil {
		log.Errorf("Sync nas hypermetro pair %s error: %v", pairID, err)
		p.cli.DeleteHyperMetroPair(pairID)
		return nil, err
	}

	return nil, nil
}

func (p *NAS) deleteHyperMetro(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	hypermetroIDs := params["hypermetroIDs"].([]string)

	for _, pairID := range hypermetroIDs {
		pair, err := p.cli.GetHyperMetroPair(pairID)
		if err != nil {
			return nil, err
		}

		status := pair["RUNNINGSTATUS"].(string)
		if status == HYPERMETROPAIR_RUNNING_STATUS_NORMAL ||
			status == HYPERMETROPAIR_RUNNING_STATUS_TO_SYNC ||
			status == HYPERMETROPAIR_RUNNING_STATUS_SYNCING {
			p.cli.StopHyperMetroPair(pairID)
		}

		err = p.cli.DeleteHyperMetroPair(pairID)
		if err != nil {
			log.Errorf("Delete nas hypermetro pair %s error: %v", pairID, err)
			return nil, err
		}
	}

	return nil, nil
}

func (p *NAS) preExpandCheckRemoteCapacity(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	// define the client
	var cli *client.Client
	if p.replicaRemoteCli != nil {
		cli = p.replicaRemoteCli
	} else if p.metroRemoteCli != nil {
		cli = p.metroRemoteCli
	} else {
		msg := fmt.Sprintf("remote client for replication and hypermetro are nil")
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	// check the remote pool
	name := params["name"].(string)
	remoteFsName := utils.GetFileSystemName(name)
	remoteFs, err := cli.GetFileSystemByName(remoteFsName)
	if err != nil {
		log.Errorf("Get filesystem %s error: %v", remoteFsName, err)
		return nil, err
	}

	if remoteFs == nil {
		msg := fmt.Sprintf("remote filesystem %s to extend does not exist", remoteFsName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	remoteParentName := remoteFs["PARENTNAME"].(string)
	newSize := params["size"].(int64)
	curSize, err := strconv.ParseInt(remoteFs["CAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	pool, err := cli.GetPoolByName(remoteParentName)
	if err != nil || pool == nil {
		msg := fmt.Sprintf("Get storage pool %s info error: %v", remoteParentName, err)
		log.Errorf(msg)
		return nil, errors.New(msg)
	}

	freeCapacity, _ := strconv.ParseInt(pool["USERFREECAPACITY"].(string), 10, 64)
	if freeCapacity < newSize-curSize {
		msg := fmt.Sprintf("storage pool %s free capacity %s is not enough to expand to %v",
			remoteParentName, pool["USERFREECAPACITY"], newSize-curSize)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return map[string]interface{}{
		"remoteFSID": remoteFs["ID"].(string),
	}, nil
}

func (p *NAS) expandFS(objID string, newSize int64, cli *client.Client) error {
	params := map[string]interface{}{
		"CAPACITY": newSize,
	}
	err := cli.UpdateFileSystem(objID, params)
	if err != nil {
		log.Errorf("Extend FileSystem %s CAPACITY %d, error: %v", objID, newSize, err)
		return err
	}
	return nil
}

func (p *NAS) expandReplicationRemoteFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsID := taskResult["remoteFSID"].(string)
	newSize := params["size"].(int64)
	err := p.expandFS(fsID, newSize, p.replicaRemoteCli)
	if err != nil {
		log.Errorf("Expand replica filesystem %s error: %v", fsID, err)
		return nil, err
	}

	return nil, err
}

func (p *NAS) expandHyperMetroRemoteFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsID := taskResult["remoteFSID"].(string)
	newSize := params["size"].(int64)
	err := p.expandFS(fsID, newSize, p.metroRemoteCli)
	if err != nil {
		log.Errorf("Expand hyperMetro filesystem %s error: %v", fsID, err)
		return nil, err
	}

	return nil, err
}

func (p *NAS) expandLocalFS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	fsID := params["localFSID"].(string)
	newSize := params["size"].(int64)
	err := p.expandFS(fsID, newSize, p.cli)
	if err != nil {
		log.Errorf("Expand filesystem %s error: %v", fsID, err)
		return nil, err
	}
	return nil, err
}

func (p *NAS) CreateSnapshot(name, snapshotName string) (map[string]interface{}, error) {
	fsName := utils.GetFileSystemName(name)
	fs, err := p.cli.GetFileSystemByName(fsName)
	if err != nil {
		log.Errorf("Get filesystem by name %s error: %v", fsName, err)
		return nil, err
	}
	if fs == nil {
		msg := fmt.Sprintf("Filesystem %s to create snapshot does not exist", fsName)
		log.Errorf(msg)
		return nil, errors.New(msg)
	}

	fsId := fs["ID"].(string)
	snapshot, err := p.cli.GetFSSnapshotByName(fsId, snapshotName)
	if err != nil {
		log.Errorf("Get filesystem snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	snapshotSize, _ := strconv.ParseInt(fs["CAPACITY"].(string), 10, 64)
	if snapshot != nil {
		log.Infof("The snapshot %s is already exist.", snapshotName)
		return p.getSnapshotReturnInfo(snapshot, snapshotSize), nil
	}

	snapshot, err = p.cli.CreateFSSnapshot(snapshotName, fsId)
	if err != nil {
		log.Errorf("Create snapshot %s for filesystem %s error: %v", snapshotName, fsId, err)
		return nil, err
	}

	return p.getSnapshotReturnInfo(snapshot, snapshotSize), nil
}

func (p *NAS) DeleteSnapshot(snapshotParentId, snapshotName string) error {
	snapshot, err := p.cli.GetFSSnapshotByName(snapshotParentId, snapshotName)
	if err != nil {
		log.Errorf("Get filesystem snapshot by name %s error: %v", snapshotName, err)
		return err
	}

	if snapshot == nil {
		log.Infof("Filesystem snapshot %s to delete does not exist", snapshotName)
		return nil
	}

	snapshotId := snapshot["ID"].(string)
	err = p.cli.DeleteFSSnapshot(snapshotId)
	if err != nil {
		log.Errorf("Delete filesystem snapshot %s error: %v", snapshotId, err)
		return err
	}

	return nil
}