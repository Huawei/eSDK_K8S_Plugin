package attacher

import (
	"errors"

	"connector"
	"utils"
	"utils/log"
)

type MetroAttacher struct {
	localAttacher  AttacherPlugin
	remoteAttacher AttacherPlugin
	protocol       string
}

func NewMetroAttacher(localAttacher, remoteAttacher AttacherPlugin, protocol string) *MetroAttacher {
	return &MetroAttacher{
		localAttacher:  localAttacher,
		remoteAttacher: remoteAttacher,
		protocol:       protocol,
	}
}

func (p *MetroAttacher) NodeStage(lunName string, parameters map[string]interface{}) (*connector.ConnectInfo, error) {
	return connectVolume(p, lunName, p.protocol, parameters)
}

// NodeUnstage to get the lun unique ID for disconnect volume
func (p *MetroAttacher) NodeUnstage(lunName string, parameters map[string]interface{}) (
	*connector.DisConnectInfo, error) {
	lun, err := p.getLunInfo(lunName)
	if lun == nil {
		return nil, err
	}

	lunUniqueID, err := utils.GetLunUniqueId(p.protocol, lun)
	if err != nil {
		return nil, err
	}

	return disConnectVolume(lunUniqueID, p.protocol)
}

func (p *MetroAttacher) mergeMappingInfo(localMapping, remoteMapping map[string]interface{}) (
	map[string]interface{}, error) {
	if localMapping == nil && remoteMapping == nil {
		msg := "both storage site of HyperMetro are failed"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if localMapping == nil {
		localMapping = remoteMapping
	} else if remoteMapping != nil {
		if p.protocol == "iscsi" {
			localMapping["tgtPortals"] = append(localMapping["tgtPortals"].([]string),
				remoteMapping["tgtPortals"].([]string)...)
			localMapping["tgtIQNs"] = append(localMapping["tgtIQNs"].([]string),
				remoteMapping["tgtIQNs"].([]string)...)
			localMapping["tgtHostLUNs"] = append(localMapping["tgtHostLUNs"].([]string),
				remoteMapping["tgtHostLUNs"].([]string)...)
		} else if p.protocol == "fc" {
			localMapping["tgtWWNs"] = append(localMapping["tgtWWNs"].([]string),
				remoteMapping["tgtWWNs"].([]string)...)
			localMapping["tgtHostLUNs"] = append(localMapping["tgtHostLUNs"].([]string),
				remoteMapping["tgtHostLUNs"].([]string)...)
		}
	}

	return localMapping, nil
}

func (p *MetroAttacher) ControllerAttach(lunName string, parameters map[string]interface{}) (map[string]interface{}, error) {
	remoteMapping, err := p.remoteAttacher.ControllerAttach(lunName, parameters)
	if err != nil {
		log.Errorf("Attach hypermetro remote volume %s error: %v", lunName, err)
		return nil, err
	}

	localMapping, err := p.localAttacher.ControllerAttach(lunName, parameters)
	if err != nil {
		log.Errorf("Attach hypermetro local volume %s error: %v", lunName, err)
		return nil, err
	}

	return p.mergeMappingInfo(localMapping, remoteMapping)
}

func (p *MetroAttacher) ControllerDetach(lunName string, parameters map[string]interface{}) (string, error) {
	rmtLunWWN, err := p.remoteAttacher.ControllerDetach(lunName, parameters)
	if err != nil {
		log.Errorf("Detach hypermetro remote volume %s error: %v", lunName, err)
		return "", err
	}

	locLunWWN, err := p.localAttacher.ControllerDetach(lunName, parameters)
	if err != nil {
		log.Errorf("Detach hypermetro local volume %s error: %v", lunName, err)
		return "", err
	}

	return p.mergeLunWWN(locLunWWN, rmtLunWWN)
}

func (p *MetroAttacher) mergeLunWWN(locLunWWN, rmtLunWWN string) (string, error) {
	if rmtLunWWN == "" && locLunWWN == "" {
		log.Infoln("both storage site of HyperMetro are failed to get lun WWN")
		return "", nil
	}

	if locLunWWN == "" {
		locLunWWN = rmtLunWWN
	}
	return locLunWWN, nil
}

func (p *MetroAttacher) getTargetRoCEPortals() ([]string, error) {
	var availablePortals []string
	localPortals, err := p.localAttacher.getTargetRoCEPortals()
	if err != nil {
		log.Warningf("Get local roce portals error: %v", err)
	}
	availablePortals = append(availablePortals, localPortals...)

	remotePortals, err := p.remoteAttacher.getTargetRoCEPortals()
	if err != nil {
		log.Warningf("Get remote roce portals error: %v", err)
	}
	availablePortals = append(availablePortals, remotePortals...)

	return availablePortals, nil
}

func (p *MetroAttacher) getLunInfo(lunName string) (map[string]interface{}, error) {
	rmtLun, err := p.remoteAttacher.getLunInfo(lunName)
	if err != nil {
		log.Warningf("Get hyperMetro remote volume %s error: %v", lunName, err)
	}

	locLun, err := p.localAttacher.getLunInfo(lunName)
	if err != nil {
		log.Warningf("Get hyperMetro local volume %s error: %v", lunName, err)
	}
	return p.mergeLunInfo(locLun, rmtLun)
}

func (p *MetroAttacher) mergeLunInfo(locLun, rmtLun map[string]interface{}) (map[string]interface{}, error) {
	if rmtLun == nil && locLun == nil {
		msg := "both storage site of HyperMetro are failed to get lun info"
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if locLun == nil {
		locLun = rmtLun
	}
	return locLun, nil
}
