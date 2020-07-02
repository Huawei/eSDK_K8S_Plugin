package attacher

import (
	"dev"
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
	wwn, err := p.ControllerAttach(lunName, parameters)
	if err != nil {
		return "", err
	}

	devPath, err := dev.GetDevPath(wwn, p.protocol)
	if err != nil {
		return "", err
	}

	return devPath, nil
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

	err = dev.DeleteDev(wwn)
	if err != nil {
		log.Errorf("Delete dev %s error: %v", wwn, err)
		return err
	}

	return err
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
