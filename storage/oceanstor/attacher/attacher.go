package attacher

import (
	"dev"
	"errors"
	"fmt"
	"proto"
	"storage/oceanstor/client"
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

func (p *Attacher) getExistHostByISCSIInitiator(iscsiInitiator string) (string, error) {
	var hostID string
	var err error

	if iscsiInitiator == "" {
		iscsiInitiator, err = proto.GetISCSIInitiator()
		if err != nil {
			log.Errorf("Get ISCSI initiator error: %v", err)
			return "", err
		}
	}

	initiator, err := p.cli.GetIscsiInitiator(iscsiInitiator)
	if err != nil {
		log.Errorf("Get ISCSI initiator %s error: %v", iscsiInitiator, err)
		return "", err
	}
	if initiator == nil {
		return "", nil
	}

	isFree := initiator["ISFREE"].(string)
	if isFree == "false" {
		hostID = initiator["PARENTID"].(string)
	}

	return hostID, nil
}

func (p *Attacher) getExistHostByFCInitiator(fcInitiators []string) (string, error) {
	var err error

	if fcInitiators == nil {
		fcInitiators, err = proto.GetFCInitiator()
		if err != nil {
			log.Errorf("Get fc initiator error: %v", err)
			return "", err
		}
	}

	var hosts []string

	for _, wwn := range fcInitiators {
		initiator, err := p.cli.GetFCInitiator(wwn)
		if err != nil {
			log.Errorf("Get FC initiator %s error: %v", wwn, err)
			return "", err
		}
		if initiator == nil {
			log.Warningf("FC initiator %s does not exist", wwn)
			continue
		}

		status := initiator["RUNNINGSTATUS"].(string)
		if status != "27" {
			log.Warningf("FC initiator %s is not online", wwn)
			continue
		}

		isFree := initiator["ISFREE"].(string)
		if isFree == "false" {
			host := initiator["PARENTID"].(string)
			hosts = append(hosts, host)
		}
	}

	if len(hosts) <= 0 {
		return "", nil
	}

	for i := 0; i < len(hosts)-1; i++ {
		if hosts[i] != hosts[i+1] {
			msg := fmt.Sprintf("There are more than 1 hosts FC initiators %v associated", fcInitiators)
			log.Errorln(msg)
			return "", errors.New(msg)
		}
	}

	return hosts[0], nil
}

func (p *Attacher) getHostIDByLocalHostname(hostname string) (string, error){

	hostToQuery := p.getHostName(hostname)
	host, err := p.cli.GetHostByName(hostToQuery)
	if err != nil {
		log.Errorf("Get host %s error: %v", hostname, err)
		return "", err
	}
	if host == nil {
		host, err = p.cli.CreateHost(hostToQuery)
		if err != nil {
			log.Errorf("Create host %s error: %v", hostToQuery, err)
			return "", err
		}
	}
	return host["ID"].(string), nil

}

func (p *Attacher) getHostID(parameters map[string]interface{}, toCreate bool) (string, error) {
	var hostID string
	var err error

	if p.protocol == "iscsi" {
		initiator, exist := parameters["ISCSIInitiator"].(string)
		if !exist {
			initiator = ""
		}

		hostID, err = p.getExistHostByISCSIInitiator(initiator)
	} else {
		initiators, exist := parameters["FCInitiators"].([]string)
		if !exist {
			initiators = nil
		}

		hostID, err = p.getExistHostByFCInitiator(initiators)
	}

	if err != nil {
		return "", err
	}
	if hostID != "" {
		log.Infof("Get host ID %s", hostID)
		return hostID, nil
	}

	hostname, exist := parameters["HostName"].(string)
	if !exist {
		hostname, err = utils.GetHostName()
		if err != nil {
			log.Errorf("Get hostname error: %v", err)
			return "", err
		}
	}

	hostToQuery := p.getHostName(hostname)
	host, err := p.cli.GetHostByName(hostToQuery)
	if err != nil {
		log.Errorf("Get host %s error: %v", hostname, err)
		return "", err
	}
	host, err = p.cli.CreateHost(hostToQuery)
	if err != nil {
		log.Errorf("Create host %s error: %v", hostToQuery, err)
		return "", err
	}
	return host["ID"].(string), nil
}

func (p *Attacher) createMapping(hostID string) (string, error) {
	mappingName := p.getMappingName(hostID)
	mapping, err := p.cli.GetMappingByName(mappingName)
	if err != nil {
		log.Errorf("Get mapping by name %s error: %v", mappingName, err)
		return "", err
	}
	if mapping == nil {
		mapping, err = p.cli.CreateMapping(mappingName)
		if err != nil {
			log.Errorf("Create mapping %s error: %v", mappingName, err)
			return "", err
		}
	}

	return mapping["ID"].(string), nil
}

func (p *Attacher) createHostGroup(hostID, mappingID string) error {
	var err error
	var hostGroup map[string]interface{}
	var hostGroupID string

	hostGroupsByHostID, err := p.cli.QueryAssociateHostGroup(21, hostID)
	if err != nil {
		log.Errorf("Query associated hostgroups of host %s error: %v", hostID, err)
		return err
	}

	hostGroupName := p.getHostGroupName(hostID)

	for _, i := range hostGroupsByHostID {
		group := i.(map[string]interface{})
		if group["NAME"].(string) == hostGroupName {
			hostGroupID = group["ID"].(string)
			goto Add_TO_MAPPING
		}
	}

	hostGroup, err = p.cli.GetHostGroupByName(hostGroupName)
	if err != nil {
		log.Errorf("Get hostgroup by name %s error: %v", hostGroupName, err)
		return err
	}
	if hostGroup == nil {
		hostGroup, err = p.cli.CreateHostGroup(hostGroupName)
		if err != nil {
			log.Errorf("Create hostgroup %s error: %v", hostGroupName, err)
			return err
		}
	}

	hostGroupID = hostGroup["ID"].(string)

	err = p.cli.AddHostToGroup(hostID, hostGroupID)
	if err != nil {
		log.Errorf("Add host %s to hostgroup %s error: %v", hostID, hostGroupID, err)
		return err
	}

Add_TO_MAPPING:
	hostGroupsByMappingID, err := p.cli.QueryAssociateHostGroup(245, mappingID)
	if err != nil {
		log.Errorf("Query associated hostgroups of mapping %s error: %v", mappingID, err)
		return err
	}

	for _, i := range hostGroupsByMappingID {
		group := i.(map[string]interface{})
		if group["NAME"].(string) == hostGroupName {
			return nil
		}
	}

	err = p.cli.AddGroupToMapping(14, hostGroupID, mappingID)
	if err != nil {
		log.Errorf("Add hostgroup %s to mapping %s error: %v", hostGroupID, mappingID, err)
		return err
	}

	return nil
}

func (p *Attacher) createLunGroup(lunID, hostID, mappingID string) error {
	var err error
	var lunGroup map[string]interface{}
	var lunGroupID string

	lunGroupsByLunID, err := p.cli.QueryAssociateLunGroup(11, lunID)
	if err != nil {
		log.Errorf("Query associated lungroups of lun %s error: %v", lunID, err)
		return err
	}

	lunGroupName := p.getLunGroupName(hostID)

	for _, i := range lunGroupsByLunID {
		group := i.(map[string]interface{})
		if group["NAME"].(string) == lunGroupName {
			lunGroupID = group["ID"].(string)
			goto Add_TO_MAPPING
		}
	}

	lunGroup, err = p.cli.GetLunGroupByName(lunGroupName)
	if err != nil {
		log.Errorf("Get lungroup by name %s error: %v", lunGroupName, err)
		return err
	}
	if lunGroup == nil {
		lunGroup, err = p.cli.CreateLunGroup(lunGroupName)
		if err != nil {
			log.Errorf("Create lungroup %s error: %v", lunGroupName, err)
			return err
		}
	}

	lunGroupID = lunGroup["ID"].(string)

	err = p.cli.AddLunToGroup(lunID, lunGroupID)
	if err != nil {
		log.Errorf("Add lun %s to group %s error: %v", lunID, lunGroupID, err)
		return err
	}

Add_TO_MAPPING:
	lunGroupsByMappingID, err := p.cli.QueryAssociateLunGroup(245, mappingID)
	if err != nil {
		log.Errorf("Query associated lungroups of mapping %s error: %v", mappingID, err)
		return err
	}

	for _, i := range lunGroupsByMappingID {
		group := i.(map[string]interface{})
		if group["NAME"].(string) == lunGroupName {
			return nil
		}
	}

	err = p.cli.AddGroupToMapping(256, lunGroupID, mappingID)
	if err != nil {
		log.Errorf("Add lungroup %s to mapping %s error: %v", lunGroupID, mappingID, err)
		return err
	}

	return nil
}
//config vstore should add initiator first
func (p *Attacher) attachISCSI(hostID string, parameters map[string]interface{}) error {

	name, err := proto.GetISCSIInitiator()
	if err != nil {
		log.Errorf("Get ISCSI initiator name error: %v", name)
		return err
	}

	initiator, err := p.cli.GetIscsiInitiator(name)
	if err != nil {
		log.Errorf("Get ISCSI initiator %s error: %v", name, err)
		return err
	}

	if initiator == nil {
		initiator, err = p.cli.AddIscsiInitiator(name)
		if err != nil {
			log.Errorf("Add initiator %s error: %v", name, err)
			return err
		}
	}

	isFree, freeExist := initiator["ISFREE"].(string)
	parent, parentExist := initiator["PARENTID"].(string)

	if freeExist && isFree == "true" {
		err := p.cli.AddIscsiInitiatorToHost(name, hostID)
		if err != nil {
			log.Errorf("Add ISCSI initiator %s to host %s error: %v", name, hostID, err)
			return err
		}
	} else if parentExist && parent != hostID {
		msg := fmt.Sprintf("ISCSI initiator %s is already associated to another host %s", name, parent)
		log.Errorln(msg)
		return errors.New(msg)
	}

	//make connection to target
	ports, err := p.cli.GetIscsiTgtPort()
	if err != nil {
		log.Errorf("Get ISCSI tgt port error: %v", err)
		return err
	}
	if ports == nil {
		msg := "No ISCSI tgt port exist"
		log.Errorln(msg)
		return errors.New(msg)
	}

	validIPs := map[string]bool{}
	for _, i := range ports {
		port := i.(map[string]interface{})
		portID := port["ID"].(string)
		portIqn := strings.Split(portID, ",")[0]
		splitIqn := strings.Split(portIqn, ":")

		if len(splitIqn) < 6 {
			continue
		}

		validIPs[splitIqn[5]] = true
	}

	portals := parameters["portals"].([]string)
	for _, ip := range portals {
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
	return nil
}

func (p *Attacher) attachFC(hostID string, parameters map[string]interface{}) error {
	fcInitiators, err := proto.GetFCInitiator()
	if err != nil {
		log.Errorf("Get fc initiator error: %v", err)
		return err
	}

	var addWWNs []string

	for _, wwn := range fcInitiators {
		initiator, err := p.cli.GetFCInitiator(wwn)
		if err != nil {
			log.Errorf("Get FC initiator %s error: %v", wwn, err)
			return err
		}
		if initiator == nil {
			log.Warningf("FC initiator %s does not exist", wwn)
			continue
		}

		status, exist := initiator["RUNNINGSTATUS"].(string)
		if !exist || status != "27" {
			log.Warningf("FC initiator %s is not online", wwn)
			continue
		}

		isFree, freeExist := initiator["ISFREE"].(string)
		parent, parentExist := initiator["PARENTID"].(string)

		if freeExist && isFree == "true" {
			addWWNs = append(addWWNs, wwn)
		} else if parentExist && parent != hostID {
			msg := fmt.Sprintf("FC initiator %s is already associated to another host %s", wwn, parent)
			log.Errorln(msg)
			return errors.New(msg)
		}
	}

	for _, wwn := range addWWNs {
		err := p.cli.AddFCInitiatorToHost(wwn, hostID)
		if err != nil {
			log.Errorf("Add initiator %s to host %s error: %v", wwn, hostID, err)
			return err
		}
	}

	return nil
}

func (p *Attacher) doMapping(hostID, lunName string, parameters map[string]interface{}) (string, error) {
	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s error: %v", lunName, err)
		return "", err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s not exist for attaching", lunName)
		log.Errorln(msg)
		return "", errors.New(msg)
	}

	lunID := lun["ID"].(string)

	mappingID, err := p.createMapping(hostID)
	if err != nil {
		log.Errorf("Create mapping for host %s error: %v", hostID, err)
		return "", err
	}

	err = p.createHostGroup(hostID, mappingID)
	if err != nil {
		log.Errorf("Create host group for host %s error: %v", hostID, err)
		return "", err
	}

	err = p.createLunGroup(lunID, hostID, mappingID)
	if err != nil {
		log.Errorf("Create lun group for host %s error: %v", hostID, err)
		return "", err
	}

	return lun["WWN"].(string), nil
}

func (p *Attacher) doUnmapping(hostID, lunName string, parameters map[string]interface{}) (string, error) {
	lun, err := p.cli.GetLunByName(lunName)
	if err != nil {
		log.Errorf("Get lun %s info error: %v", lunName, err)
		return "", err
	}
	if lun == nil {
		log.Infof("LUN %s doesn't exist while detaching", lunName)
		return "", nil
	}

	lunID := lun["ID"].(string)

	lunGroupsByLunID, err := p.cli.QueryAssociateLunGroup(11, lunID)
	if err != nil {
		log.Errorf("Query associated lungroups of lun %s error: %v", lunID, err)
		return "", err
	}

	lunGroupName := p.getLunGroupName(hostID)

	for _, i := range lunGroupsByLunID {
		group := i.(map[string]interface{})
		if group["NAME"].(string) == lunGroupName {
			lunGroupID := group["ID"].(string)
			err = p.cli.RemoveLunFromGroup(lunID, lunGroupID)
			if err != nil {
				log.Errorf("Remove lun %s from group %s error: %v", lunID, lunGroupID, err)
				return "", err
			}
		}
	}

	return lun["WWN"].(string), nil
}

func (p *Attacher) NodeStage(lunName string, parameters map[string]interface{}) error {
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

func (p *Attacher) NodeUnstage(lunName string, parameters map[string]interface{}) error {
	wwn, err := p.ControllerDetach(lunName, parameters)
	if err != nil {
		return err
	}
	if wwn == "" {
		log.Warningf("Cannot get WWN of LUN %s, the dev may leftover", lunName)
		return nil
	}

	err = dev.DeleteDev(wwn)
	if err != nil {
		log.Errorf("Delete dev %s error: %v", wwn, err)
		return err
	}

	return err
}

func (p *Attacher) ControllerAttach(lunName string, parameters map[string]interface{}) (string, error) {

	hostname,exist := parameters["HostName"].(string)
	if !exist {
		hostname,_ = utils.GetHostName()
	}
	hostID, err := p.getHostIDByLocalHostname(hostname)
	if err != nil {
		log.Errorf("Get host ID error: %v", err)
		return "", err
	}
	if hostID == "" {
		log.Errorf("Cannot get host ID while attaching %s", lunName)
		return "", nil
	}

	if p.protocol == "iscsi" {
		err = p.attachISCSI(hostID, parameters)
	} else {
		err = p.attachFC(hostID, parameters)
	}

	if err != nil {
		log.Errorf("Check %s connection error: %v", p.protocol, err)
		return "", err
	}

	wwn, err := p.doMapping(hostID, lunName, parameters)
	if err != nil {
		log.Errorf("Mapping LUN %s to host %s error: %v", lunName, hostID, err)
		return "", err
	}

	return wwn, nil
}

func (p *Attacher) ControllerDetach(lunName string, parameters map[string]interface{}) (string, error) {
	hostname,exist := parameters["HostName"].(string)
	if !exist {
		hostname,_ = utils.GetHostName()
	}
	hostToQuery := p.getHostName(hostname)
	host, err := p.cli.GetHostByName(hostToQuery)
	if err != nil {
		log.Errorf("Get host %s error: %v", hostname, err)
		return "", err
	}

	if host == nil {
		log.Infof("Host %s doesn't exist while detaching", lunName)
		return "", nil
	}
	hostID := host["ID"].(string)

	wwn, err := p.doUnmapping(hostID, lunName, parameters)
	if err != nil {
		log.Errorf("Unmapping LUN %s from host %s error: %v", lunName, hostID, err)
		return "", err
	}

	return wwn, nil
}
