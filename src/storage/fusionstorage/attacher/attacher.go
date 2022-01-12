/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package attacher provide storage mapping or unmapping
package attacher

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"connector"
	_ "connector/iscsi"
	_ "connector/local"
	"proto"
	"storage/fusionstorage/client"
	"utils"
	"utils/log"
)

type Attacher struct {
	cli      *client.Client
	protocol string
	invoker  string
	portals  []string
	hosts    map[string]string
	alua     map[string]interface{}
}

const (
	DISABLE_ALUA = "Disable_alua"
)

func NewAttacher(cli *client.Client, protocol, invoker string, portals []string,
	hosts map[string]string, alua map[string]interface{}) *Attacher {
	return &Attacher{
		cli:      cli,
		protocol: protocol,
		invoker:  invoker,
		portals:  portals,
		hosts:    hosts,
		alua:     alua,
	}
}

func (p *Attacher) getHostName(parameters map[string]interface{}) (string, error) {
	hostName, ok := parameters["HostName"].(string)
	if !ok {
		var err error

		hostName, err = utils.GetHostName()
		if err != nil {
			return "", err
		}
	}

	return hostName, nil
}

func (p *Attacher) parseISCSIPortal(iscsiPortal map[string]interface{}) string {
	if iscsiPortal["iscsiStatus"] != "active" {
		log.Errorf("ISCSI portal %v is not active", iscsiPortal)
		return ""
	}

	portal := iscsiPortal["iscsiPortal"].(string)

	portalSplit := strings.Split(portal, ":")
	if len(portalSplit) < 2 {
		log.Errorf("ISCSI portal %s is invalid", portal)
		return ""
	}

	ipStr := strings.Join(portalSplit[:len(portalSplit)-1], ":")
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Errorf("ISCSI IP %s is invalid", ipStr)
		return ""
	}

	return ip.String()
}

func (p *Attacher) needUpdateIscsiHost(host map[string]interface{}, hostAlua map[string]interface{}) bool {
	switchoverMode, ok := hostAlua["switchoverMode"]
	if !ok {
		return false
	}

	if switchoverMode != host["switchoverMode"] {
		return true
	} else if host["switchoverMode"] == DISABLE_ALUA {
		return false
	}

	pathType, ok := hostAlua["pathType"]
	if ok && pathType != host["pathType"] {
		return true
	}

	return false
}

func (p *Attacher) createIscsiHost(hostName string) error {
	host, err := p.cli.GetHostByName(hostName)
	if err != nil {
		return err
	}

	hostAlua := utils.GetAlua(p.alua, hostName)

	if host == nil {
		err = p.cli.CreateHost(hostName, hostAlua)
	} else if hostAlua != nil && p.needUpdateIscsiHost(host, hostAlua) {
		err = p.cli.UpdateHost(hostName, hostAlua)
	}

	return err
}

func (p *Attacher) getTargetPortals() ([]string, []string, error) {
	nodeResultList, err := p.cli.QueryIscsiPortal()
	if err != nil {
		log.Errorf("Get ISCSI portals error: %v", err)
		return nil, nil, err
	}

	validIPs := map[string]bool{}
	validIQNs := map[string]string{}
	for _, i := range nodeResultList {
		if i["status"] != "successful" {
			continue
		}

		iscsiPortalList, exist := i["iscsiPortalList"].([]interface{})
		if !exist {
			continue
		}

		for _, portal := range iscsiPortalList {
			iscsiPortal := portal.(map[string]interface{})
			ip := p.parseISCSIPortal(iscsiPortal)
			if len(ip) > 0 {
				validIPs[ip] = true
				validIQNs[ip] = iscsiPortal["targetName"].(string)
			}
		}
	}

	var tgtPortals []string
	var tgtIQNs []string
	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		if !validIPs[ip] {
			log.Warningf("Config ISCSI portal %s is not valid", ip)
			continue
		}

		formatIP := fmt.Sprintf("%s:3260", ip)
		tgtPortals = append(tgtPortals, formatIP)
		tgtIQNs = append(tgtIQNs, validIQNs[ip])
	}

	if tgtPortals == nil {
		msg := fmt.Sprintf("All config portal %s is not valid", p.portals)
		log.Errorln(msg)
		return nil, nil, errors.New(msg)
	}

	return tgtPortals, tgtIQNs, nil
}

