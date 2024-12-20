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

// Package manage provides manage operations for storage
package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	_ "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/nfsplus"
	connUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
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

// WithPortals build portals for the request parameters
func WithPortals(publishContext map[string]string, protocol string, portals, metroPortals []string,
) BuildParameterOption {
	return func(parameters map[string]interface{}) error {
		if filesystemMode, ok := publishContext["filesystemMode"]; ok &&
			filesystemMode == client.HyperMetroFilesystemMode && protocol == plugin.ProtocolNfsPlus {
			newPortals := append(portals, metroPortals...)
			parameters["portals"] = newPortals
			return nil
		}

		parameters["portals"] = portals
		return nil
	}
}

// WithConnector build connector for the request parameters
func WithConnector(conn connector.VolumeConnector) BuildParameterOption {
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

// CheckParam check node stage volume request parameters
func CheckParam(ctx context.Context, req *csi.NodeStageVolumeRequest) error {
	switch req.VolumeCapability.GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
	case *csi.VolumeCapability_Mount:
		fsType := utils.ToStringSafe(req.GetVolumeCapability().GetMount().GetFsType())
		if fsType != "" && !utils.IsContain(constants.FileType(fsType),
			[]constants.FileType{constants.Ext2, constants.Ext3, constants.Ext4, constants.Xfs}) {
			return utils.Errorf(ctx, "fsType %v is not correct. [%v, %v, %v, %v] are support,"+
				" Please check the storage class", fsType, constants.Ext2, constants.Ext3, constants.Ext4, constants.Xfs)
		}
	default:
		return errors.New("invalid volume capability")
	}
	return nil

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
	if protocol, exist := parameters["protocol"]; exist && protocol == plugin.ProtocolNfsPlus {
		conn = connector.GetConnector(ctx, connector.NFSPlusDriver)
	}

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
func NewManager(ctx context.Context, backendName string) (VolumeManager, error) {
	backend, err := GetBackendConfig(ctx, backendName)
	if err != nil {
		log.AddContext(ctx).Errorf("nas manager get backend failed, backendName: %s err: %v", backendName, err)
		return nil, err
	}

	switch backend.protocol {
	case plugin.ProtocolNfs:
		if len(backend.portals) != 1 {
			return nil, utils.Errorf(ctx, "portals must be one when protocol is %s", plugin.ProtocolNfs)
		}
		return NewNasManager(ctx, backend.protocol, backend.dTreeParentName, backend.portals[0:1], []string{})
	case plugin.ProtocolNfsPlus:
		if len(backend.portals) == 0 {
			return nil, utils.Errorf(ctx, "portals can not be blank when protocol is %s", plugin.ProtocolNfsPlus)
		}
		return NewNasManager(ctx, backend.protocol, backend.dTreeParentName, backend.portals, backend.metroPortals)
	case plugin.ProtocolDpc:
		return NewNasManager(ctx, backend.protocol, backend.dTreeParentName, []string{}, []string{})
	default:
		return NewSanManager(ctx, backend.protocol)
	}
}

// GetBackendConfig returns a BackendConfig if specified backendName exists in configmap.
// If backend doesn't exist in configmap, returns an error from call backend.GetBackendConfigmapByClaimName().
// If parameters and protocol doesn't exist, a custom error will be returned.
// If protocol exist and equal to nfs or nfs+, portals in parameters must exist, otherwise an error will be returned.
func GetBackendConfig(ctx context.Context, backendName string) (*BackendConfig, error) {
	backendInfo, err := getBackendConfigMap(ctx, backendName)
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
	portalList, ok := parameters["portals"].([]interface{})
	// portals can't be empty when protocol is nfs or nfs+
	if (!ok || len(portalList) == 0) && (protocol == plugin.ProtocolNfs || protocol == plugin.ProtocolNfsPlus) {
		return nil, errors.New("portals can't be empty")
	}
	if protocol == plugin.ProtocolNfs && len(portalList) != 1 {
		return nil, fmt.Errorf("%s just support one portal", protocol)
	}
	portals := pkgUtils.ConvertToStringSlice(portalList)
	metroPortals := make([]string, 0)
	if metroBackendName, ok := backendInfo["metroBackend"].(string); ok && protocol == plugin.ProtocolNfsPlus {
		metroBackendInfo, err := getBackendConfigMap(ctx, metroBackendName)
		if err != nil {
			return nil, err
		}
		metroParameters, ok := metroBackendInfo["parameters"].(map[string]interface{})
		if !ok {
			return nil, utils.Errorln(ctx, "convert metro parameters to map failed")
		}
		metroPortalList, ok := metroParameters["portals"].([]interface{})
		if !ok {
			return nil, errors.New("convert metro portals to slice failed")
		}
		if len(metroPortalList) == 0 {
			return nil, errors.New("metro portals can't be empty")
		}
		metroPortals = pkgUtils.ConvertToStringSlice(metroPortalList)
	}

	storage, ok := backendInfo["storage"]
	var dTreeParentName string
	if ok && storage == "oceanstor-dtree" {
		dTreeParentName, _ = utils.ToStringWithFlag(parameters["parentname"])
	}

	return &BackendConfig{protocol: protocol, portals: portals, metroPortals: metroPortals,
		dTreeParentName: dTreeParentName}, nil
}

func getBackendConfigMap(ctx context.Context, backendName string) (map[string]interface{}, error) {
	claimMeta := pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName)
	configmap, err := pkgUtils.GetBackendConfigmapByClaimName(ctx, claimMeta)
	if err != nil {
		return nil, err
	}
	backendInfo, err := backend.ConvertConfigmapToMap(ctx, configmap)
	if err != nil {
		return nil, err
	}

	return backendInfo, nil
}

