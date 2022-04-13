package attacher

import (
	"context"
	"errors"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
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

// NodeStage to do storage mapping and get the connector
func (p *MetroAttacher) NodeStage(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (*connector.ConnectInfo, error) {
	return connectVolume(ctx, p, lunName, p.protocol, parameters)
}

// NodeUnstage to get the lun unique ID for disconnect volume
func (p *MetroAttacher) NodeUnstage(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (*connector.DisConnectInfo, error) {
	lun, err := p.getLunInfo(ctx, lunName)
	if lun == nil {
		return nil, err
	}

	lunUniqueID, err := utils.GetLunUniqueId(ctx, p.protocol, lun)
	if err != nil {
		return nil, err
	}

	return disConnectVolume(ctx, lunUniqueID, p.protocol)
}

func (p *MetroAttacher) mergeMappingInfo(ctx context.Context,
	localMapping, remoteMapping map[string]interface{}) (
	map[string]interface{}, error) {
	if localMapping == nil && remoteMapping == nil {
		msg := "both storage site of HyperMetro are failed"
		log.AddContext(ctx).Errorln(msg)
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

func (p *MetroAttacher) ControllerAttach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	remoteMapping, err := p.remoteAttacher.ControllerAttach(ctx, lunName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Attach hypermetro remote volume %s error: %v", lunName, err)
		return nil, err
	}

	localMapping, err := p.localAttacher.ControllerAttach(ctx, lunName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Attach hypermetro local volume %s error: %v", lunName, err)
		return nil, err
	}

	return p.mergeMappingInfo(ctx, localMapping, remoteMapping)
}

func (p *MetroAttacher) ControllerDetach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (string, error) {
	rmtLunWWN, err := p.remoteAttacher.ControllerDetach(ctx, lunName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Detach hypermetro remote volume %s error: %v", lunName, err)
		return "", err
	}

	locLunWWN, err := p.localAttacher.ControllerDetach(ctx, lunName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Detach hypermetro local volume %s error: %v", lunName, err)
		return "", err
	}

	return p.mergeLunWWN(ctx, locLunWWN, rmtLunWWN)
}

func (p *MetroAttacher) mergeLunWWN(ctx context.Context, locLunWWN, rmtLunWWN string) (string, error) {
	if rmtLunWWN == "" && locLunWWN == "" {
		log.AddContext(ctx).Infoln("both storage site of HyperMetro are failed to get lun WWN")
		return "", nil
	}

	if locLunWWN == "" {
		locLunWWN = rmtLunWWN
	}
	return locLunWWN, nil
}

func (p *MetroAttacher) getTargetRoCEPortals(ctx context.Context) ([]string, error) {
	var availablePortals []string
	localPortals, err := p.localAttacher.getTargetRoCEPortals(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("Get local roce portals error: %v", err)
	}
	availablePortals = append(availablePortals, localPortals...)

	remotePortals, err := p.remoteAttacher.getTargetRoCEPortals(ctx)
	if err != nil {
		log.AddContext(ctx).Warningf("Get remote roce portals error: %v", err)
	}
	availablePortals = append(availablePortals, remotePortals...)

	return availablePortals, nil
}

func (p *MetroAttacher) getLunInfo(ctx context.Context, lunName string) (map[string]interface{}, error) {
	rmtLun, err := p.remoteAttacher.getLunInfo(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Warningf("Get hyperMetro remote volume %s error: %v", lunName, err)
	}

	locLun, err := p.localAttacher.getLunInfo(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Warningf("Get hyperMetro local volume %s error: %v", lunName, err)
	}
	return p.mergeLunInfo(ctx, locLun, rmtLun)
}

func (p *MetroAttacher) mergeLunInfo(ctx context.Context,
	locLun, rmtLun map[string]interface{}) (map[string]interface{}, error) {
	if rmtLun == nil && locLun == nil {
		msg := "both storage site of HyperMetro are failed to get lun info"
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	if locLun == nil {
		locLun = rmtLun
	}
	return locLun, nil
}
