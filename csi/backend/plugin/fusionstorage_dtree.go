/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

	v1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/volume/dtree"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func init() {
	RegPlugin(constants.FusionDTree, &FusionStorageDTreePlugin{})
}

// FusionStorageDTreePlugin implements storage StoragePlugin interface for Pacific DTree
type FusionStorageDTreePlugin struct {
	FusionStoragePlugin

	protocol   string
	parentname string
}

// NewPlugin used to create new plugin
func (p *FusionStorageDTreePlugin) NewPlugin() StoragePlugin {
	return &FusionStorageDTreePlugin{}
}

// Init used to init the plugin
func (p *FusionStorageDTreePlugin) Init(ctx context.Context, config map[string]interface{}, parameters map[string]any,
	keepLogin bool) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != constants.ProtocolNfs && protocol != constants.ProtocolDpc) {
		return fmt.Errorf("protocol must be provided and be %q or %q for %q backend",
			constants.ProtocolNfs, constants.ProtocolDpc, constants.FusionDTree)
	}
	p.protocol = protocol

	parentname, ok := parameters["parentname"]
	if ok {
		p.parentname, ok = parentname.(string)
		if !ok {
			return errors.New("parentname must be a string type")
		}
	}

	if protocol == constants.ProtocolNfs {
		portals, exist := parameters["portals"].([]any)
		if !exist || len(portals) != 1 {
			return fmt.Errorf("portals must be provided for %q %q backend and just support one portal",
				constants.FusionDTree, constants.ProtocolNfs)
		}
	}

	err := p.init(ctx, config, keepLogin)
	if err != nil {
		return err
	}
	return nil
}

// UpdateBackendCapabilities to update the backend capabilities, such as thin, thick, qos etc.
func (p *FusionStorageDTreePlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities := map[string]interface{}{
		string(constants.SupportClone): false,
		string(constants.SupportQoS):   false,
		string(constants.SupportThick): false,
		string(constants.SupportQuota): false,
		string(constants.SupportThin):  true,
	}

	err := p.updateNFS4Capability(ctx, capabilities)
	if err != nil {
		return nil, nil, err
	}
	return capabilities, nil, nil
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *FusionStorageDTreePlugin) UpdatePoolCapabilities(ctx context.Context, poolNames []string) (map[string]any,
	error) {
	capabilities := make(map[string]any)
	for _, poolName := range poolNames {
		capabilities[poolName] = map[string]any{
			string(v1.FreeCapacity):  int64(0),
			string(v1.UsedCapacity):  int64(0),
			string(v1.TotalCapacity): int64(0),
		}
	}
	return capabilities, nil
}

// Validate used to validate FusionStorageDTreePlugin parameters
func (p *FusionStorageDTreePlugin) Validate(ctx context.Context, parameters map[string]interface{}) error {
	log.AddContext(ctx).Infoln("Start to validate fusionstorage-dtree parameters.")
	err := verifyDTreeParam(ctx, parameters, constants.FusionDTree)
	if err != nil {
		return err
	}

	clientConfig, err := p.getNewClientConfig(ctx, parameters)
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

// CreateVolume used to create volume
func (p *FusionStorageDTreePlugin) CreateVolume(ctx context.Context, name string,
	parameters map[string]any) (utils.Volume, error) {
	params, err := utils.ConvertMapToStruct[CreateDTreeVolumeParameter](parameters)
	if err != nil {
		return nil, fmt.Errorf("convert parameters to struct failed when create fusionstorage-dtree: %w", err)
	}

	model, err := params.genCreateDTreeModel(name, p.parentname, p.protocol)
	if err != nil {
		return nil, err
	}

	return dtree.NewCreator(ctx, p.cli, model).Create()
}

// QueryVolume used to query volume
func (p *FusionStorageDTreePlugin) QueryVolume(_ context.Context, _ string, _ map[string]any) (utils.Volume, error) {
	return nil, errors.New("fusionstorage-dtree not support management volume feature")
}

// DeleteVolume used to delete volume
func (p *FusionStorageDTreePlugin) DeleteVolume(ctx context.Context, s string) error {
	return errors.New("fusionstorage-dtree not support DeleteVolume feature")
}

// ExpandVolume used to expand volume
func (p *FusionStorageDTreePlugin) ExpandVolume(ctx context.Context, s string, i int64) (bool, error) {
	return false, errors.New("fusionstorage-dtree not support ExpandVolume feature")
}

// AttachVolume attach volume to node and return storage mapping info.
func (p *FusionStorageDTreePlugin) AttachVolume(_ context.Context, _ string, parameters map[string]any) (map[string]any,
	error) {
	return attachDTreeVolume(parameters)
}

// ModifyVolume used to modify volume attributes
func (p *FusionStorageDTreePlugin) ModifyVolume(_ context.Context, _ string, _ pkgVolume.ModifyVolumeType,
	_ map[string]string) error {
	return errors.New("fusionstorage-dtree not support modify volume feature")
}

// CreateSnapshot used to create snapshot
func (p *FusionStorageDTreePlugin) CreateSnapshot(ctx context.Context, s string, s2 string) (map[string]interface{},
	error) {
	return nil, errors.New("fusionstorage-dtree not support snapshot feature")
}

// DeleteSnapshot used to delete snapshot
func (p *FusionStorageDTreePlugin) DeleteSnapshot(ctx context.Context, s string, s2 string) error {
	return errors.New("fusionstorage-dtree not support snapshot feature")
}

// DeleteDTreeVolume used to delete pacific dtree volume
func (p *FusionStorageDTreePlugin) DeleteDTreeVolume(ctx context.Context, dTreeName string, parentName string) error {
	return dtree.NewDeleter(ctx, p.cli, parentName, dTreeName).Delete()
}

// ExpandDTreeVolume used to expand pacific dtree volume
func (p *FusionStorageDTreePlugin) ExpandDTreeVolume(ctx context.Context, dTreeName string, parentName string,
	size int64) (bool, error) {
	param := &dtree.ExpandDTreeModel{
		ParentName: parentName,
		DTreeName:  dTreeName,
		Capacity:   size,
	}

	return false, dtree.NewExpander(ctx, p.cli, param).Expand()
}

// GetSectorSize get sector size of plugin
func (p *FusionStorageDTreePlugin) GetSectorSize() int64 {
	return constants.FusionDTreeCapacityUnit
}

func (p *FusionStorageDTreePlugin) updateNFS4Capability(ctx context.Context,
	capabilities map[string]interface{}) error {
	if capabilities == nil {
		capabilities = make(map[string]any)
	}

	nfsServiceSetting, err := p.cli.GetNFSServiceSetting(ctx)
	if err != nil {
		return err
	}

	// NFS3 is enabled by default.
	capabilities[constants.SupportNFS3] = true
	capabilities[constants.SupportNFS4] = false // pacific not support NFS4
	capabilities[constants.SupportNFS41] = false

	if nfsServiceSetting[constants.SupportNFS41] {
		capabilities[constants.SupportNFS41] = true
	}

	return nil
}

// SetProtocol sets the protocol of Pacific DTree plugin
func (p *FusionStorageDTreePlugin) SetProtocol(protocol string) {
	p.protocol = protocol
}

// SetParentName sets the parentName of Pacific DTree plugin
func (p *FusionStorageDTreePlugin) SetParentName(parentName string) {
	p.parentname = parentName
}

// GetDTreeParentName gets the parent name of dtree plugin
func (p *FusionStorageDTreePlugin) GetDTreeParentName() string {
	return p.parentname
}
