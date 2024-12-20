/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base/attacher"
	oceandisk "github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// VolumeAttacherPlugin defines interfaces of attach operations
type VolumeAttacherPlugin interface {
	ControllerAttach(context.Context, string, map[string]interface{}) (map[string]interface{}, error)
	ControllerDetach(context.Context, string, map[string]interface{}) (string, error)
}

// OceandiskAttacher implements interface VolumeAttacherPlugin
type OceandiskAttacher struct {
	*attacher.AttachmentManager
	Cli oceandisk.OceandiskClientInterface
}

// OceanDiskAttacherConfig defines the configurations of OceandiskAttacher
type OceanDiskAttacherConfig struct {
	Cli      oceandisk.OceandiskClientInterface
	Protocol string
	Invoker  string
	Portals  []string
	Alua     map[string]interface{}
}

// NewOceanDiskAttacher provides a new oceandisk attacher
func NewOceanDiskAttacher(config OceanDiskAttacherConfig) *OceandiskAttacher {
	baseAttacherConfig := attacher.AttachmentManagerConfig{
		Cli:      config.Cli,
		Protocol: config.Protocol,
		Invoker:  config.Invoker,
		Portals:  config.Portals,
		Alua:     config.Alua,
	}
	baseAttacher := attacher.NewAttachmentManager(baseAttacherConfig)

	return &OceandiskAttacher{
		AttachmentManager: baseAttacher,
		Cli:               config.Cli,
	}
}

// ControllerAttach attaches volume and maps namespace to host
func (p *OceandiskAttacher) ControllerAttach(ctx context.Context, namespaceName string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	host, err := p.GetHost(ctx, parameters, true)
	if err != nil {
		log.AddContext(ctx).Errorf("get host ID error: %v", err)
		return nil, err
	}

	hostID, ok := utils.GetValue[string](host, "ID")
	if !ok {
		return nil, errors.New("convert host[\"ID\"] to string failed")
	}
	hostName, ok := utils.GetValue[string](host, "NAME")
	if !ok {
		return nil, errors.New("convert host[\"NAME\"] to string failed")
	}

	hostAlua := utils.GetAlua(ctx, p.Alua, hostName)

	if hostAlua != nil && p.needUpdateHost(host, hostAlua) {
		err := p.Cli.UpdateHost(ctx, hostID, hostAlua)
		if err != nil {
			log.AddContext(ctx).Errorf("update host %s error: %v", hostID, err)
			return nil, err
		}
	}

	if p.Protocol == "iscsi" {
		_, err = p.AttachISCSI(ctx, hostID, parameters)
	} else if p.Protocol == "fc" || p.Protocol == "fc-nvme" {
		_, err = p.AttachFC(ctx, hostID, parameters)
	} else if p.Protocol == "roce" {
		_, err = p.AttachRoCE(ctx, hostID, parameters)
	}

	if err != nil {
		log.AddContext(ctx).Errorf("attach %s connection error: %v", p.Protocol, err)
		return nil, err
	}

	wwn, hostNamespaceId, err := p.doMapping(ctx, hostID, namespaceName)
	if err != nil {
		log.AddContext(ctx).Errorf("mapping Namespace %s to host %s error: %v", namespaceName, hostID, err)
		return nil, err
	}

	return p.GetMappingProperties(ctx, wwn, hostNamespaceId, parameters)
}

func (p *OceandiskAttacher) needUpdateHost(host map[string]interface{}, hostAlua map[string]interface{}) bool {
	accessMode, ok := hostAlua["accessMode"]
	if !ok {
		return false
	}

	if accessMode != host["accessMode"] {
		return true
	}

	return false
}

func (p *OceandiskAttacher) doMapping(ctx context.Context, hostID, namespaceName string) (string, string, error) {
	namespace, err := p.Cli.GetNamespaceByName(ctx, namespaceName)
	if err != nil {
		return "", "", err
	}
	if len(namespace) == 0 {
		return "", "", fmt.Errorf("namespace %s not exist for attaching", namespaceName)
	}

	namespaceID, ok := utils.GetValue[string](namespace, "ID")
	if !ok {
		return "", "", fmt.Errorf("convert namespaceID to string failed, data: %v", namespace["ID"])
	}
	mappingID, err := p.CreateMapping(ctx, hostID)
	if err != nil {
		return "", "", fmt.Errorf("create mapping for host %s error: %v", hostID, err)
	}

	err = p.CreateHostGroup(ctx, hostID, mappingID)
	if err != nil {
		return "", "", fmt.Errorf("create host group for host %s error: %v", hostID, err)
	}

	err = p.createNamespaceGroup(ctx, namespaceID, hostID, mappingID)
	if err != nil {
		return "", "", fmt.Errorf("create namespace group for host %s error: %v", hostID, err)
	}

	namespaceUniqueId, err := utils.GetLunUniqueId(ctx, p.Protocol, namespace)
	if err != nil {
		return "", "", err
	}

	hostNamespaceId, err := p.Cli.GetHostNamespaceId(ctx, hostID, namespaceID)
	if err != nil {
		return "", "", err
	}

	return namespaceUniqueId, hostNamespaceId, nil
}