// PublishBlock publish block device
func PublishBlock(ctx context.Context, req *csi.NodePublishVolumeRequest) error {
	volumeId := req.GetVolumeId()
	sourcePath := req.GetStagingTargetPath()
	targetPath := req.GetTargetPath()
	// If the request is to publish raw block device then create symlink of the device
	// from the staging are to publish. Do not create fs and mount
	log.AddContext(ctx).Infoln("Bind mount for the staged device on the node to publish")
	sourcePath = sourcePath + "/" + volumeId
	err := connUtils.BindMountRawBlockDevice(ctx, sourcePath, targetPath,
		req.GetVolumeCapability().GetMount().GetMountFlags())
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to bind mount for the staging path [%v] to target path [%v]",
			sourcePath, targetPath)
		return err
	}

	log.AddContext(ctx).Infof("Raw Block Volume %s is node published to %s", volumeId, targetPath)
	return nil
}

// PublishFilesystem publish filesystem
func PublishFilesystem(ctx context.Context, req *csi.NodePublishVolumeRequest) error {
	volumeId := req.GetVolumeId()
	sourcePath := req.GetStagingTargetPath()
	targetPath := req.GetTargetPath()
	backendName, volumeName := utils.SplitVolumeId(volumeId)
	bk, err := GetBackendConfig(ctx, backendName)
	if err != nil {
		log.AddContext(ctx).Errorf("publish get backend failed, backendName: %s err: %v", backendName, err)
		return err
	}

	protocol := plugin.ProtocolNfs
	opts := []string{"bind"}
	// process volume with type is dTree
	if bk.dTreeParentName != "" {
		sourcePath = bk.portals[0] + ":/" + bk.dTreeParentName + "/" + volumeName
		protocol = bk.protocol
		if req.GetVolumeCapability() != nil && req.GetVolumeCapability().GetMount() != nil &&
			req.GetVolumeCapability().GetMount().GetMountFlags() != nil {
			opts = req.GetVolumeCapability().GetMount().GetMountFlags()
		} else {
			opts = make([]string, 0)
		}
	}
	if req.GetReadonly() {
		opts = append(opts, "ro")
	}

	connectInfo := map[string]interface{}{
		"srcType":    connector.MountFSType,
		"sourcePath": sourcePath,
		"targetPath": targetPath,
		"mountFlags": strings.Join(opts, ","),
		"protocol":   protocol,
		"portals":    bk.portals,
	}

	if err = Mount(ctx, connectInfo); err != nil {
		log.AddContext(ctx).Errorf("Mount share %s to %s error: %v", sourcePath, targetPath, err)
		return err
	}

	log.AddContext(ctx).Infof("Volume %s is node published to %s", volumeId, targetPath)
	return nil
}

func getConnectorByProtocol(ctx context.Context, protocol string) connector.VolumeConnector {
	return map[string]connector.VolumeConnector{
		plugin.ProtocolNfs:     connector.GetConnector(ctx, connector.NFSDriver),
		plugin.ProtocolDpc:     connector.GetConnector(ctx, connector.NFSDriver),
		plugin.ProtocolNfsPlus: connector.GetConnector(ctx, connector.NFSPlusDriver),
	}[protocol]
}
