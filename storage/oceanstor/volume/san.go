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

	CLONEPAIR_HEALTH_STATUS_FAULT    = "1"
	CLONEPAIR_RUNNING_STATUS_UNSYNCING    = "0"
	CLONEPAIR_RUNNING_STATUS_SYNCING      = "1"
	CLONEPAIR_RUNNING_STATUS_NORMAL       = "2"
	CLONEPAIR_RUNNING_STATUS_INITIALIZING = "3"

	HYPERMETROPAIR_HEALTH_STATUS_FAULT    = "2"
	HYPERMETROPAIR_RUNNING_STATUS_TO_SYNC = "100"
	HYPERMETROPAIR_RUNNING_STATUS_SYNCING = "23"
	HYPERMETROPAIR_RUNNING_STATUS_UNKNOWN = "0"
	HYPERMETROPAIR_RUNNING_STATUS_PAUSE   = "41"
	HYPERMETROPAIR_RUNNING_STATUS_ERROR   = "94"
	HYPERMETROPAIR_RUNNING_STATUS_INVALID = "35"
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
	if v, exist := params["hypermetro"].(string); exist {
		hypermetro, err := strconv.ParseBool(v)
		if err != nil {
			log.Errorf("Hypermetro %s is invalid", v)
			return err
		}
		if hypermetro {
			err := p.getHyperMetroParams(params)
			if err != nil {
				log.Errorf("Parse hypermetro params error: %v", err)
				return err
			}
		}

		params["hypermetro"] = hypermetro
	}

	return nil
}

func (p *SAN) getHyperMetroParams(params map[string]interface{}) error {
	metroDomain, exist := params["metrodomain"].(string)
	if !exist {
		msg := "No hypermetro domain is specified for metro volume"
		log.Errorln(msg)
		return errors.New(msg)
	}

	if p.remoteCli == nil {
		msg := "No hypermetro remote backend is specified for metro volume"
		log.Errorln(msg)
		return errors.New(msg)
	}

	err := p.remoteCli.Login()
	if err != nil {
		log.Errorf("Cannot login hypermetro remote backend: %v", err)
		return err
	}

	if v, exist := params["remotestoragepool"].(string); exist {
		pool, err := p.remoteCli.GetPoolByName(v)
		if err != nil {
			log.Errorf("Get hypermetro remote storage pool %s info error: %v", v, err)
			return err
		}
		if pool == nil {
			return fmt.Errorf("Hypermetro remote storage pool %s doesn't exist", v)
		}

		params["remotePoolID"] = pool["ID"].(string)
	} else {
		return errors.New("Must specify remote storage pool to create hypermetro volume")
	}

	domainID, err := p.remoteCli.GetHyperMetroDomainID(metroDomain)
	if err != nil || domainID == "" {
		msg := fmt.Sprintf("Cannot get hypermetro domain %s ID", metroDomain)
		log.Errorln(msg)
		return errors.New(msg)
	}

	params["metroDomainID"] = domainID

	return nil
}

func (p *SAN) postCreate(params map[string]interface{}) {
	p.cli.Logout()

	hyperMetro, exist := params["hypermetro"].(bool)
	if exist && hyperMetro {
		p.remoteCli.Logout()
	}
}

