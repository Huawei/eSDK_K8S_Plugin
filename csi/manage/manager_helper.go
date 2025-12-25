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

// Package manage provides manage operations for storage
package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	_ "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/nfsplus"
	connUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/iputils"
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
			filesystemMode == storage.HyperMetroFilesystemMode && protocol == constants.ProtocolNfsPlus {
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

// WithDeviceWWN build cid mount options for the request parameters
func WithDeviceWWN(protocol, wwn string) BuildParameterOption {
	return func(parameters map[string]interface{}) error {
		if protocol != constants.ProtocolDtfs || wwn == "" {
			return nil
		}

		mountFlags, _ := utils.GetValue[string](parameters, "mountFlags")
		var opts []string
		if mountFlags != "" {
			opts = strings.Split(mountFlags, ",")
		}

		opts = append(opts, fmt.Sprintf("cid=%s", wwn))
		parameters["mountFlags"] = strings.Join(opts, ",")
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
				" Please check the storage class", fsType, constants.Ext2, constants.Ext3, constants.Ext4,
				constants.Xfs)
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
	if protocol, exist := parameters["protocol"]; exist && protocol == constants.ProtocolNfsPlus {
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
	backendConfig, err := GetBackendConfig(ctx, backendName)
	if err != nil {
		return nil, fmt.Errorf("nas manager get backendConfig failed, backendName: %s err: %w", backendName, err)
	}

	var nasProtocolList = []string{constants.ProtocolNfs, constants.ProtocolNfsPlus,
		constants.ProtocolDpc, constants.ProtocolDtfs}
	if slices.Contains(nasProtocolList, backendConfig.protocol) {
		return NewNasManager(ctx, backendConfig)
	}

	return NewSanManager(ctx, backendConfig.protocol)
}

func getDeviceWWNForDtfs(ctx context.Context, backendName string, backendInfo map[string]interface{}) (string, error) {
	_, ok := backendInfo["storageDeviceSN"]
	if ok {
		return "", nil
	}
	specifications, err := pkgUtils.GetSBCTSpecificationByClaim(ctx,
		pkgUtils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, backendName))
	if err != nil {
		return "", err
	}
	deviceWWN, ok := specifications["DeviceWWN"]
	if !ok {
		return "", fmt.Errorf("get empty DeviceWWN while use %s protocol", constants.ProtocolDtfs)
	}
	return deviceWWN, nil
}

// GetBackendConfig returns a BackendConfig if specified backendName exists in configmap.
// If backend doesn't exist in configmap, returns an error from call backend.GetBackendConfigmapByClaimName().
// If parameters and protocol doesn't exist, a custom error will be returned.
// If protocol exist and equal to nfs or nfs+, portals in parameters must exist, otherwise an error will be returned.
// If protocol is dtfs when use A-series local fs, deviceWWN must exist.
func GetBackendConfig(ctx context.Context, backendName string) (*BackendConfig, error) {
	backendInfo, err := getBackendConfigMap(ctx, backendName)
	if err != nil {
		return nil, err
	}

	parameters, ok := utils.GetValue[map[string]interface{}](backendInfo, "parameters")
	if !ok {
		return nil, fmt.Errorf("get backend info %v with invalid parameters", backendInfo)
	}
	protocol, ok := utils.GetValue[string](parameters, "protocol")
	if !ok {
		return nil, fmt.Errorf("protocol in parameters %v is invalid", parameters)
	}
	var deviceWWN string
	if protocol == constants.ProtocolDtfs {
		deviceWWN, err = getDeviceWWNForDtfs(ctx, backendName, backendInfo)
		if err != nil {
			return nil, err
		}
	}

	portalList, ok := utils.GetValue[[]interface{}](parameters, "portals")
	// portals can't be empty when protocol is nfs or nfs+
	if (!ok || len(portalList) == 0) && (protocol == constants.ProtocolNfs || protocol == constants.ProtocolNfsPlus) {
		return nil, fmt.Errorf("portals can't be empty when protocol is %s or %s",
			constants.ProtocolNfs, constants.ProtocolNfsPlus)
	}

	if protocol == constants.ProtocolNfs && len(portalList) != 1 {
		return nil, fmt.Errorf("%s just support one portal", protocol)
	}

	portals := pkgUtils.ConvertToStringSlice(portalList)
	metroPortals := make([]string, 0)
	metroBackendName, ok := utils.GetValue[string](backendInfo, "metroBackend")
	if ok && protocol == constants.ProtocolNfsPlus {
		metroPortals, err = getMetroPortals(ctx, metroBackendName)
		if err != nil {
			return nil, err
		}
	}

	storage, ok := utils.GetValue[string](backendInfo, "storage")
	if !ok {
		return nil, fmt.Errorf("storage in parameters %v is invalid", parameters)
	}

	dTreeParentName, _ := utils.GetValue[string](parameters, "parentname")
	return &BackendConfig{storage: storage, protocol: protocol, portals: portals, metroPortals: metroPortals,
		dTreeParentName: dTreeParentName, deviceWWN: deviceWWN}, nil
}

func getMetroPortals(ctx context.Context, metroBackendName string) ([]string, error) {
	metroBackendInfo, err := getBackendConfigMap(ctx, metroBackendName)
	if err != nil {
		return nil, err
	}

	metroParameters, ok := utils.GetValue[map[string]interface{}](metroBackendInfo, "parameters")
	if !ok {
		return nil, fmt.Errorf("get metro backend info %v with invalid parameters", metroBackendInfo)
	}

	metroPortalList, ok := utils.GetValue[[]interface{}](metroParameters, "portals")
	if !ok {
		return nil, errors.New("convert metro portals to slice failed")
	}

	if len(metroPortalList) == 0 {
		return nil, errors.New("metro portals can't be empty")
	}

	return pkgUtils.ConvertToStringSlice(metroPortalList), nil
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

	var opts []string
	// process volume with type is dTree
	if bk.storage == constants.OceanStorDtree || bk.storage == constants.FusionDTree {
		sourcePath, err = getDTreeSourcePath(bk, req, volumeName)
		if err != nil {
			return err
		}

		opts = getDTreeMountOptions(req)
	} else {
		opts = append(opts, "bind")
		if req.GetReadonly() {
			opts = append(opts, "ro")
		}
	}

	connectInfo := map[string]any{
		"srcType":    connector.MountFSType,
		"sourcePath": sourcePath,
		"targetPath": targetPath,
		"mountFlags": strings.Join(opts, ","),
		"protocol":   bk.protocol,
		"portals":    bk.portals,
	}

	if err = Mount(ctx, connectInfo); err != nil {
		log.AddContext(ctx).Errorf("Mount share %s to %s error: %v", sourcePath, targetPath, err)
		return err
	}

	log.AddContext(ctx).Infof("Volume %s is node published to %s", volumeId, targetPath)
	return nil
}

func getDTreeMountOptions(req *csi.NodePublishVolumeRequest) []string {
	var opts []string
	if req.GetVolumeCapability() != nil && req.GetVolumeCapability().GetMount() != nil &&
		req.GetVolumeCapability().GetMount().GetMountFlags() != nil {
		opts = req.GetVolumeCapability().GetMount().GetMountFlags()
	}

	volumeAccessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	accessMode := utils.GetAccessModeType(volumeAccessMode)
	if accessMode == "ReadOnly" {
		opts = append(opts, "ro")
	}

	return opts
}

func getDTreeSourcePath(bk *BackendConfig, req *csi.NodePublishVolumeRequest,
	volumeName string) (string, error) {
	parentName := bk.dTreeParentName

	if publishInfo, exist := req.GetPublishContext()["publishInfo"]; exist {
		dtreePublishInfo := &DTreePublishInfo{}
		err := json.Unmarshal([]byte(publishInfo), dtreePublishInfo)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal dtree publish info: %w", err)
		}

		if dtreePublishInfo.DTreeParentName != "" {
			parentName = dtreePublishInfo.DTreeParentName
		}
	}

	if parentName == "" {
		return "", fmt.Errorf("failed to get dtree parent name," +
			" please ensure that the attachRequired parameter is enabled")
	}

	sourcePathPrefix, err := generatePathPrefixByProtocol(bk.protocol, bk.portals)
	if err != nil {
		return "", fmt.Errorf("generate dtree path prefix failed, error: %v", err)
	}

	return sourcePathPrefix + parentName + "/" + volumeName, nil
}

