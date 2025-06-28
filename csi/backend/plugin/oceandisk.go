/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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

package plugin

import (
	"context"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/smartx"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// OceandiskPlugin provides oceandisk plugin base operations
type OceandiskPlugin struct {
	basePlugin

	cli client.OceandiskClientInterface
}

func (p *OceandiskPlugin) init(ctx context.Context, config map[string]interface{}, keepLogin bool) error {
	backendClientConfig, err := formatOceandiskInitParam(config)
	if err != nil {
		return err
	}

	cli, err := client.NewClient(ctx, backendClientConfig)
	if err != nil {
		return err
	}

	if err = cli.Login(ctx); err != nil {
		log.AddContext(ctx).Errorf("plugin init login failed, err: %v", err)
		return err
	}

	if err = cli.SetSystemInfo(ctx); err != nil {
		cli.Logout(ctx)
		log.AddContext(ctx).Errorf("set client info failed, err: %v", err)
		return err
	}

	p.name = backendClientConfig.Name
	p.cli = cli

	if !keepLogin {
		cli.Logout(ctx)
	}
	return nil
}

func (p *OceandiskPlugin) getBackendCapabilities() map[string]interface{} {
	capabilities := map[string]interface{}{
		"SupportThin":            true,
		"SupportThick":           false,
		"SupportQoS":             true,
		"SupportMetro":           false,
		"SupportReplication":     false,
		"SupportApplicationType": true,
		"SupportClone":           false,
		"SupportMetroNAS":        false,
	}

	return capabilities
}

func (p *OceandiskPlugin) getBackendSpecifications() map[string]interface{} {
	specifications := map[string]interface{}{
		"LocalDeviceSN": p.cli.GetDeviceSN(),
	}
	return specifications
}

// UpdateBackendCapabilities used to update backend capabilities
func (p *OceandiskPlugin) UpdateBackendCapabilities() (map[string]interface{}, map[string]interface{}, error) {
	return p.getBackendCapabilities(), p.getBackendSpecifications(), nil
}

func (p *OceandiskPlugin) updatePoolCapacities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	pools, err := p.cli.GetAllPools(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("get all pools error: %v", err)
		return nil, err
	}

	log.AddContext(ctx).Debugf("get pools: %v", pools)

	var validPools []map[string]interface{}
	for _, name := range poolNames {
		if pool, exist := pools[name].(map[string]interface{}); exist {
			validPools = append(validPools, pool)
		} else {
			log.AddContext(ctx).Warningf("pool %s does not exist", name)
		}
	}

	capacities := analyzePoolsCapacity(ctx, validPools, nil)
	return capacities, nil
}

// SupportQoSParameters checks requested QoS parameters support by Oceandisk plugin
func (p *OceandiskPlugin) SupportQoSParameters(ctx context.Context, qosConfig string) error {
	return smartx.CheckQoSParameterSupport(ctx, qosConfig)
}

// Logout is to logout the storage session
func (p *OceandiskPlugin) Logout(ctx context.Context) {
	if p.cli != nil {
		p.cli.Logout(ctx)
	}
}

// ReLogin will refresh the user session of storage
func (p *OceandiskPlugin) ReLogin(ctx context.Context) error {
	if p.cli == nil {
		return nil
	}

	return p.cli.ReLogin(ctx)
}
