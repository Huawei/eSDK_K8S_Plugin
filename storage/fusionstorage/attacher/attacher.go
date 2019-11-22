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
}
func NewAttacher(cli *client.Client, protocol, invoker string) *Attacher {
	return &Attacher{
		cli:      cli,
		protocol: protocol,
		invoker:  invoker,
	}
}

func (p *Attacher) DetachVolumeBySCSI(volumeName, manageIP string ) error{

	lun, err :=p.cli.GetVolumeByName(volumeName)
	if err != nil {
		return err
	}
	if lun == nil {
		return nil
	}
	err = p.cli.DetachVolume(volumeName, manageIP)
	if err != nil {
		log.Errorf("Detach volume %s error: %v", volumeName, err)
		return err
	}
	return nil
}

func (p *Attacher) DetachVolumeByISCSI(volumeName, hostName string ) error{
	lun, err :=p.cli.GetVolumeByName(volumeName)
	if err != nil {
		return err
	}
	if lun == nil {
		return nil
	}

	hostList, err := p.cli.QueryHostByLun(volumeName)
	if err != nil {
		return err
	}

	for _, host := range hostList {
		lunHostInfo := host.(map[string]interface{})
		if lunHostInfo["hostName"] == hostName {
			err = p.cli.DeleteLunFromHost(volumeName, hostName)
			if err != nil {
				log.Errorf("Unmap volume %s from %s error: %v", volumeName, hostName, err)
				return err
			}
		}
	}

	return nil
}

func (p *Attacher) NodeStageByISCSI(lunName string, parameters map[string]interface{}) error{
	wwn, err := p.ControllerAttach(lunName, parameters)
	if err != nil {
		return err
	}
	device := dev.ScanDev(wwn, p.protocol)
	if device == "" {
		msg := fmt.Sprintf("Cannot detect device %s", wwn)
		log.Errorln(msg)
		return errors.New(msg)
	}

	devPath := fmt.Sprintf("/dev/%s", device)
	targetPath := parameters["targetPath"].(string)
	fsType := parameters["fsType"].(string)
	mountFlags := parameters["mountFlags"].(string)

	err = dev.MountLunDev(devPath, targetPath, fsType, mountFlags)
	if err != nil {
		log.Errorf("Mount device %s to %s error: %v", devPath, targetPath, err)
		return err
	}
	return nil
}

func (p *Attacher) NodeStageBySCSI(name string, parameters map[string]interface{}) error{
	hostname := parameters["HostName"].(string)
	hosts := parameters["hosts"].(map[string]string)
	manageIP, exist := hosts[hostname]
	if !exist {
		msg := fmt.Sprintf("There is no manage IP configured for host %s", hostname)
		log.Errorln(msg)
		return errors.New(msg)
	}
	vol, err := p.cli.GetVolumeByName(name)
	if err != nil {
		log.Errorf("Get volume by name %s error: %v", name, err)
		return err
	}

	if vol == nil {
		msg := fmt.Sprintf("Volume %s to attach doesn't exist", name)
		log.Errorln(msg)
		return errors.New(msg)
	}

	err = p.cli.AttachVolume(name, manageIP)
	if err != nil {
		log.Errorf("Attach volume %s error: %v", name, err)
		return err
	}

	wwn := vol["wwn"].(string)

	devPath := fmt.Sprintf("/dev/disk/by-id/wwn-0x%s", wwn)
	dev.WaitDevOnline(devPath)

	targetPath := parameters["targetPath"].(string)
	fsType := parameters["fsType"].(string)
	mountFlags := parameters["mountFlags"].(string)

	err = dev.MountLunDev(devPath, targetPath, fsType, mountFlags)
	if err != nil {
		log.Errorf("Mount device %s to %s error: %v", devPath, targetPath, err)
		return err
	}
	return nil
}
func (p *Attacher) ControllerAttach(lunName string, parameters map[string]interface{}) (string, error) {

	err := p.attachISCSI(parameters)
	if err != nil {
		log.Errorf("Check %s connection error: %v", p.protocol, err)
		return "", err
	}
	hostName, err := utils.GetHostName()
	if err != nil {
		return "", err
	}
	wwn, err := p.doMapping(lunName, hostName)
	if err != nil {
		log.Errorf("Mapping LUN %s to host %s error: %v", lunName, hostName, err)
		return "", err
	}

	return wwn, nil
}

