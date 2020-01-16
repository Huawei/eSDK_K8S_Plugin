package volume

import (
	"encoding/json"
	"errors"
	"fmt"
	"storage/oceanstor/client"
	"storage/oceanstor/smartx"
	"strconv"
	"time"
	"utils"
	"utils/log"
	"utils/taskflow"
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

type SAN struct {
	Base
	remoteCli *client.Client
}

func NewSAN(cli, remoteCli *client.Client) *SAN {
	return &SAN{
		Base:      Base{cli: cli},
		remoteCli: remoteCli,
	}
}

func (p *SAN) preCreate(params map[string]interface{}) error {
	err := p.commonPreCreate(params)
	if err != nil {
		return err
	}

	name := params["name"].(string)
	params["name"] = utils.GetLunName(name)

	if v, exist := params["clonefrom"].(string); exist {
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

	hyperMetro, ok := params["hypermetro"].(bool)
	if ok && hyperMetro {
		taskflow.AddTask("Get-HyperMetro-Params", p.getHyperMetroParams, nil)
	}

	taskflow.AddTask("Get-System-Feature", p.getSystemDiffFeatures, nil)
	taskflow.AddTask("Create-Local-LUN", p.createLocalLun, p.revertLocalLun)
	taskflow.AddTask("Create-Local-QoS", p.createLocalQoS, p.revertLocalQoS)

	if ok && hyperMetro {
		taskflow.AddTask("Create-Remote-LUN", p.createRemoteLun, p.revertRemoteLun)
		taskflow.AddTask("Create-Remote-QoS", p.createRemoteQoS, p.revertRemoteQoS)
		taskflow.AddTask("Create-HyperMetro", p.createHyperMetro, p.revertHyperMetro)
	}

	err = taskflow.Run(params)
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
		taskflow.AddTask("Delete-Remote-LUN", p.deleteRemoteLun, nil)
	}

	if rss["LunCopy"] == "TRUE" {
		taskflow.AddTask("Delete-Local-LunCopy", p.deleteLocalLunCopy, nil)
	}

	if rss["HyperCopy"] == "TRUE" {
		taskflow.AddTask("Delete-Local-HyperCopy", p.deleteLocalHyperCopy, nil)
	}

	taskflow.AddTask("Delete-Local-LUN", p.deleteLocalLun, nil)

	params := map[string]interface{}{
		"lun":   lun,
		"lunID": lun["ID"].(string),
	}

	return taskflow.Run(params)
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

		_, exist := params["clonefrom"]
		if exist {
			lun, err = p.clone(params, taskResult)
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
	} else {
		params["capacity"] = srcLunCapacity
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

	cloneSpeed := params["clonespeed"].(int)
	ClonePair, err := p.cli.CreateClonePair(srcLunID, dstLunID, cloneSpeed)
	if err != nil {
		log.Errorf("Create ClonePair from %s to %s error: %v", srcLunID, dstLunID, err)
		p.cli.DeleteLun(dstLunID)
		return nil, err
	}

	ClonePairID := ClonePair["ID"].(string)

	if srcLunCapacity < cloneLunCapacity {
		err = p.cli.ExtendLun(dstLunID, cloneLunCapacity)
		if err != nil {
			log.Errorf("Extend clone lun %s error: %v", dstLunID, err)
			p.cli.DeleteClonePair(ClonePairID)
			p.cli.DeleteLun(dstLunID)

			return nil, err
		}
	}

	err = p.cli.SyncClonePair(ClonePairID)
	if err != nil {
		log.Errorf("Start ClonePair %s error: %v", ClonePairID, err)
		p.cli.DeleteClonePair(ClonePairID)
		p.cli.DeleteLun(dstLunID)

		return nil, err
	}

	err = p.waitClonePairFinish(ClonePairID)
	if err != nil {
		log.Errorf("Wait ClonePair %s finish error: %v", ClonePairID, err)
		return nil, err
	}

	return dstLun, nil
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
	lunCopyName := fmt.Sprintf("k8s_luncopy_%s_to_%s", snapshotID, dstLunID)

	lunCopy, err := p.cli.GetLunCopyByName(lunCopyName)
	if err != nil {
		smartX.DeleteLunSnapshot(snapshotID)
		p.cli.DeleteLun(dstLunID)

		return nil, err
	}
	if lunCopy == nil {
		clonespeed := params["clonespeed"].(int)
		lunCopy, err = p.cli.CreateLunCopy(lunCopyName, snapshotID, dstLunID, clonespeed)
		if err != nil {
			log.Errorf("Create luncopy from %s to %s error: %v", snapshotID, dstLunID, err)
			smartX.DeleteLunSnapshot(snapshotID)
			p.cli.DeleteLun(dstLunID)

			return nil, err
		}
	}

	lunCopyID := lunCopy["ID"].(string)

	err = p.cli.StartLunCopy(lunCopyID)
	if err != nil {
		log.Errorf("Start luncopy %s error: %v", lunCopyID, err)
		p.cli.DeleteLunCopy(lunCopyID)
		smartX.DeleteLunSnapshot(snapshotID)
		p.cli.DeleteLun(dstLunID)

		return nil, err
	}

	err = p.waitLunCopyFinish(lunCopyName)
	if err != nil {
		log.Errorf("Wait luncopy %s finish error: %v", lunCopyID, err)
		return nil, err
	}

	return dstLun, nil
}

func (p *SAN) clone(params map[string]interface{}, taskResult map[string]interface{}) (map[string]interface{}, error) {
	isSupportClonePair := taskResult["isSupportClonePair"].(bool)
	if isSupportClonePair {
		return p.clonePair(params)
	} else {
		return p.lunCopy(params)
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

func (p *SAN) deleteLunCopy(lunCopyName string) error {
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
	if err == nil && snapshot != nil {
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

	p.deleteLunCopy(lunCopyName)
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

	lun, err := p.remoteCli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get remote LUN %s error: %v", lunName, err)
		return nil, err
	}

	if lun == nil {
		params["parentid"] = taskResult["remotePoolID"].(string)

		lun, err = p.remoteCli.CreateLun(params)
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

	err := p.remoteCli.DeleteLun(lunID)
	return err
}

func (p *SAN) createRemoteQoS(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	qos, exist := params["qos"].(map[string]int)
	if !exist {
		return nil, nil
	}

	lunID := taskResult["remoteLunID"].(string)
	lun, err := p.remoteCli.GetLunByID(lunID)
	if err != nil {
		return nil, err
	}

	qosID, exist := lun["IOCLASSID"].(string)
	if !exist || qosID == "" {
		smartX := smartx.NewSmartX(p.remoteCli)
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

	smartX := smartx.NewSmartX(p.remoteCli)
	err := smartX.DeleteQos(qosID, lunID, "lun")
	return err
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
		_, needFirstSync := params["clonefrom"]
		pair, err := p.cli.CreateHyperMetroPair(domainID, localLunID, remoteLunID, needFirstSync)
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

	remotePool, exist := params["remotestoragepool"].(string)
	if !exist || len(remotePool) == 0 {
		msg := "No remote pool is specified for metro volume"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	pool, err := p.remoteCli.GetPoolByName(remotePool)
	if err != nil {
		log.Errorf("Get hypermetro remote storage pool %s info error: %v", remotePool, err)
		return nil, err
	}
	if pool == nil {
		return nil, fmt.Errorf("Hypermetro remote storage pool %s doesn't exist", remotePool)
	}

	domain, err := p.remoteCli.GetHyperMetroDomain(metroDomain)
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
		"remotePoolID":  pool["ID"].(string),
		"metroDomainID": domain["ID"].(string),
	}, nil
}

func (p *SAN) getSystemDiffFeatures(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	systemInfo, err := p.cli.GetSystem()
	if err != nil {
		log.Errorf("Get system info %s error: %v", systemInfo, err)
		return nil, err
	}

	clonePairFlag := utils.IsSupportClonePair(systemInfo)

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
		err := p.deleteLunCopy(lunCopyName)
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
	lun := params["lun"].(map[string]interface{})
	lunID := params["lunID"].(string)

	qosID, exist := lun["IOCLASSID"].(string)
	if exist && qosID != "" {
		smartX := smartx.NewSmartX(p.cli)
		err := smartX.DeleteQos(qosID, lunID, "lun")
		if err != nil {
			log.Errorf("Remove lun %s from qos %s error: %v", lunID, qosID, err)
			return nil, err
		}
	}

	err := p.cli.DeleteLun(lunID)
	if err != nil {
		log.Errorf("Delete lun %s error: %v", lunID, err)
		return nil, err
	}

	return nil, nil
}

func (p *SAN) deleteRemoteLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	remoteLunID, ok := taskResult["remoteLunID"].(string)
	if !ok {
		// No remote lun exists, directly return.
		return nil, nil
	}

	lun, err := p.remoteCli.GetLunByID(remoteLunID)
	if err != nil {
		log.Errorf("Get hypermetro remote lun by ID %s error: %v", remoteLunID, err)
		return nil, err
	}

	qosID, exist := lun["IOCLASSID"].(string)
	if exist && qosID != "" {
		smartX := smartx.NewSmartX(p.remoteCli)
		err := smartX.DeleteQos(qosID, remoteLunID, "lun")
		if err != nil {
			log.Errorf("Remove hypermetro remote lun %s from qos %s error: %v", remoteLunID, qosID, err)
			return nil, err
		}
	}

	err = p.remoteCli.DeleteLun(remoteLunID)
	if err != nil {
		log.Errorf("Delete hypermetro remote lun %s error: %v", remoteLunID, err)
		return nil, err
	}

	return nil, nil
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

	return map[string]interface{}{
		"remoteLunID": pair["REMOTEOBJID"].(string),
	}, nil
}
