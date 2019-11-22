package volume

import (
	"errors"
	"fmt"
	"storage/oceanstor/client"
	"storage/oceanstor/smartx"
	"strconv"
	"utils"
	"utils/log"
)

type Base struct {
	cli *client.Client
}

func (p *Base) commonPreCreate(params map[string]interface{}) error {
	if v, exist := params["alloctype"].(string); exist && v == "thick" {
		params["alloctype"] = 0
	} else {
		params["alloctype"] = 1
	}

	if v, exist := params["clonefrom"].(string); exist && v != "" {
		_, params["clonefrom"] = utils.GetBackendAndVolume(v)

		if v, exist := params["clonespeed"].(string); exist {
			speed, err := strconv.Atoi(v)
			if err != nil || speed < 1 || speed > 4 {
				return fmt.Errorf("Error config %s for clonespeed", v)
			}
			params["clonespeed"] = speed
		} else {
			params["clonespeed"] = 3
		}
	}

	if v, exist := params["storagepool"].(string); exist {
		err := p.cli.Login()
		if err != nil {
			return err
		}

		pool, err := p.cli.GetPoolByName(v)
		if err != nil {
			log.Errorf("Get storage pool %s info error: %v", v, err)
			return err
		}
		if pool == nil {
			return fmt.Errorf("Storage pool %s doesn't exist", v)
		}
		params["poolID"] = pool["ID"].(string)
	} else {
		return errors.New("Must specify storage pool to create volume")
	}

	if v, exist := params["qos"].(string); exist {
		qos, err := smartx.VerifyQos(v)
		if err != nil {
			log.Errorf("Verify qos %s error: %v", v, err)
			return err
		}

		params["qos"] = qos
	}

	return nil
}
