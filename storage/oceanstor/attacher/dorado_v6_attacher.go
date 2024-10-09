/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package attacher

import (
	"context"
	"errors"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// DoradoV6Attacher implements interface VolumeAttacherPlugin
type DoradoV6Attacher struct {
	VolumeAttacher
}

const (
	// AccessModeBalanced defines balanced mode of access
	AccessModeBalanced = "0"
)

func newDoradoV6Attacher(config VolumeAttacherConfig) VolumeAttacherPlugin {
	return &DoradoV6Attacher{
		VolumeAttacher: VolumeAttacher{
			cli:      config.Cli,
			protocol: config.Protocol,
			invoker:  config.Invoker,
			portals:  config.Portals,
			alua:     config.Alua,
		},
	}
}

func (p *DoradoV6Attacher) needUpdateHost(host map[string]interface{}, hostAlua map[string]interface{}) bool {
	accessMode, ok := hostAlua["accessMode"]
	if !ok {
		return false
	}

	if accessMode != host["accessMode"] {
		return true
	} else if host["accessMode"] == AccessModeBalanced {
		return false
	}

	hyperMetroPathOptimized, ok := hostAlua["hyperMetroPathOptimized"]
	if ok && hyperMetroPathOptimized != host["hyperMetroPathOptimized"] {
		return true
	}

	return false
}

// ControllerAttach attaches volume and maps lun to host
func (p *DoradoV6Attacher) ControllerAttach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	host, err := p.getHost(ctx, parameters, true)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host ID error: %v", err)
		return nil, err
	}

	hostID, ok := host["ID"].(string)
	if !ok {
		return nil, errors.New("convert host[\"ID\"] to string failed")
	}
	hostAlua := utils.GetAlua(ctx, p.alua, host["NAME"].(string))

	if hostAlua != nil && p.needUpdateHost(host, hostAlua) {
		err := p.cli.UpdateHost(ctx, hostID, hostAlua)
		if err != nil {
			log.AddContext(ctx).Errorf("Update host %s error: %v", hostID, err)
			return nil, err
		}
	}

	if p.protocol == "iscsi" {
		_, err = p.VolumeAttacher.attachISCSI(ctx, hostID, parameters)
	} else if p.protocol == "fc" || p.protocol == "fc-nvme" {
		_, err = p.VolumeAttacher.attachFC(ctx, hostID, parameters)
	} else if p.protocol == "roce" {
		_, err = p.VolumeAttacher.attachRoCE(ctx, hostID, parameters)
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Attach %s connection error: %v", p.protocol, err)
		return nil, err
	}

	wwn, hostLunId, err := p.doMapping(ctx, hostID, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Mapping LUN %s to host %s error: %v", lunName, hostID, err)
		return nil, err
	}

	return p.getMappingProperties(ctx, wwn, hostLunId, parameters)
}
