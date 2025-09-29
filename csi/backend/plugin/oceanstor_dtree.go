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

	v1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// OceanstorDTreePlugin implements storage StoragePlugin interface
type OceanstorDTreePlugin struct {
	OceanstorPlugin

	portals    []string
	parentName string
}

func init() {
	RegPlugin(constants.OceanStorDtree, &OceanstorDTreePlugin{})
}

// NewPlugin used to create new plugin
func (p *OceanstorDTreePlugin) NewPlugin() StoragePlugin {
	return &OceanstorDTreePlugin{}
}

// Init used to init the plugin
func (p *OceanstorDTreePlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	parentname, ok := parameters["parentname"]
	if ok {
		p.parentName, ok = parentname.(string)
		if !ok {
			return errors.New("parentname must be a string type")
		}
	}

	var err error
	_, p.portals, err = verifyProtocolAndPortals(parameters, constants.OceanStorDtree)
	if err != nil {
		log.Errorf("verify protocol and portals failed, err: %v", err)
		return err
	}

	err = p.init(ctx, config, keepLogin)
	if err != nil {
		log.AddContext(ctx).Errorf("init dtree plugin failed, data:")
		return err
	}

	return nil
}

func (p *OceanstorDTreePlugin) getDTreeObj() *volume.DTree {
	return volume.NewDTree(p.cli)
}

// CreateVolume used to create volume
func (p *OceanstorDTreePlugin) CreateVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {
	if p == nil {
		return nil, errors.New("empty dtree plugin")
	}
	if parameters == nil {
		return nil, errors.New("empty parameters")
	}

	var err error
	if p.product.IsDoradoV6OrV7() {
		name, err = getVolumeNameFromPVNameOrParameters(name, parameters)
		if err != nil {
			return nil, err
		}
	}

	parentname := p.parentName
	scParentname, _ := utils.GetValue[string](parameters, "parentname")
	parentname, err = getValidParentname(scParentname, p.parentName)
	if err != nil {
		return nil, err
	}

	parameters["vstoreId"] = p.vStoreId
	parameters["parentname"] = parentname
	params := getParams(ctx, name, parameters)

	volObj, err := p.getDTreeObj().Create(ctx, params)
	if err != nil {
		return nil, err
	}
	volObj.SetDTreeParentName(parentname)

	return volObj, nil
}

// QueryVolume used to query volume
func (p *OceanstorDTreePlugin) QueryVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {

	return nil, errors.New("oceanstor-dtree does not support DTree feature")
}

// DeleteDTreeVolume used to delete DTree volume
func (p *OceanstorDTreePlugin) DeleteDTreeVolume(ctx context.Context, dTreeName, parentName string) error {
	if p == nil {
		return errors.New("empty dtree plugin")
	}
	params := map[string]any{
		"parentname": parentName,
		"name":       dTreeName,
		"vstoreid":   p.vStoreId,
	}

	return p.getDTreeObj().Delete(ctx, params)

}

// ExpandDTreeVolume used to expand DTree volume
func (p *OceanstorDTreePlugin) ExpandDTreeVolume(ctx context.Context,
	dTreeName, parentName string, spaceHardQuota int64) (bool, error) {
	dTree := p.getDTreeObj()

	// The unit of DTree's quota is bytes, but not sector size.
	spaceHardQuota = utils.TransK8SCapacity(spaceHardQuota, p.GetSectorSize())
	err := dTree.Expand(ctx, parentName, dTreeName, p.vStoreId, spaceHardQuota)
	if err != nil {
		return false, fmt.Errorf("failed to expand dtree volume: %w", err)
	}
	log.AddContext(ctx).Infof("expand dTree volume success, parentName: %v, dTreeName: %v,"+
		" vStoreId: %v, spaceHardQuota: %v", parentName, dTreeName, p.vStoreId, spaceHardQuota)
	return false, nil
}

// AttachVolume attach volume to node and return storage mapping info.
func (p *OceanstorDTreePlugin) AttachVolume(_ context.Context, _ string, parameters map[string]any) (map[string]any,
	error) {
	return attachDTreeVolume(parameters)
}

// DeleteVolume used to delete volume
func (p *OceanstorDTreePlugin) DeleteVolume(ctx context.Context, name string) error {
	return errors.New("not implement")

}

