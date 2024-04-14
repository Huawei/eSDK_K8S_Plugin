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

package plugin

import (
	"context"
	"errors"
	"fmt"

	v1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/pkg/constants"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/storage/oceanstor/client"
	"huawei-csi-driver/storage/oceanstor/volume"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	// DTreeStorage defines DTree storage name
	DTreeStorage = "oceanstor-dtree"
)

// OceanstorDTreePlugin implements storage Plugin interface
type OceanstorDTreePlugin struct {
	OceanstorPlugin

	portals    []string
	parentName string
}

func init() {
	RegPlugin(DTreeStorage, &OceanstorDTreePlugin{})
}

// NewPlugin used to create new plugin
func (p *OceanstorDTreePlugin) NewPlugin() Plugin {
	return &OceanstorDTreePlugin{}
}

// Init used to init the plugin
func (p *OceanstorDTreePlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	var exist bool
	p.parentName, exist = utils.ToStringWithFlag(parameters["parentname"])
	if !exist || p.parentName == "" {
		return pkgUtils.Errorf(ctx, "Verify parentname: [%v] failed. \nParentname must be provided for "+
			"oceanstor-dtree backend\n", parameters["parentname"])
	}

	var err error
	_, p.portals, err = verifyProtocolAndPortals(parameters)
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

	size, ok := parameters["size"].(int64)
	if !ok || !utils.IsCapacityAvailable(size, SectorSize) {
		msg := fmt.Sprintf("Create Volume: the capacity %d is not an integer multiple of 512.", size)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	parameters["vstoreId"] = p.vStoreId
	parameters["parentname"] = p.parentName
	params := p.getParams(ctx, name, parameters)

	volObj, err := p.getDTreeObj().Create(ctx, params)
	if err != nil {
		return nil, err
	}
	volObj.SetDTreeParentName(p.parentName)

	return volObj, nil
}

// QueryVolume used to query volume
func (p *OceanstorDTreePlugin) QueryVolume(ctx context.Context, name string, parameters map[string]interface{}) (
	utils.Volume, error) {

	return nil, errors.New(" not implement")
}

// DeleteDTreeVolume used to delete DTree volume
func (p *OceanstorDTreePlugin) DeleteDTreeVolume(ctx context.Context, params map[string]interface{}) error {
	if p == nil {
		return errors.New("empty dtree plugin")
	}
	if params == nil {
		return errors.New("empty parameters")
	}
	params["vstoreid"] = p.vStoreId
	params["parentname"] = p.parentName

	return p.getDTreeObj().Delete(ctx, params)

}

// ExpandDTreeVolume used to expand DTree volume
func (p *OceanstorDTreePlugin) ExpandDTreeVolume(ctx context.Context, params map[string]interface{}) (bool, error) {
	dTree := p.getDTreeObj()

	dTreeName, _ := utils.ToStringWithFlag(params["name"])
	spaceHardQuota, ok := params["spacehardquota"].(int64)
	if !ok {
		log.AddContext(ctx).Errorln("expand dTree volume failed, spacehardquota is not found")
		return false, errors.New("spacehardquota not found")
	}

	if !utils.IsCapacityAvailable(spaceHardQuota, SectorSize) {
		msg := fmt.Sprintf("Create Volume: the capacity %d is not an integer multiple of 512.", spaceHardQuota)
		log.AddContext(ctx).Errorln(msg)
		return false, errors.New(msg)
	}

	parentName, _ := utils.ToStringWithFlag(params["parentname"])
	err := dTree.Expand(ctx, parentName, dTreeName, p.vStoreId, 0, spaceHardQuota)
	if err != nil {
		log.AddContext(ctx).Errorf("expand dTree volume failed, ")
		return false, err
	}
	log.AddContext(ctx).Infof("expand dTree volume success, parentName: %v, dTreeName: %v,"+
		" vStoreId: %v, spaceHardQuota: %v", params, dTreeName, p.vStoreId, spaceHardQuota)
	return false, nil
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

	clientConfig, err := p.getNewClientConfig(ctx, param)
	if err != nil {
		return err
	}

	err = verifyOceanstorDTreeParam(ctx, param)
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

func verifyOceanstorDTreeParam(ctx context.Context, config map[string]interface{}) error {
	// verify storage
	storage, exist := utils.ToStringWithFlag(config["storage"])
	if !exist || storage != DTreeStorage {
		msg := fmt.Sprintf("Verify storage: [%v] failed. \nstorage must be %s", config["storage"], DTreeStorage)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}
	// verify parameters
	parameters, exist := config["parameters"].(map[string]interface{})
	if !exist {
		msg := fmt.Sprintf("Verify parameters: [%v] failed. \nparameters must be provided", config["parameters"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	// verify parent name
	parentName, exist := utils.ToStringWithFlag(parameters["parentname"])
	if !exist || parentName == "" {
		msg := fmt.Sprintf("Verify parentname: [%v] failed. \nParentname must be provided for "+
			"oceanstor-dtree backend\n", parameters["parentname"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	// verify protocol portals
	_, _, err := verifyProtocolAndPortals(parameters)
	if err != nil {
		return pkgUtils.Errorf(ctx, "check nas parameter failed, err: %v", err)
	}

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
	capabilities[string(constants.SupportLabel)] = false
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
	// NFS3 is enabled by default.
	capabilities["SupportNFS3"] = true
	capabilities["SupportNFS4"] = false
	capabilities["SupportNFS41"] = false

	if !nfsServiceSetting["SupportNFS3"] {
		capabilities["SupportNFS3"] = false
	}
	if nfsServiceSetting["SupportNFS4"] {
		capabilities["SupportNFS4"] = true
	}
	if nfsServiceSetting["SupportNFS41"] {
		capabilities["SupportNFS41"] = true
	}

	return nil
}

// updateSmartThin for fileSystem on dorado storage, only Thin is supported
func (p *OceanstorDTreePlugin) updateSmartThin(capabilities map[string]interface{}) error {
	if capabilities == nil {
		return nil
	}
	if p.product == "Dorado" || p.product == "DoradoV6" {
		capabilities["SupportThin"] = true
	}
	return nil
}