func (p *SAN) Create(params map[string]interface{}) error {
	name := params["name"].(string)
	if taskStatusCache[name] {
		delete(taskStatusCache,name)
		return nil
	}
	err := p.preCreate(params)
	if err != nil {
		return err
	}

	taskflow := taskflow.NewTaskFlow("Create-LUN-Volume")

	taskflow.AddTask("Get-System-Feature", p.getSystemDiffFeatures, nil)
	taskflow.AddTask("Create-Local-LUN", p.createLocalLun, p.deleteLocalLun)
	taskflow.AddTask("Create-Local-QoS", p.createLocalQoS, p.deleteLocalQoS)

	hyperMetro, exist := params["hypermetro"].(bool)
	if exist && hyperMetro {
		taskflow.AddTask("Create-Remote-LUN", p.createRemoteLun, p.deleteRemoteLun)
		taskflow.AddTask("Create-Remote-QoS", p.createRemoteQoS, p.deleteRemoteQoS)
		taskflow.AddTask("Create-HyperMetro", p.createHyperMetro, p.deleteHyperMetro)
	}

	err = taskflow.Run(params)
	if err != nil {
		taskflow.Revert()
		return err
	}
	taskStatusCache[name] = true //任务已完成

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

	lunID := lun["ID"].(string)
	rssStr := lun["HASRSSOBJECT"].(string)

	var rss map[string]string
	json.Unmarshal([]byte(rssStr), &rss)

	if rss["HyperMetro"] == "TRUE" {
		pair, err := p.cli.GetHyperMetroPairByLocalObjID(lunID)
		if err != nil {
			log.Errorf("Get hypermetro pair by local obj ID %s error: %v", lunID, err)
			return err
		}
		if pair != nil {
			pairID := pair["ID"].(string)

			p.cli.StopHyperMetroPair(pairID)
			p.cli.DeleteHyperMetroPair(pairID)
		}

		remoteLunID := pair["REMOTEOBJID"].(string)
		err = p.remoteCli.Login()
		p.remoteCli.DeleteLun(remoteLunID)
	}

	if rss["LunCopy"] == "TRUE" {
		lunCopyName, err := p.getLunCopyOfLunID(lunID)
		if err != nil {
			log.Errorf("Get luncopy of LUN %s error: %v", lunName, err)
			return err
		}

		if lunCopyName != "" {
			err := p.deleteLunCopy(lunCopyName)
			if err != nil {
				log.Errorf("Try to delete luncopy of lun %s error: %v", lunID, err)
				return err
			}
		}
	}

	qosID, exist := lun["IOCLASSID"].(string)
	if exist && qosID != "" {
		smartX := smartx.NewSmartX(p.cli)
		err := smartX.DeleteQos(qosID, lunID, "lun")
		if err != nil {
			log.Errorf("Remove lun %s from qos %s error: %v", lunID, qosID, err)
			return err
		}
	}

	err = p.cli.DeleteLun(lunID)
	if err != nil {
		log.Errorf("Delete lun %s error: %v", lunName, err)
		return err
	}

	return nil
}

