/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package smartx provides operations for storage qos
package smartx

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	kilo                       = 1000
	minLatency, maxLatency     = 500, 1500
	minIops, maxIops           = 99, 999999999
	minBandwidth, maxBandwidth = 0, 999999999
	ioType                     = 2
)

var (
	validator = map[string]func(int) bool{
		"IOTYPE": func(value int) bool {
			return value == ioType
		},
		"MAXBANDWIDTH": func(value int) bool {
			return minBandwidth < value && value <= maxBandwidth

		},
		"MINBANDWIDTH": func(value int) bool {
			return minBandwidth < value && value <= maxBandwidth

		},
		"MAXIOPS": func(value int) bool {
			return minIops < value && value <= maxIops

		},
		"MINIOPS": func(value int) bool {
			return minIops < value && value <= maxIops

		},
		"LATENCY": func(value int) bool {
			// User request Latency values in millisecond but during extraction values are converted in microsecond
			// as required in Oceandisk QoS create interface
			return value == minLatency || value == maxLatency
		},
	}

	oceandiskParameters = []string{"MAXBANDWIDTH", "MINBANDWIDTH", "MAXIOPS", "MINIOPS", "LATENCY"}
)

// CheckQoSParameterSupport verify QoS supported parameters and value validation
func CheckQoSParameterSupport(ctx context.Context, qosConfig string) error {
	qosParam, err := ExtractQoSParameters(ctx, qosConfig)
	if err != nil {
		return utils.Errorln(ctx, err.Error())
	}

	err = validateQoSParametersSupport(qosParam)
	if err != nil {
		return utils.Errorln(ctx, err.Error())
	}

	return nil
}

func validateQoSParametersSupport(qosParam map[string]float64) error {
	// validate QoS parameters and parameter ranges
	for k, v := range qosParam {
		f, exist := validator[k]
		if !exist {
			return fmt.Errorf("%s is a invalid key for Oceandisk QoS", k)
		}

		if !f(int(v)) { // silently ignoring decimal number
			return fmt.Errorf("%s of qos parameter has invalid value", k)
		}
	}
	return nil
}

// ExtractQoSParameters unmarshal QoS configuration parameters
func ExtractQoSParameters(ctx context.Context, qosConfig string) (map[string]float64, error) {
	var unmarshalParams map[string]interface{}
	params := make(map[string]float64)

	err := json.Unmarshal([]byte(qosConfig), &unmarshalParams)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal qos parameters[ %s ] error: %v", qosConfig, err)
	}

	// translate values based on Oceandisk product's QoS create interface
	for key, val := range unmarshalParams {
		// all numbers are unmarshalled as float64 in unmarshalParams
		// assert for other than number
		value, ok := val.(float64)
		if !ok {
			return nil, fmt.Errorf("invalid QoS parameter [%s] with value type [%T]", key, val)
		}

		if key == "LATENCY" {
			// convert Latency from millisecond to microsecond
			params[key] = value * kilo
		} else {
			params[key] = value
		}
	}

	return params, nil
}

// ValidateQoSParameters check QoS parameters
func ValidateQoSParameters(qosParam map[string]float64) (map[string]int, error) {
	// ensure at least one parameter
	paramExist := false
	for _, param := range oceandiskParameters {
		if _, exist := qosParam[param]; exist {
			paramExist = true
			break
		}
	}

	if !paramExist {
		return nil, fmt.Errorf("missing one of QoS parameter %v ", strings.Join(oceandiskParameters, ","))
	}

	// validate QoS param value
	validatedParameters := make(map[string]int)
	for key, value := range qosParam {
		// check if not integer
		if !big.NewFloat(value).IsInt() {
			return nil, fmt.Errorf("the QoS parameter %s has invalid value type [%T]. "+
				"It should be integer", key, value)
		}
		validatedParameters[key] = int(value)
	}

	return validatedParameters, nil
}

// Client provides smartx client
type Client struct {
	cli client.OceandiskClientInterface
}

// NewSmartX inits a new smartx client
func NewSmartX(cli client.OceandiskClientInterface) *Client {
	return &Client{
		cli: cli,
	}
}

func (p *Client) getQosName(objID string) string {
	now := time.Now().Format("20060102150405")
	return fmt.Sprintf("k8s_namespace%s_%s", objID, now)
}

