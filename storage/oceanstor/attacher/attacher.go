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

// Package attacher provide operations of volume attach
package attacher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"huawei-csi-driver/connector/nvme"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	hostGroupType = 14
	lunGroupType  = 256
)

// AttacherPlugin defines interfaces of attach operations
type AttacherPlugin interface {
	ControllerAttach(context.Context, string, map[string]interface{}) (map[string]interface{}, error)
	ControllerDetach(context.Context, string, map[string]interface{}) (string, error)
	getTargetRoCEPortals(context.Context) ([]string, error)
	getLunInfo(context.Context, string) (map[string]interface{}, error)
}

// Attacher defines attacher to attach volume
type Attacher struct {
	cli      client.BaseClientInterface
	protocol string
	invoker  string
	portals  []string
	alua     map[string]interface{}
}

// NewAttacher init a new attacher
func NewAttacher(
	product string,
	cli client.BaseClientInterface,
	protocol, invoker string,
	portals []string,
	alua map[string]interface{}) AttacherPlugin {
	switch product {
	case "DoradoV6":
		return newDoradoV6Attacher(cli, protocol, invoker, portals, alua)
	default:
		return newOceanStorAttacher(cli, protocol, invoker, portals, alua)
	}
}

func (p *Attacher) getHostName(postfix string) string {
	host := fmt.Sprintf("k8s_%s", postfix)
	if len(host) <= 31 {
		return host
	}

	return host[:31]
}

func (p *Attacher) getHostGroupName(postfix string) string {
	return fmt.Sprintf("k8s_%s_hostgroup_%s", p.invoker, postfix)
}

func (p *Attacher) getLunGroupName(postfix string) string {
	return fmt.Sprintf("k8s_%s_lungroup_%s", p.invoker, postfix)
}

func (p *Attacher) getMappingName(postfix string) string {
	return fmt.Sprintf("k8s_%s_mapping_%s", p.invoker, postfix)
}

func (p *Attacher) getHost(ctx context.Context,
	parameters map[string]interface{},
	toCreate bool) (map[string]interface{}, error) {
	var err error

	hostname, exist := parameters["HostName"].(string)
	if !exist {
		log.AddContext(ctx).Errorf("Get hostname error: %v", err)
		return nil, err
	}

	hostToQuery := p.getHostName(hostname)
	host, err := p.cli.GetHostByName(ctx, hostToQuery)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host %s error: %v", hostToQuery, err)
		return nil, err
	}
	if host == nil && toCreate {
		host, err = p.cli.CreateHost(ctx, hostToQuery)
		if err != nil {
			log.AddContext(ctx).Errorf("Create host %s error: %v", hostToQuery, err)
			return nil, err
		}
	}

	if host != nil {
		return host, nil
	}

	if toCreate {
		return nil, fmt.Errorf("cannot create host %s", hostToQuery)
	}

	return nil, nil
}

func (p *Attacher) createMapping(ctx context.Context, hostID string) (string, error) {
	mappingName := p.getMappingName(hostID)
	mapping, err := p.cli.GetMappingByName(ctx, mappingName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get mapping by name %s error: %v", mappingName, err)
		return "", err
	}
	if mapping == nil {
		mapping, err = p.cli.CreateMapping(ctx, mappingName)
		if err != nil {
			log.AddContext(ctx).Errorf("Create mapping %s error: %v", mappingName, err)
			return "", err
		}
	}

	return mapping["ID"].(string), nil
}

