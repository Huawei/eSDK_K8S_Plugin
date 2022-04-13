package smartx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"huawei-csi-driver/storage/fusionstorage/client"
	"huawei-csi-driver/utils/log"
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

func VerifyQos(ctx context.Context, qosConfig string) (map[string]int, error) {
	var msg string
	var params map[string]int

	err := json.Unmarshal([]byte(qosConfig), &params)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal %s error: %v", qosConfig, err)
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
	log.AddContext(ctx).Errorln(msg)
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

func (p *QoS) AddQoS(ctx context.Context, volName string, params map[string]int) (string, error) {
	qosName := p.getQosName("volume")
	err := p.cli.CreateQoS(ctx, qosName, params)
	if err != nil {
		log.AddContext(ctx).Errorf("Create qos %v error: %v", params, err)
		return "", err
	}

	err = p.cli.AssociateQoSWithVolume(ctx, volName, qosName)
	if err != nil {

		err := p.RemoveQoS(ctx, volName)
		if err != nil {
			log.AddContext(ctx).Errorf("Revert Create qos %s error: %v", params, err)
			return "", err
		}

		return "", fmt.Errorf("associate qos %s with volume %s error: %v", qosName, volName, err)
	}

	return qosName, nil
}

func (p *QoS) RemoveQoS(ctx context.Context, volName string) error {
	qosName, err := p.cli.GetQoSNameByVolume(ctx, volName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get QoS of volume %s error: %v", volName, err)
		return err
	}

	if qosName == "" {
		return nil
	}

	err = p.cli.DisassociateQoSWithVolume(ctx, volName, qosName)
	if err != nil {
		log.AddContext(ctx).Errorf("Disassociate QoS %s of volume %s error: %v", qosName, volName, err)
		return err
	}

	qosAssociateObjCount, err := p.cli.GetAssociateCountOfQoS(ctx, qosName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get Objs of QoS %s error: %v", qosName, err)
		return err
	}

	if qosAssociateObjCount != 0 {
		log.AddContext(ctx).Warningf("The Qos %s associate objs count %d. Please delete QoS manually",
			qosName, qosAssociateObjCount)
		return nil
	}

	err = p.cli.DeleteQoS(ctx, qosName)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete QoS %s error: %v", qosName, err)
		return err
	}

	return nil
}
