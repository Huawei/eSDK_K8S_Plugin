package volume

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/src/storage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/src/storage/oceanstor/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/taskflow"
)

const (
	LUNCOPY_HEALTH_STATUS_FAULT    = "2"
	LUNCOPY_RUNNING_STATUS_QUEUING = "37"
	LUNCOPY_RUNNING_STATUS_COPYING = "39"
	LUNCOPY_RUNNING_STATUS_STOP    = "38"
	LUNCOPY_RUNNING_STATUS_PAUSED  = "41"

	CLONEPAIR_HEALTH_STATUS_FAULT         = "1"
	CLONEPAIR_RUNNING_STATUS_UNSYNCING    = "0"
	CLONEPAIR_RUNNING_STATUS_SYNCING      = "1"
	CLONEPAIR_RUNNING_STATUS_NORMAL       = "2"
	CLONEPAIR_RUNNING_STATUS_INITIALIZING = "3"

	SNAPSHOT_RUNNING_STATUS_ACTIVE   = "43"
	SNAPSHOT_RUNNING_STATUS_INACTIVE = "45"
)

type SAN struct {
	Base
}

func NewSAN(cli, metroRemoteCli, replicaRemoteCli *client.Client) *SAN {
	return &SAN{
		Base: Base{
			cli:              cli,
			metroRemoteCli:   metroRemoteCli,
			replicaRemoteCli: replicaRemoteCli,
		},
	}
}

func (p *SAN) preCreate(params map[string]interface{}) error {
	err := p.commonPreCreate(params)
	if err != nil {
		return err
	}

	name := params["name"].(string)
	params["name"] = utils.GetLunName(name)

	if v, exist := params["sourcevolumename"].(string); exist {
		params["clonefrom"] = utils.GetLunName(v)
	} else if v, exist := params["sourcesnapshotname"].(string); exist {
		params["fromSnapshot"] = utils.GetSnapshotName(v)
	} else if v, exist := params["clonefrom"].(string); exist {
		params["clonefrom"] = utils.GetLunName(v)
	}

	return nil
}

func (p *SAN) Create(params map[string]interface{}) error {
	err := p.preCreate(params)
	if err != nil {
		return err
	}

	taskflow := taskflow.NewTaskFlow("Create-LUN-Volume")

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

	taskflow.AddTask("Get-System-Feature", p.getSystemDiffFeatures, nil)
	taskflow.AddTask("Create-Local-LUN", p.createLocalLun, p.revertLocalLun)
	taskflow.AddTask("Create-Local-QoS", p.createLocalQoS, p.revertLocalQoS)

	if replicationOK && replication {
		taskflow.AddTask("Create-Remote-LUN", p.createRemoteLun, p.revertRemoteLun)
		taskflow.AddTask("Create-Remote-QoS", p.createRemoteQoS, p.revertRemoteQoS)
		taskflow.AddTask("Create-Replication-Pair", p.createReplicationPair, nil)
	} else if hyperMetroOK && hyperMetro {
		taskflow.AddTask("Create-Remote-LUN", p.createRemoteLun, p.revertRemoteLun)
		taskflow.AddTask("Create-Remote-QoS", p.createRemoteQoS, p.revertRemoteQoS)
		taskflow.AddTask("Create-HyperMetro", p.createHyperMetro, p.revertHyperMetro)
	}

	_, err = taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return err
	}

	return nil
}

func (p *SAN) Delete(name string) error {
	lunName := utils.GetLunName(name)
	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun by name %s error: %v", lunName, err)
		return err
	}
	if lun == nil {
		log.Infof("Lun %s to delete does not exist", lunName)
		return nil
	}

	rssStr := lun["HASRSSOBJECT"].(string)

	var rss map[string]string
	json.Unmarshal([]byte(rssStr), &rss)

	taskflow := taskflow.NewTaskFlow("Delete-LUN-Volume")

	if rss["HyperMetro"] == "TRUE" {
		taskflow.AddTask("Delete-HyperMetro", p.deleteHyperMetro, nil)
		taskflow.AddTask("Delete-HyperMetro-Remote-LUN", p.deleteHyperMetroRemoteLun, nil)
	}

	if rss["RemoteReplication"] == "TRUE" {
		taskflow.AddTask("Delete-Replication-Pair", p.deleteReplicationPair, nil)
		taskflow.AddTask("Delete-Replication-Remote-LUN", p.deleteReplicationRemoteLun, nil)
	}

	if rss["LunCopy"] == "TRUE" {
		taskflow.AddTask("Delete-Local-LunCopy", p.deleteLocalLunCopy, nil)
	}

	if rss["HyperCopy"] == "TRUE" {
		taskflow.AddTask("Delete-Local-HyperCopy", p.deleteLocalHyperCopy, nil)
	}

	taskflow.AddTask("Delete-Local-LUN", p.deleteLocalLun, nil)

	params := map[string]interface{}{
		"lun":     lun,
		"lunID":   lun["ID"].(string),
		"lunName": lunName,
	}

	_, err = taskflow.Run(params)
	return err
}