// ExpandVolume used to expand volume
func (p *OceanstorDTreePlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	return false, errors.New("not implement")
}

// Validate used to validate OceanstorDTreePlugin parameters
func (p *OceanstorDTreePlugin) Validate(ctx context.Context, param map[string]interface{}) error {
	log.AddContext(ctx).Infoln("Start to validate OceanstorDTreePlugin parameters.")

	clientConfig, err := getNewClientConfig(ctx, param)
	if err != nil {
		log.AddContext(ctx).Errorln("validate OceanstorDTreePlugin parameters failed, err:", err.Error())
		return err
	}

	err = verifyDTreeParam(ctx, param, constants.OceanStorDtree)
	if err != nil {
		return err
	}

	// Login verification
	cli, err := client.NewClient(ctx, clientConfig)
	if err != nil {
		return err
	}

	err = cli.ValidateLogin(ctx)
	if err != nil {
		return err
	}

	cli.Logout(ctx)

	return nil
}

// CreateSnapshot used to create snapshot
func (p *OceanstorDTreePlugin) CreateSnapshot(ctx context.Context, s, s2 string) (map[string]interface{}, error) {
	return nil, errors.New("not implement")

}

// DeleteSnapshot used to delete snapshot
func (p *OceanstorDTreePlugin) DeleteSnapshot(ctx context.Context, s, s2 string) error {
	return errors.New("not implement")
}

// UpdateBackendCapabilities used to update backend capabilities
func (p *OceanstorDTreePlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities, specifications, err := p.OceanstorPlugin.UpdateBackendCapabilities(ctx)
	if err != nil {
		return nil, nil, err
	}

	// close dTree pvc label switch
	capabilities[string(constants.SupportMetro)] = false
	capabilities[string(constants.SupportMetroNAS)] = false
	capabilities[string(constants.SupportReplication)] = false
	capabilities[string(constants.SupportClone)] = false
	capabilities[string(constants.SupportApplicationType)] = false
	capabilities[string(constants.SupportQoS)] = false

	err = p.updateSmartThin(capabilities)
	if err != nil {
		return nil, nil, err
	}

	err = p.updateNFS4Capability(ctx, capabilities)
	if err != nil {
		return nil, nil, err
	}

	return capabilities, specifications, nil
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *OceanstorDTreePlugin) UpdatePoolCapabilities(ctx context.Context, poolNames []string) (map[string]interface{},
	error) {
	capabilities := make(map[string]interface{})

	for _, poolName := range poolNames {
		capabilities[poolName] = map[string]interface{}{
			string(v1.FreeCapacity):  int64(0),
			string(v1.UsedCapacity):  int64(0),
			string(v1.TotalCapacity): int64(0),
		}
	}
	return capabilities, nil

}

func (p *OceanstorDTreePlugin) updateNFS4Capability(ctx context.Context, capabilities map[string]interface{}) error {
	if capabilities == nil {
		capabilities = make(map[string]interface{})
	}

	nfsServiceSetting, err := p.cli.GetNFSServiceSetting(ctx)
	if err != nil {
		return err
	}

	updateCapabilityByNfsServiceSetting(capabilities, nfsServiceSetting)
	return nil
}

// updateSmartThin for fileSystem on dorado storage, only Thin is supported
func (p *OceanstorDTreePlugin) updateSmartThin(capabilities map[string]interface{}) error {
	if capabilities == nil {
		return nil
	}
	if p.product.IsDorado() || p.product.IsDoradoV6OrV7() {
		capabilities["SupportThin"] = true
	}
	return nil
}

// ModifyVolume used to modify volume hyperMetro status
func (p *OceanstorDTreePlugin) ModifyVolume(ctx context.Context, volumeName string,
	modifyType pkgVolume.ModifyVolumeType, param map[string]string) error {

	return errors.New("not implement")
}

// SetParentName sets the parentName of Oceanstor DTree plugin
func (p *OceanstorDTreePlugin) SetParentName(parentName string) {
	p.parentName = parentName
}

// GetDTreeParentName gets the parent name of dtree plugin
func (p *OceanstorDTreePlugin) GetDTreeParentName() string {
	return p.parentName
}