func (p *OceandiskAttacher) createNamespaceGroup(ctx context.Context, namespaceID, hostID, mappingID string) error {
	var err error
	var namespaceGroup map[string]interface{}

	namespaceGroupsByNamespaceID, err := p.Cli.QueryAssociateNamespaceGroup(ctx,
		oceandisk.AssociateObjTypeNamespace, namespaceID)
	if err != nil {
		return fmt.Errorf("query associated namespace groups of namespace %s error: %v", namespaceID, err)
	}

	namespaceGroupName := p.getNamespaceGroupName(hostID)
	for _, i := range namespaceGroupsByNamespaceID {
		group, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert group to map failed, data: %v", i)
			continue
		}
		if group["NAME"].(string) == namespaceGroupName {
			namespaceGroupID, ok := utils.GetValue[string](group, "ID")
			if !ok {
				return errors.New("convert group[\"ID\"] to string failed")
			}
			return p.addToNamespaceGroupMapping(ctx, namespaceGroupName, namespaceGroupID, mappingID)
		}
	}

	namespaceGroup, err = p.Cli.GetNamespaceGroupByName(ctx, namespaceGroupName)
	if err != nil {
		return fmt.Errorf("get namespacegroup by name %s error: %v", namespaceGroupName, err)
	}
	if len(namespaceGroup) == 0 {
		namespaceGroup, err = p.Cli.CreateNamespaceGroup(ctx, namespaceGroupName)
		if err != nil {
			log.AddContext(ctx).Errorf("create namespacegroup %s error: %v", namespaceGroupName, err)
			return err
		}
	}

	namespaceGroupID, ok := utils.GetValue[string](namespaceGroup, "ID")
	if !ok {
		return errors.New("createNamespaceGroup failed, caused by not found namespace group id")
	}
	err = p.Cli.AddNamespaceToGroup(ctx, namespaceID, namespaceGroupID)
	if err != nil {
		return fmt.Errorf("add namespace %s to group %s error: %v", namespaceID, namespaceGroupID, err)
	}

	return p.addToNamespaceGroupMapping(ctx, namespaceGroupName, namespaceGroupID, mappingID)
}

// getNamespaceGroupName generates namespace group name
func (p *OceandiskAttacher) getNamespaceGroupName(postfix string) string {
	return fmt.Sprintf("k8s_%s_namespacegroup_%s", p.Invoker, postfix)
}

func (p *OceandiskAttacher) addToNamespaceGroupMapping(ctx context.Context,
	groupName, groupID, mappingID string) error {
	namespaceGroupsByMappingID, err := p.Cli.QueryAssociateNamespaceGroup(ctx,
		base.AssociateObjTypeMapping, mappingID)
	if err != nil {
		return fmt.Errorf("query associated namespace groups of mapping %s error: %v", mappingID, err)
	}

	for _, i := range namespaceGroupsByMappingID {
		group, ok := i.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid group type. Expected 'map[string]interface{}', found %T", i)
		}
		if group["NAME"].(string) == groupName {
			return nil
		}
	}

	err = p.Cli.AddGroupToMapping(ctx, oceandisk.AssociateObjTypeNamespaceGroup, groupID, mappingID)
	if err != nil {
		return fmt.Errorf("add namespace group %s to mapping %s error: %v", groupID, mappingID, err)
	}

	return nil
}

// ControllerDetach detaches volume and unmaps namespace from host
func (p *OceandiskAttacher) ControllerDetach(ctx context.Context,
	namespaceName string, parameters map[string]interface{}) (string, error) {
	host, err := p.GetHost(ctx, parameters, false)
	if err != nil {
		return "", fmt.Errorf("get host ID error: %v", err)
	}
	if host == nil {
		log.AddContext(ctx).Infof("host doesn't exist while detaching %s", namespaceName)
		return "", nil
	}

	hostID, ok := utils.GetValue[string](host, "ID")
	if !ok {
		return "", fmt.Errorf("convert hostID to string failed, data: %v", host["ID"])
	}

	wwn, err := p.doUnmapping(ctx, hostID, namespaceName)
	if err != nil {
		return "", err
	}

	return wwn, nil
}

func (p *OceandiskAttacher) doUnmapping(ctx context.Context, hostID, namespaceName string) (string, error) {
	namespace, err := p.Cli.GetNamespaceByName(ctx, namespaceName)
	if err != nil {
		return "", fmt.Errorf("get namespace %s info error: %v", namespaceName, err)
	}
	if len(namespace) == 0 {
		log.AddContext(ctx).Infof("namespace %s doesn't exist while detaching", namespaceName)
		return "", nil
	}
	namespaceID, ok := utils.GetValue[string](namespace, "ID")
	if !ok {
		return "", fmt.Errorf("convert namespaceID to string failed, data: %v", namespace["ID"])
	}
	namespaceGroupsByNamespaceID, err := p.Cli.QueryAssociateNamespaceGroup(ctx,
		oceandisk.AssociateObjTypeNamespace, namespaceID)
	if err != nil {
		return "", fmt.Errorf("query associated namespacegroups of namespace %s error: %v", namespaceID, err)
	}

	namespaceGroupName := p.getNamespaceGroupName(hostID)
	for _, i := range namespaceGroupsByNamespaceID {
		group, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert group to map failed, data: %v", i)
			continue
		}
		if group["NAME"].(string) == namespaceGroupName {
			namespaceGroupID, ok := utils.GetValue[string](group, "ID")
			if !ok {
				return "", fmt.Errorf("convert namespaceGroupID to string failed, data: %v", group["ID"])
			}
			err = p.Cli.RemoveNamespaceFromGroup(ctx, namespaceID, namespaceGroupID)
			if err != nil {
				return "", fmt.Errorf("remove namespace %s from group %s error: %v",
					namespaceID, namespaceGroupID, err)
			}
		}
	}

	namespaceUniqueId, err := utils.GetLunUniqueId(ctx, p.Protocol, namespace)
	if err != nil {
		return "", fmt.Errorf("unmapping Namespace %s from host %s error: %v", namespaceName, hostID, err)
	}
	return namespaceUniqueId, nil
}