func (p *SAN) createLocalLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["name"].(string)

	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get LUN %s error: %v", lunName, err)
		return nil, err
	}

	clonePairFlag := taskResult["isSupportClonePair"].(bool)
	cloneFrom, exist := params["clonefrom"].(string)
	if lun == nil {
		if exist {
			cloneFrom = utils.GetLunName(cloneFrom)
			params["clonefrom"] = cloneFrom
			if clonePairFlag {
				lun, err = p.clonePair(params)
			} else {
				lun, err = p.clone(params)
			}
		} else {
			lun, err = p.cli.CreateLun(params)
		}

		if err != nil {
			log.Errorf("Create LUN %s error: %v", lunName, err)
			return nil, err
		}
	} else {
		if exist {
			lunID := lun["ID"].(string)
			if clonePairFlag {
				err = p.waitClonePairFinish(lunID)
				if err != nil {
					return nil, err
				}
			} else {
				lunCopyName, err := p.getLunCopyOfLunID(lunID)
				if err != nil {
					return nil, err
				}

				if lunCopyName != "" {
					err = p.waitLunCopyFinish(lunCopyName)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	return map[string]interface{}{
		"localLunID": lun["ID"].(string),
	}, nil
}

func (p *SAN) clonePair(params map[string]interface{}) (map[string]interface{}, error){
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

	cloneSpeed := params["clonespeed"].(string)
	ClonePair, err := p.cli.CreateClonePair(srcLunID, dstLunID, cloneSpeed)
	if err != nil {
		log.Errorf("Create ClonePair from %s to %s error: %v", srcLunID, dstLunID, err)
		p.cli.DeleteLun(dstLunID)
		return nil, err
	}

	ClonePairID := ClonePair["ID"].(string)
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

	if srcLunCapacity < cloneLunCapacity {
		err = p.cli.ExtendLun(dstLunID, cloneLunCapacity)
		if err != nil {
			log.Errorf("Extend clone lun %s error: %v", dstLunID, err)
			p.cli.DeleteLun(dstLunID)
			return nil, err
		}
	}
	return dstLun, nil
}

func (p *SAN) clone(params map[string]interface{}) (map[string]interface{}, error) {
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
		return nil, err
	}
	if lunCopy == nil {
		clonespeed := params["clonespeed"].(string)
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

func (p *SAN) deleteLocalLun(taskResult map[string]interface{}) error {
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

func (p *SAN) deleteLocalQoS(taskResult map[string]interface{}) error {
	lunID := taskResult["localLunID"].(string)
	qosID := taskResult["localQosID"].(string)

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

func (p *SAN) waitClonePairFinish(clonePairID string) error{
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


func (p *SAN) createRemoteLun(params, taskResult map[string]interface{}) (map[string]interface{}, error) {
	lunName := params["name"].(string)

	lun, err := p.remoteCli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get remote LUN %s error: %v", lunName, err)
		return nil, err
	}

	if lun == nil {
		params["parentid"] = params["remotePoolID"].(string)

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

func (p *SAN) deleteRemoteLun(taskResult map[string]interface{}) error {
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
	lun, err := p.cli.GetLunByID(lunID)
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

func (p *SAN) deleteRemoteQoS(taskResult map[string]interface{}) error {
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
	domainID := params["metroDomainID"].(string)
	localLunID := taskResult["localLunID"].(string)
	remoteLunID := taskResult["remoteLunID"].(string)

	pair, err := p.cli.GetHyperMetroPairByLocalObjID(localLunID)
	if err != nil {
		log.Errorf("Get hypermetro pair by local obj ID %s error: %v", localLunID, err)
		return nil, err
	}

	var pairID string

	if pair == nil {
		pair, err := p.cli.CreateHyperMetroPair(domainID, localLunID, remoteLunID)
		if err != nil {
			log.Errorf("Create hypermetro pair between lun (%s-%s) error: %v", localLunID, remoteLunID, err)
			return nil, err
		}

		pairID = pair["ID"].(string)

		err = p.cli.SyncHyperMetroPair(pairID)
		if err != nil {
			log.Errorf("Sync hypermetro pair %s error: %v", pairID, err)
			p.cli.DeleteHyperMetroPair(pairID)
			return nil, err
		}
	} else {
		pairID = pair["ID"].(string)
	}

	err = utils.WaitUntil(func() (bool, error) {
		pair, err := p.cli.GetHyperMetroPair(pairID)
		if err != nil {
			return false, err
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
	}, time.Hour*6, time.Second*30)

	if err != nil {
		p.cli.StopHyperMetroPair(pairID)
		p.cli.DeleteHyperMetroPair(pairID)

		return nil, err
	}

	return map[string]interface{}{
		"hyperMetroPairID": pairID,
	}, nil
}

func (p *SAN) deleteHyperMetro(taskResult map[string]interface{}) error {
	hyperMetroPairID, exist := taskResult["hyperMetroPairID"].(string)
	if !exist {
		return nil
	}

	err := p.cli.StopHyperMetroPair(hyperMetroPairID)
	if err != nil {
		log.Warningf("Stop hypermetro pair %s error: %v", hyperMetroPairID, err)
	}

	err = p.cli.DeleteHyperMetroPair(hyperMetroPairID)
	return err
}

func (p *SAN) getSystemDiffFeatures(params, taskResult map[string]interface{}) (map[string]interface{}, error){
	systemInfo, err := p.cli.GetSystem()
	if err != nil {
		log.Errorf("Get system info %s error: %v", systemInfo, err)
		return nil, err
	}
	clonePairFlag := utils.IsSupportClonePair(systemInfo)

	return map[string]interface{} {
		"isSupportClonePair": clonePairFlag,
	}, nil
}
