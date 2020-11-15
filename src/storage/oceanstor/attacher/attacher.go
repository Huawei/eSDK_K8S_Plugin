package attacher

import (
	"errors"
	"fmt"
	"net"
	"proto"
	"storage/oceanstor/client"
	"strings"
	"utils"
	"utils/log"
)

type AttacherPlugin interface {
	ControllerAttach(string, map[string]interface{}) (string, error)
	ControllerDetach(string, map[string]interface{}) (string, error)
	NodeStage(string, map[string]interface{}) (string, error)
	NodeUnstage(string, map[string]interface{}) error
	getTargetISCSIPortals() ([]string, error)
	getTargetRoCEPortals() ([]string, error)
}

type Attacher struct {
	cli      *client.Client
	protocol string
	invoker  string
	portals  []string
	alua     map[string]interface{}
}

func NewAttacher(
	product string,
	cli *client.Client,
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

func (p *Attacher) getHost(parameters map[string]interface{}, toCreate bool) (map[string]interface{}, error) {
	var err error

	hostname, exist := parameters["HostName"].(string)
	if !exist {
		hostname, err = utils.GetHostName()
		if err != nil {
			log.Errorf("Get hostname error: %v", err)
			return nil, err
		}
	}

	hostToQuery := p.getHostName(hostname)
	host, err := p.cli.GetHostByName(hostToQuery)
	if err != nil {
		log.Errorf("Get host %s error: %v", hostToQuery, err)
		return nil, err
	}
	if host == nil && toCreate {
		host, err = p.cli.CreateHost(hostToQuery)
		if err != nil {
			log.Errorf("Create host %s error: %v", hostToQuery, err)
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
	} else if initiator["MULTIPATHTYPE"] == MULTIPATHTYPE_DEFAULT {
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

func (p *Attacher) getTargetISCSIPortals() ([]string, error) {
	ports, err := p.cli.GetIscsiTgtPort()
	if err != nil {
		log.Errorf("Get ISCSI tgt port error: %v", err)
		return nil, err
	}
	if ports == nil {
		msg := "No ISCSI tgt port exist"
		log.Errorln(msg)
		return nil, errors.New(msg)
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

	var availablePortals []string
	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		if !validIPs[ip] {
			log.Warningf("ISCSI portal %s is not valid", ip)
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

func (p *Attacher) getTargetRoCEPortals() ([]string, error) {
	var availablePortals []string
	for _, portal := range p.portals {
		ip := net.ParseIP(portal).String()
		rocePortal, err := p.cli.GetRoCEPortalByIP(ip)
		if err != nil {
			log.Errorf("Get RoCE tgt portal error: %v", err)
			return nil, err
		}

		if rocePortal == nil {
			log.Warningf("the config portal %s does not exit.", ip)
			continue
		}

		supportProtocol, exist := rocePortal["SUPPORTPROTOCOL"].(string)
		if !exist {
			msg := "current storage does not support NVMe"
			log.Errorln(msg)
			return nil, errors.New(msg)
		}

		if supportProtocol != "64" { // 64 means NVME protocol
			log.Warningf("the config portal %s does not support NVME.", ip)
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

func (p *Attacher) attachISCSI(hostID string) (map[string]interface{}, error) {
	name, err := proto.GetISCSIInitiator()
	if err != nil {
		log.Errorf("Get ISCSI initiator name error: %v", name)
		return nil, err
	}

	initiator, err := p.cli.GetIscsiInitiator(name)
	if err != nil {
		log.Errorf("Get ISCSI initiator %s error: %v", name, err)
		return nil, err
	}

	if initiator == nil {
		initiator, err = p.cli.AddIscsiInitiator(name)
		if err != nil {
			log.Errorf("Add initiator %s error: %v", name, err)
			return nil, err
		}
	}

	isFree, freeExist := initiator["ISFREE"].(string)
	parent, parentExist := initiator["PARENTID"].(string)

	if freeExist && isFree == "true" {
		err := p.cli.AddIscsiInitiatorToHost(name, hostID)
		if err != nil {
			log.Errorf("Add ISCSI initiator %s to host %s error: %v", name, hostID, err)
			return nil, err
		}
	} else if parentExist && parent != hostID {
		msg := fmt.Sprintf("ISCSI initiator %s is already associated to another host %s", name, parent)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return initiator, nil
}

func (p *Attacher) attachFC(hostID string) ([]map[string]interface{}, error) {
	fcInitiators, err := proto.GetFCInitiator()
	if err != nil {
		log.Errorf("Get fc initiator error: %v", err)
		return nil, err
	}

	var addWWNs []string
	var hostInitiators []map[string]interface{}

	for _, wwn := range fcInitiators {
		initiator, err := p.cli.GetFCInitiator(wwn)
		if err != nil {
			log.Errorf("Get FC initiator %s error: %v", wwn, err)
			return nil, err
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
			return nil, errors.New(msg)
		}

		hostInitiators = append(hostInitiators, initiator)
	}

	for _, wwn := range addWWNs {
		err := p.cli.AddFCInitiatorToHost(wwn, hostID)
		if err != nil {
			log.Errorf("Add initiator %s to host %s error: %v", wwn, hostID, err)
			return nil, err
		}
	}

	return hostInitiators, nil
}

func (p *Attacher) attachRoCE(hostID string) (map[string]interface{}, error) {
	name, err := proto.GetRoCEInitiator()
	if err != nil {
		log.Errorf("Get RoCE initiator name error: %v", name)
		return nil, err
	}

	initiator, err := p.cli.GetRoCEInitiator(name)
	if err != nil {
		log.Errorf("Get RoCE initiator %s error: %v", name, err)
		return nil, err
	}

	if initiator == nil {
		initiator, err = p.cli.AddRoCEInitiator(name)
		if err != nil {
			log.Errorf("Add initiator %s error: %v", name, err)
			return nil, err
		}
	}

	isFree, freeExist := initiator["ISFREE"].(string)
	parent, parentExist := initiator["PARENTID"].(string)

	if freeExist && isFree == "true" {
		err := p.cli.AddRoCEInitiatorToHost(name, hostID)
		if err != nil {
			log.Errorf("Add RoCE initiator %s to host %s error: %v", name, hostID, err)
			return nil, err
		}
	} else if parentExist && parent != hostID {
		msg := fmt.Sprintf("RoCE initiator %s is already associated to another host %s", name, parent)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	return initiator, nil
}

func (p *Attacher) doMapping(hostID, lunName string) (string, error) {
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

	lunUniqueId, err := utils.GetLunUniqueId(p.protocol, lun)
	if err != nil {
		return "", err
	}
	return lunUniqueId, nil
}

func (p *Attacher) doUnmapping(hostID, lunName string) (string, error) {
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

	lunUniqueId, err := utils.GetLunUniqueId(p.protocol, lun)
	if err != nil {
		return "", err
	}
	return lunUniqueId, nil
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

	return disConnectVolume(wwn, p.protocol)
}

func (p *Attacher) ControllerDetach(lunName string, parameters map[string]interface{}) (string, error) {
	host, err := p.getHost(parameters, false)
	if err != nil {
		log.Infof("Get host ID error: %v", err)
		return "", err
	}
	if host == nil {
		log.Infof("Host doesn't exist while detaching %s", lunName)
		return "", nil
	}

	hostID := host["ID"].(string)
	wwn, err := p.doUnmapping(hostID, lunName)
	if err != nil {
		log.Errorf("Unmapping LUN %s from host %s error: %v", lunName, hostID, err)
		return "", err
	}

	return wwn, nil
}
