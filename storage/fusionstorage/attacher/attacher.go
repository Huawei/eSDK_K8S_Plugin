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

// Package attacher provide storage mapping or unmapping
package attacher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"huawei-csi-driver/connector"
	_ "huawei-csi-driver/connector/iscsi"
	_ "huawei-csi-driver/connector/local"
	"huawei-csi-driver/storage/fusionstorage/client"
	"huawei-csi-driver/storage/oceanstor/attacher"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// Attacher defines attacher client
type Attacher struct {
	cli      *client.Client
	protocol string
	invoker  string
	portals  []string
	hosts    map[string]string
	alua     map[string]interface{}
}

const (
	// DisableAlua defines switchover mode disable alua
	DisableAlua = "Disable_alua"
)

// NewAttacher used to init a new attacher
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

func (p *Attacher) getHostName(ctx context.Context, parameters map[string]interface{}) (string, error) {
	hostName, ok := parameters["HostName"].(string)
	if !ok {
		return "", fmt.Errorf("can not find host name,parameters:%v", parameters)
	}

	return hostName, nil
}

func (p *Attacher) parseISCSIPortal(ctx context.Context, iscsiPortal map[string]interface{}) string {
	if iscsiPortal["iscsiStatus"] != "active" {
		log.AddContext(ctx).Errorf("ISCSI portal %v is not active", iscsiPortal)
		return ""
	}

	portal, exist := iscsiPortal["iscsiPortal"].(string)
	if !exist {
		log.AddContext(ctx).Errorf("the key iscsiPortal does not exist in the iSCSIPortal %v", iscsiPortal)
		return ""
	}

	portalSplit := strings.Split(portal, ":")
	if len(portalSplit) < 2 {
		log.AddContext(ctx).Errorf("ISCSI portal %s is invalid", portal)
		return ""
	}

	ipStr := strings.Join(portalSplit[:len(portalSplit)-1], ":")
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.AddContext(ctx).Errorf("ISCSI IP %s is invalid", ipStr)
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
	} else if host["switchoverMode"] == DisableAlua {
		return false
	}

	pathType, ok := hostAlua["pathType"]
	if ok && pathType != host["pathType"] {
		return true
	}

	return false
}

func (p *Attacher) createIscsiHost(ctx context.Context, hostName string) error {
	host, err := p.cli.GetHostByName(ctx, hostName)
	if err != nil {
		return err
	}

	hostAlua := utils.GetAlua(ctx, p.alua, hostName)

	if host == nil {
		err = p.cli.CreateHost(ctx, hostName, hostAlua)
	} else if hostAlua != nil && p.needUpdateIscsiHost(host, hostAlua) {
		err = p.cli.UpdateHost(ctx, hostName, hostAlua)
	}

	return err
}

