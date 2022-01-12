package smartx

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"storage/oceanstor/client"
	"strings"
	"time"
	"utils"
	"utils/log"
)

type qosParameterValidators map[string]func(int) bool
type qosParameterList map[string]struct{}

var (
	oceanStorQosValidators = map[string]qosParameterValidators{
		utils.OceanStorDoradoV6: doradoV6ParameterValidators,
		utils.OceanStorDorado:   doradoParameterValidators,
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
		utils.OceanStorDorado: {
			"MAXBANDWIDTH": struct{}{},
			"MAXIOPS":      struct{}{},
		},
		utils.OceanStorV3: oceanStorCommonParameters,
		utils.OceanStorV5: oceanStorCommonParameters,
	}
)

// CheckQoSParameterSupport verify QoS supported parameters and value validation
func CheckQoSParameterSupport(product, qosConfig string) error {
	qosParam, err := ExtractQoSParameters(product, qosConfig)
	if err != nil {
		return err
	}

	err = validateQoSParametersSupport(product, qosParam)
	if err != nil {
		return err
	}

	return nil
}

func validateQoSParametersSupport(product string, qosParam map[string]float64) error {
	var lowerLimit, upperLimit bool

	// decide validators based on product
	validator, ok := oceanStorQosValidators[product]
	if !ok {
		msg := fmt.Sprintf("QoS is currently not supported for OceanStor %s", product)
		log.Errorf(msg)
		return errors.New(msg)
	}

	// validate QoS parameters and parameter ranges
	for k, v := range qosParam {
		f, exist := validator[k]
		if !exist {
			err := fmt.Errorf("%s is a invalid key for OceanStor %s QoS", k, product)
			log.Errorln(err.Error())
			return err
		}

		if !f(int(v)) { // silently ignoring decimal number
			err := fmt.Errorf("%s of qos parameter has invalid value", k)
			log.Errorln(err.Error())
			return err
		}

		if strings.HasPrefix(k, "MIN") || strings.HasPrefix(k, "LATENCY") {
			lowerLimit = true
		} else if strings.HasPrefix(k, "MAX") {
			upperLimit = true
		}
	}

	if product != utils.OceanStorDoradoV6 && lowerLimit && upperLimit {
		err := fmt.Errorf("Cannot specify both lower and upper limits in qos for OceanStor %s", product)
		log.Errorln(err.Error())
		return err
	}

	return nil
}

// ExtractQoSParameters unmarshal QoS configuration parameters
func ExtractQoSParameters(product string, qosConfig string) (map[string]float64, error) {
	var unmarshalParams map[string]interface{}
	params := make(map[string]float64)

	err := json.Unmarshal([]byte(qosConfig), &unmarshalParams)
	if err != nil {
		log.Errorf("Failed to unmarshal qos parameters[ %s ] error: %v", qosConfig, err)
		return nil, err
	}

	// translate values based on OceanStor product's QoS create interface
	for key, val := range unmarshalParams {
		// all numbers are unmarshalled as float64 in unmarshalParams
		// assert for other than number
		value, ok := val.(float64)
		if !ok {
			msg := fmt.Sprintf("Invalid QoS parameter [%s] with value type [%T] for OceanStor %s",
				key, val, product)
			log.Errorln(msg)
			return nil, errors.New(msg)
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

	qosID, ok := qos["ID"].(string)
	if !ok {
		return "", errors.New("qos ID is expected as string")
	}

	qosStatus, ok := qos["ENABLESTATUS"].(string)
	if !ok {
		return "", errors.New("ENABLESTATUS parameter is expected as string")
	}

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

	snapshotID, ok := snapshot["ID"].(string)
	if !ok {
		return nil, errors.New("snapshot ID is expected as string")
	}
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

	snapshotID, ok := snapshot["ID"].(string)
	if !ok {
		return "", errors.New("snapshot ID is expected as string")
	}
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
