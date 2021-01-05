package plugin

import (
	"errors"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/src/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
)

const (
	CAPACITY_UNIT int64 = 1024 * 1024
)

type FusionStoragePlugin struct {
	basePlugin
	cli *client.Client
}

func (p *FusionStoragePlugin) init(config map[string]interface{}, keepLogin bool) error {
	url, exist := config["url"].(string)
	if !exist {
		return errors.New("url must be provided")
	}

	user, exist := config["user"].(string)
	if !exist {
		return errors.New("user must be provided")
	}

	password, exist := config["password"].(string)
	if !exist {
		return errors.New("password must be provided")
	}

	cli := client.NewClient(url, user, password)
	err := cli.Login()
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
