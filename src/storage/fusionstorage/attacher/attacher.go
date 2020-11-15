package attacher

import (
	"connector"
	_ "connector/iscsi"
	"dev"
	"errors"
	"fmt"
	"net"
	"proto"
	"storage/fusionstorage/client"
	"strings"
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

func (p *Attacher) getTargetPortals() ([]string, error) {
	nodeResultList, err := p.cli.QueryIscsiPortal()
	if err != nil {
		log.Errorf("Get ISCSI portals error: %v", err)
		return nil, err
	}

	validIPs := map[string]bool{}

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
			}
		}
	}

	var availablePortals []string
	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		if !validIPs[ip] {
			log.Warningf("Config ISCSI portal %s is not valid", ip)
			continue
		}
		availablePortals = append(availablePortals, ip)
	}

	if availablePortals == nil {
		msg := fmt.Sprintf("All config portal %s is not valid", p.portals)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return availablePortals, nil
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
	lun, err := p.cli.GetVolumeByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return "", err
	}
	if lun == nil {
		log.Infof("LUN %s doesn't exist while detaching", lunName)
		return "", nil
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

func (p *Attacher) iSCSIControllerAttach(lunName string, parameters map[string]interface{}) (string, error) {
	hostName, err := p.getHostName(parameters)
	if err != nil {
		log.Errorf("Get host name error: %v", err)
		return "", err
	}

	err = p.createIscsiHost(hostName)
	if err != nil {
		log.Errorf("Create ISCSI host %s error: %v", hostName, err)
		return "", err
	}

	err = p.attachIscsiInitiatorToHost(hostName)
	if err != nil {
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

	return lun["wwn"].(string), nil
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

func (p *Attacher) NodeStage(lunName string, parameters map[string]interface{}) (string, error) {
	var devPath string
	if p.protocol == "iscsi" {
		tgtPortals, err := p.getTargetPortals()
		if err != nil {
			return "", err
		}

		wwn, err := p.iSCSIControllerAttach(lunName, parameters)
		if err != nil {
			return "", err
		}

		lenPortals := len(tgtPortals)
		var tgtLunWWNs []string
		for i := 0; i < lenPortals; i++ {
			tgtLunWWNs = append(tgtLunWWNs, wwn)
		}
		connMap := map[string]interface{}{
			"tgtPortals": tgtPortals,
			"tgtLunWWNs":  tgtLunWWNs,
		}

		conn := connector.GetConnector(connector.ISCSIDriver)
		devPath, err = conn.ConnectVolume(connMap)
		if err != nil {
			return "", err
		}

	} else {
		wwn, err := p.SCSIControllerAttach(lunName, parameters)
		if err != nil {
			return "", err
		}

		devPath = fmt.Sprintf("/dev/disk/by-id/wwn-0x%s", wwn)
		dev.WaitDevOnline(devPath)
	}

	return devPath, nil
}

func (p *Attacher) NodeUnstage(lunName string, parameters map[string]interface{}) error {
	wwn, err := p.ControllerDetach(lunName, parameters)
	if err != nil {
		return err
	}
	if wwn == "" {
		log.Warningf("Cannot get WWN of LUN %s, the dev may leftover", lunName)
		return nil
	}

	if p.protocol == "iscsi" {
		conn := connector.GetConnector(connector.ISCSIDriver)
		err := conn.DisConnectVolume(wwn)
		if err != nil {
			log.Errorf("Delete dev %s error: %v", wwn, err)
			return err
		}
	}

	return nil
}