func (p *Attacher) getValidTargetIP(node map[string]interface{}) map[string]bool {
	validIPs := map[string]bool{}
	iscsiPortalList, ok := node["iscsiPortalList"].([]interface{})

	if !ok || len(iscsiPortalList) == 0 {
		return validIPs
	}

	for _, iscsiPortal := range iscsiPortalList {
		iscsiPortalInfo := iscsiPortal.(map[string]interface{})
		if iscsiPortalInfo["iscsiStatus"] == "active" {
			ipAndPort := iscsiPortalInfo["iscsiPortal"].(string)
			standardIpAndPort := net.ParseIP(ipAndPort)
			if standardIpAndPort.To4() != nil {  // IPV4
				ipv4 := strings.Split(ipAndPort,":")[0]
				validIPs[ipv4] = true
			} else {
				ipv6AndPortList := strings.Split(ipAndPort,":")
				ipv6List := ipv6AndPortList[:len(ipv6AndPortList) - 1]
				ipv6 := strings.Join(ipv6List, ":")
				validIPs[ipv6] = true
			}
		}
	}

	return validIPs
}

func (p *Attacher) attachISCSI(parameters map[string]interface{}) error {
	nodeResultList, err := p.cli.QueryIscsiPortal()

	if err != nil {
		log.Errorf("Get ISCSI portal error: %v", err)
		return err
	}
	if len(nodeResultList) == 0 {
		msg := "No ISCSI portal exist"
		log.Errorln(msg)
		return errors.New(msg)
	}

	validIPs := map[string]bool{}
	for _, nodeResult := range nodeResultList {
		node := nodeResult.(map[string]interface{})
		if node["status"] == "successful" {
			validTargetIPs := p.getValidTargetIP(node)
			for ip, _ := range validTargetIPs {
				validIPs[ip] = true
			}
		}
	}

	portals := parameters["portals"].([]string)

	for _, ip := range portals {
		ip = net.ParseIP(ip).String()
		if !validIPs[ip] {
			msg := fmt.Sprintf("ISCSI portal %s is not valid at backend", ip)
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

	for _, ip := range portals {
		ip = net.ParseIP(ip).String()
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
	hostName, err := utils.GetHostName()
	if err != nil {
		log.Errorln("Get local host error: %v", err)
		return err
	}
	host, err := p.cli.GetHostByName(hostName)
	if err != nil {
		log.Infof("Get host error: %v", err)
		return err
	}
	if host == nil {
		err = p.cli.CreateHost(hostName)
		if err != nil {
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
	if initiator == nil {
		err  = p.cli.CreateInitiator(initiatorName)
		if err !=nil {
			return err
		}
		err = p.cli.AddPortToHost(initiatorName, hostName)
		if err != nil {
			return  err
		}
	}else {
		existHost, _:= p.cli.QueryHostByPort(initiatorName)
		if existHost == ""{
			err = p.cli.AddPortToHost(initiatorName, hostName)
			if err != nil {
				return  err
			}
		}else if existHost == hostName {
			return nil
		}else {
			msg := fmt.Sprintf("ISCSI initiator %s is already associated to another host %s", initiatorName, existHost)
			log.Errorln(msg)
			return errors.New(msg)
		}
	}
	return nil
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
	mapping, _ := p.cli.QueryHostFromVolume(lunName, hostName)
	if mapping == nil {
		err = p.cli.AddLunToHost(lunName, hostName)
		if err != nil {
			return "", err
		}
	}
	return lun["wwn"].(string), nil
}