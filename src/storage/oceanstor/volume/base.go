package volume

import (
	"errors"
	"fmt"
	"storage/oceanstor/client"
	"storage/oceanstor/smartx"
	"strconv"
	"utils/log"
)

const (
	HYPERMETROPAIR_HEALTH_STATUS_FAULT    = "2"
	HYPERMETROPAIR_RUNNING_STATUS_NORMAL  = "1"
	HYPERMETROPAIR_RUNNING_STATUS_TO_SYNC = "100"
	HYPERMETROPAIR_RUNNING_STATUS_SYNCING = "23"
	HYPERMETROPAIR_RUNNING_STATUS_UNKNOWN = "0"
	HYPERMETROPAIR_RUNNING_STATUS_PAUSE   = "41"
	HYPERMETROPAIR_RUNNING_STATUS_ERROR   = "94"
	HYPERMETROPAIR_RUNNING_STATUS_INVALID = "35"

	HYPERMETRODOMAIN_RUNNING_STATUS_NORMAL = "1"
)

type Base struct {
	cli              *client.Client
	metroRemoteCli   *client.Client
	replicaRemoteCli *client.Client
}

func (p *Base) commonPreCreate(params map[string]interface{}) error {
	analyzers := [...]func(map[string]interface{}) error{
		p.getAllocType,
		p.getCloneSpeed,
		p.getPoolID,
		p.getQoS,
	}

	for _, analyzer := range analyzers {
		err := analyzer(params)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Base) getAllocType(params map[string]interface{}) error {
	if v, exist := params["alloctype"].(string); exist && v == "thick" {
		params["alloctype"] = 0
	} else {
		params["alloctype"] = 1
	}

	return nil
}

func (p *Base) getCloneSpeed(params map[string]interface{}) error {
	_, cloneExist := params["clonefrom"].(string)
	_, srcVolumeExist := params["sourcevolumename"].(string)
	_, srcSnapshotExist := params["sourcesnapshotname"].(string)
	if !(cloneExist || srcVolumeExist || srcSnapshotExist) {
		return nil
	}

	if v, exist := params["clonespeed"].(string); exist && v != "" {
		speed, err := strconv.Atoi(v)
		if err != nil || speed < 1 || speed > 4 {
			return fmt.Errorf("error config %s for clonespeed", v)
		}
		params["clonespeed"] = speed
	} else {
		params["clonespeed"] = 3
	}

	return nil
}

func (p *Base) getPoolID(params map[string]interface{}) error {
	poolName, exist := params["storagepool"].(string)
	if !exist || poolName == "" {
		return errors.New("must specify storage pool to create volume")
	}

	pool, err := p.cli.GetPoolByName(poolName)
	if err != nil {
		log.Errorf("Get storage pool %s info error: %v", poolName, err)
		return err
	}
	if pool == nil {
		return fmt.Errorf("Storage pool %s doesn't exist", poolName)
	}

	params["poolID"] = pool["ID"].(string)

	return nil
}

func (p *Base) getQoS(params map[string]interface{}) error {
	if v, exist := params["qos"].(string); exist && v != "" {
		qos, err := smartx.VerifyQos(v)
		if err != nil {
			log.Errorf("Verify qos %s error: %v", v, err)
			return err
		}

		params["qos"] = qos
	}

	return nil
}

func (p *Base) getRemotePoolID(params map[string]interface{}, remoteCli *client.Client) (string, error) {
	remotePool, exist := params["remotestoragepool"].(string)
	if !exist || len(remotePool) == 0 {
		msg := "no remote pool is specified"
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	pool, err := remoteCli.GetPoolByName(remotePool)
	if err != nil {
		log.Errorf("Get remote storage pool %s info error: %v", remotePool, err)
		return "", err
	}
	if pool == nil {
		return "", fmt.Errorf("remote storage pool %s doesn't exist", remotePool)
	}

	return pool["ID"].(string), nil
}

func (p *Base) preExpandCheckCapacity(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	// check the local pool
	localParentName := params["localParentName"].(string)
	expandSize := params["expandSize"].(int64)
	pool, err := p.cli.GetPoolByName(localParentName)
	if err != nil || pool == nil {
		msg := fmt.Sprintf("Get storage pool %s info error: %v", localParentName, err)
		log.Errorf(msg)
		return nil, errors.New(msg)
	}
	freeCapacity, _ := strconv.ParseInt(pool["USERFREECAPACITY"].(string), 10, 64)
	if freeCapacity < expandSize {
		msg := fmt.Sprintf("storage pool %s free capacity %s is not enough to expand to %v",
			localParentName, pool["USERFREECAPACITY"], expandSize)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return nil, nil
}

func (p *Base) getSnapshotReturnInfo(snapshot map[string]interface{}, snapshotSize int64) map[string]interface{} {
	snapshotCreated, _ := strconv.ParseInt(snapshot["TIMESTAMP"].(string), 10, 64)
	snapshotSizeBytes := snapshotSize * 512
	return map[string]interface{}{
		"CreationTime": snapshotCreated,
		"SizeBytes":    snapshotSizeBytes,
		"ParentID":     snapshot["PARENTID"].(string),
	}
}

func (p *Base) createReplicationPair(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	resType := taskResult["resType"].(int)
	remoteDeviceID := taskResult["remoteDeviceID"].(string)

	var localID string
	var remoteID string

	if resType == 11 {
		localID = taskResult["localLunID"].(string)
		remoteID = taskResult["remoteLunID"].(string)
	} else {
		localID = taskResult["localFSID"].(string)
		remoteID = taskResult["remoteFSID"].(string)
	}

	data := map[string]interface{}{
		"LOCALRESID":       localID,
		"LOCALRESTYPE":     resType,
		"REMOTEDEVICEID":   remoteDeviceID,
		"REMOTERESID":      remoteID,
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

func (p *Base) getRemoteDeviceID(deviceSN string) (string, error) {
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
