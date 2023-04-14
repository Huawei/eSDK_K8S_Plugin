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

package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/backend"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// BuildParameterOption define build function
type BuildParameterOption func(map[string]interface{}) error

// BuildParameters build request parameters based on the incoming build function
func BuildParameters(opts ...BuildParameterOption) (map[string]interface{}, error) {
	var parameters = make(map[string]interface{})
	for _, opt := range opts {
		if err := opt(parameters); err != nil {
			return nil, err
		}
	}
	return parameters, nil
}

// WithControllerPublishInfo build publishInfo for the request parameters
func WithControllerPublishInfo(ctx context.Context, req *csi.NodeStageVolumeRequest) BuildParameterOption {
	return func(parameters map[string]interface{}) error {
		publishInfoJson, ok := req.PublishContext["publishInfo"]
		if !ok {
			return fmt.Errorf("publishInfo doesn't exist, PublishContext:%v", req.PublishContext)
		}

		publishInfo := &ControllerPublishInfo{}
		err := json.Unmarshal([]byte(publishInfoJson), publishInfo)
		if err != nil {
			log.AddContext(ctx).Errorf("publishInfo unmarshal fail, error:%v", err)
			return err
		}

		parameters["publishInfo"] = publishInfo
		return nil
	}
}

// WithMultiPathType build multiPathType for the request parameters
func WithMultiPathType(protocol string) BuildParameterOption {
	return func(parameters map[string]interface{}) error {
		publishInfo, exist := parameters["publishInfo"].(*ControllerPublishInfo)
		if !exist {
			return errors.New("build multiPathType failed, caused by publishInfo is not exist")
		}

		publishInfo.VolumeUseMultiPath = app.GetGlobalConfig().VolumeUseMultiPath
		if protocol == "iscsi" || protocol == "fc" {
			publishInfo.MultiPathType = app.GetGlobalConfig().ScsiMultiPathType
		} else if protocol == "roce" || protocol == "fc-nvme" {
			publishInfo.MultiPathType = app.GetGlobalConfig().NvmeMultiPathType
		}
		return nil
	}
}

// WithProtocol build protocol for the request parameters
func WithProtocol(protocol string) BuildParameterOption {
	return func(parameters map[string]interface{}) error {
		parameters["protocol"] = protocol
		return nil
	}
}

// WithConnector build connector for the request parameters
func WithConnector(conn connector.Connector) BuildParameterOption {
	return func(parameters map[string]interface{}) error {
		parameters["connector"] = conn
		return nil
	}
}

// WithVolumeCapability build volume capability for the request parameters
func WithVolumeCapability(ctx context.Context, req *csi.NodeStageVolumeRequest) BuildParameterOption {
	return func(parameters map[string]interface{}) error {
		volumeId := req.GetVolumeId()
		parameters["volumeId"] = volumeId

		switch req.VolumeCapability.GetAccessType().(type) {
		case *csi.VolumeCapability_Block:
			log.AddContext(ctx).Infoln("The request is to create volume of type Block")
			stagePath := req.GetStagingTargetPath() + "/" + volumeId
			parameters["stagingPath"] = stagePath
			parameters["volumeMode"] = "Block"
		case *csi.VolumeCapability_Mount:
			log.AddContext(ctx).Infoln("The request is to create volume of type filesystem")
			mnt := req.GetVolumeCapability().GetMount()
			opts := mnt.GetMountFlags()
			volumeAccessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
			accessMode := utils.GetAccessModeType(volumeAccessMode)

			if accessMode == "ReadOnly" {
				opts = append(opts, "ro")
			}

			parameters["targetPath"] = req.GetStagingTargetPath()
			parameters["fsType"] = mnt.GetFsType()
			parameters["mountFlags"] = strings.Join(opts, ",")
			parameters["accessMode"] = volumeAccessMode
			parameters["fsPermission"] = req.VolumeContext["fsPermission"]
		default:
			return errors.New("invalid volume capability")
		}
		return nil
	}
}

