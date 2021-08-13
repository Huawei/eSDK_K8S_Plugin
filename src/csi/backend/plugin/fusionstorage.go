package plugin

import (
	"errors"
	"storage/fusionstorage/client"
	"strings"
	"utils"
	"utils/log"
	"utils/pwd"
)

const (
	CAPACITY_UNIT int64 = 1024 * 1024
)

type FusionStoragePlugin struct {
	basePlugin
	cli      *client.Client
}

func (p *FusionStoragePlugin) init(config map[string]interface{}, keepLogin bool) error {
	configUrls, exist := config["urls"].([]interface{})
	if !exist || len(configUrls) <= 0 {
		return errors.New("urls must be provided")
	}

	url := configUrls[0].(string)

	user, exist := config["user"].(string)
	if !exist {
		return errors.New("user must be provided")
	}

	password, exist := config["password"].(string)
	if !exist {
		return errors.New("password must be provided")
	}

	keyText, exist := config["keyText"].(string)
	if !exist {
		return errors.New("keyText must be provided")
	}

	decrypted, err := pwd.Decrypt(password, keyText)
	if err != nil {
		return err
	}

	parallelNum, _ := config["parallelNum"].(string)
	cli := client.NewClient(url, user, decrypted, parallelNum)
	err = cli.Login()
	if err != nil {
		return err
	}

	if !keepLogin {
		cli.Logout()
	}

	p.cli = cli
	return nil
}

func (p *FusionStoragePlugin) getParams(name string, parameters map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":     name,
		"capacity": utils.RoundUpSize(parameters["size"].(int64), CAPACITY_UNIT),
	}

	paramKeys := []string{
		"storagepool",
		"cloneFrom",
		"authClient",
	}

	for _, key := range paramKeys {
		if v, exist := parameters[key].(string); exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	return params, nil
}

func (p *FusionStoragePlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":  true,
		"SupportThick": false,
		"SupportQoS":   false,
	}

	return capabilities, nil
}

func (p *FusionStoragePlugin) UpdatePoolCapabilities(poolNames []string) (map[string]interface{}, error) {
	// To keep connection token alive
	p.cli.KeepAlive()

	pools, err := p.cli.GetAllPools()
	if err != nil {
		log.Errorf("Get fusionstorage pools error: %v", err)
		return nil, err
	}

	log.Debugf("Get pools: %v", pools)

	capabilities := make(map[string]interface{})

	for _, name := range poolNames {
		if i, exist := pools[name]; exist {
			pool := i.(map[string]interface{})

			totalCapacity := int64(pool["totalCapacity"].(float64))
			usedCapacity := int64(pool["usedCapacity"].(float64))

			freeCapacity := (totalCapacity - usedCapacity) * CAPACITY_UNIT
			capabilities[name] = map[string]interface{}{
				"FreeCapacity": freeCapacity,
			}
		}
	}

	return capabilities, nil
}