func (p *Attacher) createHostGroup(ctx context.Context, hostID, mappingID string) error {
	var err error
	var hostGroup map[string]interface{}
	var hostGroupID string

	hostGroupsByHostID, err := p.cli.QueryAssociateHostGroup(ctx, 21, hostID)
	if err != nil {
		log.AddContext(ctx).Errorf("Query associated hostgroups of host %s error: %v",
			hostID, err)
		return err
	}

	hostGroupName := p.getHostGroupName(hostID)

	for _, i := range hostGroupsByHostID {
		group, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert group to map failed, data: %v", i)
			continue
		}
		if group["NAME"].(string) == hostGroupName {
			hostGroupID, ok = group["ID"].(string)
			if !ok {
				log.AddContext(ctx).Warningf("convert hostGroupID to string failed, data: %v", group["ID"])
				continue
			}
			return p.addToHostGroupMapping(ctx, hostGroupName, hostGroupID, mappingID)
		}
	}

	hostGroup, err = p.cli.GetHostGroupByName(ctx, hostGroupName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get hostgroup by name %s error: %v", hostGroupName, err)
		return err
	}
	if hostGroup == nil {
		hostGroup, err = p.cli.CreateHostGroup(ctx, hostGroupName)
		if err != nil {
			log.AddContext(ctx).Errorf("Create hostgroup %s error: %v", hostGroupName, err)
			return err
		}
	}

	hostGroupID, ok := hostGroup["ID"].(string)
	if !ok {
		return errors.New("createHostGroup failed, caused by not found hostGroup id")
	}

	err = p.cli.AddHostToGroup(ctx, hostID, hostGroupID)
	if err != nil {
		log.AddContext(ctx).Errorf("Add host %s to hostgroup %s error: %v",
			hostID, hostGroupID, err)
		return err
	}

	return p.addToHostGroupMapping(ctx, hostGroupName, hostGroupID, mappingID)
}

func (p *Attacher) addToHostGroupMapping(ctx context.Context, groupName, groupID, mappingID string) error {
	hostGroupsByMappingID, err := p.cli.QueryAssociateHostGroup(ctx, 245, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Query associated host groups of mapping %s error: %v", mappingID, err)
		return err
	}

	for _, i := range hostGroupsByMappingID {
		group, ok := i.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid group type. Expected 'map[string]interface{}', found %T", i)
		}
		if group["NAME"].(string) == groupName {
			return nil
		}
	}

	err = p.cli.AddGroupToMapping(ctx, hostGroupType, groupID, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Add host group %s to mapping %s error: %v",
			groupID, mappingID, err)
		return err
	}

	return nil
}

func (p *Attacher) createLunGroup(ctx context.Context, lunID, hostID, mappingID string) error {
	var err error
	var lunGroup map[string]interface{}

	lunGroupsByLunID, err := p.cli.QueryAssociateLunGroup(ctx, 11, lunID)
	if err != nil {
		log.AddContext(ctx).Errorf("Query associated lun groups of lun %s error: %v", lunID, err)
		return err
	}

	lunGroupName := p.getLunGroupName(hostID)
	for _, i := range lunGroupsByLunID {
		group, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert group to map failed, data: %v", i)
			continue
		}
		if group["NAME"].(string) == lunGroupName {
			lunGroupID, ok := group["ID"].(string)
			if !ok {
				return errors.New("convert group[\"ID\"] to string failed")
			}
			return p.addToLUNGroupMapping(ctx, lunGroupName, lunGroupID, mappingID)
		}
	}

	lunGroup, err = p.cli.GetLunGroupByName(ctx, lunGroupName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lungroup by name %s error: %v", lunGroupName, err)
		return err
	}
	if lunGroup == nil {
		lunGroup, err = p.cli.CreateLunGroup(ctx, lunGroupName)
		if err != nil {
			log.AddContext(ctx).Errorf("Create lungroup %s error: %v", lunGroupName, err)
			return err
		}
	}

	lunGroupID, ok := lunGroup["ID"].(string)
	if !ok {
		return errors.New("createLunGroup failed, caused by not found lun group id")
	}
	err = p.cli.AddLunToGroup(ctx, lunID, lunGroupID)
	if err != nil {
		log.AddContext(ctx).Errorf("Add lun %s to group %s error: %v", lunID, lunGroupID, err)
		return err
	}

	return p.addToLUNGroupMapping(ctx, lunGroupName, lunGroupID, mappingID)
}

func (p *Attacher) addToLUNGroupMapping(ctx context.Context, groupName, groupID, mappingID string) error {
	lunGroupsByMappingID, err := p.cli.QueryAssociateLunGroup(ctx, 245, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Query associated lun groups of mapping %s error: %v", mappingID, err)
		return err
	}

	for _, i := range lunGroupsByMappingID {
		group, ok := i.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid group type. Expected 'map[string]interface{}', found %T", i)
		}
		if group["NAME"].(string) == groupName {
			return nil
		}
	}

	err = p.cli.AddGroupToMapping(ctx, lunGroupType, groupID, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Add lun group %s to mapping %s error: %v",
			groupID, mappingID, err)
		return err
	}

	return nil
}