// ReflectToMap use reflection to convert ControllerPublishInfo to map, where key of map is json tag
// and value of map is field value
func (c *ControllerPublishInfo) ReflectToMap() map[string]interface{} {
	resultMap := map[string]interface{}{}

	ctxType := reflect.TypeOf(*c)
	ctxValue := reflect.ValueOf(*c)
	for i := 0; i < ctxType.NumField(); i++ {
		resultMap[ctxType.Field(i).Tag.Get("json")] = ctxValue.Field(i).Interface()
	}
	return resultMap
}

// ExtractWwn extract wwn from the request parameters
func ExtractWwn(parameters map[string]interface{}) (string, error) {
	publishInfo, exist := parameters["publishInfo"].(*ControllerPublishInfo)
	if !exist {
		return "", errors.New("extract wwn failed, caused by publishInfo does not exist")
	}

	protocol, exist := parameters["protocol"]
	if !exist {
		return "", errors.New("extract wwn failed, caused by protocol does not exist")
	}

	wwn := publishInfo.TgtLunWWN
	if protocol == "roce" || protocol == "fc-nvme" {
		wwn = publishInfo.TgtLunGuid
	}
	return wwn, nil
}

// Mount use nfs protocol to mount
func Mount(ctx context.Context, parameters map[string]interface{}) error {
	conn := connector.GetConnector(ctx, connector.NFSDriver)
	_, err := conn.ConnectVolume(ctx, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Mount share %v to %v error: %v",
			parameters["sourcePath"], parameters["targetPath"], err)
		return err
	}
	return nil
}

// Unmount use nfs protocol to unmount
func Unmount(ctx context.Context, targetPath string) error {
	conn := connector.GetConnector(ctx, connector.NFSDriver)
	return conn.DisConnectVolume(ctx, targetPath)
}

// NewManager build a manager instance, such as NasManager, SanManager
func NewManager(ctx context.Context, backendName string) (Manager, error) {
	config, err := getBackendConfig(ctx, backendName)
	if err != nil {
		return nil, err
	}

	if config.protocol == "nfs" && len(config.portals) == 0 {
		return nil, utils.Errorf(ctx, "portals can not be blank when protocol is %s ", config.protocol)
	}

	var portal string
	if config.protocol == "nfs" {
		portal = config.portals[0]
	}

	if config.protocol == "nfs" || config.protocol == "dpc" {
		return NewNasManager(ctx, config.protocol, portal)
	}

	return NewSanManager(ctx, config.protocol)
}

// getBackendConfig returns a BackendConfig if specified backendName exists in configmap.
// If backend doesn't exist in configmap, returns an error from call backend.GetBackendConfigmapByClaimName().
// If parameters and protocol doesn't exist, a custom error will be returned.
// If protocol exist and equal to nfs, portals in parameters must exist, otherwise an error will be returned.
func getBackendConfig(ctx context.Context, backendName string) (*BackendConfig, error) {
	claimMeta := pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName)
	configmap, err := pkgUtils.GetBackendConfigmapByClaimName(ctx, claimMeta)
	if err != nil {
		return nil, err
	}
	backendInfo, err := backend.ConvertConfigmapToMap(ctx, configmap)
	if err != nil {
		return nil, err
	}

	parameters, ok := backendInfo["parameters"].(map[string]interface{})
	if !ok {
		return nil, utils.Errorln(ctx, "convert parameters to map failed")
	}

	protocol, ok := parameters["protocol"].(string)
	if !ok {
		return nil, fmt.Errorf("protocol can not be empty, parameters:%v", parameters)
	}

	var portals []string
	if protocol == "nfs" {
		portalList, ok := parameters["portals"].([]interface{})
		if !ok || len(portalList) != 1 {
			return nil, fmt.Errorf("%s just support one portal", protocol)
		}

		for _, elem := range portalList {
			portal, ok := elem.(string)
			if !ok {
				return nil, fmt.Errorf("portals not string slice, current portal type :%s",
					reflect.TypeOf(portals[0]).Name())
			}
			portals = append(portals, portal)
		}
	}

	return &BackendConfig{protocol: protocol, portals: portals}, nil
}
