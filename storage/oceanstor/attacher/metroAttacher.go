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

package attacher

import (
	"context"
	"errors"

	"huawei-csi-driver/utils/log"
)

// MetroAttacher implements interface AttacherPlugin
type MetroAttacher struct {
	localAttacher  AttacherPlugin
	remoteAttacher AttacherPlugin
	protocol       string
}

// NewMetroAttacher inits a new metro attacher
func NewMetroAttacher(localAttacher, remoteAttacher AttacherPlugin, protocol string) *MetroAttacher {
	return &MetroAttacher{
		localAttacher:  localAttacher,
		remoteAttacher: remoteAttacher,
		protocol:       protocol,
	}
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

// ControllerAttach attaches local and remote volume
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

// ControllerDetach detaches local and remote volume
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
