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

	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type OceanStorAttacher struct {
	Attacher
}

const (
	MULTIPATHTYPE_DEFAULT = "0"
)

func newOceanStorAttacher(
	cli client.BaseClientInterface,
	protocol,
	invoker string,
	portals []string,
	alua map[string]interface{}) AttacherPlugin {
	return &OceanStorAttacher{
		Attacher: Attacher{
			cli:      cli,
			protocol: protocol,
			invoker:  invoker,
			portals:  portals,
			alua:     alua,
		},
	}
}

func (p *OceanStorAttacher) needUpdateInitiatorAlua(initiator map[string]interface{},
	hostAlua map[string]interface{}) bool {
	multiPathType, ok := hostAlua["MULTIPATHTYPE"]
	if !ok {
		return false
	}

	if multiPathType != initiator["MULTIPATHTYPE"] {
		return true
	} else if initiator["MULTIPATHTYPE"] == MULTIPATHTYPE_DEFAULT {
		return false
	}

	failoverMode, ok := hostAlua["FAILOVERMODE"]
	if ok && failoverMode != initiator["FAILOVERMODE"] {
		return true
	}

	specialModeType, ok := hostAlua["SPECIALMODETYPE"]
	if ok && specialModeType != initiator["SPECIALMODETYPE"] {
		return true
	}

	pathType, ok := hostAlua["PATHTYPE"]
	if ok && pathType != initiator["PATHTYPE"] {
		return true
	}

	return false
}

func (p *OceanStorAttacher) attachISCSI(ctx context.Context, hostID, hostName string,
	parameters map[string]interface{}) error {
	iscsiInitiator, err := p.Attacher.attachISCSI(ctx, hostID, parameters)
	if err != nil {
		return err
	}

	hostAlua := utils.GetAlua(ctx, p.alua, hostName)
	if hostAlua != nil && p.needUpdateInitiatorAlua(iscsiInitiator, hostAlua) {
		err = p.cli.UpdateIscsiInitiator(ctx, iscsiInitiator["ID"].(string), hostAlua)
	}

	return err
}

func (p *OceanStorAttacher) attachFC(ctx context.Context, hostID, hostName string,
	parameters map[string]interface{}) error {
	fcInitiators, err := p.Attacher.attachFC(ctx, hostID, parameters)
	if err != nil {
		return err
	}

	hostAlua := utils.GetAlua(ctx, p.alua, hostName)
	if hostAlua != nil {
		for _, i := range fcInitiators {
			if !p.needUpdateInitiatorAlua(i, hostAlua) {
				continue
			}

			err := p.cli.UpdateFCInitiator(ctx, i["ID"].(string), hostAlua)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *OceanStorAttacher) attachRoCE(ctx context.Context, hostID string, parameters map[string]interface{}) error {
	_, err := p.Attacher.attachRoCE(ctx, hostID, parameters)
	return err
}

func (p *OceanStorAttacher) ControllerAttach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (
	map[string]interface{}, error) {
	host, err := p.getHost(ctx, parameters, true)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host ID error: %v", err)
		return nil, err
	}

	hostID, ok := host["ID"].(string)
	if !ok {
		return nil, errors.New("convert host[\"ID\"] to string failed")
	}
	hostName, ok := host["NAME"].(string)
	if !ok {
		return nil, errors.New("convert host[\"NAME\"] to string failed")
	}

	if p.protocol == "iscsi" {
		err = p.attachISCSI(ctx, hostID, hostName, parameters)
	} else if p.protocol == "fc" || p.protocol == "fc-nvme" {
		err = p.attachFC(ctx, hostID, hostName, parameters)
	} else if p.protocol == "roce" {
		err = p.attachRoCE(ctx, hostID, parameters)
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