func (p *SAN) Expand(name string, newSize int64) (bool, error) {
	lunName := utils.GetLunName(name)
	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun by name %s error: %v", lunName, err)
		return false, err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s to expand does not exist", lunName)
		log.Errorf(msg)
		return false, errors.New(msg)
	}

	isAttached := lun["EXPOSEDTOINITIATOR"] == "true"
	curSize, _ := strconv.ParseInt(lun["CAPACITY"].(string), 10, 64)
	if newSize <= curSize {
		log.Infof("Lun %s newSize %d must be greater than curSize %d", lunName, newSize, curSize)
		return isAttached, nil
	}

	rssStr := lun["HASRSSOBJECT"].(string)
	var rss map[string]string
	json.Unmarshal([]byte(rssStr), &rss)

	expandTask := taskflow.NewTaskFlow("Expand-LUN-Volume")
	expandTask.AddTask("Expand-PreCheck-Capacity", p.preExpandCheckCapacity, nil)

	if rss["HyperMetro"] == "TRUE" {
		expandTask.AddTask("Expand-HyperMetro-Remote-PreCheck-Capacity", p.preExpandHyperMetroCheckRemoteCapacity, nil)
		expandTask.AddTask("Suspend-HyperMetro", p.suspendHyperMetro, nil)
		expandTask.AddTask("Expand-HyperMetro-Remote-LUN", p.expandHyperMetroRemoteLun, nil)
	}

	if rss["RemoteReplication"] == "TRUE" {
		expandTask.AddTask("Expand-Replication-Remote-PreCheck-Capacity", p.preExpandReplicationCheckRemoteCapacity, nil)
		expandTask.AddTask("Split-Replication", p.splitReplication, nil)
		expandTask.AddTask("Expand-Replication-Remote-LUN", p.expandReplicationRemoteLun, nil)
	}

	expandTask.AddTask("Expand-Local-Lun", p.expandLocalLun, nil)

	if rss["HyperMetro"] == "TRUE" {
		expandTask.AddTask("Sync-HyperMetro", p.syncHyperMetro, nil)
	}

	if rss["RemoteReplication"] == "TRUE" {
		expandTask.AddTask("Sync-Replication", p.syncReplication, nil)
	}

	params := map[string]interface{}{
		"name":            name,
		"size":            newSize,
		"expandSize":      newSize - curSize,
		"lunID":           lun["ID"].(string),
		"localParentName": lun["PARENTNAME"].(string),
	}
	_, err = expandTask.Run(params)
	return isAttached, err
}

