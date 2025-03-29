/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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

	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/proto"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/attacher"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const oceandiskSanBackend = "oceandisk-san"

// OceandiskSanPlugin implements storage StoragePlugin interface
type OceandiskSanPlugin struct {
	OceandiskPlugin
	attacher *attacher.OceandiskAttacher
}

func init() {
	RegPlugin(oceandiskSanBackend, &OceandiskSanPlugin{})
}

// NewPlugin used to create new plugin
func (p *OceandiskSanPlugin) NewPlugin() StoragePlugin {
	return &OceandiskSanPlugin{}
}

// Init used to init the plugin
func (p *OceandiskSanPlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "iscsi" && protocol != "fc" && protocol != "roce") {
		return errors.New("protocol must be provided as 'iscsi', 'fc' or " +
			"'roce' for oceandisk-san backend")
	}

	alua, _ := parameters["ALUA"].(map[string]interface{})

	var ips []string
	var err error
	if protocol == "iscsi" || protocol == "roce" {
		portals, exist := parameters["portals"].([]interface{})
		if !exist {
			return fmt.Errorf("portals are required to configure for %s backend", protocol)
		}

		ips, err = proto.VerifyIscsiPortals(ctx, portals)
		if err != nil {
			return err
		}
	}

	err = p.init(ctx, config, keepLogin)
	if err != nil {
		return err
	}

	p.attacher = attacher.NewOceanDiskAttacher(attacher.OceanDiskAttacherConfig{
		Cli:      p.cli,
		Protocol: protocol,
		Invoker:  csiInvoker,
		Portals:  ips,
		Alua:     alua,
	})

	return nil
}

func (p *OceandiskSanPlugin) getSanObj() *volume.SAN {
	return volume.NewSAN(p.cli)
}

// CreateVolume used to create volume
func (p *OceandiskSanPlugin) CreateVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (utils.Volume, error) {

	params := getParams(ctx, name, parameters)
	san := p.getSanObj()

	return san.Create(ctx, params)
}

// QueryVolume used to query volume
func (p *OceandiskSanPlugin) QueryVolume(ctx context.Context, name string, params map[string]interface{}) (
	utils.Volume, error) {
	san := p.getSanObj()
	return san.Query(ctx, name)
}

// DeleteVolume used to delete volume
func (p *OceandiskSanPlugin) DeleteVolume(ctx context.Context, name string) error {
	san := p.getSanObj()
	return san.Delete(ctx, name)
}

// ExpandVolume used to expand volume
func (p *OceandiskSanPlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	san := p.getSanObj()
	return san.Expand(ctx, name, size)
}

// AttachVolume attach volume to node,return storage mapping info.
func (p *OceandiskSanPlugin) AttachVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	namespace, err := p.cli.GetNamespaceByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get namespace %s error: %v", name, err)
	}

	if namespace == nil {
		return nil, fmt.Errorf("get empty namespace info, namespaceName: %v", name)
	}

	return p.attacher.ControllerAttach(ctx, name, parameters)
}

// DetachVolume used to detach volume from node
func (p *OceandiskSanPlugin) DetachVolume(ctx context.Context, name string, parameters map[string]interface{}) error {
	namespace, err := p.cli.GetNamespaceByName(ctx, name)
	if err != nil {
		return fmt.Errorf("get namespace %s error: %v", name, err)
	}

	if namespace == nil {
		log.AddContext(ctx).Warningf("namespace %s to detach doesn't exist", name)
		return nil
	}

	_, err = p.attacher.ControllerDetach(ctx, name, parameters)
	return err
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *OceandiskSanPlugin) UpdatePoolCapabilities(ctx context.Context,
	poolNames []string) (map[string]interface{}, error) {
	return p.updatePoolCapacities(ctx, poolNames)
}

// CreateSnapshot used to create snapshot
func (p *OceandiskSanPlugin) CreateSnapshot(ctx context.Context,
	namespaceName, snapshotName string) (map[string]interface{}, error) {
	return nil, errors.New("oceandisk does not support snapshot feature")
}

// DeleteSnapshot used to delete snapshot
func (p *OceandiskSanPlugin) DeleteSnapshot(ctx context.Context,
	snapshotParentID, snapshotName string) error {
	return errors.New("oceandisk does not support snapshot feature")
}

