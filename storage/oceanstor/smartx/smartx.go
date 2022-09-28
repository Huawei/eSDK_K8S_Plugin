/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

package smartx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type qosParameterValidators map[string]func(int) bool
type qosParameterList map[string]struct{}

var (
	oceanStorQosValidators = map[string]qosParameterValidators{
		utils.OceanStorDoradoV6: doradoV6ParameterValidators,
		utils.OceanStorDoradoV3: doradoParameterValidators,
		utils.OceanStorV3:       oceanStorV3V5ParameterValidators,
		utils.OceanStorV5:       oceanStorV3V5ParameterValidators,
	}

	doradoParameterValidators = map[string]func(int) bool{
		"IOTYPE": func(value int) bool {
			return value == 2
		},
		"MAXBANDWIDTH": func(value int) bool {
			return value > 0
		},
		"MAXIOPS": func(value int) bool {
			return value > 99
		},
	}

	oceanStorV3V5ParameterValidators = map[string]func(int) bool{
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

	doradoV6ParameterValidators = map[string]func(int) bool{
		"IOTYPE": func(value int) bool {
			return value == 2
		},
		"MAXBANDWIDTH": func(value int) bool {
			return value > 0 && value <= 999999999

		},
		"MINBANDWIDTH": func(value int) bool {
			return value > 0 && value <= 999999999

		},
		"MAXIOPS": func(value int) bool {
			return value > 99 && value <= 999999999

		},
		"MINIOPS": func(value int) bool {
			return value > 99 && value <= 999999999

		},
		"LATENCY": func(value int) bool {
			// User request Latency values in millisecond but during extraction values are converted in microsecond
			// as required in OceanStor DoradoV6 QoS create interface
			return value == 500 || value == 1500
		},
	}

	oceanStorCommonParameters = qosParameterList{
		"MAXBANDWIDTH": struct{}{},
		"MINBANDWIDTH": struct{}{},
		"MAXIOPS":      struct{}{},
		"MINIOPS":      struct{}{},
		"LATENCY":      struct{}{},
	}

	// one of parameter is mandatory for respective products
	oceanStorQoSMandatoryParameters = map[string]qosParameterList{
		utils.OceanStorDoradoV6: oceanStorCommonParameters,
		utils.OceanStorDoradoV3: {
			"MAXBANDWIDTH": struct{}{},
			"MAXIOPS":      struct{}{},
		},
		utils.OceanStorV3: oceanStorCommonParameters,
		utils.OceanStorV5: oceanStorCommonParameters,
	}
)

// CheckQoSParameterSupport verify QoS supported parameters and value validation
func CheckQoSParameterSupport(ctx context.Context, product, qosConfig string) error {
	qosParam, err := ExtractQoSParameters(ctx, product, qosConfig)
	if err != nil {
		return err
	}

	err = validateQoSParametersSupport(ctx, product, qosParam)
	if err != nil {
		return err
	}

	return nil
}

func validateQoSParametersSupport(ctx context.Context, product string, qosParam map[string]float64) error {
	var lowerLimit, upperLimit bool

	// decide validators based on product
	validator, ok := oceanStorQosValidators[product]
	if !ok {
		return utils.Errorf(ctx, "QoS is currently not supported for OceanStor %s", product)
	}

	// validate QoS parameters and parameter ranges
	for k, v := range qosParam {
		f, exist := validator[k]
		if !exist {
			return utils.Errorf(ctx, "%s is a invalid key for OceanStor %s QoS", k, product)
		}

		if !f(int(v)) { // silently ignoring decimal number
			return utils.Errorf(ctx, "%s of qos parameter has invalid value", k)
		}

		if strings.HasPrefix(k, "MIN") || strings.HasPrefix(k, "LATENCY") {
			lowerLimit = true
		} else if strings.HasPrefix(k, "MAX") {
			upperLimit = true
		}
	}

	if product != utils.OceanStorDoradoV6 && lowerLimit && upperLimit {
		return utils.Errorf(ctx, "Cannot specify both lower and upper limits in qos for OceanStor %s", product)
	}

	return nil
}

// ExtractQoSParameters unmarshal QoS configuration parameters
func ExtractQoSParameters(ctx context.Context, product string, qosConfig string) (map[string]float64, error) {
	var unmarshalParams map[string]interface{}
	params := make(map[string]float64)

	err := json.Unmarshal([]byte(qosConfig), &unmarshalParams)
	if err != nil {
		return nil, utils.Errorf(ctx, "Failed to unmarshal qos parameters[ %s ] error: %v", qosConfig, err)
	}

	// translate values based on OceanStor product's QoS create interface
	for key, val := range unmarshalParams {
		// all numbers are unmarshalled as float64 in unmarshalParams
		// assert for other than number
		value, ok := val.(float64)
		if !ok {
			return nil, utils.Errorf(ctx, "Invalid QoS parameter [%s] with value type [%T] for OceanStor %s",
				key, val, product)
		}

		if product == utils.OceanStorDoradoV6 && key == "LATENCY" {
			// convert OceanStoreDoradoV6 Latency from millisecond to microsecond
			params[key] = value * 1000
			continue
		}

		params[key] = value
	}

	return params, nil
}

// ValidateQoSParameters check QoS parameters
func ValidateQoSParameters(product string, qosParam map[string]float64) (map[string]int, error) {
	// ensure at least one parameter
	params := oceanStorQoSMandatoryParameters[product]
	paramExist := false
	for param := range params {
		if _, exist := qosParam[param]; exist {
			paramExist = true
			break
		}
	}
	if !paramExist {
		optionalParam := make([]string, 0)
		for param := range params {
			optionalParam = append(optionalParam, param)
		}
		return nil, fmt.Errorf("missing one of QoS parameter %v ", optionalParam)
	}

	// validate QoS param value
	validatedParameters := make(map[string]int)
	for key, value := range qosParam {
		// check if not integer
		if !big.NewFloat(value).IsInt() {
			return nil, fmt.Errorf("QoS parameter %s has invalid value type [%T]. "+
				"It should be integer", key, value)
		}
		validatedParameters[key] = int(value)
	}

	return validatedParameters, nil
}

type SmartX struct {
	cli client.BaseClientInterface
}

func NewSmartX(cli client.BaseClientInterface) *SmartX {
	return &SmartX{
		cli: cli,
	}
}

func (p *SmartX) getQosName(objID, objType string) string {
	now := time.Now().Format("20060102150405")
	return fmt.Sprintf("k8s_%s%s_%s", objType, objID, now)
}

func (p *SmartX) CreateQos(ctx context.Context,
	objID, objType, vStoreID string,
	params map[string]int) (string, error) {
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

		if objType == "fs" {
			err = p.cli.UpdateFileSystem(ctx, objID, data)
		} else {
			err = p.cli.UpdateLun(ctx, objID, data)
		}

		if err != nil {
			log.AddContext(ctx).Errorf("Upgrade obj %s of type %s IOPRIORITY error: %v", objID, objID, err)
			return "", err
		}
	}

	name := p.getQosName(objID, objType)
	qos, err := p.cli.CreateQos(ctx, name, objID, objType, vStoreID, params)
	if err != nil {
		log.AddContext(ctx).Errorf("Create qos %v for obj %s of type %s error: %v",
			params, objID, objType, err)
		return "", err
	}

	qosID, ok := qos["ID"].(string)
	if !ok {
		return "", errors.New("qos ID is expected as string")
	}

	qosStatus, ok := qos["ENABLESTATUS"].(string)
	if !ok {
		return "", errors.New("ENABLESTATUS parameter is expected as string")
	}

	if qosStatus == "false" {
		err := p.cli.ActivateQos(ctx, qosID, vStoreID)
		if err != nil {
			log.AddContext(ctx).Errorf("Activate qos %s error: %v", qosID, err)
			return "", err
		}
	}

	return qosID, nil
}

