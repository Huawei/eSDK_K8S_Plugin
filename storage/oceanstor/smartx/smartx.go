package smartx

import (
	"encoding/json"
	"errors"
	"fmt"
	"storage/oceanstor/client"
	"strings"
	"time"
	"utils/log"
)

var (
	VALID_QOS_KEY = map[string]func(int) bool{
		"IOTYPE": func(value int) bool {
			return value == 0 || value == 1 || value == 2
		},
		"MAXBANDWIDTH": func(value int) bool {
			return value > 0
		},
		"MINBANDWIDTH": func(value int) bool {
			return value > 0
		},
		"MAXIOPS": func(value int) bool {
			return value > 0
		},
		"MINIOPS": func(value int) bool {
			return value > 0
		},
		"LATENCY": func(value int) bool {
			return value > 0
		},
	}
)

func VerifyQos(qosConfig string) (map[string]int, error) {
	var msg string
	var lowerLimit, upperLimit bool
	var params map[string]int

	err := json.Unmarshal([]byte(qosConfig), &params)
	if err != nil {
		log.Errorf("Unmarshal %s error: %v", qosConfig, err)
		return nil, err
	}

	for k, v := range params {
		f, exist := VALID_QOS_KEY[k]
		if !exist {
			msg = fmt.Sprintf("%s is a invalid key for QoS", k)
			goto ERROR
		}

		if !f(v) {
			msg = fmt.Sprintf("%s of qos specs is invalid", k)
			goto ERROR
		}

		if strings.HasPrefix(k, "MIN") || strings.HasPrefix(k, "LATENCY") {
			lowerLimit = true
		} else if strings.HasPrefix(k, "MAX") {
			upperLimit = true
		}
	}

	if lowerLimit && upperLimit {
		msg = fmt.Sprintf("Cannot specify both lower and upper limits for qos")
		goto ERROR
	}

	return params, nil

ERROR:
	log.Errorln(msg)
	return nil, errors.New(msg)
}

type SmartX struct {
	cli *client.Client
}

func NewSmartX(cli *client.Client) *SmartX {
	return &SmartX{
		cli: cli,
	}
}

func (p *SmartX) getQosName(objID, objType string) string {
	now := time.Now().Format("20060102150405")
	return fmt.Sprintf("k8s_%s%s_%s", objType, objID, now)
}

func (p *SmartX) CreateQos(objID, objType string, params map[string]int) (string, error) {
	var err error
	var lowerLimit bool

	for k, _ := range params {
		if strings.HasPrefix(k, "MIN") || strings.HasPrefix(k, "LATENCY") {
			lowerLimit = true
		}
	}

	if lowerLimit {
		data := map[string]interface{}{
			"IOPRIORITY": 3,
		}

		if objType == "fs" {
			err = p.cli.UpdateFileSystem(objID, data)
		} else {
			err = p.cli.UpdateLun(objID, data)
		}

		if err != nil {
			log.Errorf("Upgrade obj %s of type %s IOPRIORITY error: %v", objID, objID, err)
			return "", err
		}
	}

	name := p.getQosName(objID, objType)
	qos, err := p.cli.CreateQos(name, objID, objType, params)
	if err != nil {
		log.Errorf("Create qos %v for obj %s of type %s error: %v", params, objID, objType, err)
		return "", err
	}

	qosID := qos["ID"].(string)

	qosStatus := qos["ENABLESTATUS"].(string)
	if qosStatus == "false" {
		err := p.cli.ActivateQos(qosID)
		if err != nil {
			log.Errorf("Activate qos %s error: %v", qosID, err)
			return "", err
		}
	}

	return qosID, nil
}

func (p *SmartX) DeleteQos(qosID, objID, objType string) error {
	qos, err := p.cli.GetQosByID(qosID)
	if err != nil {
		log.Errorf("Get qos by ID %s error: %v", qosID, err)
		return err
	}

	var listObj string
	var listStr string
	var objList []string

	if objType == "fs" {
		listObj = "FSLIST"
	} else {
		listObj = "LUNLIST"
	}

	listStr = qos[listObj].(string)
	err = json.Unmarshal([]byte(listStr), &objList)
	if err != nil {
		log.Errorf("Unmarshal %s error: %v", listStr, err)
		return err
	}

	var leftList []string
	for _, i := range objList {
		if i != objID {
			leftList = append(leftList, i)
		}
	}

	if len(leftList) > 0 {
		log.Warningf("There're some other obj %v associated to qos %s", leftList, qosID)
		params := map[string]interface{}{
			listObj: leftList,
		}
		err := p.cli.UpdateQos(qosID, params)
		if err != nil {
			log.Errorf("Remove obj %s of type %s from qos %s error: %v", objID, objType, qosID, err)
			return err
		}

		return nil
	}

	err = p.cli.DeactivateQos(qosID)
	if err != nil {
		log.Errorf("Deactivate qos %s error: %v", qosID, err)
		return err
	}

	err = p.cli.DeleteQos(qosID)
	if err != nil {
		log.Errorf("Delete qos %s error: %v", qosID, err)
		return err
	}

	return nil
}

func (p *SmartX) CreateLunSnapshot(name, srcLunID string) (map[string]interface{}, error) {
	snapshot, err := p.cli.CreateLunSnapshot(name, srcLunID)
	if err != nil {
		log.Errorf("Create snapshot %s for lun %s error: %v", name, srcLunID, err)
		return nil, err
	}

	snapshotID := snapshot["ID"].(string)
	err = p.cli.ActivateLunSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Activate snapshot %s error: %v", snapshotID, err)
		p.cli.DeleteLunSnapshot(snapshotID)
		return nil, err
	}

	return snapshot, nil
}

func (p *SmartX) DeleteLunSnapshot(snapshotID string) error {
	err := p.cli.DeactivateLunSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Deactivate snapshot %s error: %v", snapshotID, err)
		return err
	}

	err = p.cli.DeleteLunSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Delete snapshot %s error: %v", snapshotID, err)
		return err
	}

	return nil
}

func (p *SmartX) CreateFSSnapshot(name, srcFSID string) (string, error) {
	snapshot, err := p.cli.CreateFSSnapshot(name, srcFSID)
	if err != nil {
		log.Errorf("Create snapshot %s for FS %s error: %v", name, srcFSID, err)
		return "", err
	}

	snapshotID := snapshot["ID"].(string)
	return snapshotID, nil
}

func (p *SmartX) DeleteFSSnapshot(snapshotID string) error {
	err := p.cli.DeleteFSSnapshot(snapshotID)
	if err != nil {
		log.Errorf("Delete FS snapshot %s error: %v", snapshotID, err)
		return err
	}

	return nil
}