// UpdateBackendCapabilities to update the block storage capabilities
func (p *OceandiskSanPlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	return p.OceandiskPlugin.UpdateBackendCapabilities()
}

// Validate used to validate OceandiskSanPlugin parameters
func (p *OceandiskSanPlugin) Validate(ctx context.Context, param map[string]interface{}) error {
	log.AddContext(ctx).Infoln("Start to validate OceandiskSanPlugin parameters.")

	err := p.verifyOceandiskSanParam(ctx, param)
	if err != nil {
		return err
	}

	clientConfig, err := p.getNewClientConfig(ctx, param)
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

func (p *OceandiskSanPlugin) getNewClientConfig(ctx context.Context,
	param map[string]interface{}) (*client.NewClientConfig, error) {
	data := &client.NewClientConfig{}
	configUrls, ok := utils.GetValue[[]interface{}](param, "urls")
	if !ok || len(configUrls) <= 0 {
		return data, fmt.Errorf("verify urls: [%v] failed. urls must be provided", param["urls"])
	}

	for _, configUrl := range configUrls {
		url, ok := configUrl.(string)
		if !ok {
			return data, fmt.Errorf("verify url: [%v] failed. url convert to string failed", configUrl)
		}
		data.Urls = append(data.Urls, url)
	}

	data.User, ok = utils.GetValue[string](param, "user")
	if !ok {
		return data, fmt.Errorf("can not convert user type %T to string", param["user"])
	}

	data.SecretName, ok = utils.GetValue[string](param, "secretName")
	if !ok {
		return data, fmt.Errorf("can not convert secretName type %T to string", param["secretName"])
	}

	data.SecretNamespace, ok = utils.GetValue[string](param, "secretNamespace")
	if !ok {
		return data, fmt.Errorf("can not convert secretNamespace type %T to string", param["secretNamespace"])
	}

	data.BackendID, ok = utils.GetValue[string](param, "backendID")
	if !ok {
		return data, fmt.Errorf("can not convert backendID type %T to string", param["backendID"])
	}

	data.ParallelNum, _ = utils.GetValue[string](param, "maxClientThreads")
	data.UseCert, _ = utils.GetValue[bool](param, "useCert")
	data.CertSecretMeta, _ = utils.GetValue[string](param, "certSecret")

	return data, nil
}

func (p *OceandiskSanPlugin) verifyOceandiskSanParam(ctx context.Context, config map[string]interface{}) error {
	parameters, exist := config["parameters"].(map[string]interface{})
	if !exist {
		return fmt.Errorf("verify parameters: [%v] failed. parameters must be provided", config["parameters"])
	}

	return verifyOceandiskProtocolParams(ctx, parameters)
}

func verifyOceandiskProtocolParams(ctx context.Context, parameters map[string]interface{}) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "iscsi" && protocol != "fc" && protocol != "roce") {
		return fmt.Errorf("verify protocol: [%v] failed. protocol must be provided and be one of "+
			"[iscsi, fc, roce] for oceandisk-san backend", parameters["protocol"])
	}

	if protocol == "iscsi" || protocol == "roce" {
		portals, exist := parameters["portals"].([]interface{})
		if !exist {
			return fmt.Errorf("verify portals: [%v] failed. portals are required to configure for "+
				"iscsi or roce for oceandisk-san backend", parameters["portals"])
		}

		_, err := proto.VerifyIscsiPortals(ctx, portals)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteDTreeVolume used to delete DTree volume
func (p *OceandiskSanPlugin) DeleteDTreeVolume(_ context.Context, _ string, _ string) error {
	return errors.New("oceandisk does not support dtree feature")
}

// ExpandDTreeVolume used to expand DTree volume
func (p *OceandiskSanPlugin) ExpandDTreeVolume(context.Context, string, string, int64) (bool, error) {
	return false, errors.New("oceandisk does not support dtree feature")
}

// ModifyVolume used to modify volume hyperMetro status
func (p *OceandiskSanPlugin) ModifyVolume(ctx context.Context, volumeName string,
	modifyType pkgVolume.ModifyVolumeType, param map[string]string) error {
	return errors.New("oceandisk does not support volume modify feature")
}

// GetSectorSize gets the sector size of plugin
func (p *OceandiskSanPlugin) GetSectorSize() int64 {
	return SectorSize
}