func (p *Attacher) needUpdateInitiatorAlua(initiator map[string]interface{}) bool {
	if p.alua == nil {
		return false
	}

	multiPathType, ok := p.alua["MULTIPATHTYPE"]
	if !ok {
		return false
	}

	if multiPathType != initiator["MULTIPATHTYPE"] {
		return true
	} else if initiator["MULTIPATHTYPE"] == MultiPathTypeDefault {
		return false
	}

	failoverMode, ok := p.alua["FAILOVERMODE"]
	if ok && failoverMode != initiator["FAILOVERMODE"] {
		return true
	}

	specialModeType, ok := p.alua["SPECIALMODETYPE"]
	if ok && specialModeType != initiator["SPECIALMODETYPE"] {
		return true
	}

	pathType, ok := p.alua["PATHTYPE"]
	if ok && pathType != initiator["PATHTYPE"] {
		return true
	}

	return false
}

func (p *Attacher) getISCSIProperties(ctx context.Context, wwn, hostLunId string, parameters map[string]interface{}) (
	map[string]interface{}, error) {
	tgtPortals, tgtIQNs, err := p.getTargetISCSIProperties(ctx)
	if err != nil {
		return nil, err
	}

	lenPortals := len(tgtPortals)
	var tgtHostLUNs []string
	for i := 0; i < lenPortals; i++ {
		tgtHostLUNs = append(tgtHostLUNs, hostLunId)
	}

	return map[string]interface{}{
		"tgtPortals":  tgtPortals,
		"tgtIQNs":     tgtIQNs,
		"tgtHostLUNs": tgtHostLUNs,
		"tgtLunWWN":   wwn,
	}, nil
}

func (p *Attacher) getFCProperties(ctx context.Context, wwn, hostLunId string, parameters map[string]interface{}) (
	map[string]interface{}, error) {
	tgtWWNs, err := p.getTargetFCProperties(ctx, parameters)
	if err != nil {
		return nil, err
	}

	lenWWNs := len(tgtWWNs)
	var tgtHostLUNs []string
	for i := 0; i < lenWWNs; i++ {
		tgtHostLUNs = append(tgtHostLUNs, hostLunId)
	}

	return map[string]interface{}{
		"tgtLunWWN":   wwn,
		"tgtWWNs":     tgtWWNs,
		"tgtHostLUNs": tgtHostLUNs,
	}, nil
}

func (p *Attacher) getFCNVMeProperties(ctx context.Context, wwn, hostLunId string, parameters map[string]interface{}) (
	map[string]interface{}, error) {
	portWWNList, err := p.getTargetFCNVMeProperties(ctx, parameters)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"portWWNList": portWWNList,
		"tgtLunGuid":  wwn,
	}, nil
}

func (p *Attacher) getRoCEProperties(ctx context.Context, wwn, hostLunId string, parameters map[string]interface{}) (
	map[string]interface{}, error) {
	tgtPortals, err := p.getTargetRoCEPortals(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tgtPortals": tgtPortals,
		"tgtLunGuid": wwn,
	}, nil
}

func (p *Attacher) getMappingProperties(ctx context.Context,
	wwn, hostLunId string, parameters map[string]interface{}) (map[string]interface{}, error) {
	if p.protocol == "iscsi" {
		return p.getISCSIProperties(ctx, wwn, hostLunId, parameters)
	} else if p.protocol == "fc" {
		return p.getFCProperties(ctx, wwn, hostLunId, parameters)
	} else if p.protocol == "fc-nvme" {
		return p.getFCNVMeProperties(ctx, wwn, hostLunId, parameters)
	} else if p.protocol == "roce" {
		return p.getRoCEProperties(ctx, wwn, hostLunId, parameters)
	}

	return nil, utils.Errorf(ctx, "UnSupport protocol %s", p.protocol)
}

