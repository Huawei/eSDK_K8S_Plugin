package smartx

import (
	"encoding/json"
	"errors"
	"fmt"
	"storage/fusionstorage/client"
	"time"
	"utils/log"
)

var (
	ValidQosKey = map[string]func(int) bool{
		"maxMBPS": func(value int) bool {
			return value > 0
		},
		"maxIOPS": func(value int) bool {
			return value > 0
		},
	}
)

func VerifyQos(qosConfig string) (map[string]int, error) {
	var msg string
	var params map[string]int

	err := json.Unmarshal([]byte(qosConfig), &params)
	if err != nil {
		log.Errorf("Unmarshal %s error: %v", qosConfig, err)
		return nil, err
	}

	for k, v := range params {
		f, exist := ValidQosKey[k]
		if !exist {
			msg = fmt.Sprintf("%s is an invalid key for QoS", k)
			goto ERROR
		}

		if !f(v) {
			msg = fmt.Sprintf("%s of qos specs is invalid", k)
			goto ERROR
		}
	}

	return params, nil

ERROR:
	log.Errorln(msg)
	return nil, errors.New(msg)
}

type QoS struct {
	cli *client.Client
}

func NewQoS(cli *client.Client) *QoS {
	return &QoS{
		cli: cli,
	}
}

func (p *QoS) getQosName(objType string) string {
	now := time.Now().Format("20060102150405")
	return fmt.Sprintf("k8s_%s_%s", objType, now)
}

func (p *QoS) AddQoS(volName string, params map[string]int) (string, error) {
	qosName := p.getQosName("volume")
	err := p.cli.CreateQoS(qosName, params)
	if err != nil {
		log.Errorf("Create qos %v error: %v", params, err)
		return "", err
	}

	err = p.cli.AssociateQoSWithVolume(volName, qosName)
	if err != nil {

		err := p.RemoveQoS(volName)
		if err != nil {
			log.Errorf("Revert Create qos %s error: %v", params, err)
			return "", err
		}

		return "", fmt.Errorf("associate qos %s with volume %s error: %v", qosName, volName, err)
	}

	return qosName, nil
}

func (p *QoS) RemoveQoS(volName string) error {
	qosName, err := p.cli.GetQoSNameByVolume(volName)
	if err != nil {
		log.Errorf("Get QoS of volume %s error: %v", volName, err)
		return err
	}

	if qosName == "" {
		return nil
	}

	err = p.cli.DisassociateQoSWithVolume(volName, qosName)
	if err != nil {
		log.Errorf("Disassociate QoS %s of volume %s error: %v", qosName, volName, err)
		return err
	}

	qosAssociateObjCount, err := p.cli.GetAssociateCountOfQoS(qosName)
	if err != nil {
		log.Errorf("Get Objs of QoS %s error: %v", qosName, err)
		return err
	}

	if qosAssociateObjCount != 0 {
		log.Warningf("The Qos %s associate objs count %d. Please delete QoS manually", qosName, qosAssociateObjCount)
		return nil
	}

	err = p.cli.DeleteQoS(qosName)
	if err != nil {
		log.Errorf("Delete QoS %s error: %v", qosName, err)
		return err
	}

	return nil
}