func (p *Attacher) attachIscsiInitiatorToHost(hostName string) error {
	initiatorName, err := proto.GetISCSIInitiator()
	if err != nil {
		return err
	}

	initiator, err := p.cli.GetInitiatorByName(initiatorName)
	if err != nil {
		return err
	}

	var addInitiator bool

	if initiator == nil {
		err := p.cli.CreateInitiator(initiatorName)
		if err != nil {
			return err
		}

		addInitiator = true
	} else {
		host, err := p.cli.QueryHostByPort(initiatorName)
		if err != nil {
			return err
		}

		if len(host) == 0 {
			addInitiator = true
		} else if host != hostName {
			return fmt.Errorf("ISCSI initiator %s is already associated to another host %s", initiatorName, host)
		}
	}

	if addInitiator {
		err := p.cli.AddPortToHost(initiatorName, hostName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Attacher) isVolumeAddToHost(lunName, hostName string) (bool, error) {
	hosts, err := p.cli.QueryHostOfVolume(lunName)
	if err != nil {
		return false, err
	}

	for _, i := range hosts {
		if i["hostName"].(string) == hostName {
			return true, nil
		}
	}

	return false, nil
}

func (p *Attacher) doMapping(lunName, hostName string) (string, error) {
	lun, err := p.cli.GetVolumeByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return "", err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s not exist for attaching", lunName)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	if p.protocol == "iscsi" {
		isAdded, err := p.isVolumeAddToHost(lunName, hostName)
		if err != nil {
			return "", err
		}

		if !isAdded {
			err := p.cli.AddLunToHost(lunName, hostName)
			if err != nil {
				return "", err
			}
		}
	} else {
		manageIP, exist := p.hosts[hostName]
		if !exist {
			return "", fmt.Errorf("No manage IP configured for host %s", hostName)
		}

		err := p.cli.AttachVolume(lunName, manageIP)
		if err != nil {
			return "", err
		}
	}

	return lun["wwn"].(string), nil
}

func (p *Attacher) doUnmapping(lunName, hostName string) (string, error) {
	lun, err := p.getLunInfo(lunName)
	if lun == nil {
		return "", err
	}

	if p.protocol == "iscsi" {
		isAdded, err := p.isVolumeAddToHost(lunName, hostName)
		if err != nil {
			return "", err
		}

		if isAdded {
			err := p.cli.DeleteLunFromHost(lunName, hostName)
			if err != nil {
				return "", err
			}
		}
	} else {
		manageIP, exist := p.hosts[hostName]
		if !exist {
			return "", fmt.Errorf("No manage IP configured for host %s", hostName)
		}

		err := p.cli.DetachVolume(lunName, manageIP)
		if err != nil {
			return "", err
		}
	}

	return lun["wwn"].(string), nil
}

func (p *Attacher) getMappingProperties(wwn, hostLunId string, volumeUseMultiPath bool) (
	map[string]interface{}, error) {
	tgtPortals, tgtIQNs, err := p.getTargetPortals()
	if err != nil {
		return nil, err
	}

	lenPortals := len(tgtPortals)
	var tgtHostLUNs []string
	for i := 0; i < lenPortals; i++ {
		tgtHostLUNs = append(tgtHostLUNs, hostLunId)
	}

	connectInfo := map[string]interface{}{
		"tgtLunWWN":          wwn,
		"tgtPortals":         tgtPortals,
		"tgtIQNs":            tgtIQNs,
		"tgtHostLUNs":        tgtHostLUNs,
		"volumeUseMultiPath": volumeUseMultiPath,}
	return connectInfo, nil
}

func (p *Attacher) iSCSIControllerAttach(lunName string, parameters map[string]interface{}) (
	map[string]interface{}, error) {
	hostName, err := p.getHostName(parameters)
	if err != nil {
		log.Errorf("Get host name error: %v", err)
		return nil, err
	}

	err = p.createIscsiHost(hostName)
	if err != nil {
		log.Errorf("Create iSCSI host %s error: %v", hostName, err)
		return nil, err
	}

	err = p.attachIscsiInitiatorToHost(hostName)
	if err != nil {
		return nil, err
	}

	lun, err := p.cli.GetVolumeByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return nil, err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s not exist for attaching", lunName)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	isAdded, err := p.isVolumeAddToHost(lunName, hostName)
	if err != nil {
		return nil, err
	}

	if !isAdded {
		err := p.cli.AddLunToHost(lunName, hostName)
		if err != nil {
			return nil, err
		}
	}

	hostLunId, err := p.cli.GetHostLunId(hostName, lunName)
	if err != nil {
		return nil, err
	}

	volumeUseMultiPath, ok := parameters["volumeUseMultiPath"].(bool)
	if !ok {
		volumeUseMultiPath = true
	}

	return p.getMappingProperties(lun["wwn"].(string), hostLunId, volumeUseMultiPath)
}

func (p *Attacher) SCSIControllerAttach(lunName string, parameters map[string]interface{}) (string, error) {
	hostName, err := p.getHostName(parameters)
	if err != nil {
		log.Errorf("Get host name error: %v", err)
		return "", err
	}

	lun, err := p.cli.GetVolumeByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return "", err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s not exist for attaching", lunName)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	manageIP, exist := p.hosts[hostName]
	if !exist {
		return "", fmt.Errorf("No manage IP configured for host %s", hostName)
	}

	err = p.cli.AttachVolume(lunName, manageIP)
	if err != nil {
		return "", err
	}

	return lun["wwn"].(string), nil
}

func (p *Attacher) ControllerDetach(lunName string, parameters map[string]interface{}) (string, error) {
	hostName, err := p.getHostName(parameters)
	if err != nil {
		log.Errorf("Get host name error: %v", err)
		return "", err
	}
	if hostName == "" {
		log.Infof("Host doesn't exist while detaching %s", lunName)
		return "", nil
	}

	wwn, err := p.doUnmapping(lunName, hostName)
	if err != nil {
		log.Errorf("Unmapping LUN %s from host %s error: %v", lunName, hostName, err)
		return "", err
	}

	return wwn, nil
}

func (p *Attacher) NodeStage(lunName string, parameters map[string]interface{}) (*connector.ConnectInfo, error) {
	var conn connector.Connector
	var mappingInfo map[string]interface{}
	var err error
	if p.protocol == "iscsi" {
		mappingInfo, err = p.iSCSIControllerAttach(lunName, parameters)
		if err != nil {
			return nil, err
		}

		conn = connector.GetConnector(connector.ISCSIDriver)
	} else {
		tgtLunWWN, err := p.SCSIControllerAttach(lunName, parameters)
		if err != nil {
			return nil, err
		}

		mappingInfo = map[string]interface{}{"tgtLunWWN": tgtLunWWN}
		conn = connector.GetConnector(connector.LocalDriver)
	}

	return &connector.ConnectInfo{
		Conn:        conn,
		MappingInfo: mappingInfo,
	}, nil
}

func (p *Attacher) NodeUnstage(lunName string, parameters map[string]interface{}) (*connector.DisConnectInfo, error) {
	lun, err := p.getLunInfo(lunName)
	if lun == nil {
		return nil, err
	}

	var conn connector.Connector
	if p.protocol == "iscsi" {
		conn = connector.GetConnector(connector.ISCSIDriver)
	} else {
		conn = connector.GetConnector(connector.LocalDriver)
	}

	tgtLunWWN, ok := lun["wwn"].(string)
	if !ok {
		return nil, errors.New("there is no wwn in lun info")
	}

	return &connector.DisConnectInfo{
		Conn:   conn,
		TgtLun: tgtLunWWN,
	}, nil
}

func (p *Attacher) getLunInfo(lunName string) (map[string]interface{}, error) {
	lun, err := p.cli.GetVolumeByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return nil, err
	}
	if lun == nil {
		log.Infof("LUN %s doesn't exist while detaching", lunName)
		return nil, nil
	}
	return lun, nil
}
