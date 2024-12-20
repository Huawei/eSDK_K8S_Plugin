/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base/attacher"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// DoradoV6Attacher implements interface VolumeAttacherPlugin
type DoradoV6Attacher struct {
	VolumeAttacher
}

const (
	// AccessModeBalanced defines balanced mode of access
	AccessModeBalanced = "0"
)

func newDoradoV6OrV7Attacher(config VolumeAttacherConfig) VolumeAttacherPlugin {
	baseAttacherConfig := attacher.AttachmentManagerConfig{
		Cli:      config.Cli,
		Protocol: config.Protocol,
		Invoker:  config.Invoker,
		Portals:  config.Portals,
		Alua:     config.Alua,
	}
	baseAttacher := attacher.NewAttachmentManager(baseAttacherConfig)

	return &DoradoV6Attacher{
		VolumeAttacher: VolumeAttacher{
			AttachmentManager: baseAttacher,
			Cli:               config.Cli,
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
	host, err := p.GetHost(ctx, parameters, true)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host ID error: %v", err)
		return nil, err
	}

	hostID, ok := host["ID"].(string)
	if !ok {
		return nil, errors.New("convert host[\"ID\"] to string failed")
	}
	hostAlua := utils.GetAlua(ctx, p.Alua, host["NAME"].(string))

	if hostAlua != nil && p.needUpdateHost(host, hostAlua) {
		err := p.Cli.UpdateHost(ctx, hostID, hostAlua)
		if err != nil {
			log.AddContext(ctx).Errorf("Update host %s error: %v", hostID, err)
			return nil, err
		}
	}

	if p.Protocol == "iscsi" {
		_, err = p.VolumeAttacher.AttachISCSI(ctx, hostID, parameters)
	} else if p.Protocol == "fc" || p.Protocol == "fc-nvme" {
		_, err = p.VolumeAttacher.AttachFC(ctx, hostID, parameters)
	} else if p.Protocol == "roce" {
		_, err = p.VolumeAttacher.AttachRoCE(ctx, hostID, parameters)
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Attach %s connection error: %v", p.Protocol, err)
		return nil, err
	}

	wwn, hostLunId, err := p.doMapping(ctx, hostID, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Mapping LUN %s to host %s error: %v", lunName, hostID, err)
		return nil, err
	}

	return p.GetMappingProperties(ctx, wwn, hostLunId, parameters)
}