func (p *Attacher) getTargetISCSIProperties(ctx context.Context) ([]string, []string, error) {
	ports, err := p.cli.GetIscsiTgtPort(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Get iSCSI tgt port error: %v", err)
		return nil, nil, err
	}
	if ports == nil {
		msg := "no iSCSI tgt port exist"
		log.AddContext(ctx).Errorln(msg)
		return nil, nil, errors.New(msg)
	}

	validIPs := map[string]bool{}
	validIQNs := map[string]string{}
	for _, i := range ports {
		port, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert port to map failed, data: %v", i)
			continue
		}
		portID, ok := port["ID"].(string)
		if !ok {
			log.AddContext(ctx).Warningf("convert portID to string failed, data: %v", port["ID"])
			continue
		}
		portIqn := strings.Split(strings.Split(portID, ",")[0], "+")[1]
		splitIqn := strings.Split(portIqn, ":")

		if len(splitIqn) < 6 {
			continue
		}

		validIPs[splitIqn[5]] = true
		validIQNs[splitIqn[5]] = portIqn
	}

	var tgtPortals []string
	var tgtIQNs []string
	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		if !validIPs[ip] {
			log.AddContext(ctx).Warningf("ISCSI portal %s is not valid", ip)
			continue
		}

		formatIP := fmt.Sprintf("%s:3260", ip)
		tgtPortals = append(tgtPortals, formatIP)
		tgtIQNs = append(tgtIQNs, validIQNs[ip])
	}

	if len(tgtPortals) == 0 {
		msg := fmt.Sprintf("All config portal %s is not valid", p.portals)
		log.AddContext(ctx).Errorln(msg)
		return nil, nil, errors.New(msg)
	}

	return tgtPortals, tgtIQNs, nil
}

func (p *Attacher) getTargetRoCEPortals(ctx context.Context) ([]string, error) {
	var availablePortals []string
	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		rocePortal, err := p.cli.GetRoCEPortalByIP(ctx, ip)
		if err != nil {
			log.AddContext(ctx).Errorf("Get RoCE tgt portal error: %v", err)
			return nil, err
		}

		if rocePortal == nil {
			log.AddContext(ctx).Warningf("the config portal %s does not exist.", ip)
			continue
		}

		supportProtocol, exist := rocePortal["SUPPORTPROTOCOL"].(string)
		if !exist {
			msg := "current storage does not support NVMe"
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}

		if supportProtocol != "64" { // 64 means NVME protocol
			log.AddContext(ctx).Warningf("the config portal %s does not support NVME.", ip)
			continue
		}

		availablePortals = append(availablePortals, ip)
	}

	if len(availablePortals) == 0 {
		msg := fmt.Sprintf("All config portal %s is not valid", p.portals)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return availablePortals, nil
}

func (p *Attacher) getTargetFCNVMeProperties(ctx context.Context, parameters map[string]interface{}) ([]nvme.PortWWNPair, error) {

	fcInitiators, err := GetMultipleInitiators(ctx, FC, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get fc initiator error:%v", err)
		return nil, err
	}

	var ret []nvme.PortWWNPair
	for _, hostInitiator := range fcInitiators {
		tgtWWNs, err := p.cli.GetFCTargetWWNs(ctx, hostInitiator)
		if err != nil {
			return nil, err
		}

		for _, tgtWWN := range tgtWWNs {
			ret = append(ret, nvme.PortWWNPair{InitiatorPortWWN: hostInitiator, TargetPortWWN: tgtWWN})
		}
	}

	log.AddContext(ctx).Infof("Get target fc-nvme properties:%#v", ret)
	return ret, nil
}

