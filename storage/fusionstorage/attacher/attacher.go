package attacher

import (
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
}

func NewAttacher(cli *client.Client, protocol, invoker string, portals []string, hosts map[string]string) *Attacher {
	return &Attacher{
		cli:      cli,
		protocol: protocol,
		invoker:  invoker,
		portals:  portals,
		hosts:    hosts,
	}
}

func (p *Attacher) getHost(parameters map[string]interface{}, toCreate bool) (string, error) {
	var err error

	hostName, exist := parameters["HostName"].(string)
	if !exist {
		hostName, err = utils.GetHostName()
		if err != nil {
			return "", err
		}
	}

	if p.protocol == "iscsi" {
		host, err := p.cli.GetHostByName(hostName)
		if err != nil {
			return "", err
		}

		if host != nil {
			return hostName, nil
		}

		if !toCreate {
			return "", nil
		}

		err = p.cli.CreateHost(hostName)
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

	ipStr := strings.Join(portalSplit[:len(portalSplit) - 1], ":")
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Errorf("ISCSI IP %s is invalid", ipStr)
		return ""
	}

	return ip.String()
}

func (p *Attacher) attachISCSI(hostName string, parameters map[string]interface{}) error {
	nodeResultList, err := p.cli.QueryIscsiPortal()
	if err != nil {
		log.Errorf("Get ISCSI portals error: %v", err)
		return err
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

	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		if !validIPs[ip] {
			msg := fmt.Sprintf("Config ISCSI portal %s is not valid", ip)
			log.Errorln(msg)
			return errors.New(msg)
		}

		output, err := utils.ExecShellCmd("iscsiadm -m discovery -t sendtargets -p %s", ip)
		if err != nil {
			log.Errorf("Cannot connect ISCSI portal %s: %v", ip, output)
			return err
		}
	}

	// Need ignore error here
	output, _ := utils.ExecShellCmd("iscsiadm -m session")

	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		if strings.Contains(output, ip) {
			log.Infof("Already login iscsi target %s, no need login again", ip)
			continue
		}

		output, err := utils.ExecShellCmd("iscsiadm -m node -p %s --login", ip)
		if err != nil {
			log.Errorf("Login iscsi target %s error: %s", ip, output)
			return err
		}
	}

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

func (p *Attacher) ControllerAttach(lunName string, parameters map[string]interface{}) (string, error) {
	hostName, err := p.getHost(parameters, true)
	if err != nil {
		log.Errorf("Get host name error: %v", err)
		return "", err
	}

	if p.protocol == "iscsi" {
		err := p.attachISCSI(hostName, parameters)
		if err != nil {
			log.Errorf("Attach %s connection error: %v", p.protocol, err)
			return "", err
		}
	}

	wwn, err := p.doMapping(lunName, hostName)
	if err != nil {
		log.Errorf("Mapping LUN %s to host %s error: %v", lunName, hostName, err)
		return "", err
	}

	return wwn, nil
}

func (p *Attacher) ControllerDetach(lunName string, parameters map[string]interface{}) (string, error) {
	hostName, err := p.getHost(parameters, false)
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
	wwn, err := p.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	var devPath string

	if p.protocol == "iscsi" {
		device := dev.ScanDev(wwn, p.protocol)
		if device == "" {
			msg := fmt.Sprintf("Cannot detect device %s", wwn)
			log.Errorln(msg)
			return "", errors.New(msg)
		}

		devPath = fmt.Sprintf("/dev/%s", device)
	} else {
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
		err := dev.DeleteDev(wwn)
		if err != nil {
			log.Errorf("Delete dev %s error: %v", wwn, err)
			return err
		}
	}

	return nil
}
