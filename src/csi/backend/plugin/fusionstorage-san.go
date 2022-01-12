/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package plugin provide storage function
package plugin

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"connector"
	"proto"
	"storage/fusionstorage/attacher"
	"storage/fusionstorage/client"
	"storage/fusionstorage/volume"

	"utils"
	"utils/log"
)

type FusionStorageSanPlugin struct {
	FusionStoragePlugin
	hosts    map[string]string
	protocol string
	portals  []string
	alua     map[string]interface{}

	storageOnline bool
	clientCount   int
	clientMutex   sync.Mutex
}

func init() {
	RegPlugin("fusionstorage-san", &FusionStorageSanPlugin{})
}

func (p *FusionStorageSanPlugin) NewPlugin() Plugin {
	return &FusionStorageSanPlugin{
		hosts: make(map[string]string),
	}
}

func (p *FusionStorageSanPlugin) Init(config, parameters map[string]interface{}, keepLogin bool) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist {
		log.Errorf("protocol must be configured in backend %v", parameters)
		return errors.New("protocol must be configured")
	}

	portals, exist := parameters["portals"].([]interface{})
	if !exist || len(portals) == 0 {
		log.Errorf("portals must be configured in backend %v", parameters)
		return errors.New("portals must be configured")
	}

	if strings.ToLower(protocol) == "scsi" {
		scsi := portals[0].(map[string]interface{})
		for k, v := range scsi {
			manageIP := v.(string)
			ip := net.ParseIP(manageIP)
			if ip == nil {
				return fmt.Errorf("Manage IP %s of host %s is invalid", manageIP, k)
			}

			p.hosts[k] = manageIP
		}

		p.protocol = "scsi"
	} else if strings.ToLower(protocol) == "iscsi" {
		portals, err := proto.VerifyIscsiPortals(portals)
		if err != nil {
			return err
		}

		p.portals = portals
		p.protocol = "iscsi"
		p.alua, _ = parameters["ALUA"].(map[string]interface{})
	} else {
		msg := fmt.Sprintf("protocol %s configured is error. Just support iscsi and scsi", protocol)
		log.Errorln(msg)
		return errors.New(msg)
	}

	err := p.init(config, keepLogin)
	if err != nil {
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) getParams(name string, parameters map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":     name,
		"capacity": utils.RoundUpSize(parameters["size"].(int64), CAPACITY_UNIT),
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

func (p *FusionStorageSanPlugin) CreateVolume(name string, parameters map[string]interface{}) (utils.Volume, error) {
	size, ok := parameters["size"].(int64)
	// for fusionStorage block, the unit is MiB
	if !ok || !utils.IsCapacityAvailable(size, CAPACITY_UNIT) {
		msg := fmt.Sprintf("Create Volume: the capacity %d is not an integer multiple of %d.",
			size, CAPACITY_UNIT)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	params, err := p.getParams(name, parameters)
	if err != nil {
		return nil, err
	}

	san := volume.NewSAN(p.cli)
	volObj, err := san.Create(params)
	if err != nil {
		return nil, err
	}

	return volObj, nil
}

func (p *FusionStorageSanPlugin) DeleteVolume(name string) error {
	san := volume.NewSAN(p.cli)
	return san.Delete(name)
}

func (p *FusionStorageSanPlugin) ExpandVolume(name string, size int64) (bool, error) {
	// for fusionStorage block, the unit is MiB
	if !utils.IsCapacityAvailable(size, CAPACITY_UNIT) {
		msg := fmt.Sprintf("Expand Volume: the capacity %d is not an integer multiple of %d.",
			size, CAPACITY_UNIT)
		log.Errorln(msg)
		return false, errors.New(msg)
	}
	san := volume.NewSAN(p.cli)
	newSize := utils.TransVolumeCapacity(size, CAPACITY_UNIT)
	isAttach, err := san.Expand(name, newSize)
	return isAttach, err
}

func (p *FusionStorageSanPlugin) DetachVolume(name string, parameters map[string]interface{}) error {
	localAttacher := attacher.NewAttacher(p.cli, p.protocol, "csi", p.portals, p.hosts, p.alua)
	_, err := localAttacher.ControllerDetach(name, parameters)
	if err != nil {
		log.Errorf("Detach volume %s error: %v", name, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) mutexGetClient() (*client.Client, error) {
	p.clientMutex.Lock()
	var err error
	if !p.storageOnline || p.clientCount == 0 {
		err = p.cli.Login()
		p.storageOnline = err == nil
		if err == nil {
			p.clientCount++
		}
	} else {
		p.clientCount++
	}

	p.clientMutex.Unlock()
	return p.cli, err
}

func (p *FusionStorageSanPlugin) getClient() (*client.Client, error) {
	return p.mutexGetClient()
}

func (p *FusionStorageSanPlugin) mutexReleaseClient(plugin *FusionStorageSanPlugin, cli *client.Client) {
	plugin.clientMutex.Lock()
	defer plugin.clientMutex.Unlock()
	plugin.clientCount--
	if plugin.clientCount == 0 {
		cli.Logout()
		p.storageOnline = false
	}
}

func (p *FusionStorageSanPlugin) releaseClient(cli *client.Client) {
	if p.storageOnline {
		p.mutexReleaseClient(p, cli)
	}
}

func (p *FusionStorageSanPlugin) getStageVolumeInfo(name string, parameters map[string]interface{}) (
	*connector.ConnectInfo, error) {
	cli, err := p.getClient()
	if err != nil {
		return nil, err
	}
	defer p.releaseClient(cli)

	localAttacher := attacher.NewAttacher(cli, p.protocol, "csi", p.portals, p.hosts, p.alua)
	connectInfo, err := localAttacher.NodeStage(name, parameters)
	if err != nil {
		log.Errorf("Stage volume %s error: %v", name, err)
		return nil, err
	}

	return connectInfo, nil
}

func (p *FusionStorageSanPlugin) StageVolume(name string, parameters map[string]interface{}) error {
	connectInfo, err := p.getStageVolumeInfo(name, parameters)
	if err != nil {
		return err
	}

	devPath, err := p.lunConnectVolume(connectInfo)
	if err != nil {
		return err
	}

	return p.lunStageVolume(name, devPath, parameters)
}

func (p *FusionStorageSanPlugin) getUnStageVolumeInfo(name string, parameters map[string]interface{}) (
	*connector.DisConnectInfo, error) {
	cli, err := p.getClient()
	if err != nil {
		return nil, err
	}
	defer p.releaseClient(cli)

	localAttacher := attacher.NewAttacher(cli, p.protocol, "csi", p.portals, p.hosts, p.alua)
	disconnectInfo, err := localAttacher.NodeUnstage(name, parameters)
	if err != nil {
		log.Errorf("Unstage volume %s error: %v", name, err)
		return nil, err
	}

	return disconnectInfo, nil
}

func (p *FusionStorageSanPlugin) UnstageVolume(name string, parameters map[string]interface{}) error {
	err := p.unstageVolume(name, parameters)
	if err != nil {
		return err
	}

	disconnectInfo, err := p.getUnStageVolumeInfo(name, parameters)
	if err != nil {
		return err
	}

	if disconnectInfo == nil {
		return nil
	}

	return p.lunDisconnectVolume(disconnectInfo)
}

func (p *FusionStorageSanPlugin) UpdateBackendCapabilities() (map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		"SupportThin":  true,
		"SupportThick": false,
		"SupportQoS":   true,
		"SupportClone": true,
	}

	return capabilities, nil
}

func (p *FusionStorageSanPlugin) NodeExpandVolume(name, volumePath string) error {
	cli, err := p.getClient()
	if err != nil {
		return err
	}
	defer p.releaseClient(cli)

	lun, err := cli.GetVolumeByName(name)
	if err != nil {
		log.Errorf("Get lun %s error: %v", name, err)
		return err
	}
	if lun == nil {
		msg := fmt.Sprintf("LUN %s to expand doesn't exist", name)
		log.Errorln(msg)
		return errors.New(msg)
	}

	wwn := lun["wwn"].(string)
	err = connector.ResizeBlock(wwn)
	if err != nil {
		log.Errorf("Lun %s resize error: %v", wwn, err)
		return err
	}

	err = connector.ResizeMountPath(volumePath)
	if err != nil {
		log.Errorf("MountPath %s resize error: %v", volumePath, err)
		return err
	}

	return nil
}

func (p *FusionStorageSanPlugin) CreateSnapshot(lunName, snapshotName string) (map[string]interface{}, error) {
	san := volume.NewSAN(p.cli)

	snapshotName = utils.GetFusionStorageSnapshotName(snapshotName)
	snapshot, err := san.CreateSnapshot(lunName, snapshotName)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (p *FusionStorageSanPlugin) DeleteSnapshot(snapshotParentId, snapshotName string) error {
	san := volume.NewSAN(p.cli)

	snapshotName = utils.GetFusionStorageSnapshotName(snapshotName)
	err := san.DeleteSnapshot(snapshotName)
	if err != nil {
		return err
	}

	return nil
}
