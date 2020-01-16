package plugin

import (
	"errors"
	"storage/oceanstor/client"
	"strconv"
	"strings"
	"utils"
	"utils/log"
	"utils/pwd"

	volUtil "k8s.io/kubernetes/pkg/volume/util"
)

type OceanstorPlugin struct {
	cli     *client.Client
	product string
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

	vstoreName, _ := config["vstoreName"].(string)

	cli := client.NewClient(urls, user, decrypted, vstoreName)
	err = cli.Login()
	if err != nil {
		return err
	}

	p.cli = cli
	p.product = product

	return nil
}

func (p *OceanstorPlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	features, err := p.cli.GetLicenseFeature()
	if err != nil {
		log.Errorf("Get license feature error: %v", err)
		return nil, err
	}

	log.Debugf("Get license feature: %v", features)

	supportThin := utils.IsSupportFeature(features, "SmartThin")
	supportThick := p.product != "Dorado"
	supportQoS := utils.IsSupportFeature(features, "SmartQoS")
	supportMetro := utils.IsSupportFeature(features, "HyperMetro")

	capabilities := map[string]interface{}{
		"SupportThin":  supportThin,
		"SupportThick": supportThick,
		"SupportQoS":   supportQoS,
		"SupportMetro": supportMetro,
	}

	return capabilities, nil
}

func (p *OceanstorPlugin) getParams(name string, parameters map[string]interface{}) map[string]interface{} {
	params := map[string]interface{}{
		"name":        name,
		"description": "Created from Kubernetes CSI",
		"capacity":    volUtil.RoundUpSize(parameters["size"].(int64), 512),
	}

	paramKeys := []string{
		"storagepool",
		"allocType",
		"qos",
		"authClient",
		"cloneFrom",
		"cloneSpeed",
		"hyperMetro",
		"metroDomain",
		"remoteStoragePool",
	}

	for _, key := range paramKeys {
		if v, exist := parameters[key]; exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	if v, exist := params["hypermetro"].(string); exist {
		params["hypermetro"] = utils.StrToBool(v)
	}

	return params
}

func (p *OceanstorPlugin) analyzePoolsCapacity(pools []map[string]interface{}) map[string]interface{} {
	capabilities := make(map[string]interface{})

	for _, pool := range pools {
		name := pool["NAME"].(string)
		freeCapacity, _ := strconv.ParseInt(pool["USERFREECAPACITY"].(string), 10, 64)

		capabilities[name] = map[string]interface{}{
			"FreeCapacity": freeCapacity * 512,
		}
	}

	return capabilities
}