func (p *Attacher) getTargetFCProperties(ctx context.Context, parameters map[string]interface{}) ([]string, error) {
	fcInitiators, err := GetMultipleInitiators(ctx, FC, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get fc initiator error: %v", err)
		return nil, err
	}

	validTgtWWNs := make(map[string]bool)
	for _, wwn := range fcInitiators {
		tgtWWNs, err := p.cli.GetFCTargetWWNs(ctx, wwn)
		if err != nil {
			return nil, err
		}

		if tgtWWNs == nil {
			continue
		}

		for _, tgtWWN := range tgtWWNs {
			validTgtWWNs[tgtWWN] = true
		}
	}

	var tgtWWNs []string
	for tgtWWN := range validTgtWWNs {
		tgtWWNs = append(tgtWWNs, tgtWWN)
	}

	if len(tgtWWNs) == 0 {
		msg := fmt.Sprintf("There is no alaivable target wwn of host initiators %v in storage.", fcInitiators)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return tgtWWNs, nil
}

func (p *Attacher) attachISCSI(ctx context.Context, hostID string, parameters map[string]interface{}) (map[string]interface{}, error) {
	name, err := GetSingleInitiator(ctx, ISCSI, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get ISCSI initiator name error: %v", err)
		return nil, err
	}

	initiator, err := p.cli.GetIscsiInitiator(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get ISCSI initiator %s error: %v", name, err)
		return nil, err
	}

	if initiator == nil {
		initiator, err = p.cli.AddIscsiInitiator(ctx, name)
		if err != nil {
			log.AddContext(ctx).Errorf("Add initiator %s error: %v", name, err)
			return nil, err
		}
	}

	isFree, freeExist := initiator["ISFREE"].(string)
	if !freeExist {
		log.AddContext(ctx).Warningf("convert isFree to string failed, data: %v", initiator["ISFREE"])
	}
	parent, parentExist := initiator["PARENTID"].(string)
	if !parentExist {
		log.AddContext(ctx).Warningf("convert parentID to string failed, data: %v", initiator["PARENTID"])
	}
	if freeExist && isFree == "true" {
		err := p.cli.AddIscsiInitiatorToHost(ctx, name, hostID)
		if err != nil {
			log.AddContext(ctx).Errorf("Add ISCSI initiator %s to host %s error: %v", name, hostID, err)
			return nil, err
		}
	} else if parentExist && parent != hostID {
		msg := fmt.Sprintf("ISCSI initiator %s is already associated to another host %s", name, parent)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return initiator, nil
}

func (p *Attacher) attachFC(ctx context.Context, hostID string, parameters map[string]interface{}) ([]map[string]interface{}, error) {
	fcInitiators, err := GetMultipleInitiators(ctx, FC, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get fc initiator error: %v", err)
		return nil, err
	}

	var addWWNs []string
	var hostInitiators []map[string]interface{}

	for _, wwn := range fcInitiators {
		initiator, err := p.cli.GetFCInitiator(ctx, wwn)
		if err != nil {
			log.AddContext(ctx).Errorf("Get FC initiator %s error: %v", wwn, err)
			return nil, err
		}
		if initiator == nil {
			log.AddContext(ctx).Warningf("FC initiator %s does not exist", wwn)
			continue
		}

		status, exist := initiator["RUNNINGSTATUS"].(string)
		if !exist || status != "27" {
			log.AddContext(ctx).Warningf("FC initiator %s is not online", wwn)
			continue
		}

		isFree, freeExist := initiator["ISFREE"].(string)
		if !freeExist {
			log.AddContext(ctx).Warningf("convert isFree to string failed, data: %v", initiator["ISFREE"])
		}
		parent, parentExist := initiator["PARENTID"].(string)
		if !parentExist {
			log.AddContext(ctx).Warningf("convert parentID to string failed, data: %v", initiator["PARENTID"])
		}

		if freeExist && isFree == "true" {
			addWWNs = append(addWWNs, wwn)
		} else if parentExist && parent != hostID {
			msg := fmt.Sprintf("FC initiator %s is already associated to another host %s", wwn, parent)
			log.AddContext(ctx).Errorln(msg)
			return nil, errors.New(msg)
		}

		hostInitiators = append(hostInitiators, initiator)
	}

	for _, wwn := range addWWNs {
		err := p.cli.AddFCInitiatorToHost(ctx, wwn, hostID)
		if err != nil {
			log.AddContext(ctx).Errorf("Add initiator %s to host %s error: %v", wwn, hostID, err)
			return nil, err
		}
	}

	return hostInitiators, nil
}

func (p *Attacher) attachRoCE(ctx context.Context, hostID string, parameters map[string]interface{}) (map[string]interface{}, error) {
	name, err := GetSingleInitiator(ctx, ROCE, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get RoCE initiator name error: %v", err)
		return nil, err
	}

	initiator, err := p.cli.GetRoCEInitiator(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get RoCE initiator %s error: %v", name, err)
		return nil, err
	}

	if initiator == nil {
		initiator, err = p.cli.AddRoCEInitiator(ctx, name)
		if err != nil {
			log.AddContext(ctx).Errorf("Add initiator %s error: %v", name, err)
			return nil, err
		}
	}

	isFree, freeExist := initiator["ISFREE"].(string)
	if !freeExist {
		log.AddContext(ctx).Warningf("convert isFree to string failed, data: %v", initiator["ISFREE"])
	}
	parent, parentExist := initiator["PARENTID"].(string)
	if !parentExist {
		log.AddContext(ctx).Warningf("convert parentID to string failed, data: %v", initiator["PARENTID"])
	}
	if freeExist && isFree == "true" {
		err := p.cli.AddRoCEInitiatorToHost(ctx, name, hostID)
		if err != nil {
			log.AddContext(ctx).Errorf("Add RoCE initiator %s to host %s error: %v", name, hostID, err)
			return nil, err
		}
	} else if parentExist && parent != hostID {
		msg := fmt.Sprintf("RoCE initiator %s is already associated to another host %s", name, parent)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return initiator, nil
}

func (p *Attacher) doMapping(ctx context.Context, hostID, lunName string) (string, string, error) {
	lun, err := p.cli.GetLunByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s error: %v", lunName, err)
		return "", "", err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s not exist for attaching", lunName)
		log.AddContext(ctx).Errorln(msg)
		return "", "", errors.New(msg)
	}

	lunID, ok := lun["ID"].(string)
	if !ok {
		return "", "", pkgUtils.Errorf(ctx, "convert lunID to string failed, data: %v", lun["ID"])
	}
	mappingID, err := p.createMapping(ctx, hostID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create mapping for host %s error: %v", hostID, err)
		return "", "", err
	}

	err = p.createHostGroup(ctx, hostID, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create host group for host %s error: %v", hostID, err)
		return "", "", err
	}

	err = p.createLunGroup(ctx, lunID, hostID, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create lun group for host %s error: %v", hostID, err)
		return "", "", err
	}

	lunUniqueId, err := utils.GetLunUniqueId(ctx, p.protocol, lun)
	if err != nil {
		return "", "", err
	}

	hostLunId, err := p.cli.GetHostLunId(ctx, hostID, lunID)
	if err != nil {
		return "", "", err
	}

	return lunUniqueId, hostLunId, nil
}

func (p *Attacher) doUnmapping(ctx context.Context, hostID, lunName string) (string, error) {
	lun, err := p.cli.GetLunByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s info error: %v", lunName, err)
		return "", err
	}
	if lun == nil {
		log.AddContext(ctx).Infof("LUN %s doesn't exist while detaching", lunName)
		return "", nil
	}
	lunID, ok := lun["ID"].(string)
	if !ok {
		return "", pkgUtils.Errorf(ctx, "convert lunID to string failed, data: %v", lun["ID"])
	}
	lunGroupsByLunID, err := p.cli.QueryAssociateLunGroup(ctx, 11, lunID)
	if err != nil {
		log.AddContext(ctx).Errorf("Query associated lungroups of lun %s error: %v", lunID, err)
		return "", err
	}

	lunGroupName := p.getLunGroupName(hostID)
	for _, i := range lunGroupsByLunID {
		group, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert group to map failed, data: %v", i)
			continue
		}
		if group["NAME"].(string) == lunGroupName {
			lunGroupID, ok := group["ID"].(string)
			if !ok {
				return "", pkgUtils.Errorf(ctx, "convert lunGroupID to string failed, data: %v", group["ID"])
			}
			err = p.cli.RemoveLunFromGroup(ctx, lunID, lunGroupID)
			if err != nil {
				log.AddContext(ctx).Errorf("Remove lun %s from group %s error: %v",
					lunID, lunGroupID, err)
				return "", err
			}
		}
	}

	lunUniqueId, err := utils.GetLunUniqueId(ctx, p.protocol, lun)
	if err != nil {
		return "", err
	}
	return lunUniqueId, nil
}

// ControllerDetach detaches volume and unmaps lun from host
func (p *Attacher) ControllerDetach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (string, error) {
	host, err := p.getHost(ctx, parameters, false)
	if err != nil {
		log.AddContext(ctx).Infof("Get host ID error: %v", err)
		return "", err
	}
	if host == nil {
		log.AddContext(ctx).Infof("Host doesn't exist while detaching %s", lunName)
		return "", nil
	}

	hostID, ok := host["ID"].(string)
	if !ok {
		return "", pkgUtils.Errorf(ctx, "convert hostID to string failed, data: %v", host["ID"])
	}
	wwn, err := p.doUnmapping(ctx, hostID, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmapping LUN %s from host %s error: %v", lunName, hostID, err)
		return "", err
	}

	return wwn, nil
}

func (p *Attacher) getLunInfo(ctx context.Context, lunName string) (map[string]interface{}, error) {
	lun, err := p.cli.GetLunByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s info error: %v", lunName, err)
		return nil, err
	}
	if lun == nil {
		log.AddContext(ctx).Infof("LUN %s doesn't exist while detaching", lunName)
		return nil, nil
	}
	return lun, nil
}
