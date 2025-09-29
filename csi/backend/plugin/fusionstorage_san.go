/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

// Package plugin provide storage function
package plugin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/proto"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/attacher"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// FusionStorageSanPlugin implements storage StoragePlugin interface
type FusionStorageSanPlugin struct {
	FusionStoragePlugin
	hosts      map[string]string
	protocol   string
	portals    []string
	iscsiLinks int
	alua       map[string]interface{}

	storageOnline bool
	clientCount   int
	clientMutex   sync.Mutex
}

func init() {
	RegPlugin("fusionstorage-san", &FusionStorageSanPlugin{})
}

// NewPlugin used to create new plugin
func (p *FusionStorageSanPlugin) NewPlugin() StoragePlugin {
	return &FusionStorageSanPlugin{
		hosts: make(map[string]string),
	}
}

// Init used to init the plugin
func (p *FusionStorageSanPlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	protocol, ok := utils.GetValue[string](parameters, "protocol")
	if !ok {
		return fmt.Errorf("protocol must be configured in parameters %v", parameters)
	}

	if !slices.Contains(constants.FusionstorageProtocol, protocol) {
		return fmt.Errorf("protocol %s configured is invalid. Just support iscsi and scsi", protocol)
	}

	portals, _ := utils.GetValue[[]interface{}](parameters, "portals")
	if protocol == constants.ProtocolScsi {
		err := p.fillScsiParams(portals, parameters)
		if err != nil {
			return err
		}
	}

	if protocol == constants.ProtocolIscsi {
		err := p.fillIscsiParams(ctx, portals, parameters)
		if err != nil {
			return err
		}
	}

	err := p.init(ctx, config, keepLogin)
	if err != nil {
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) fillIscsiParams(ctx context.Context,
	portals []interface{}, parameters map[string]interface{}) error {
	var err error
	var validPortals []string
	if len(portals) > 0 {
		validPortals, err = proto.VerifyIscsiPortals(ctx, portals)
		if err != nil {
			return err
		}
	} else {
		iscsiLinksStr, ok := utils.GetValue[string](parameters, "iscsiLinks")
		if !ok {
			return fmt.Errorf("one of portals or iscsiLinks must be provided in parameters %v", parameters)
		}

		iscsiLinks, err := strconv.Atoi(iscsiLinksStr)
		if err != nil {
			return fmt.Errorf("iscsiLinks %s can not convert to int", iscsiLinksStr)
		}

		if iscsiLinks <= 0 {
			return fmt.Errorf("iscsiLinks must be greater than zero, now is %d", iscsiLinks)
		}

		p.iscsiLinks = iscsiLinks
	}

	p.portals = validPortals
	p.protocol = constants.ProtocolIscsi
	p.alua, _ = utils.GetValue[map[string]interface{}](parameters, "ALUA")
	return nil
}

func (p *FusionStorageSanPlugin) fillScsiParams(portals []interface{}, parameters map[string]interface{}) error {
	if len(portals) == 0 {
		return fmt.Errorf("portals must be configured in parameters %v while protocol is scsi", parameters)
	}

	scsi, ok := portals[0].(map[string]interface{})
	if !ok {
		return errors.New("scsi portals convert to map[string]interface{} failed")
	}
	for k, v := range scsi {
		manageIP, ok := v.(string)
		if !ok {
			continue
		}
		ip := net.ParseIP(manageIP)
		if ip == nil {
			return fmt.Errorf("manage IP %s of host %s is invalid", manageIP, k)
		}

		p.hosts[k] = manageIP
	}

	p.protocol = constants.ProtocolScsi
	return nil
}

func (p *FusionStorageSanPlugin) getParams(name string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":        name,
		"description": parameters["description"].(string),
		"capacity":    utils.RoundUpSize(parameters["size"].(int64), CapacityUnit),
	}

	paramKeys := []string{
		"storagepool",
		"cloneFrom",
		"sourceSnapshotName",
		"sourceVolumeName",
		"snapshotParentId",
		"qos",
	}

	for _, key := range paramKeys {
		if v, exist := parameters[key].(string); exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	return params, nil
}

// CreateVolume used to create volume
func (p *FusionStorageSanPlugin) CreateVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	params, err := p.getParams(name, parameters)
	if err != nil {
		return nil, err
	}

	san := volume.NewSAN(p.cli)
	volObj, err := san.Create(ctx, params)
	if err != nil {
		return nil, err
	}

	return volObj, nil
}

// QueryVolume used to query volume
func (p *FusionStorageSanPlugin) QueryVolume(ctx context.Context, name string, params map[string]interface{}) (
	utils.Volume, error) {
	san := volume.NewSAN(p.cli)
	return san.Query(ctx, name)
}

// DeleteVolume used to delete volume
func (p *FusionStorageSanPlugin) DeleteVolume(ctx context.Context, name string) error {
	san := volume.NewSAN(p.cli)
	return san.Delete(ctx, name)
}

// ExpandVolume used to expand volume
func (p *FusionStorageSanPlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	san := volume.NewSAN(p.cli)
	return san.Expand(ctx, name, size)
}

