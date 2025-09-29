/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

// Package smartx provides operations for a-series storage qos
package smartx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/aseries/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	kilo                                          = 1000
	latency500MicroSecond, latency1500MicroSecond = 500, 1500
	minIops, maxIops                              = 100, 999999999
	minBandwidth, maxBandwidth                    = 1, 999999999
	ioType                                        = 2

	lowIOPriority    = 1
	mediumIOPriority = 2
	highIOPriority   = 3

	latencyType string = "LATENCY"
)

var (
	validator = map[string]func(int) bool{
		"IOTYPE": func(value int) bool {
			return value == ioType
		},
		"MAXBANDWIDTH": func(value int) bool {
			return minBandwidth <= value && value <= maxBandwidth
		},
		"MINBANDWIDTH": func(value int) bool {
			return minBandwidth <= value && value <= maxBandwidth
		},
		"MAXIOPS": func(value int) bool {
			return minIops <= value && value <= maxIops
		},
		"MINIOPS": func(value int) bool {
			return minIops <= value && value <= maxIops
		},
		"LATENCY": func(value int) bool {
			// User request Latency values in millisecond but during extraction values are converted in microsecond
			return value == latency500MicroSecond || value == latency1500MicroSecond
		},
	}

	aSeriesParameters = []string{"MAXBANDWIDTH", "MINBANDWIDTH", "MAXIOPS", "MINIOPS", "LATENCY"}
)

// Client provides smartx client for a-series
type Client struct {
	cli client.OceanASeriesClientInterface
}

// NewSmartX inits a new smartx client for a-series
func NewSmartX(cli client.OceanASeriesClientInterface) *Client {
	return &Client{
		cli: cli,
	}
}

// CheckQoSParametersValueRange verify QoS supported parameters and value validation
func CheckQoSParametersValueRange(ctx context.Context, qosConfig string) error {
	qosParam, err := ExtractQoSParameters(ctx, qosConfig)
	if err != nil {
		return err
	}

	err = checkQoSParametersRangeWithValidator(qosParam)
	if err != nil {
		return err
	}

	return nil
}

// ConvertQoSParametersValueToInt check QoS parameters
func ConvertQoSParametersValueToInt(qosParam map[string]float64) (map[string]int, error) {
	// ensure at least one parameter
	paramExist := false
	for _, param := range aSeriesParameters {
		if _, exist := qosParam[param]; exist {
			paramExist = true
			break
		}
	}

	if !paramExist {
		return nil, fmt.Errorf("please make sure at least one parameter exists: %v, now: %v",
			strings.Join(aSeriesParameters, ","), qosParam)
	}

	validatedParameters := make(map[string]int, len(qosParam))
	for key, value := range qosParam {
		if !big.NewFloat(value).IsInt() {
			return nil, fmt.Errorf("the QoS parameter %s has invalid value type [%T],"+
				" It should be integer", key, value)
		}

		validatedParameters[key] = int(value)
	}

	return validatedParameters, nil
}

// ExtractQoSParameters unmarshal QoS configuration parameters
func ExtractQoSParameters(ctx context.Context, qosConfig string) (map[string]float64, error) {
	var params map[string]float64
	err := json.Unmarshal([]byte(qosConfig), &params)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal qos parameters: %s failed, error: %w", qosConfig, err)
	}

	if params == nil {
		return nil, nil
	}

	if val, exist := params[latencyType]; exist {
		params[latencyType] = val * kilo
	}

	return params, nil
}

// CreateQos used to create QoS and return its id
func (p *Client) CreateQos(ctx context.Context, objID, vStoreID string, params map[string]int) (
	string, error) {

	err := p.updateIOPriority(ctx, objID, params)
	if err != nil {
		return "", err
	}

	qosName := p.getQosName(objID, "fs")
	qos, err := p.cli.CreateQos(ctx, base.CreateQoSArgs{
		Name:     qosName,
		ObjID:    objID,
		ObjType:  "fs",
		VStoreID: vStoreID,
		Params:   params,
	})
	if err != nil {
		return "", err
	}

	qosID, ok := utils.GetValue[string](qos, "ID")
	if !ok {
		return "", errors.New("qos ID is expected as string")
	}

	enableStatus, ok := utils.GetValue[string](qos, "ENABLESTATUS")
	if !ok {
		return "", errors.New("parameter ENABLESTATUS is expected as string")
	}

	if enableStatus == "false" {
		err = p.cli.ActivateQos(ctx, qosID, vStoreID)
		if err != nil {
			return "", err
		}
	}

	return qosID, nil
}

// DeleteQos deletes qos by id
func (p *Client) DeleteQos(ctx context.Context, qosID, objID, vStoreID string) error {
	qos, err := p.cli.GetQosByID(ctx, qosID, vStoreID)
	if err != nil {
		return err
	}

	listObjKey := "FSLIST"
	listStr, ok := utils.GetValue[string](qos, listObjKey)
	if !ok {
		return fmt.Errorf("get qos %s failed, listStr: %v", listObjKey, qos[listObjKey])
	}

	var objList []string
	err = json.Unmarshal([]byte(listStr), &objList)
	if err != nil {
		return fmt.Errorf("unmarshal qos listStr:%s failed, error: %w", listStr, err)
	}

	var leftList []string
	for _, i := range objList {
		if i != objID {
			leftList = append(leftList, i)
		}
	}

	if len(leftList) > 0 {
		log.AddContext(ctx).Warningf("There're some other fs: %v associated to qos: %s", leftList, qosID)
		params := map[string]interface{}{
			listObjKey: leftList,
		}
		err = p.cli.UpdateQos(ctx, qosID, vStoreID, params)
		if err != nil {
			return err
		}

		return nil
	}

	err = p.cli.DeactivateQos(ctx, qosID, vStoreID)
	if err != nil {
		return err
	}

	err = p.cli.DeleteQos(ctx, qosID, vStoreID)
	if err != nil {
		return err
	}

	return nil
}

func checkQoSParametersRangeWithValidator(qosParam map[string]float64) error {
	// validate QoS parameters and parameter ranges
	for k, v := range qosParam {
		f, exist := validator[k]
		if !exist {
			return fmt.Errorf("%s is a invalid key for a-series QoS", k)
		}

		// silently ignoring decimal number
		if !f(int(v)) {
			return fmt.Errorf("%s of qos parameter has invalid value: %d", k, int(v))
		}
	}

	return nil
}

func (p *Client) getQosName(objID, objType string) string {
	now := time.Now().Format("20060102150405")
	return fmt.Sprintf("k8s_%s%s_%s", objType, objID, now)
}

func (p *Client) updateIOPriority(ctx context.Context, objID string, params map[string]int) error {
	lowerLimit := false
	for k := range params {
		if strings.HasPrefix(k, "MIN") || strings.HasPrefix(k, "LATENCY") {
			lowerLimit = true
			break
		}
	}

	if lowerLimit {
		data := map[string]interface{}{
			"IOPRIORITY": highIOPriority,
		}

		return p.cli.UpdateFileSystem(ctx, objID, data)
	}

	return nil
}
