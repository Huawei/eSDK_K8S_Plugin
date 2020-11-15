package attacher

import (
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

func (p *MetroAttacher) NodeStage(lunName string, parameters map[string]interface{}) (string, error) {
	return connectVolume(p, lunName, p.protocol, parameters)
}

func (p *MetroAttacher) NodeUnstage(lunName string, parameters map[string]interface{}) error {
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

func (p *MetroAttacher) ControllerAttach(lunName string, parameters map[string]interface{}) (string, error) {
	_, err := p.remoteAttacher.ControllerAttach(lunName, parameters)
	if err != nil {
		log.Errorf("Attach hypermetro remote volume %s error: %v", lunName, err)
		return "", err
	}

	lunWWN, err := p.localAttacher.ControllerAttach(lunName, parameters)
	if err != nil {
		log.Errorf("Attach hypermetro local volume %s error: %v", lunName, err)
		p.remoteAttacher.ControllerDetach(lunName, parameters)
		return "", err
	}

	return lunWWN, nil
}

func (p *MetroAttacher) ControllerDetach(lunName string, parameters map[string]interface{}) (string, error) {
	_, err := p.remoteAttacher.ControllerDetach(lunName, parameters)
	if err != nil {
		log.Errorf("Detach hypermetro remote volume %s error: %v", lunName, err)
		return "", err
	}

	lunWWN, err := p.localAttacher.ControllerDetach(lunName, parameters)
	if err != nil {
		log.Errorf("Detach hypermetro local volume %s error: %v", lunName, err)
		return "", err
	}

	return lunWWN, nil
}

func (p *MetroAttacher) getTargetISCSIPortals() ([]string, error) {
	var availablePortals []string
	localPortals, err := p.localAttacher.getTargetISCSIPortals()
	if err != nil {
		return nil, err
	}
	availablePortals = append(availablePortals, localPortals...)

	remotePortals, err := p.remoteAttacher.getTargetISCSIPortals()
	if err != nil {
		return nil, err
	}
	availablePortals = append(availablePortals, remotePortals...)

	return availablePortals, nil
}

func (p *MetroAttacher) getTargetRoCEPortals() ([]string, error) {
	var availablePortals []string
	localPortals, err := p.localAttacher.getTargetRoCEPortals()
	if err != nil {
		return nil, err
	}
	availablePortals = append(availablePortals, localPortals...)

	remotePortals, err := p.remoteAttacher.getTargetRoCEPortals()
	if err != nil {
		return nil, err
	}
	availablePortals = append(availablePortals, remotePortals...)

	return availablePortals, nil
}