// AttachVolume attach volume to node and return storage mapping info.
func (p *FusionStorageSanPlugin) AttachVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	localAttacher := attacher.NewAttacher(attacher.VolumeAttacherConfig{
		Cli:        p.cli,
		Protocol:   p.protocol,
		Invoker:    "csi",
		Portals:    p.portals,
		IscsiLinks: p.iscsiLinks,
		Hosts:      p.hosts,
		Alua:       p.alua,
	})
	mappingInfo, err := localAttacher.ControllerAttach(ctx, name, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("attach volume %s error: %v", name, err)
		return nil, err
	}

	return mappingInfo, nil
}

// DetachVolume used to detach volume from node
func (p *FusionStorageSanPlugin) DetachVolume(ctx context.Context,
	name string,
	parameters map[string]interface{}) error {
	localAttacher := attacher.NewAttacher(attacher.VolumeAttacherConfig{
		Cli:      p.cli,
		Protocol: p.protocol,
		Invoker:  "csi",
		Portals:  p.portals,
		Hosts:    p.hosts,
		Alua:     p.alua,
	})
	_, err := localAttacher.ControllerDetach(ctx, name, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Detach volume %s error: %v", name, err)
		return err
	}

	return nil
}

// GetSectorSize get sector size of plugin
func (p *FusionStorageSanPlugin) GetSectorSize() int64 {
	return constants.FusionAllocUnitBytes
}

func (p *FusionStorageSanPlugin) mutexReleaseClient(ctx context.Context,
	plugin *FusionStorageSanPlugin,
	cli *client.RestClient) {
	plugin.clientMutex.Lock()
	defer plugin.clientMutex.Unlock()
	plugin.clientCount--
	if plugin.clientCount == 0 {
		cli.Logout(ctx)
		p.storageOnline = false
	}
}

func (p *FusionStorageSanPlugin) releaseClient(ctx context.Context, cli *client.RestClient) {
	if p.storageOnline {
		p.mutexReleaseClient(ctx, p, cli)
	}
}

// UpdateBackendCapabilities used to update backend capabilities
func (p *FusionStorageSanPlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":  true,
		"SupportThick": false,
		"SupportQoS":   true,
		"SupportClone": true,
	}
	return capabilities, nil, nil
}

// CreateSnapshot used to create snapshot
func (p *FusionStorageSanPlugin) CreateSnapshot(ctx context.Context,
	lunName, snapshotName string) (map[string]interface{}, error) {
	san := volume.NewSAN(p.cli)

	snapshotName = utils.GetFusionStorageSnapshotName(snapshotName)
	snapshot, err := san.CreateSnapshot(ctx, lunName, snapshotName)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// DeleteSnapshot used to delete snapshot
func (p *FusionStorageSanPlugin) DeleteSnapshot(ctx context.Context,
	snapshotParentID, snapshotName string) error {
	san := volume.NewSAN(p.cli)

	snapshotName = utils.GetFusionStorageSnapshotName(snapshotName)
	err := san.DeleteSnapshot(ctx, snapshotName)
	if err != nil {
		return err
	}

	return nil
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *FusionStorageSanPlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	return p.updatePoolCapabilities(ctx, poolNames, FusionStorageSan)
}

// Validate used to validate FusionStorageSanPlugin parameters
func (p *FusionStorageSanPlugin) Validate(ctx context.Context, param map[string]interface{}) error {
	log.AddContext(ctx).Infoln("Start to validate FusionStorageSanPlugin parameters.")

	err := p.verifyFusionStorageSanParam(param)
	if err != nil {
		return err
	}

	clientConfig, err := p.getNewClientConfig(ctx, param)
	if err != nil {
		return err
	}

	// Login verification
	cli := client.NewIRestClient(ctx, clientConfig)
	err = cli.ValidateLogin(ctx)
	if err != nil {
		return err
	}
	cli.Logout(ctx)

	return nil
}

func (p *FusionStorageSanPlugin) verifyFusionStorageSanParam(config map[string]interface{}) error {
	parameters, ok := utils.GetValue[map[string]interface{}](config, "parameters")
	if !ok {
		return errors.New("parameters in config must be provided")
	}

	protocol, ok := utils.GetValue[string](parameters, "protocol")
	if !ok || !slices.Contains(constants.FusionstorageProtocol, protocol) {
		return fmt.Errorf("verify protocol: [%v] failed, protocol must be provided and "+
			"be one of \"scsi\" or \"iscsi\" for fusionstorage-san backend", parameters["protocol"])
	}

	portals, _ := utils.GetValue[[]interface{}](parameters, "portals")
	if protocol == constants.ProtocolScsi && len(portals) == 0 {
		return fmt.Errorf("verify parameters [%v] failed, "+
			"portals must be provided while protocol is scsi", parameters)
	}

	if protocol == constants.ProtocolIscsi {
		iscsiLinksStr, linkOk := utils.GetValue[string](parameters, "iscsiLinks")
		if len(portals) == 0 && !linkOk {
			return fmt.Errorf("verify parameters [%v] failed, "+
				"one of portals or iscsiLinks must be provided while protocol is iscsi", parameters)
		}

		if linkOk {
			iscsiLinks, err := strconv.Atoi(iscsiLinksStr)
			if err != nil {
				return fmt.Errorf("iscsiLinks %s can not convert to int", iscsiLinksStr)
			}

			if iscsiLinks <= 0 {
				return fmt.Errorf("verify iscsiLinks [%d] failed, "+
					"iscsiLinks value must be greater than zero", iscsiLinks)
			}
		}
	}

	return nil
}
