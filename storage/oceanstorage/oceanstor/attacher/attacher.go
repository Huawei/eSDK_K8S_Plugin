/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

// Package attacher provide operations of volume attach
package attacher

import (
	"context"
	"errors"
	"fmt"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base/attacher"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// VolumeAttacherPlugin defines interfaces of attach operations
type VolumeAttacherPlugin interface {
	ControllerAttach(context.Context, string, map[string]interface{}) (map[string]interface{}, error)
	ControllerDetach(context.Context, string, map[string]interface{}) (string, error)
	GetTargetRoCEPortals(context.Context) ([]string, error)
	getLunInfo(context.Context, string) (map[string]interface{}, error)
}

// VolumeAttacher defines attacher to attach volume
type VolumeAttacher struct {
	*attacher.AttachmentManager
	Cli client.OceanstorClientInterface
}

// VolumeAttacherConfig defines the configurations of VolumeAttacher
type VolumeAttacherConfig struct {
	Product  constants.OceanstorVersion
	Cli      client.OceanstorClientInterface
	Protocol string
	Invoker  string
	Portals  []string
	Alua     map[string]interface{}
}

// NewAttacher init a new attacher
func NewAttacher(config VolumeAttacherConfig) VolumeAttacherPlugin {
	if config.Product.IsDoradoV6OrV7() {
		return newDoradoV6OrV7Attacher(config)
	}
	return newOceanStorAttacher(config)
}

// getLunGroupName generates lun group name
func (p *VolumeAttacher) getLunGroupName(postfix string) string {
	return fmt.Sprintf("k8s_%s_lungroup_%s", p.Invoker, postfix)
}

func (p *VolumeAttacher) createLunGroup(ctx context.Context, lunID, hostID, mappingID string) error {
	var err error
	var lunGroup map[string]interface{}

	lunGroupsByLunID, err := p.Cli.QueryAssociateLunGroup(ctx, base.AssociateObjTypeLUN, lunID)
	if err != nil {
		log.AddContext(ctx).Errorf("Query associated lun groups of lun %s error: %v", lunID, err)
		return err
	}

	lunGroupName := p.getLunGroupName(hostID)
	for _, i := range lunGroupsByLunID {
		group, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert group to map failed, data: %v", i)
			continue
		}
		if group["NAME"].(string) == lunGroupName {
			lunGroupID, ok := group["ID"].(string)
			if !ok {
				return errors.New("convert group[\"ID\"] to string failed")
			}
			return p.addToLUNGroupMapping(ctx, lunGroupName, lunGroupID, mappingID)
		}
	}

	lunGroup, err = p.Cli.GetLunGroupByName(ctx, lunGroupName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lungroup by name %s error: %v", lunGroupName, err)
		return err
	}
	if lunGroup == nil {
		lunGroup, err = p.Cli.CreateLunGroup(ctx, lunGroupName)
		if err != nil {
			log.AddContext(ctx).Errorf("Create lungroup %s error: %v", lunGroupName, err)
			return err
		}
	}

	lunGroupID, ok := lunGroup["ID"].(string)
	if !ok {
		return errors.New("createLunGroup failed, caused by not found lun group id")
	}
	err = p.Cli.AddLunToGroup(ctx, lunID, lunGroupID)
	if err != nil {
		log.AddContext(ctx).Errorf("Add lun %s to group %s error: %v", lunID, lunGroupID, err)
		return err
	}

	return p.addToLUNGroupMapping(ctx, lunGroupName, lunGroupID, mappingID)
}

func (p *VolumeAttacher) addToLUNGroupMapping(ctx context.Context, groupName, groupID, mappingID string) error {
	lunGroupsByMappingID, err := p.Cli.QueryAssociateLunGroup(ctx, base.AssociateObjTypeMapping, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Query associated lun groups of mapping %s error: %v", mappingID, err)
		return err
	}

	for _, i := range lunGroupsByMappingID {
		group, ok := i.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid group type. Expected 'map[string]interface{}', found %T", i)
		}
		if group["NAME"].(string) == groupName {
			return nil
		}
	}

	err = p.Cli.AddGroupToMapping(ctx, base.AssociateObjTypeLUNGroup, groupID, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Add lun group %s to mapping %s error: %v",
			groupID, mappingID, err)
		return err
	}

	return nil
}

func (p *VolumeAttacher) needUpdateInitiatorAlua(initiator map[string]interface{}) bool {
	if p.Alua == nil {
		return false
	}

	multiPathType, ok := p.Alua["MULTIPATHTYPE"]
	if !ok {
		return false
	}

	if multiPathType != initiator["MULTIPATHTYPE"] {
		return true
	} else if initiator["MULTIPATHTYPE"] == MultiPathTypeDefault {
		return false
	}

	failoverMode, ok := p.Alua["FAILOVERMODE"]
	if ok && failoverMode != initiator["FAILOVERMODE"] {
		return true
	}

	specialModeType, ok := p.Alua["SPECIALMODETYPE"]
	if ok && specialModeType != initiator["SPECIALMODETYPE"] {
		return true
	}

	pathType, ok := p.Alua["PATHTYPE"]
	if ok && pathType != initiator["PATHTYPE"] {
		return true
	}

	return false
}