func (p *Attacher) getTargetPortals(ctx context.Context) ([]string, []string, error) {
	nodeResultList, err := p.cli.QueryIscsiPortal(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Get ISCSI portals error: %v", err)
		return nil, nil, err
	}

	validIPs := make(map[string]bool)
	validIQNs := make(map[string]string)
	for _, i := range nodeResultList {
		if i["status"] != "successful" {
			continue
		}

		iscsiPortalList, exist := i["iscsiPortalList"].([]interface{})
		if !exist {
			continue
		}

		err = p.parseiSCSIPortalList(ctx, iscsiPortalList, validIPs, validIQNs)
		if err != nil {
			log.AddContext(ctx).Errorf("parse ISCSI portals error: %v", err)
			return nil, nil, err
		}
	}

	var tgtPortals []string
	var tgtIQNs []string
	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		if !validIPs[ip] {
			log.AddContext(ctx).Warningf("Config ISCSI portal %s is not valid", ip)
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

func (p *Attacher) parseiSCSIPortalList(ctx context.Context,
	iscsiPortalList []interface{}, validIPs map[string]bool, validIQNs map[string]string) error {
	for _, portal := range iscsiPortalList {
		iscsiPortal, exist := portal.(map[string]interface{})
		if !exist {
			return errors.New("the iscsiPortalList type is incorrect")
		}
		ip := p.parseISCSIPortal(ctx, iscsiPortal)
		if len(ip) > 0 {
			validIPs[ip] = true
			validIQNs[ip], exist = iscsiPortal["targetName"].(string)
			if !exist {
				return errors.New("key targetName does not exist in IscsiPortal")
			}
		}
	}
	return nil
}

func (p *Attacher) attachIscsiInitiatorToHost(ctx context.Context, hostName string) error {
	parameters := map[string]interface{}{
		"HostName": hostName,
	}

	initiatorName, err := attacher.GetSingleInitiator(ctx, attacher.ISCSI, parameters)
	if err != nil {
		return err
	}

	initiator, err := p.cli.GetInitiatorByName(ctx, initiatorName)
	if err != nil {
		return err
	}

	var addInitiator bool

	if initiator == nil {
		err := p.cli.CreateInitiator(ctx, initiatorName)
		if err != nil {
			return err
		}

		addInitiator = true
	} else {
		host, err := p.cli.QueryHostByPort(ctx, initiatorName)
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
		err := p.cli.AddPortToHost(ctx, initiatorName, hostName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Attacher) isVolumeAddToHost(ctx context.Context, lunName, hostName string) (bool, error) {
	hosts, err := p.cli.QueryHostOfVolume(ctx, lunName)
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

func (p *Attacher) doMapping(ctx context.Context, lunName, hostName string) (string, error) {
	lun, err := p.cli.GetVolumeByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s error: %v", lunName, err)
		return "", err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s not exist for attaching", lunName)
		log.AddContext(ctx).Errorln(msg)
		return "", errors.New(msg)
	}

	if p.protocol == "iscsi" {
		isAdded, err := p.isVolumeAddToHost(ctx, lunName, hostName)
		if err != nil {
			return "", err
		}

		if !isAdded {
			err := p.cli.AddLunToHost(ctx, lunName, hostName)
			if err != nil {
				return "", err
			}
		}
	} else {
		manageIP, exist := p.hosts[hostName]
		if !exist {
			return "", fmt.Errorf("No manage IP configured for host %s", hostName)
		}

		err := p.cli.AttachVolume(ctx, lunName, manageIP)
		if err != nil {
			return "", err
		}
	}

	return lun["wwn"].(string), nil
}

func (p *Attacher) doUnmapping(ctx context.Context, lunName, hostName string) (string, error) {
	lun, err := p.getLunInfo(ctx, lunName)
	if lun == nil {
		return "", err
	}

	if p.protocol == "iscsi" {
		isAdded, err := p.isVolumeAddToHost(ctx, lunName, hostName)
		if err != nil {
			return "", err
		}

		if isAdded {
			err := p.cli.DeleteLunFromHost(ctx, lunName, hostName)
			if err != nil {
				return "", err
			}
		}
	} else {
		manageIP, exist := p.hosts[hostName]
		if !exist {
			return "", fmt.Errorf("No manage IP configured for host %s", hostName)
		}

		err := p.cli.DetachVolume(ctx, lunName, manageIP)
		if err != nil {
			return "", err
		}
	}

	return lun["wwn"].(string), nil
}

func (p *Attacher) getMappingProperties(ctx context.Context,
	wwn, hostLunId string, parameters map[string]interface{}) (map[string]interface{}, error) {
	tgtPortals, tgtIQNs, err := p.getTargetPortals(ctx)
	if err != nil {
		return nil, err
	}

	lenPortals := len(tgtPortals)
	var tgtHostLUNs []string
	for i := 0; i < lenPortals; i++ {
		tgtHostLUNs = append(tgtHostLUNs, hostLunId)
	}

	connectInfo := map[string]interface{}{
		"tgtLunWWN":   wwn,
		"tgtPortals":  tgtPortals,
		"tgtIQNs":     tgtIQNs,
		"tgtHostLUNs": tgtHostLUNs}

	return connectInfo, nil
}

func (p *Attacher) iSCSIControllerAttach(ctx context.Context, lunInfo utils.Volume,
	parameters map[string]interface{}) (
	map[string]interface{}, error) {
	hostName, err := p.getHostName(ctx, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host name error: %v", err)
		return nil, err
	}

	err = p.createIscsiHost(ctx, hostName)
	if err != nil {
		log.AddContext(ctx).Errorf("Create iSCSI host %s error: %v", hostName, err)
		return nil, err
	}

	err = p.attachIscsiInitiatorToHost(ctx, hostName)
	if err != nil {
		return nil, err
	}

	isAdded, err := p.isVolumeAddToHost(ctx, lunInfo.GetVolumeName(), hostName)
	if err != nil {
		return nil, err
	}

	if !isAdded {
		err := p.cli.AddLunToHost(ctx, lunInfo.GetVolumeName(), hostName)
		if err != nil {
			return nil, err
		}
	}

	hostLunId, err := p.cli.GetHostLunId(ctx, hostName, lunInfo.GetVolumeName())
	if err != nil {
		return nil, err
	}

	lunWWN, err := lunInfo.GetLunWWN()
	if err != nil {
		return nil, err
	}
	return p.getMappingProperties(ctx, lunWWN, hostLunId, parameters)
}

// SCSIControllerAttach used to attach volume to host
func (p *Attacher) SCSIControllerAttach(ctx context.Context,
	lunInfo utils.Volume,
	parameters map[string]interface{}) (string, error) {
	hostName, err := p.getHostName(ctx, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host name error: %v", err)
		return "", err
	}

	manageIP, exist := p.hosts[hostName]
	if !exist {
		return "", fmt.Errorf("No manage IP configured for host %s", hostName)
	}

	err = p.cli.AttachVolume(ctx, lunInfo.GetVolumeName(), manageIP)
	if err != nil {
		return "", err
	}

	lunWWN, err := lunInfo.GetLunWWN()
	if err != nil {
		return "", err
	}

	return lunWWN, nil
}

// ControllerDetach used to detach volume from host
func (p *Attacher) ControllerDetach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (string, error) {
	hostName, err := p.getHostName(ctx, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Get host name error: %v", err)
		return "", err
	}
	if hostName == "" {
		log.AddContext(ctx).Infof("Host doesn't exist while detaching %s", lunName)
		return "", nil
	}

	wwn, err := p.doUnmapping(ctx, lunName, hostName)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmapping LUN %s from host %s error: %v", lunName, hostName, err)
		return "", err
	}

	return wwn, nil
}

// ControllerAttach used to attach volume and return mapping info
func (p *Attacher) ControllerAttach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (map[string]interface{}, error) {

	var mappingInfo map[string]interface{}

	lun, err := p.getLunInfo(ctx, lunName)
	lunInfo := utils.NewVolume(lunName)
	if wwn, ok := lun["wwn"].(string); ok {
		lunInfo.SetLunWWN(wwn)
	}

	if p.protocol == "iscsi" {
		mappingInfo, err = p.iSCSIControllerAttach(ctx, lunInfo, parameters)
		if err != nil {
			return nil, err
		}
	} else {
		tgtLunWWN, err := p.SCSIControllerAttach(ctx, lunInfo, parameters)
		if err != nil {
			return nil, err
		}

		mappingInfo = map[string]interface{}{"tgtLunWWN": tgtLunWWN}
	}
	return mappingInfo, nil
}

// NodeStage used to stage node
func (p *Attacher) NodeStage(ctx context.Context,
	lunInfo utils.Volume,
	parameters map[string]interface{}) (*connector.ConnectInfo, error) {
	var conn connector.Connector
	var mappingInfo map[string]interface{}
	var err error
	if p.protocol == "iscsi" {
		mappingInfo, err = p.iSCSIControllerAttach(ctx, lunInfo, parameters)
		if err != nil {
			return &connector.ConnectInfo{}, err
		}

		conn = connector.GetConnector(ctx, connector.ISCSIDriver)
	} else {
		tgtLunWWN, err := p.SCSIControllerAttach(ctx, lunInfo, parameters)
		if err != nil {
			return &connector.ConnectInfo{}, err
		}

		mappingInfo = map[string]interface{}{"tgtLunWWN": tgtLunWWN}
		conn = connector.GetConnector(ctx, connector.LocalDriver)
	}

	return &connector.ConnectInfo{
		Conn:        conn,
		MappingInfo: mappingInfo,
	}, nil
}

// NodeUnstage used to unstage node
func (p *Attacher) NodeUnstage(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (*connector.DisConnectInfo, error) {
	lun, err := p.getLunInfo(ctx, lunName)
	if lun == nil {
		return nil, err
	}

	var conn connector.Connector
	if p.protocol == "iscsi" {
		conn = connector.GetConnector(ctx, connector.ISCSIDriver)
	} else {
		conn = connector.GetConnector(ctx, connector.LocalDriver)
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

func (p *Attacher) getLunInfo(ctx context.Context, lunName string) (map[string]interface{}, error) {
	lun, err := p.cli.GetVolumeByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s error: %v", lunName, err)
		return nil, err
	}
	if lun == nil {
		log.AddContext(ctx).Infof("LUN %s doesn't exist while detaching", lunName)
		return nil, nil
	}
	return lun, nil
}