// generatePathPrefixByProtocol used to get source path prefix
// e.g.
//   - For ProtocolNfs with IPv4 portal "192.168.1.1", it returns "192.168.1.1:/"
//   - For ProtocolNfs with IPv6 portal "2001:db8::1", it returns "[2001:db8::1]:/"
//   - For ProtocolDpc, it returns "/"
//   - For unsupported protocols, it returns an error
func generatePathPrefixByProtocol(protocol string, portals []string) (string, error) {
	switch protocol {
	case constants.ProtocolNfs, constants.ProtocolNfsPlus:
		if len(portals) == 0 {
			return "", fmt.Errorf("no portal provided for NFS or NFS+ protocol")
		}
		wrapper := iputils.NewIPDomainWrapper(portals[0])
		if wrapper == nil {
			return "", fmt.Errorf("portal [%s] is invalid", portals[0])
		}
		return wrapper.GetFormatPortalIP() + ":/", nil
	case constants.ProtocolDpc, constants.ProtocolDtfs:
		return "/", nil
	default:
		return "", fmt.Errorf("protocol [%s] is not supported", protocol)
	}
}

func getConnectorByProtocol(ctx context.Context, protocol string) connector.VolumeConnector {
	return map[string]connector.VolumeConnector{
		constants.ProtocolNfs:     connector.GetConnector(ctx, connector.NFSDriver),
		constants.ProtocolDpc:     connector.GetConnector(ctx, connector.NFSDriver),
		constants.ProtocolDtfs:    connector.GetConnector(ctx, connector.NFSDriver),
		constants.ProtocolNfsPlus: connector.GetConnector(ctx, connector.NFSPlusDriver),
	}[protocol]
}