func (p *VolumeAttacher) doMapping(ctx context.Context, hostID, lunName string) (string, string, error) {
	lun, err := p.Cli.GetLunByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s error: %v", lunName, err)
		return "", "", err
	}
	if lun == nil {
		msg := fmt.Sprintf("Lun %s not exist for attaching", lunName)
		log.AddContext(ctx).Errorln(msg)
		return "", "", errors.New(msg)
	}

	lunID, ok := lun["ID"].(string)
	if !ok {
		return "", "", pkgUtils.Errorf(ctx, "convert lunID to string failed, data: %v", lun["ID"])
	}
	mappingID, err := p.CreateMapping(ctx, hostID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create mapping for host %s error: %v", hostID, err)
		return "", "", err
	}

	err = p.CreateHostGroup(ctx, hostID, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create host group for host %s error: %v", hostID, err)
		return "", "", err
	}

	err = p.createLunGroup(ctx, lunID, hostID, mappingID)
	if err != nil {
		log.AddContext(ctx).Errorf("Create lun group for host %s error: %v", hostID, err)
		return "", "", err
	}

	lunUniqueId, err := utils.GetLunUniqueId(ctx, p.Protocol, lun)
	if err != nil {
		return "", "", err
	}

	hostLunId, err := p.Cli.GetHostLunId(ctx, hostID, lunID)
	if err != nil {
		return "", "", err
	}

	return lunUniqueId, hostLunId, nil
}

func (p *VolumeAttacher) doUnmapping(ctx context.Context, hostID, lunName string) (string, error) {
	lun, err := p.Cli.GetLunByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s info error: %v", lunName, err)
		return "", err
	}
	if lun == nil {
		log.AddContext(ctx).Infof("LUN %s doesn't exist while detaching", lunName)
		return "", nil
	}
	lunID, ok := lun["ID"].(string)
	if !ok {
		return "", pkgUtils.Errorf(ctx, "convert lunID to string failed, data: %v", lun["ID"])
	}
	lunGroupsByLunID, err := p.Cli.QueryAssociateLunGroup(ctx, base.AssociateObjTypeLUN, lunID)
	if err != nil {
		log.AddContext(ctx).Errorf("Query associated lungroups of lun %s error: %v", lunID, err)
		return "", err
	}

	lunGroupName := p.getLunGroupName(hostID)
	for _, i := range lunGroupsByLunID {
		group, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert group to map failed, data: %v", i)
			continue
		}
		if group["NAME"].(string) == lunGroupName {
			lunGroupID, ok := group["ID"].(string)
			if !ok {
				return "", pkgUtils.Errorf(ctx, "convert lunGroupID to string failed, data: %v", group["ID"])
			}
			err = p.Cli.RemoveLunFromGroup(ctx, lunID, lunGroupID)
			if err != nil {
				log.AddContext(ctx).Errorf("Remove lun %s from group %s error: %v",
					lunID, lunGroupID, err)
				return "", err
			}
		}
	}

	lunUniqueId, err := utils.GetLunUniqueId(ctx, p.Protocol, lun)
	if err != nil {
		return "", err
	}
	return lunUniqueId, nil
}

// ControllerDetach detaches volume and unmaps lun from host
func (p *VolumeAttacher) ControllerDetach(ctx context.Context,
	lunName string,
	parameters map[string]interface{}) (string, error) {
	host, err := p.GetHost(ctx, parameters, false)
	if err != nil {
		log.AddContext(ctx).Infof("Get host ID error: %v", err)
		return "", err
	}
	if host == nil {
		log.AddContext(ctx).Infof("Host doesn't exist while detaching %s", lunName)
		return "", nil
	}

	hostID, ok := host["ID"].(string)
	if !ok {
		return "", pkgUtils.Errorf(ctx, "convert hostID to string failed, data: %v", host["ID"])
	}
	wwn, err := p.doUnmapping(ctx, hostID, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmapping LUN %s from host %s error: %v", lunName, hostID, err)
		return "", err
	}

	return wwn, nil
}

func (p *VolumeAttacher) getLunInfo(ctx context.Context, lunName string) (map[string]interface{}, error) {
	lun, err := p.Cli.GetLunByName(ctx, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s info error: %v", lunName, err)
		return nil, err
	}
	if lun == nil {
		log.AddContext(ctx).Infof("LUN %s doesn't exist while detaching", lunName)
		return nil, nil
	}
	return lun, nil
}