func (p *SmartX) DeleteQos(ctx context.Context, qosID, objID, objType, vStoreID string) error {
	qos, err := p.cli.GetQosByID(ctx, qosID, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get qos by ID %s error: %v", qosID, err)
		return err
	}

	var objList []string

	listObj := "LUNLIST"
	if objType == "fs" {
		listObj = "FSLIST"
	}

	listStr, ok := qos[listObj].(string)
	if !ok {
		return errors.New("qos volume list is expected as marshaled string")
	}

	err = json.Unmarshal([]byte(listStr), &objList)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal %s error: %v", listStr, err)
		return err
	}

	var leftList []string
	for _, i := range objList {
		if i != objID {
			leftList = append(leftList, i)
		}
	}

	if len(leftList) > 0 {
		log.AddContext(ctx).Warningf("There're some other obj %v associated to qos %s", leftList, qosID)
		params := map[string]interface{}{
			listObj: leftList,
		}
		err := p.cli.UpdateQos(ctx, qosID, vStoreID, params)
		if err != nil {
			log.AddContext(ctx).Errorf("Remove obj %s of type %s from qos %s error: %v",
				objID, objType, qosID, err)
			return err
		}

		return nil
	}

	err = p.cli.DeactivateQos(ctx, qosID, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("Deactivate qos %s error: %v", qosID, err)
		return err
	}

	err = p.cli.DeleteQos(ctx, qosID, vStoreID)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete qos %s error: %v", qosID, err)
		return err
	}

	return nil
}

func (p *SmartX) CreateLunSnapshot(ctx context.Context, name, srcLunID string) (map[string]interface{}, error) {
	snapshot, err := p.cli.CreateLunSnapshot(ctx, name, srcLunID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create snapshot %s for lun %s error: %v", name, srcLunID, err)
		return nil, err
	}

	snapshotID, ok := snapshot["ID"].(string)
	if !ok {
		return nil, errors.New("snapshot ID is expected as string")
	}
	err = p.cli.ActivateLunSnapshot(ctx, snapshotID)
	if err != nil {
		log.AddContext(ctx).Errorf("Activate snapshot %s error: %v", snapshotID, err)
		_ = p.cli.DeleteLunSnapshot(ctx, snapshotID)
		return nil, err
	}

	return snapshot, nil
}

func (p *SmartX) DeleteLunSnapshot(ctx context.Context, snapshotID string) error {
	err := p.cli.DeactivateLunSnapshot(ctx, snapshotID)
	if err != nil {
		log.AddContext(ctx).Errorf("Deactivate snapshot %s error: %v", snapshotID, err)
		return err
	}

	err = p.cli.DeleteLunSnapshot(ctx, snapshotID)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete snapshot %s error: %v", snapshotID, err)
		return err
	}

	return nil
}

func (p *SmartX) CreateFSSnapshot(ctx context.Context, name, srcFSID string) (string, error) {
	snapshot, err := p.cli.CreateFSSnapshot(ctx, name, srcFSID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create snapshot %s for FS %s error: %v", name, srcFSID, err)
		return "", err
	}

	snapshotID, ok := snapshot["ID"].(string)
	if !ok {
		return "", errors.New("snapshot ID is expected as string")
	}
	return snapshotID, nil
}

func (p *SmartX) DeleteFSSnapshot(ctx context.Context, snapshotID string) error {
	err := p.cli.DeleteFSSnapshot(ctx, snapshotID)
	if err != nil {

		log.AddContext(ctx).Errorf("Delete FS snapshot %s error: %v", snapshotID, err)
		return err
	}

	return nil
}
