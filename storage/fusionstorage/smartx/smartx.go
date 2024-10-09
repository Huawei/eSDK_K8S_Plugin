/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

// Package smartx provides operations for qos
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
	// ValidQosKey defines valid qos key
	ValidQosKey = map[string]func(int) bool{
		"maxMBPS": func(value int) bool {
			return value > 0
		},
		"maxIOPS": func(value int) bool {
			return value > 0
		},
	}
)

// VerifyQos verifies qos config and return formatted params
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
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}

		if !f(v) {
			msg = fmt.Sprintf("%s of qos specs is invalid", k)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}
	}

	return params, nil
}

// QoS provides qos client
type QoS struct {
	cli *client.RestClient
}

// NewQoS inits a new qos client
func NewQoS(cli *client.RestClient) *QoS {
	return &QoS{
		cli: cli,
	}
}

// ConstructQosNameByCurrentTime constructs qos name by current time
func ConstructQosNameByCurrentTime(objType string) string {
	now := time.Now().Format("20060102150405")
	return fmt.Sprintf("k8s_%s_%s", objType, now)
}

// AddQoS create a qos and associate the qos with volume
func (p *QoS) AddQoS(ctx context.Context, volName string, params map[string]int) (string, error) {
	qosName := ConstructQosNameByCurrentTime("volume")
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

// RemoveQoS removes qos of the volume
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
