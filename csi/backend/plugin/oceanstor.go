package plugin

import (
	"errors"
	"fmt"
	"regexp"
	"storage/oceanstor/client"
	"storage/oceanstor/smartx"
	"strconv"
	"utils"
	"utils/log"
	"utils/pwd"

	volUtil "k8s.io/kubernetes/pkg/volume/util"
)

type OceanstorPlugin struct {
	cli     *client.Client
	product string
	version string
}

func (p *OceanstorPlugin) init(config map[string]interface{}) error {
	configUrls, exist := config["urls"].([]interface{})
	if !exist || len(configUrls) <= 0 {
		return errors.New("urls must be provided")
	}

	var urls []string
	for _, i := range configUrls {
		urls = append(urls, i.(string))
	}

	user, exist := config["user"].(string)
	if !exist {
		return errors.New("user must be provided")
	}

	password, exist := config["password"].(string)
	if !exist {
		return errors.New("password must be provided")
	}

	product, exist := config["product"].(string)
	if !exist {
		return errors.New("product must be provided")
	}
	if product != "V3" && product != "V5" && product != "Dorado" {
		return errors.New("product only support config: V3, V5, Dorado")
	}

	decrypted, err := pwd.Decrypt(password)
	if err != nil {
		return err
	}

	var vstorName string
	if name, exist := config["vstoreName"].(string); exist {
		vstorName = name
	}
	cli := client.NewClient(urls, user, decrypted,vstorName)
	err = cli.Login()
	if err != nil {
		return err
	}

	system, err := cli.GetSystem()
	if err != nil {
		return err
	}

	p.cli = cli
	p.version = system["PRODUCTMODE"].(string)
	p.product = product

	return nil
}

func (p *OceanstorPlugin) checkFeatureValid(features map[string]int, feature string) bool {
	var support bool

	status, exist := features[feature]
	if exist {
		support = status == 1 || status == 2
	}

	return support
}

func (p *OceanstorPlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	features, err := p.cli.GetLicenseFeature()
	if err != nil {
		log.Errorf("Get license feature error: %v", err)
		return nil, err
	}

	supportThin := p.checkFeatureValid(features, "SmartThin")
	supportThick := p.product != "Dorado"

	supportQoS := p.checkFeatureValid(features, "SmartQoS")

	capabilities := map[string]interface{}{
		"SupportThin":  supportThin,
		"SupportThick": supportThick,
		"SupportQoS":   supportQoS,
	}

	return capabilities, nil
}

func (p *OceanstorPlugin) UpdatePoolCapabilities(poolNames []string) (map[string]interface{}, error) {
	pools, err := p.cli.GetAllPools()
	if err != nil {
		log.Errorf("Get all pools error: %v", err)
		return nil, err
	}

	capabilities := make(map[string]interface{})

	for _, name := range poolNames {
		if i, exist := pools[name]; exist {
			pool := i.(map[string]interface{})
			err := p.checkPoolValid(pool)
			if err != nil {
				return nil, err
			}

			userFreeCapacity := pool["USERFREECAPACITY"].(string)
			freeCapacity, err := strconv.ParseInt(userFreeCapacity, 10, 64)
			if err != nil {
				log.Errorf("Convert string %s to int64 error: %v", userFreeCapacity, err)
				return nil, err
			}

			capabilities[name] = map[string]interface{}{
				"FreeCapacity": freeCapacity * 512,
			}
		}
	}

	return capabilities, nil
}

func (p *OceanstorPlugin) checkPoolValid(pool map[string]interface{}) error {
	return nil
}

func (p *OceanstorPlugin) getParams(size int64, name ,poolName string, parameters map[string]string) (map[string]interface{}, error) {

	params := map[string]interface{}{
		"description": "Created from Kubernetes CSI",
		"capacity":    volUtil.RoundUpSize(size, 512),
	}

	volumeType, exist := parameters["volumeType"]
	if !exist || len(volumeType) == 0 {
		msg := fmt.Sprint("VolumeType is empty or don't config this parameter")
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	if volumeType == "lun" {
		name = utils.GetLunName(name)
		params["name"] = name
	} else if volumeType== "fs" {
		name = utils.GetFileSystemName(name)
		authClient, exist := parameters["authClient"]
		if !exist || authClient == " " {
			msg := fmt.Sprint("AuthClient must be provided for NAS")
			log.Errorln(msg)
			return nil, errors.New(msg)
		}
		params["authclient"] = parameters["authClient"]
		params["name"] = name
	}else if volumeType == "9000" {
		params["product"] = "9000"
		params["name"] = name
		return params, nil
	}else {
		msg := fmt.Sprintf("Do not support this volume type : %s", volumeType)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	pool, err := p.cli.GetPoolByName(poolName)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, fmt.Errorf("Storage pool %s doesn't exist", poolName)
	}
	params["parentid"] = pool["ID"].(string)

	if qosConfig, exist := parameters["qos"]; exist && qosConfig != "" {
		qos, err := smartx.VerifyQos(qosConfig)
		if err != nil {
			log.Errorf("Verify qos %s error: %v", qosConfig, err)
			return nil, err
		}

		params["qos"] = qos
	}

	allocType := 1
	if parameters["allocType"] == "thick" {
		allocType = 0
	}

	params["alloctype"] = allocType

	if cloneFrom, exist := parameters["cloneFrom"]; exist && cloneFrom != "" {
		params["clonefrom"] = cloneFrom
	}

	if cloneSpeed, exist := parameters["cloneSpeed"]; exist {
		match, _ := regexp.MatchString("^[1-4]$", cloneSpeed)
		if !match {
			msg := fmt.Sprint("Only support form 1 to 4 for cloneSpeed parameter")
			log.Errorln(msg)
			return nil, errors.New(msg)
		}
		params["clonespeed"] = cloneSpeed
	}else {
		params["clonespeed"] = "3"
	}

	return params, nil
}