// CreateQos creates qos and return its id
func (p *Client) CreateQos(ctx context.Context, objID string, params map[string]int) (string, error) {
	var err error
	var lowerLimit bool

	for k := range params {
		if strings.HasPrefix(k, "MIN") || strings.HasPrefix(k, "LATENCY") {
			lowerLimit = true
		}
	}

	if lowerLimit {
		data := map[string]interface{}{
			"IOPRIORITY": 3,
		}

		err = p.cli.UpdateNamespace(ctx, objID, data)

		if err != nil {
			return "", utils.Errorf(ctx, "upgrade obj %s of type %s IOPRIORITY error: %v", objID, objID, err)
		}
	}

	name := p.getQosName(objID)
	qos, err := p.cli.CreateQos(ctx, p.getCreateQosArgs(name, objID, params))
	if err != nil {
		return "", utils.Errorf(ctx, "create qos %v for obj %s of type namespace error: %v", params, objID, err)
	}

	qosID, ok := utils.GetValue[string](qos, "ID")
	if !ok {
		return "", utils.Errorf(ctx, "qos ID is expected as string, get %T", qos["ID"])
	}

	qosStatus, ok := utils.GetValue[string](qos, "ENABLESTATUS")
	if !ok {
		return "", utils.Errorf(ctx, "qos ENABLESTATUS is expected as string, get %T", qos["ENABLESTATUS"])
	}

	if qosStatus == "false" {
		err := p.cli.ActivateQos(ctx, qosID, "")
		if err != nil {
			return "", utils.Errorf(ctx, "activate qos %s error: %v", qosID, err)
		}
	}

	return qosID, nil
}

// DeleteQos deletes qos by id
func (p *Client) DeleteQos(ctx context.Context, qosID, objID string) error {
	qos, err := p.cli.GetQosByID(ctx, qosID, "")
	if err != nil {
		return utils.Errorf(ctx, "get qos by ID %s error: %v", qosID, err)
	}

	objList, err := getObjIdListByQos(qos)
	if err != nil {
		return utils.Errorf(ctx, "delete qos %s failed, error: %v", qosID, err)
	}

	var leftList []string
	for _, i := range objList {
		if i != objID {
			leftList = append(leftList, i)
		}
	}

	if len(leftList) > 0 {
		log.AddContext(ctx).Warningf("there are some other obj %v associated to qos %s", leftList, qosID)
		params := map[string]interface{}{
			"LUNLIST": leftList,
		}
		err := p.cli.UpdateQos(ctx, qosID, "", params)
		if err != nil {
			return utils.Errorf(ctx, "remove obj %s of type namespace from qos %s error: %v", objID, qosID, err)
		}

		return nil
	}

	err = p.cli.DeactivateQos(ctx, qosID, "")
	if err != nil {
		return utils.Errorf(ctx, "deactivate qos %s error: %v", qosID, err)
	}

	err = p.cli.DeleteQos(ctx, qosID, "")
	if err != nil {
		return utils.Errorf(ctx, "delete qos %s error: %v", qosID, err)
	}

	return nil
}

func getObjIdListByQos(qos map[string]interface{}) ([]string, error) {
	listStr, ok := utils.GetValue[string](qos, "LUNLIST")
	if !ok {
		return nil, fmt.Errorf("qos volume list is expected as marshaled string, get %T", qos["LUNLIST"])
	}

	var objList []string
	err := json.Unmarshal([]byte(listStr), &objList)
	if err != nil {
		return nil, fmt.Errorf("unmarshal %s error: %v", listStr, err)
	}

	return objList, nil
}

// DeleteQosByNamespaceId deletes qos by namespace id
func (p *Client) DeleteQosByNamespaceId(ctx context.Context, namespaceId string) error {
	qosList, err := p.cli.GetAllQos(ctx)
	if err != nil {
		return utils.Errorf(ctx, "get all qos failed, error: %v", err)
	}

	for _, qos := range qosList {
		qosID, ok := utils.GetValue[string](qos, "ID")
		if !ok {
			log.AddContext(ctx).Warningf("qos ID is expected as string, get %T", qos["ID"])
			continue
		}

		objList, err := getObjIdListByQos(qos)
		if err != nil {
			log.AddContext(ctx).Warningf("get qos %s related namespaces failed, error: %v", qosID, err)
			continue
		}

		index := slices.Index(objList, namespaceId)
		if index == -1 {
			continue
		}

		objList = slices.Delete(objList, index, index+1)
		if len(objList) > 0 {
			log.AddContext(ctx).Warningf("there are some other obj %v associated to qos %v", objList, qos)
			params := map[string]interface{}{"LUNLIST": objList}
			err := p.cli.UpdateQos(ctx, qosID, "", params)
			if err != nil {
				return utils.Errorf(ctx, "remove namespace %s from qos %s error: %v", namespaceId, qosID, err)
			}
			continue
		}

		err = p.cli.DeactivateQos(ctx, qosID, "")
		if err != nil {
			return utils.Errorf(ctx, "deactivate qos %s error: %v", qosID, err)
		}

		err = p.cli.DeleteQos(ctx, qosID, "")
		if err != nil {
			return utils.Errorf(ctx, "delete qos %s error: %v", qosID, err)
		}
	}

	return nil
}

func (p *Client) getCreateQosArgs(name, objID string, params map[string]int) base.CreateQoSArgs {
	return base.CreateQoSArgs{
		Name:   name,
		ObjID:  objID,
		Params: params,
	}
}