func (p *SAN) createLocalLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["name"].(string)

	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get LUN %s error: %v", lunName, err)
		return nil, err
	}

	if lun == nil {
		params["parentid"] = params["poolID"].(string)

		if _, exist := params["clonefrom"]; exist {
			lun, err = p.clone(params, taskResult)
		} else if _, exist := params["fromSnapshot"]; exist {
			lun, err = p.createFromSnapshot(params, taskResult)
		} else {
			lun, err = p.cli.CreateLun(params)
		}

		if err != nil {
			log.Errorf("Create LUN %s error: %v", lunName, err)
			return nil, err
		}
	} else {
		err := p.waitCloneFinish(lun, taskResult)
		if err != nil {
			log.Errorf("Wait clone finish for LUN %s error: %v", lunName, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"localLunID": lun["ID"].(string),
	}, nil
}

func (p *SAN) clonePair(params map[string]interface{}) (map[string]interface{}, error) {
	cloneFrom := params["clonefrom"].(string)
	srcLun, err := p.cli.GetLunByName(cloneFrom)
	if err != nil {
		log.Errorf("Get clone src LUN %s error: %v", cloneFrom, err)
		return nil, err
	}
	if srcLun == nil {
		msg := fmt.Sprintf("Clone src LUN %s does not exist", cloneFrom)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	srcLunCapacity, err := strconv.ParseInt(srcLun["CAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}
	cloneLunCapacity := params["capacity"].(int64)
	if cloneLunCapacity < srcLunCapacity {
		msg := fmt.Sprintf("Clone LUN capacity must be >= src %s", cloneFrom)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	dstLun, err := p.cli.GetLunByName(params["name"].(string))
	if err != nil {
		return nil, err
	}
	if dstLun == nil {
		copyParams := utils.CopyMap(params)
		copyParams["capacity"] = srcLunCapacity

		dstLun, err = p.cli.CreateLun(copyParams)
		if err != nil {
			return nil, err
		}
	}
	srcLunID := srcLun["ID"].(string)
	dstLunID := dstLun["ID"].(string)

	cloneSpeed := params["clonespeed"].(int)
	err = p.createClonePair(srcLunID, dstLunID, cloneLunCapacity, srcLunCapacity, cloneSpeed)
	if err != nil {
		log.Errorf("Create clone pair, source lun ID %s, target lun ID %s error: %s", srcLunID, dstLunID, err)
		p.cli.DeleteLun(dstLunID)
		return nil, err
	}

	return dstLun, nil
}

func (p *SAN) fromSnapshotByClonePair(params map[string]interface{}) (map[string]interface{}, error) {
	srcSnapshotName := params["fromSnapshot"].(string)
	srcSnapshot, err := p.cli.GetLunSnapshotByName(srcSnapshotName)
	if err != nil {
		return nil, err
	}
	if srcSnapshot == nil {
		msg := fmt.Sprintf("Clone snapshot %s does not exist", srcSnapshotName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	srcSnapshotCapacity, err := strconv.ParseInt(srcSnapshot["USERCAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	cloneLunCapacity := params["capacity"].(int64)
	if cloneLunCapacity < srcSnapshotCapacity {
		msg := fmt.Sprintf("Clone target LUN capacity must be >= src snapshot %s", srcSnapshotName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	dstLun, err := p.cli.GetLunByName(params["name"].(string))
	if err != nil {
		return nil, err
	}
	if dstLun == nil {
		copyParams := utils.CopyMap(params)
		copyParams["capacity"] = srcSnapshotCapacity

		dstLun, err = p.cli.CreateLun(copyParams)
		if err != nil {
			return nil, err
		}
	}

	srcSnapshotID := srcSnapshot["ID"].(string)
	dstLunID := dstLun["ID"].(string)
	cloneSpeed := params["clonespeed"].(int)
	err = p.createClonePair(srcSnapshotID, dstLunID, cloneLunCapacity, srcSnapshotCapacity, cloneSpeed)
	if err != nil {
		log.Errorf("Clone snapshot by clone pair, source snapshot ID %s, target lun ID %s error: %s", srcSnapshotID, dstLunID, err)

		p.cli.DeleteLun(dstLunID)
		return nil, err
	}

	return dstLun, nil
}

func (p *SAN) createClonePair(srcLunID, dstLunID string, cloneLunCapacity, srcLunCapacity int64, cloneSpeed int) error {
	ClonePair, err := p.cli.CreateClonePair(srcLunID, dstLunID, cloneSpeed)
	if err != nil {
		log.Errorf("Create ClonePair from %s to %s error: %v", srcLunID, dstLunID, err)
		return err
	}

	ClonePairID := ClonePair["ID"].(string)
	if srcLunCapacity < cloneLunCapacity {
		err = p.cli.ExtendLun(dstLunID, cloneLunCapacity)
		if err != nil {
			log.Errorf("Extend clone lun %s error: %v", dstLunID, err)
			p.cli.DeleteClonePair(ClonePairID)
			return err
		}
	}

	err = p.cli.SyncClonePair(ClonePairID)
	if err != nil {
		log.Errorf("Start ClonePair %s error: %v", ClonePairID, err)
		p.cli.DeleteClonePair(ClonePairID)
		return err
	}

	err = p.waitClonePairFinish(ClonePairID)
	if err != nil {
		log.Errorf("Wait ClonePair %s finish error: %v", ClonePairID, err)
		return err
	}

	return nil
}

func (p *SAN) lunCopy(params map[string]interface{}) (map[string]interface{}, error) {
	clonefrom := params["clonefrom"].(string)
	srcLun, err := p.cli.GetLunByName(clonefrom)
	if err != nil {
		log.Errorf("Get clone src LUN %s error: %v", clonefrom, err)
		return nil, err
	}
	if srcLun == nil {
		msg := fmt.Sprintf("Clone src LUN %s does not exist", clonefrom)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	srcLunCapacity, err := strconv.ParseInt(srcLun["CAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	cloneLunCapacity := params["capacity"].(int64)
	if cloneLunCapacity < srcLunCapacity {
		msg := fmt.Sprintf("Clone LUN capacity must be >= src %s", clonefrom)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	dstLun, err := p.cli.GetLunByName(params["name"].(string))
	if err != nil {
		return nil, err
	}
	if dstLun == nil {
		dstLun, err = p.cli.CreateLun(params)
		if err != nil {
			return nil, err
		}
	}

	srcLunID := srcLun["ID"].(string)
	dstLunID := dstLun["ID"].(string)
	snapshotName := fmt.Sprintf("k8s_lun_%s_to_%s_snap", srcLunID, dstLunID)

	smartX := smartx.NewSmartX(p.cli)
	snapshot, err := p.cli.GetLunSnapshotByName(snapshotName)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		snapshot, err = smartX.CreateLunSnapshot(snapshotName, srcLunID)
		if err != nil {
			log.Errorf("Create snapshot %s error: %v", snapshotName, err)
			p.cli.DeleteLun(dstLunID)

			return nil, err
		}
	}

	snapshotID := snapshot["ID"].(string)
	cloneSpeed := params["clonespeed"].(int)
	lunCopyName, err := p.createLunCopy(snapshotID, dstLunID, cloneSpeed, true)
	if err != nil {
		log.Errorf("Create lun copy, source snapshot ID %s, target lun ID %s error: %s", snapshotID, dstLunID, err)
		smartX.DeleteLunSnapshot(snapshotID)
		p.cli.DeleteLun(dstLunID)
		return nil, err
	}

	err = p.waitLunCopyFinish(lunCopyName)
	if err != nil {
		log.Errorf("Wait luncopy %s finish error: %v", lunCopyName, err)
		return nil, err
	}

	err = p.deleteLunCopy(lunCopyName, true)
	if err != nil {
		log.Errorf("Delete luncopy %s error: %v", lunCopyName, err)
		return nil, err
	}

	return dstLun, nil
}

func (p *SAN) fromSnapshotByLunCopy(params map[string]interface{}) (map[string]interface{}, error) {
	srcSnapshotName := params["fromSnapshot"].(string)
	srcSnapshot, err := p.cli.GetLunSnapshotByName(srcSnapshotName)
	if err != nil {
		return nil, err
	}
	if srcSnapshot == nil {
		msg := fmt.Sprintf("Clone src snapshot %s does not exist", srcSnapshotName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	srcSnapshotCapacity, err := strconv.ParseInt(srcSnapshot["USERCAPACITY"].(string), 10, 64)
	if err != nil {
		return nil, err
	}

	cloneLunCapacity := params["capacity"].(int64)
	if cloneLunCapacity < srcSnapshotCapacity {
		msg := fmt.Sprintf("Clone LUN capacity must be >= src snapshot%s", srcSnapshotName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	dstLun, err := p.cli.GetLunByName(params["name"].(string))
	if err != nil {
		return nil, err
	}
	if dstLun == nil {
		dstLun, err = p.cli.CreateLun(params)
		if err != nil {
			return nil, err
		}
	}

	srcSnapshotID := srcSnapshot["ID"].(string)
	dstLunID := dstLun["ID"].(string)
	cloneSpeed := params["clonespeed"].(int)
	lunCopyName, err := p.createLunCopy(srcSnapshotID, dstLunID, cloneSpeed, false)
	if err != nil {
		log.Errorf("Create LunCopy, source snapshot ID %s, target lun ID %s error: %s", srcSnapshotID, dstLunID, err)
		p.cli.DeleteLun(dstLunID)
		return nil, err
	}

	err = p.waitLunCopyFinish(lunCopyName)
	if err != nil {
		log.Errorf("Wait luncopy %s finish error: %v", lunCopyName, err)
		return nil, err
	}

	err = p.deleteLunCopy(lunCopyName, false)
	if err != nil {
		log.Errorf("Delete luncopy %s error: %v", lunCopyName, err)
		return nil, err
	}

	return dstLun, nil
}

func (p *SAN) createLunCopy(snapshotID, dstLunID string, cloneSpeed int, isDeleteSnapshot bool) (string, error) {
	lunCopyName := fmt.Sprintf("k8s_luncopy_%s_to_%s", snapshotID, dstLunID)

	lunCopy, err := p.cli.GetLunCopyByName(lunCopyName)
	if err != nil {
		return "", err
	}

	if lunCopy == nil {
		lunCopy, err = p.cli.CreateLunCopy(lunCopyName, snapshotID, dstLunID, cloneSpeed)
		if err != nil {
			log.Errorf("Create luncopy from %s to %s error: %v", snapshotID, dstLunID, err)
			return "", err
		}
	}

	lunCopyID := lunCopy["ID"].(string)

	err = p.cli.StartLunCopy(lunCopyID)
	if err != nil {
		log.Errorf("Start luncopy %s error: %v", lunCopyID, err)
		p.cli.DeleteLunCopy(lunCopyID)
		return "", err
	}

	return lunCopyName, nil
}

func (p *SAN) clone(params map[string]interface{}, taskResult map[string]interface{}) (map[string]interface{}, error) {
	isSupportClonePair := taskResult["isSupportClonePair"].(bool)
	if isSupportClonePair {
		return p.clonePair(params)
	} else {
		return p.lunCopy(params)
	}
}

func (p *SAN) createFromSnapshot(params map[string]interface{}, taskResult map[string]interface{}) (map[string]interface{}, error) {
	isSupportClonePair := taskResult["isSupportClonePair"].(bool)
	if isSupportClonePair {
		return p.fromSnapshotByClonePair(params)
	} else {
		return p.fromSnapshotByLunCopy(params)
	}
}

func (p *SAN) revertLocalLun(taskResult map[string]interface{}) error {
	lunID, exist := taskResult["localLunID"].(string)
	if !exist || lunID == "" {
		return nil
	}

	err := p.cli.DeleteLun(lunID)
	return err
}

func (p *SAN) createLocalQoS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	lunID := taskResult["localLunID"].(string)
	lun, err := p.cli.GetLunByID(lunID)
	if err != nil {
		return nil, err
	}

	qosID, exist := lun["IOCLASSID"].(string)
	if !exist || qosID == "" {
		smartX := smartx.NewSmartX(p.cli)
		qosID, err = smartX.CreateQos(lunID, "lun", qos)
		if err != nil {
			log.Errorf("Create qos %v for lun %s error: %v", qos, lunID, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"localQosID": qosID,
	}, nil
}

func (p *SAN) revertLocalQoS(taskResult map[string]interface{}) error {
	lunID, lunIDExist := taskResult["localLunID"].(string)
	qosID, qosIDExist := taskResult["localQosID"].(string)
	if !lunIDExist || !qosIDExist {
		return nil
	}

	smartX := smartx.NewSmartX(p.cli)
	err := smartX.DeleteQos(qosID, lunID, "lun")
	return err
}

func (p *SAN) getLunCopyOfLunID(lunID string) (string, error) {
	lun, err := p.cli.GetLunByID(lunID)
	if err != nil {
		return "", err
	}

	lunCopyIDStr, exist := lun["LUNCOPYIDS"].(string)
	if !exist || lunCopyIDStr == "" {
		return "", nil
	}

	var lunCopyIDs []string

	json.Unmarshal([]byte(lunCopyIDStr), &lunCopyIDs)
	if len(lunCopyIDs) <= 0 {
		return "", nil
	}

	lunCopyID := lunCopyIDs[0]
	lunCopy, err := p.cli.GetLunCopyByID(lunCopyID)
	if err != nil {
		return "", err
	}

	return lunCopy["NAME"].(string), nil
}

func (p *SAN) deleteLunCopy(lunCopyName string, isDeleteSnapshot bool) error {
	lunCopy, err := p.cli.GetLunCopyByName(lunCopyName)
	if err != nil {
		return err
	}
	if lunCopy == nil {
		return nil
	}

	lunCopyID := lunCopy["ID"].(string)
	runningStatus := lunCopy["RUNNINGSTATUS"].(string)
	if runningStatus == LUNCOPY_RUNNING_STATUS_QUEUING ||
		runningStatus == LUNCOPY_RUNNING_STATUS_COPYING {
		p.cli.StopLunCopy(lunCopyID)
	}

	err = p.cli.DeleteLunCopy(lunCopyID)
	if err != nil {
		return err
	}

	snapshotName := lunCopy["SOURCELUNNAME"].(string)
	snapshot, err := p.cli.GetLunSnapshotByName(snapshotName)
	if err == nil && snapshot != nil && isDeleteSnapshot {
		snapshotID := snapshot["ID"].(string)
		smartX := smartx.NewSmartX(p.cli)
		smartX.DeleteLunSnapshot(snapshotID)
	}

	return nil
}

func (p *SAN) waitLunCopyFinish(lunCopyName string) error {
	err := utils.WaitUntil(func() (bool, error) {
		lunCopy, err := p.cli.GetLunCopyByName(lunCopyName)
		if err != nil {
			return false, err
		}
		if lunCopy == nil {
			return true, nil
		}

		healthStatus := lunCopy["HEALTHSTATUS"].(string)
		if healthStatus == LUNCOPY_HEALTH_STATUS_FAULT {
			return false, fmt.Errorf("Luncopy %s is at fault status", lunCopyName)
		}

		runningStatus := lunCopy["RUNNINGSTATUS"].(string)
		if runningStatus == LUNCOPY_RUNNING_STATUS_QUEUING ||
			runningStatus == LUNCOPY_RUNNING_STATUS_COPYING {
			return false, nil
		} else if runningStatus == LUNCOPY_RUNNING_STATUS_STOP ||
			runningStatus == LUNCOPY_RUNNING_STATUS_PAUSED {
			return false, fmt.Errorf("Luncopy %s is stopped", lunCopyName)
		} else {
			return true, nil
		}
	}, time.Hour*6, time.Second*5)

	if err != nil {
		return err
	}

	return nil
}

func (p *SAN) waitClonePairFinish(clonePairID string) error {
	err := utils.WaitUntil(func() (bool, error) {
		clonePair, err := p.cli.GetClonePairInfo(clonePairID)
		if err != nil {
			return false, err
		}
		if clonePair == nil {
			return true, nil
		}

		healthStatus := clonePair["copyStatus"].(string)
		if healthStatus == CLONEPAIR_HEALTH_STATUS_FAULT {
			return false, fmt.Errorf("ClonePair %s is at fault status", clonePairID)
		}

		runningStatus := clonePair["syncStatus"].(string)
		if runningStatus == CLONEPAIR_RUNNING_STATUS_NORMAL {
			return true, nil
		} else if runningStatus == CLONEPAIR_RUNNING_STATUS_SYNCING ||
			runningStatus == CLONEPAIR_RUNNING_STATUS_INITIALIZING ||
			runningStatus == CLONEPAIR_RUNNING_STATUS_UNSYNCING {
			return false, nil
		} else {
			return false, fmt.Errorf("ClonePair %s running status is abnormal", clonePairID)
		}
	}, time.Hour*6, time.Second*5)

	if err != nil {
		return err
	}

	p.cli.DeleteClonePair(clonePairID)
	return nil
}

func (p *SAN) waitCloneFinish(lun map[string]interface{}, taskResult map[string]interface{}) error {
	lunID := lun["ID"].(string)

	isSupportClonePair := taskResult["isSupportClonePair"].(bool)
	if isSupportClonePair {
		// ID of clone pair is the same as destination LUN ID
		err := p.waitClonePairFinish(lunID)
		if err != nil {
			return err
		}
	} else {
		lunCopyName, err := p.getLunCopyOfLunID(lunID)
		if err != nil {
			return err
		}

		if len(lunCopyName) > 0 {
			err := p.waitLunCopyFinish(lunCopyName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *SAN) createRemoteLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["name"].(string)
	remoteCli := taskResult["remoteCli"].(*client.Client)

	lun, err := remoteCli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get remote LUN %s error: %v", lunName, err)
		return nil, err
	}

	if lun == nil {
		params["parentid"] = taskResult["remotePoolID"].(string)

		lun, err = remoteCli.CreateLun(params)
		if err != nil {
			log.Errorf("Create remote LUN %s error: %v", lunName, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"remoteLunID": lun["ID"].(string),
	}, nil
}

func (p *SAN) revertRemoteLun(taskResult map[string]interface{}) error {
	lunID, exist := taskResult["remoteLunID"].(string)
	if !exist {
		return nil
	}

	remoteCli := taskResult["remoteCli"].(*client.Client)
	return remoteCli.DeleteLun(lunID)
}

func (p *SAN) createRemoteQoS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	lunID := taskResult["remoteLunID"].(string)
	remoteCli := taskResult["remoteCli"].(*client.Client)

	lun, err := remoteCli.GetLunByID(lunID)
	if err != nil {
		return nil, err
	}

	qosID, exist := lun["IOCLASSID"].(string)
	if !exist || qosID == "" {
		smartX := smartx.NewSmartX(remoteCli)
		qosID, err = smartX.CreateQos(lunID, "lun", qos)
		if err != nil {
			log.Errorf("Create qos %v for lun %s error: %v", qos, lunID, err)
			return nil, err
		}
	}

	return map[string]interface{}{
		"remoteQosID": qosID,
	}, nil
}

func (p *SAN) revertRemoteQoS(taskResult map[string]interface{}) error {
	lunID, lunIDExist := taskResult["remoteLunID"].(string)
	qosID, qosIDExist := taskResult["remoteQosID"].(string)
	if !lunIDExist || !qosIDExist {
		return nil
	}

	remoteCli := taskResult["remoteCli"].(*client.Client)
	smartX := smartx.NewSmartX(remoteCli)
	return smartX.DeleteQos(qosID, lunID, "lun")
}

func (p *SAN) createHyperMetro(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	domainID := taskResult["metroDomainID"].(string)
	localLunID := taskResult["localLunID"].(string)
	remoteLunID := taskResult["remoteLunID"].(string)

	pair, err := p.cli.GetHyperMetroPairByLocalObjID(localLunID)
	if err != nil {
		log.Errorf("Get hypermetro pair by local obj ID %s error: %v", localLunID, err)
		return nil, err
	}

	var pairID string

	if pair == nil {
		_, needFirstSync1 := params["clonefrom"]
		_, needFirstSync2 := params["fromSnapshot"]
		needFirstSync := needFirstSync1 || needFirstSync2
		data := map[string]interface{}{
			"DOMAINID":       domainID,
			"HCRESOURCETYPE": 1,
			"ISFIRSTSYNC":    needFirstSync,
			"LOCALOBJID":     localLunID,
			"REMOTEOBJID":    remoteLunID,
			"SPEED":          4,
		}

		pair, err := p.cli.CreateHyperMetroPair(data)
		if err != nil {
			log.Errorf("Create hypermetro pair between lun (%s-%s) error: %v", localLunID, remoteLunID, err)
			return nil, err
		}

		pairID = pair["ID"].(string)

		if needFirstSync {
			err := p.cli.SyncHyperMetroPair(pairID)
			if err != nil {
				log.Errorf("Sync hypermetro pair %s error: %v", pairID, err)
				p.cli.DeleteHyperMetroPair(pairID)
				return nil, err
			}

			err = p.waitHyperMetroSyncFinish(pairID)
			if err != nil {
				log.Errorf("Wait hypermetro pair %s sync done error: %v", pairID, err)
				p.cli.DeleteHyperMetroPair(pairID)
				return nil, err
			}
		}
	} else {
		pairID = pair["ID"].(string)

		err := p.waitHyperMetroSyncFinish(pairID)
		if err != nil {
			log.Errorf("Wait hypermetro pair %s sync done error: %v", pairID, err)
			p.cli.DeleteHyperMetroPair(pairID)
			return nil, err
		}
	}

	return map[string]interface{}{
		"hyperMetroPairID": pairID,
	}, nil
}

func (p *SAN) waitHyperMetroSyncFinish(pairID string) error {
	err := utils.WaitUntil(func() (bool, error) {
		pair, err := p.cli.GetHyperMetroPair(pairID)
		if err != nil {
			return false, err
		}
		if pair == nil {
			msg := fmt.Sprintf("Something wrong with hypermetro pair %s", pairID)
			log.Errorln(msg)
			return false, errors.New(msg)
		}

		healthStatus := pair["HEALTHSTATUS"].(string)
		if healthStatus == HYPERMETROPAIR_HEALTH_STATUS_FAULT {
			return false, fmt.Errorf("Hypermetro pair %s is fault", pairID)
		}

		runningStatus := pair["RUNNINGSTATUS"].(string)
		if runningStatus == HYPERMETROPAIR_RUNNING_STATUS_TO_SYNC ||
			runningStatus == HYPERMETROPAIR_RUNNING_STATUS_SYNCING {
			return false, nil
		} else if runningStatus == HYPERMETROPAIR_RUNNING_STATUS_UNKNOWN ||
			runningStatus == HYPERMETROPAIR_RUNNING_STATUS_PAUSE ||
			runningStatus == HYPERMETROPAIR_RUNNING_STATUS_ERROR ||
			runningStatus == HYPERMETROPAIR_RUNNING_STATUS_INVALID {
			return false, fmt.Errorf("Hypermetro pair %s is at running status %s", pairID, runningStatus)
		} else {
			return true, nil
		}
	}, time.Hour*6, time.Second*5)

	if err != nil {
		p.cli.StopHyperMetroPair(pairID)
		return err
	}

	return nil
}

func (p *SAN) revertHyperMetro(taskResult map[string]interface{}) error {
	hyperMetroPairID, exist := taskResult["hyperMetroPairID"].(string)
	if !exist {
		return nil
	}

	err := p.cli.StopHyperMetroPair(hyperMetroPairID)
	if err != nil {
		log.Warningf("Stop hypermetro pair %s error: %v", hyperMetroPairID, err)
	}

	return p.cli.DeleteHyperMetroPair(hyperMetroPairID)
}

func (p *SAN) getHyperMetroParams(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	metroDomain, exist := params["metrodomain"].(string)
	if !exist || len(metroDomain) == 0 {
		msg := "No hypermetro domain is specified for metro volume"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if p.metroRemoteCli == nil {
		msg := "remote client for hypermetro is nil"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	remotePoolID, err := p.getRemotePoolID(params, p.metroRemoteCli)
	if err != nil {
		return nil, err
	}

	domain, err := p.metroRemoteCli.GetHyperMetroDomainByName(metroDomain)
	if err != nil || domain == nil {
		msg := fmt.Sprintf("Cannot get hypermetro domain %s ID", metroDomain)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	if status := domain["RUNNINGSTATUS"].(string); status != HYPERMETRODOMAIN_RUNNING_STATUS_NORMAL {
		msg := fmt.Sprintf("Hypermetro domain %s status is not normal", metroDomain)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return map[string]interface{}{
		"remotePoolID":  remotePoolID,
		"remoteCli":     p.metroRemoteCli,
		"metroDomainID": domain["ID"].(string),
	}, nil
}

func (p *SAN) getSystemDiffFeatures(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	systemInfo, err := p.cli.GetSystem()
	if err != nil {
		log.Errorf("Get system info error: %v", err)
		return nil, err
	}

	clonePairFlag := utils.IsDoradoV6(systemInfo)

	return map[string]interface{}{
		"isSupportClonePair": clonePairFlag,
	}, nil
}

func (p *SAN) deleteLocalLunCopy(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunID := params["lunID"].(string)

	lunCopyName, err := p.getLunCopyOfLunID(lunID)
	if err != nil {
		log.Errorf("Get luncopy of LUN %s error: %v", lunID, err)
		return nil, err
	}

	if lunCopyName != "" {
		err := p.deleteLunCopy(lunCopyName, true)
		if err != nil {
			log.Errorf("Try to delete luncopy of lun %s error: %v", lunID, err)
			return nil, err
		}
	}

	return nil, nil
}

func (p *SAN) deleteLocalHyperCopy(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunID := params["lunID"].(string)

	// ID of clone pair is the same as destination LUN ID
	clonePair, err := p.cli.GetClonePairInfo(lunID)
	if err != nil {
		log.Errorf("Get clone pair %s error: %v", lunID, err)
		return nil, err
	}
	if clonePair == nil {
		return nil, nil
	}

	clonePairID := clonePair["ID"].(string)
	err = p.cli.DeleteClonePair(clonePairID)
	if err != nil {
		log.Errorf("Delete clone pair %s error: %v", clonePairID, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) deleteLocalLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["lunName"].(string)
	err := p.deleteLun(lunName, p.cli)
	return nil, err
}

func (p *SAN) deleteHyperMetroRemoteLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.metroRemoteCli == nil {
		log.Warningln("HyperMetro remote cli is nil, the remote lun will be leftover")
		return nil, nil
	}

	lunName := params["lunName"].(string)
	err := p.deleteLun(lunName, p.metroRemoteCli)
	return nil, err
}

func (p *SAN) deleteHyperMetro(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunID := params["lunID"].(string)

	pair, err := p.cli.GetHyperMetroPairByLocalObjID(lunID)
	if err != nil {
		log.Errorf("Get hypermetro pair by local obj ID %s error: %v", lunID, err)
		return nil, err
	}
	if pair == nil {
		return nil, nil
	}

	pairID := pair["ID"].(string)
	status := pair["RUNNINGSTATUS"].(string)

	if status == HYPERMETROPAIR_RUNNING_STATUS_NORMAL ||
		status == HYPERMETROPAIR_RUNNING_STATUS_TO_SYNC ||
		status == HYPERMETROPAIR_RUNNING_STATUS_SYNCING {
		p.cli.StopHyperMetroPair(pairID)
	}

	err = p.cli.DeleteHyperMetroPair(pairID)
	if err != nil {
		log.Errorf("Delete hypermetro pair %s error: %v", pairID, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) preExpandCheckRemoteCapacity(params map[string]interface{}, cli *client.Client) (string, error) {
	// check the remote pool
	name := params["name"].(string)
	remoteLunName := utils.GetLunName(name)
	remoteLun, err := cli.GetLunByName(remoteLunName)
	if err != nil {
		log.Errorf("Get lun by name %s error: %v", remoteLunName, err)
		return "", err
	}
	if remoteLun == nil {
		msg := fmt.Sprintf("remote lun %s to extend does not exist", remoteLunName)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	remoteParentName := remoteLun["PARENTNAME"].(string)
	newSize := params["size"].(int64)
	curSize, err := strconv.ParseInt(remoteLun["CAPACITY"].(string), 10, 64)
	if err != nil {
		return "", err
	}

	pool, err := cli.GetPoolByName(remoteParentName)
	if err != nil || pool == nil {
		log.Errorf("Get storage pool %s info error: %v", remoteParentName, err)
		return "", err
	}

	freeCapacity, _ := strconv.ParseInt(pool["USERFREECAPACITY"].(string), 10, 64)
	if freeCapacity < newSize-curSize {
		msg := fmt.Sprintf("storage pool %s free capacity %s is not enough to expand to %v",
			remoteParentName, pool["USERFREECAPACITY"], newSize-curSize)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	return remoteLun["ID"].(string), nil
}

func (p *SAN) preExpandHyperMetroCheckRemoteCapacity(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	remoteLunID, err := p.preExpandCheckRemoteCapacity(params, p.metroRemoteCli)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"remoteLunID": remoteLunID,
	}, nil
}

func (p *SAN) preExpandReplicationCheckRemoteCapacity(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	remoteLunID, err := p.preExpandCheckRemoteCapacity(params, p.replicaRemoteCli)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"remoteLunID": remoteLunID,
	}, nil
}

func (p *SAN) suspendHyperMetro(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunID := params["lunID"].(string)

	pair, err := p.cli.GetHyperMetroPairByLocalObjID(lunID)
	if err != nil {
		log.Errorf("Get hypermetro pair by local obj ID %s error: %v", lunID, err)
		return nil, err
	}
	if pair == nil {
		return nil, nil
	}

	pairID := pair["ID"].(string)
	status := pair["RUNNINGSTATUS"].(string)

	if status == HYPERMETROPAIR_RUNNING_STATUS_NORMAL ||
		status == HYPERMETROPAIR_RUNNING_STATUS_TO_SYNC ||
		status == HYPERMETROPAIR_RUNNING_STATUS_SYNCING {
		err := p.cli.StopHyperMetroPair(pairID)
		if err != nil {
			log.Errorf("Suspend san hypermetro pair %s error: %v", pairID, err)
			return nil, err
		}
	}
	return map[string]interface{}{
		"hyperMetroPairID": pairID,
	}, nil
}

func (p *SAN) expandHyperMetroRemoteLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	remoteLunID := taskResult["remoteLunID"].(string)
	newSize := params["size"].(int64)

	err := p.metroRemoteCli.ExtendLun(remoteLunID, newSize)
	if err != nil {
		log.Errorf("Extend hypermetro remote lun %s error: %v", remoteLunID, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) syncHyperMetro(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	pairID := taskResult["hyperMetroPairID"].(string)
	if pairID == "" {
		return nil, nil
	}

	err := p.cli.SyncHyperMetroPair(pairID)
	if err != nil {
		log.Errorf("Sync san hypermetro pair %s error: %v", pairID, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) expandLocalLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunID := params["lunID"].(string)
	newSize := params["size"].(int64)

	err := p.cli.ExtendLun(lunID, newSize)
	if err != nil {
		log.Errorf("Expand lun %s error: %v", lunID, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) CreateSnapshot(lunName, snapshotName string) (map[string]interface{}, error) {
	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun by name %s error: %v", lunName, err)
		return nil, err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s to create snapshot does not exist", lunName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	lunId := lun["ID"].(string)
	snapshot, err := p.cli.GetLunSnapshotByName(snapshotName)
	if err != nil {
		log.Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	if snapshot != nil {
		snapshotParentId := snapshot["PARENTID"].(string)
		if snapshotParentId != lunId {
			msg := fmt.Sprintf("Snapshot %s is already exist, but the parent LUN %s is incompatible", snapshotName, lunName)
			log.Errorln(msg)
			return nil, errors.New(msg)
		} else {
			snapshotSize, _ := strconv.ParseInt(snapshot["USERCAPACITY"].(string), 10, 64)
			return p.getSnapshotReturnInfo(snapshot, snapshotSize), nil
		}
	}

	taskflow := taskflow.NewTaskFlow("Create-LUN-Snapshot")
	taskflow.AddTask("Create-Snapshot", p.createSnapshot, p.revertSnapshot)
	taskflow.AddTask("Active-Snapshot", p.activateSnapshot, nil)

	params := map[string]interface{}{
		"lunID":        lunId,
		"snapshotName": snapshotName,
	}

	result, err := taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return nil, err
	}

	snapshot, err = p.cli.GetLunSnapshotByName(snapshotName)
	if err != nil {
		log.Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
		return nil, err
	}

	snapshotSize, _ := strconv.ParseInt(result["snapshotSize"].(string), 10, 64)
	return p.getSnapshotReturnInfo(snapshot, snapshotSize), nil
}

func (p *SAN) DeleteSnapshot(snapshotName string) error {
	snapshot, err := p.cli.GetLunSnapshotByName(snapshotName)
	if err != nil {
		log.Errorf("Get lun snapshot by name %s error: %v", snapshotName, err)
		return err
	}

	if snapshot == nil {
		log.Infof("Lun snapshot %s to delete does not exist", snapshotName)
		return nil
	}

	taskflow := taskflow.NewTaskFlow("Delete-LUN-Snapshot")
	taskflow.AddTask("Deactivate-Snapshot", p.deactivateSnapshot, nil)
	taskflow.AddTask("Delete-Snapshot", p.deleteSnapshot, nil)

	params := map[string]interface{}{
		"snapshotId": snapshot["ID"].(string),
	}

	_, err = taskflow.Run(params)
	return err
}

func (p *SAN) createSnapshot(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunID := params["lunID"].(string)
	snapshotName := params["snapshotName"].(string)

	snapshot, err := p.cli.CreateLunSnapshot(snapshotName, lunID)
	if err != nil {
		log.Errorf("Create snapshot %s for lun %s error: %v", snapshotName, lunID, err)
		return nil, err
	}

	err = p.waitSnapshotReady(snapshotName)
	if err != nil {
		log.Errorf("Wait snapshot ready by name %s error: %v", snapshotName, err)
		return nil, err
	}

	return map[string]interface{}{
		"snapshotId":   snapshot["ID"].(string),
		"snapshotSize": snapshot["USERCAPACITY"].(string),
	}, nil
}

func (p *SAN) waitSnapshotReady(snapshotName string) error {
	err := utils.WaitUntil(func() (bool, error) {
		snapshot, err := p.cli.GetLunSnapshotByName(snapshotName)
		if err != nil {
			return false, err
		}
		if snapshot == nil {
			msg := fmt.Sprintf("Something wrong with snapshot %s", snapshotName)
			log.Errorln(msg)
			return false, errors.New(msg)
		}

		runningStatus := snapshot["RUNNINGSTATUS"].(string)
		if err != nil {
			return false, err
		}

		if runningStatus == SNAPSHOT_RUNNING_STATUS_ACTIVE || runningStatus == SNAPSHOT_RUNNING_STATUS_INACTIVE {
			return true, nil
		} else {
			return false, nil
		}
	}, time.Hour*6, time.Second*5)

	if err != nil {
		return err
	}
	return nil
}

func (p *SAN) revertSnapshot(taskResult map[string]interface{}) error {
	snapshotID := taskResult["snapshotId"].(string)

	err := p.cli.DeleteLunSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Delete snapshot %s error: %v", snapshotID, err)
		return err
	}

	return nil
}

func (p *SAN) activateSnapshot(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	snapshotID := taskResult["snapshotId"].(string)

	err := p.cli.ActivateLunSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Activate snapshot %s error: %v", snapshotID, err)
		return nil, err
	}
	return nil, nil
}

func (p *SAN) revertActivateSnapshot(taskResult map[string]interface{}) error {
	snapshotID := taskResult["snapshotId"].(string)

	err := p.cli.DeactivateLunSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Deactivate snapshot %s error: %v", snapshotID, err)
		return err
	}
	return nil
}

func (p *SAN) deleteSnapshot(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	snapshotID := params["snapshotId"].(string)

	err := p.cli.DeleteLunSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Delete snapshot %s error: %v", snapshotID, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) deactivateSnapshot(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	snapshotID := params["snapshotId"].(string)

	err := p.cli.DeactivateLunSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Deactivate snapshot %s error: %v", snapshotID, err)
		return nil, err
	}
	return nil, nil
}

func (p *SAN) getReplicationParams(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.replicaRemoteCli == nil {
		msg := "remote client for replication is nil"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	remotePoolID, err := p.getRemotePoolID(params, p.replicaRemoteCli)
	if err != nil {
		return nil, err
	}

	remoteSystem, err := p.replicaRemoteCli.GetSystem()
	if err != nil {
		log.Errorf("Remote device is abnormal: %v", err)
		return nil, err
	}

	sn := remoteSystem["ID"].(string)
	remoteDeviceID, err := p.getRemoteDeviceID(sn)
	if err != nil {
		return nil, err
	}

	res := map[string]interface{}{
		"remotePoolID":   remotePoolID,
		"remoteCli":      p.replicaRemoteCli,
		"remoteDeviceID": remoteDeviceID,
		"resType":        11,
	}

	return res, nil
}

func (p *SAN) deleteReplicationPair(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunID := params["lunID"].(string)

	pairs, err := p.cli.GetReplicationPairByResID(lunID, 11)
	if err != nil {
		return nil, err
	}

	if pairs == nil || len(pairs) == 0 {
		return nil, nil
	}

	for _, pair := range pairs {
		pairID := pair["ID"].(string)

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

func (p *SAN) deleteReplicationRemoteLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	if p.replicaRemoteCli == nil {
		log.Warningln("Replication remote cli is nil, the remote lun will be leftover")
		return nil, nil
	}

	lunName := params["lunName"].(string)
	err := p.deleteLun(lunName, p.replicaRemoteCli)
	return nil, err
}

func (p *SAN) deleteLun(name string, cli *client.Client) error {
	lun, err := cli.GetLunByName(name)
	if err != nil {
		log.Errorf("Get lun by name %s error: %v", name, err)
		return err
	}
	if lun == nil {
		log.Infof("Lun %s to delete does not exist", name)
		return nil
	}

	lunID := lun["ID"].(string)

	qosID, exist := lun["IOCLASSID"].(string)
	if exist && qosID != "" {
		smartX := smartx.NewSmartX(cli)
		err := smartX.DeleteQos(qosID, lunID, "lun")
		if err != nil {
			log.Errorf("Remove lun %s from qos %s error: %v", lunID, qosID, err)
			return err
		}
	}

	err = cli.DeleteLun(lunID)
	if err != nil {
		log.Errorf("Delete lun %s error: %v", lunID, err)
		return err
	}

	return nil
}

func (p *SAN) splitReplication(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunID := params["lunID"].(string)

	pairs, err := p.cli.GetReplicationPairByResID(lunID, 11)
	if err != nil {
		return nil, err
	}

	if pairs == nil || len(pairs) == 0 {
		return nil, nil
	}

	replicationPairIDs := []string{}

	for _, pair := range pairs {
		pairID := pair["ID"].(string)

		runningStatus := pair["RUNNINGSTATUS"].(string)
		if runningStatus != REPLICATION_PAIR_RUNNING_STATUS_NORMAL &&
			runningStatus != REPLICATION_PAIR_RUNNING_STATUS_SYNC {
			continue
		}

		err := p.cli.SplitReplicationPair(pairID)
		if err != nil {
			return nil, err
		}

		replicationPairIDs = append(replicationPairIDs, pairID)
	}

	return map[string]interface{}{
		"replicationPairIDs": replicationPairIDs,
	}, nil
}

func (p *SAN) expandReplicationRemoteLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	remoteLunID := taskResult["remoteLunID"].(string)
	newSize := params["size"].(int64)

	err := p.replicaRemoteCli.ExtendLun(remoteLunID, newSize)
	if err != nil {
		log.Errorf("Extend replication remote lun %s error: %v", remoteLunID, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) syncReplication(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	replicationPairIDs := taskResult["replicationPairIDs"].([]string)

	for _, pairID := range replicationPairIDs {
		err := p.cli.SyncReplicationPair(pairID)
		if err != nil {
			log.Errorf("Sync san replication pair %s error: %v", pairID, err)
			return nil, err
		}
	}

	return nil, nil
}
