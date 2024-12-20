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

// Package attacher provide base operations for volume attach
package attacher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector/nvme"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	splitIqnLength    = 6
	maxHostNameLength = 31
)

// BaseAttacherClientInterface defines client interfaces need to be implemented for base attacher
type BaseAttacherClientInterface interface {
	base.FC
	base.Host
	base.Iscsi
	base.Mapping
	base.RoCE
}

// AttachmentManager provides base operations for attach
type AttachmentManager struct {
	Cli      BaseAttacherClientInterface
	Protocol string
	Invoker  string
	Portals  []string
	Alua     map[string]interface{}
}

// AttachmentManagerConfig defines the configurations of AttachmentManager
type AttachmentManagerConfig struct {
	Cli      BaseAttacherClientInterface
	Protocol string
	Invoker  string
	Portals  []string
	Alua     map[string]interface{}
}

// NewAttachmentManager init a new AttachmentManager
func NewAttachmentManager(config AttachmentManagerConfig) *AttachmentManager {
	return &AttachmentManager{
		Cli:      config.Cli,
		Protocol: config.Protocol,
		Invoker:  config.Invoker,
		Portals:  config.Portals,
		Alua:     config.Alua,
	}
}

func (p *AttachmentManager) getHostName(postfix string) string {
	host := fmt.Sprintf("k8s_%s", postfix)
	if len(host) <= maxHostNameLength {
		return host
	}

	return host[:maxHostNameLength]
}

func (p *AttachmentManager) getHostGroupName(postfix string) string {
	return fmt.Sprintf("k8s_%s_hostgroup_%s", p.Invoker, postfix)
}

func (p *AttachmentManager) getMappingName(postfix string) string {
	return fmt.Sprintf("k8s_%s_mapping_%s", p.Invoker, postfix)
}

// GetHost gets an exist host or create a new host by host name from params
func (p *AttachmentManager) GetHost(ctx context.Context,
	parameters map[string]interface{},
	toCreate bool) (map[string]interface{}, error) {
	var err error

	hostname, exist := parameters["HostName"].(string)
	if !exist {
		log.AddContext(ctx).Errorf("Get hostname error: %v", err)
		return nil, err
	}

	hostToQuery := p.getHostName(hostname)
	host, err := p.Cli.GetHostByName(ctx, hostToQuery)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host %s error: %v", hostToQuery, err)
		return nil, err
	}
	if host == nil && toCreate {
		host, err = p.Cli.CreateHost(ctx, hostToQuery)
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

// CreateMapping creates mapping by hostID
func (p *AttachmentManager) CreateMapping(ctx context.Context, hostID string) (string, error) {
	mappingName := p.getMappingName(hostID)
	mapping, err := p.Cli.GetMappingByName(ctx, mappingName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get mapping by name %s error: %v", mappingName, err)
		return "", err
	}
	if mapping == nil {
		mapping, err = p.Cli.CreateMapping(ctx, mappingName)
		if err != nil {
			log.AddContext(ctx).Errorf("Create mapping %s error: %v", mappingName, err)
			return "", err
		}
	}

	return mapping["ID"].(string), nil
}

// CreateHostGroup creates or gets a host group, add host to the group and add the group to mapping
func (p *AttachmentManager) CreateHostGroup(ctx context.Context, hostID, mappingID string) error {
	var err error
	var hostGroup map[string]interface{}
	var hostGroupID string

	hostGroupsByHostID, err := p.Cli.QueryAssociateHostGroup(ctx, base.AssociateObjTypeHost, hostID)
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

	hostGroup, err = p.Cli.GetHostGroupByName(ctx, hostGroupName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get hostgroup by name %s error: %v", hostGroupName, err)
		return err
	}
	if hostGroup == nil {
		hostGroup, err = p.Cli.CreateHostGroup(ctx, hostGroupName)
		if err != nil {
			log.AddContext(ctx).Errorf("Create hostgroup %s error: %v", hostGroupName, err)
			return err
		}
	}

	hostGroupID, ok := hostGroup["ID"].(string)
	if !ok {
		return errors.New("createHostGroup failed, caused by not found hostGroup id")
	}

	err = p.Cli.AddHostToGroup(ctx, hostID, hostGroupID)
	if err != nil {
		log.AddContext(ctx).Errorf("Add host %s to hostgroup %s error: %v",
			hostID, hostGroupID, err)
		return err
	}

	return p.addToHostGroupMapping(ctx, hostGroupName, hostGroupID, mappingID)
}

func (p *AttachmentManager) addToHostGroupMapping(ctx context.Context, groupName, groupID, mappingID string) error {
	hostGroupsByMappingID, err := p.Cli.QueryAssociateHostGroup(ctx, base.AssociateObjTypeMapping, mappingID)
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

	err = p.Cli.AddGroupToMapping(ctx, base.AssociateObjTypeHostGroup, groupID, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Add host group %s to mapping %s error: %v",
			groupID, mappingID, err)
		return err
	}

	return nil
}

func (p *AttachmentManager) getISCSIProperties(ctx context.Context, wwn, hostLunId string, parameters map[string]any) (
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

func (p *AttachmentManager) getFCProperties(ctx context.Context, wwn, hostLunId string, parameters map[string]any) (
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

func (p *AttachmentManager) getFCNVMeProperties(ctx context.Context, wwn, hostLunId string, parameters map[string]any) (
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

func (p *AttachmentManager) getRoCEProperties(ctx context.Context, wwn, hostLunId string, parameters map[string]any) (
	map[string]interface{}, error) {
	tgtPortals, err := p.GetTargetRoCEPortals(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tgtPortals": tgtPortals,
		"tgtLunGuid": wwn,
	}, nil
}

// GetMappingProperties gets the mapping properties
func (p *AttachmentManager) GetMappingProperties(ctx context.Context,
	wwn, hostLunId string, parameters map[string]interface{}) (map[string]interface{}, error) {
	if p.Protocol == "iscsi" {
		return p.getISCSIProperties(ctx, wwn, hostLunId, parameters)
	} else if p.Protocol == "fc" {
		return p.getFCProperties(ctx, wwn, hostLunId, parameters)
	} else if p.Protocol == "fc-nvme" {
		return p.getFCNVMeProperties(ctx, wwn, hostLunId, parameters)
	} else if p.Protocol == "roce" {
		return p.getRoCEProperties(ctx, wwn, hostLunId, parameters)
	}

	return nil, utils.Errorf(ctx, "UnSupport protocol %s", p.Protocol)
}

func (p *AttachmentManager) getTargetISCSIProperties(ctx context.Context) ([]string, []string, error) {
	ports, err := p.Cli.GetIscsiTgtPort(ctx)
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

		if len(splitIqn) < splitIqnLength {
			continue
		}

		validIPs[splitIqn[splitIqnLength-1]] = true
		validIQNs[splitIqn[splitIqnLength-1]] = portIqn
	}

	var tgtPortals []string
	var tgtIQNs []string
	for _, portal := range p.Portals {
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
		msg := fmt.Sprintf("All config portal %s is not valid", p.Portals)
		log.AddContext(ctx).Errorln(msg)
		return nil, nil, errors.New(msg)
	}

	return tgtPortals, tgtIQNs, nil
}

// GetTargetRoCEPortals gets target roce portals
func (p *AttachmentManager) GetTargetRoCEPortals(ctx context.Context) ([]string, error) {
	var availablePortals []string
	for _, portal := range p.Portals {
		ip := net.ParseIP(portal).String()
		rocePortal, err := p.Cli.GetRoCEPortalByIP(ctx, ip)
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
		msg := fmt.Sprintf("All config portal %s is not valid", p.Portals)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	return availablePortals, nil
}

func (p *AttachmentManager) getTargetFCNVMeProperties(ctx context.Context,
	parameters map[string]interface{}) ([]nvme.PortWWNPair, error) {
	fcInitiators, err := GetMultipleInitiators(ctx, FC, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get fc initiator error:%v", err)
		return nil, err
	}

	var ret []nvme.PortWWNPair
	for _, hostInitiator := range fcInitiators {
		tgtWWNs, err := p.Cli.GetFCTargetWWNs(ctx, hostInitiator)
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

func (p *AttachmentManager) getTargetFCProperties(ctx context.Context, parameters map[string]any) ([]string, error) {
	fcInitiators, err := GetMultipleInitiators(ctx, FC, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get fc initiator error: %v", err)
		return nil, err
	}

	validTgtWWNs := make(map[string]bool)
	for _, wwn := range fcInitiators {
		tgtWWNs, err := p.Cli.GetFCTargetWWNs(ctx, wwn)
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

// AttachISCSI generates the relationship between iscsi initiator and host
func (p *AttachmentManager) AttachISCSI(ctx context.Context,
	hostID string, parameters map[string]interface{}) (map[string]interface{}, error) {
	name, err := GetSingleInitiator(ctx, ISCSI, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get ISCSI initiator name error: %v", err)
		return nil, err
	}

	initiator, err := p.Cli.GetIscsiInitiator(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get ISCSI initiator %s error: %v", name, err)
		return nil, err
	}

	if initiator == nil {
		initiator, err = p.Cli.AddIscsiInitiator(ctx, name)
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
		err := p.Cli.AddIscsiInitiatorToHost(ctx, name, hostID)
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

// AttachFC generates the relationship between fc initiator and host
func (p *AttachmentManager) AttachFC(ctx context.Context,
	hostID string, parameters map[string]interface{}) ([]map[string]interface{}, error) {
	fcInitiators, err := GetMultipleInitiators(ctx, FC, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get fc initiator error: %v", err)
		return nil, err
	}

	var addWWNs []string
	var hostInitiators []map[string]interface{}

	for _, wwn := range fcInitiators {
		initiator, err := p.Cli.GetFCInitiator(ctx, wwn)
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
		err := p.Cli.AddFCInitiatorToHost(ctx, wwn, hostID)
		if err != nil {
			log.AddContext(ctx).Errorf("Add initiator %s to host %s error: %v", wwn, hostID, err)
			return nil, err
		}
	}

	return hostInitiators, nil
}

// AttachRoCE generates the relationship between roce initiator and host
func (p *AttachmentManager) AttachRoCE(ctx context.Context,
	hostID string, parameters map[string]interface{}) (map[string]interface{}, error) {
	name, err := GetSingleInitiator(ctx, ROCE, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get RoCE initiator name error: %v", err)
		return nil, err
	}

	initiator, err := p.Cli.GetRoCEInitiator(ctx, name)
	if err != nil {
		log.AddContext(ctx).Errorf("Get RoCE initiator %s error: %v", name, err)
		return nil, err
	}

	if initiator == nil {
		initiator, err = p.Cli.AddRoCEInitiator(ctx, name)
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
		err := p.Cli.AddRoCEInitiatorToHost(ctx, name, hostID)
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
