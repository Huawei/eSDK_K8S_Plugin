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

package driver

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/helper"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/handler"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	// RWX defines access mode RWX
	RWX = "ReadWriteMany"
	// Block defines volume mode block
	Block = "Block"
	// FileSystem defines volume mode filesystem
	FileSystem = "FileSystem"

	maxDescriptionLength = 255

	volumeTypeDTree               = "dtree"
	volumeTypeFileSystem          = "fs"
	volumeTypeLun                 = "lun"
	maxReservedSnapshotSpaceRatio = 50
)

var (
	nfsProtocolMap = map[string]string{
		// nfsvers=3.0 is not support
		"nfsvers=3":   "nfs3",
		"nfsvers=4":   "nfs4",
		"nfsvers=4.0": "nfs4",
		"nfsvers=4.1": "nfs41",
		"nfsvers=4.2": "nfs42",
	}

	annManageVolumeName  = "/manageVolumeName"
	annManageBackendName = "/manageBackendName"
	annFileSystemMode    = "/fileSystemMode"
	annVolumeName        = "/volumeName"
)

func addNFSProtocol(ctx context.Context, mountFlag string, parameters map[string]interface{}) error {
	for _, singleFlag := range strings.Split(mountFlag, ",") {
		singleFlag = strings.TrimSpace(singleFlag)
		if strings.HasPrefix(singleFlag, "nfsvers=") {
			value, ok := nfsProtocolMap[singleFlag]
			if !ok {
				return utils.Errorf(ctx, "unsupported nfs protocol version [%s].", singleFlag)
			}

			if parameters["nfsProtocol"] != nil {
				return utils.Errorf(ctx, "Duplicate nfs protocol [%s].", mountFlag)
			}

			parameters["nfsProtocol"] = value
			log.AddContext(ctx).Infof("Add nfs protocol: %v", parameters["nfsProtocol"])
		}
	}

	return nil
}

func processNFSProtocol(ctx context.Context, req *csi.CreateVolumeRequest,
	parameters map[string]interface{}) error {
	for _, v := range req.GetVolumeCapabilities() {
		for _, mountFlag := range v.GetMount().GetMountFlags() {
			err := addNFSProtocol(ctx, mountFlag, parameters)
			if err != nil {
				return err
			}
		}

		if parameters["nfsProtocol"] != nil {
			break
		}
	}

	return nil
}

func verifyExpandArguments(ctx context.Context, req *csi.ControllerExpandVolumeRequest, backend *model.Backend) error {
	if req.GetCapacityRange() == nil {
		return errors.New("no capacity range provided")
	}

	minSize := req.GetCapacityRange().GetRequiredBytes()
	maxSize := req.GetCapacityRange().GetLimitBytes()
	if 0 < maxSize && maxSize < minSize {
		return errors.New("limitBytes is smaller than requiredBytes")
	}

	if support, err := isSupportExpandVolume(ctx, req, backend); !support {
		return err
	}

	err := verifySectorSize(ctx, req.GetVolumeId(), backend, minSize)
	if err != nil {
		return err
	}

	return nil
}

func verifySectorSize(ctx context.Context, volumeId string, backend *model.Backend, minSize int64) error {
	volumeAttrs, err := app.GetGlobalConfig().K8sUtils.GetVolumeAttrByVolumeId(volumeId)
	if err != nil {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("failed to verify expand arguments: %v", err))
	}

	return utils.IsCapacityAvailable(minSize, backend.Plugin.GetSectorSize(), utils.CopyMap(volumeAttrs))
}

func isSupportExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest, b *model.Backend) (
	bool, error) {
	if b.Storage == constants.OceanStorNas || b.Storage == constants.OceanStorDtree ||
		b.Storage == constants.FusionNas || b.Storage == constants.FusionDTree {
		log.AddContext(ctx).Debugf("Storage is [%s], support expand volume.", b.Storage)
		return true, nil
	}

	volumeCapability := req.GetVolumeCapability()
	if volumeCapability == nil {
		return false, utils.Errorln(ctx, "Expand volume failed, req.GetVolumeCapability() is empty.")
	}

	if volumeCapability.GetAccessMode().GetMode() == csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER &&
		volumeCapability.GetBlock() == nil {
		return false, utils.Errorf(ctx, "The PVC %s is a \"lun\" type, volumeMode is \"Filesystem\", "+
			"accessModes is \"ReadWriteMany\", can not support expand volume.", req.GetVolumeId())
	}

	if volumeCapability.GetAccessMode().GetMode() == csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY {
		return false, utils.Errorf(ctx, "The PVC %s is a \"lun\" type,  accessModes is \"ReadOnlyMany\", "+
			"can not support expand volume.", req.GetVolumeId())
	}

	return true, nil
}

func validateModeAndType(req *csi.CreateVolumeRequest, parameters map[string]interface{}) string {
	// validate volumeMode and volumeType
	volumeCapabilities := req.GetVolumeCapabilities()
	if len(volumeCapabilities) == 0 {
		return "Volume Capabilities missing in request"
	}

	var volumeMode string
	var accessMode string
	for _, mode := range volumeCapabilities {
		if mode.GetBlock() != nil {
			volumeMode = Block
		} else {
			volumeMode = FileSystem
		}
		if mode.GetAccessMode().GetMode() == csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER {
			accessMode = RWX
		}
	}

	if volumeMode == Block &&
		(parameters["volumeType"] == volumeTypeFileSystem || parameters["volumeType"] == volumeTypeDTree) {
		return fmt.Sprintf("VolumeMode is block but volumeType is %s. Please check the storage class",
			parameters["volumeType"])
	}

	if accessMode == RWX && volumeMode == FileSystem && parameters["volumeType"] == volumeTypeLun {
		return "If volumeType in the sc.yaml file is set to \"lun\" and volumeMode in the pvc.yaml file is " +
			"set to \"Filesystem\", accessModes in the pvc.yaml file cannot be set to \"ReadWriteMany\"."
	}

	fsType := utils.ToStringSafe(parameters["fsType"])
	if fsType != "" && !utils.IsContain(constants.FileType(fsType), []constants.FileType{constants.Ext2,
		constants.Ext3, constants.Ext4, constants.Xfs}) {
		return fmt.Sprintf("fsType %v is not correct, [%v, %v, %v, %v] are support."+
			" Please check the storage class ", fsType, constants.Ext2, constants.Ext3, constants.Ext4, constants.Xfs)
	}

	return ""
}

func processAccessibilityRequirements(ctx context.Context, req *csi.CreateVolumeRequest,
	parameters map[string]interface{}) {

	accessibleTopology := req.GetAccessibilityRequirements()
	if accessibleTopology == nil {
		log.AddContext(ctx).Infoln("Empty accessibility requirements in create volume request")
		return
	}

	var requisiteTopologies = make([]map[string]string, 0)
	for _, requisite := range accessibleTopology.GetRequisite() {
		requirement := make(map[string]string)
		for k, v := range requisite.GetSegments() {
			requirement[k] = v
		}
		requisiteTopologies = append(requisiteTopologies, requirement)
	}

	var preferredTopologies = make([]map[string]string, 0)
	for _, preferred := range accessibleTopology.GetPreferred() {
		preference := make(map[string]string)
		for k, v := range preferred.GetSegments() {
			preference[k] = v
		}
		preferredTopologies = append(preferredTopologies, preference)
	}

	parameters[backend.Topology] = backend.AccessibleTopology{
		RequisiteTopologies: requisiteTopologies,
		PreferredTopologies: preferredTopologies,
	}

	log.AddContext(ctx).Infof("accessibility Requirements in create volume %+v", parameters[backend.Topology])
}

func processVolumeContentSource(ctx context.Context, req *csi.CreateVolumeRequest,
	parameters map[string]interface{}) error {

	contentSource := req.GetVolumeContentSource()
	if contentSource == nil {
		return nil
	}

	if contentSnapshot := contentSource.GetSnapshot(); contentSnapshot != nil {
		sourceSnapshotId := contentSnapshot.GetSnapshotId()
		sourceBackendName, snapshotParentId, sourceSnapshotName := utils.SplitSnapshotId(sourceSnapshotId)
		parameters["sourceSnapshotName"] = sourceSnapshotName
		parameters["snapshotParentId"] = snapshotParentId
		parameters["backend"] = sourceBackendName
		log.AddContext(ctx).Infof("Start to create volume from snapshot %s, param: %+v",
			sourceSnapshotName, parameters)
	} else if contentVolume := contentSource.GetVolume(); contentVolume != nil {
		sourceVolumeId := contentVolume.GetVolumeId()
		sourceBackendName, sourceVolumeName := utils.SplitVolumeId(sourceVolumeId)
		parameters["sourceVolumeName"] = sourceVolumeName
		parameters["backend"] = sourceBackendName
		log.AddContext(ctx).Infof("Start to create volume from volume %s", sourceVolumeName)
	} else {
		log.AddContext(ctx).Errorf("The source [%+v] is not snapshot either volume", contentSource)
		return status.Error(codes.InvalidArgument, "the source ID provided by user is invalid")
	}

	return nil
}

func getAccessibleTopologies(ctx context.Context, req *csi.CreateVolumeRequest,
	pool *model.StoragePool) []*csi.Topology {
	accessibleTopologies := make([]*csi.Topology, 0)
	if req.GetAccessibilityRequirements() != nil &&
		len(req.GetAccessibilityRequirements().GetRequisite()) != 0 {
		supportedTopology := handler.NewCacheWrapper().LoadCacheBackendTopologies(ctx, pool.Parent)
		if len(supportedTopology) > 0 {
			for _, segment := range supportedTopology {
				accessibleTopologies = append(accessibleTopologies, &csi.Topology{Segments: segment})
			}
		}
	}
	return accessibleTopologies
}

func getAttributes(req *csi.CreateVolumeRequest, vol utils.Volume, backendName string) map[string]string {
	attributes := map[string]string{
		"backend":                          backendName,
		"name":                             vol.GetVolumeName(),
		"fsPermission":                     req.Parameters["fsPermission"],
		constants.DTreeParentKey:           vol.GetDTreeParentName(),
		constants.DisableVerifyCapacityKey: req.Parameters[constants.DisableVerifyCapacityKey],
	}

	if lunWWN, err := vol.GetLunWWN(); err == nil {
		attributes["lunWWN"] = lunWWN
	}
	return attributes
}

func getVolumeResponse(accessibleTopologies []*csi.Topology,
	attributes map[string]string,
	volumeId string, size int64) *csi.Volume {
	return &csi.Volume{
		VolumeId:           volumeId,
		CapacityBytes:      size,
		VolumeContext:      attributes,
		AccessibleTopology: accessibleTopologies,
	}
}

func makeCreateVolumeResponse(ctx context.Context, req *csi.CreateVolumeRequest, vol utils.Volume,
	pool *model.StoragePool) *csi.Volume {
	contentSource := req.GetVolumeContentSource()

	accessibleTopologies := getAccessibleTopologies(ctx, req, pool)
	attributes := getAttributes(req, vol, pool.Parent)
	csiVolume := getVolumeResponse(accessibleTopologies, attributes, pool.Parent+"."+vol.GetVolumeName(), vol.GetSize())
	if contentSource != nil {
		csiVolume.ContentSource = contentSource
	}

	return csiVolume
}

func checkStorageClassParameters(ctx context.Context, parameters map[string]interface{}) error {
	// check fsPermission parameter in sc
	err := checkFsPermission(ctx, parameters)
	if err != nil {
		return err
	}

	// check reservedSnapshotSpaceRatio parameter in sc
	err = checkReservedSnapshotSpaceRatio(ctx, parameters)
	if err != nil {
		return err
	}

	return nil
}

func checkFsPermission(ctx context.Context, parameters map[string]interface{}) error {
	fsPermission, exist := parameters["fsPermission"].(string)
	if !exist {
		return nil
	}

	reg := regexp.MustCompile(`^[0-7][0-7][0-7]$`)
	match := reg.FindStringSubmatch(fsPermission)
	if match == nil {
		errMsg := fmt.Sprintf("fsPermission [%s] in storageClass.yaml format must be [0-7][0-7][0-7].", fsPermission)
		log.AddContext(ctx).Errorln(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

func processDescription(ctx context.Context, parameters map[string]interface{}) error {
	description, exist := parameters["description"].(string)
	if !exist {
		// Set description default value
		parameters["description"] = "Created from Kubernetes CSI"
		return nil
	}

	if len(description) > maxDescriptionLength {
		errMsg := fmt.Sprintf("StorageClass parameter \"description\": [%v] invalid, the length exceeds %d.",
			description, maxDescriptionLength)
		log.AddContext(ctx).Errorln(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

func processParentName(ctx context.Context, parameters map[string]interface{}) error {
	parentNameParam, exist := parameters["parentname"]
	if !exist {
		return nil
	}

	parentName, ok := parentNameParam.(string)
	if !ok {
		return fmt.Errorf("parentname in StorageClass must be a string type, but got: %v", parameters["parentname"])
	}
	if parentName == "" {
		return nil
	}

	if _, exist := parameters["backend"]; !exist {
		return fmt.Errorf("when parentname is configured in StorageClass, backend must be configured together")
	}

	return nil
}

func checkReservedSnapshotSpaceRatio(ctx context.Context, parameters map[string]interface{}) error {
	reservedSnapshotSpaceRatioString, exist := parameters["reservedSnapshotSpaceRatio"].(string)
	if !exist {
		return nil
	}

	reservedSnapshotSpaceRatio, err := strconv.Atoi(reservedSnapshotSpaceRatioString)
	if err != nil {
		errMsg := fmt.Sprintf("Convert [%s] to int failed, please check parameter reservedSnapshotSpaceRatio "+
			"in storageclass.", reservedSnapshotSpaceRatioString)
		log.AddContext(ctx).Errorln(errMsg)
		return errors.New(errMsg)
	}

	if reservedSnapshotSpaceRatio < 0 || reservedSnapshotSpaceRatio > maxReservedSnapshotSpaceRatio {
		errMsg := fmt.Sprintf("reservedSnapshotSpaceRatio: [%v] must in range [0, 50], please check this "+
			"parameter in storageclass.", reservedSnapshotSpaceRatioString)
		log.AddContext(ctx).Errorln(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

func checkCreateVolumeRequest(ctx context.Context, req *csi.CreateVolumeRequest) error {
	capacityRange := req.GetCapacityRange()
	if capacityRange == nil || capacityRange.RequiredBytes <= 0 {
		msg := "CreateVolume CapacityRange must be provided"
		log.AddContext(ctx).Errorln(msg)
		return status.Error(codes.InvalidArgument, msg)
	}

	parameters := utils.CopyMap(req.GetParameters())
	err := checkStorageClassParameters(ctx, parameters)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	msg := validateModeAndType(req, parameters)
	if msg != "" {
		log.AddContext(ctx).Errorln(msg)
		return status.Error(codes.InvalidArgument, msg)
	}

	return nil
}

func processCreateVolumeParameters(ctx context.Context, req *csi.CreateVolumeRequest) (map[string]interface{}, error) {
	parameters := utils.CopyMap(req.GetParameters())

	parameters["size"] = req.GetCapacityRange().RequiredBytes

	backendName, exist := parameters["backend"].(string)
	if exist {
		parameters["backend"] = helper.GetBackendName(backendName)
	}

	cloneFrom, exist := parameters["cloneFrom"].(string)
	if exist && cloneFrom != "" {
		parameters["backend"], parameters["cloneFrom"] = utils.SplitVolumeId(cloneFrom)
	}

	// process volume content source, snapshot or clone
	err := processVolumeContentSource(ctx, req, parameters)
	if err != nil {
		return parameters, err
	}

	// process accessibility requirements, topology
	processAccessibilityRequirements(ctx, req, parameters)

	err = processNFSProtocol(ctx, req, parameters)
	if err != nil {
		return nil, err
	}

	// process description parameter in sc
	err = processDescription(ctx, parameters)
	if err != nil {
		return nil, err
	}

	if err := processParentName(ctx, parameters); err != nil {
		return nil, err
	}

	return parameters, nil
}

func processCreateVolumeParametersAfterSelect(parameters map[string]interface{},
	localPool *model.StoragePool, remotePool *model.StoragePool) error {

	parameters["storagepool"] = localPool.Name
	if remotePool != nil {
		parameters["metroDomain"] = backend.GetMetroDomain(remotePool.Parent)
		parameters["vStorePairID"] = backend.GetMetrovStorePairID(remotePool.Parent)
		parameters["remoteStoragePool"] = remotePool.Name
	}

	parameters["accountName"] = backend.GetAccountName(localPool.Parent)

	size := utils.GetValueOrFallback(parameters, "size", int64(0))
	return utils.IsCapacityAvailable(size, localPool.Plugin.GetSectorSize(), parameters)
}

// createVolume used to create a lun/filesystem in huawei storage
func (d *CsiDriver) createVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	parameters, err := processCreateVolumeParameters(ctx, req)
	if err != nil {
		return nil, err
	}
	storagePoolPair, err := d.backendSelector.SelectPoolPair(ctx, req.GetCapacityRange().RequiredBytes, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot select pool for volume creation: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = processCreateVolumeParametersAfterSelect(parameters, storagePoolPair.Local, storagePoolPair.Remote)
	if err != nil {
		log.AddContext(ctx).Errorln(err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	vol, err := storagePoolPair.Local.Plugin.CreateVolume(ctx, req.GetName(), parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Create volume %s error: %v", req.GetName(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	recordCapacityChanged(ctx, req.GetCapacityRange().GetRequiredBytes(),
		vol.GetSize(), storagePoolPair.Local.Plugin.GetSectorSize())

	log.AddContext(ctx).Infof("Volume %s is created", req.GetName())
	res := &csi.CreateVolumeResponse{
		Volume: makeCreateVolumeResponse(ctx, req, vol, storagePoolPair.Local),
	}

	return res, nil
}

func recordCapacityChanged(ctx context.Context, required, actual, sectorSize int64) {
	if required < actual {
		log.AddContext(ctx).Infof("Required capacity is %d, actual capacity is %d, "+
			"when this parameter is set to true,"+
			" if required capacity is not an integer multiple of the sector size %d,"+
			" it is automatically filled up.", required, actual, sectorSize)
	}
}

// In the volume import scenario, only the fields in the annotation are obtained.
// Other information are ignored (e.g. the capacity, backend, and QoS ...).
func (d *CsiDriver) manageVolume(ctx context.Context, req *csi.CreateVolumeRequest, volumeName, backendName string) (
	*csi.CreateVolumeResponse, error) {
	log.AddContext(ctx).Infof("Start to manage Volume %s for backend %s.", volumeName, backendName)
	selectBackend, err := d.backendSelector.SelectBackend(ctx, helper.GetBackendName(backendName))
	if selectBackend == nil {
		log.AddContext(ctx).Errorf("Backend %s doesn't exist. Manage Volume %s failed.",
			helper.GetBackendName(backendName), volumeName)
		return &csi.CreateVolumeResponse{}, fmt.Errorf("backend %s doesn't exist. Manage Volume %s failed",
			backendName, volumeName)
	}

	// clone volume can not be set when manage volume
	if req.GetVolumeContentSource() != nil {
		return &csi.CreateVolumeResponse{}, utils.Errorf(ctx,
			"Manage volume %s can not set the source content.", volumeName)
	}

	parameters, err := processCreateVolumeParameters(ctx, req)
	if err != nil {
		return nil, err
	}

	// valid the backend basic info, such as: volumeType, allocType, authClient
	if err = backend.ValidateBackend(ctx, selectBackend, parameters); err != nil {
		return nil, err
	}

	vol, err := selectBackend.Plugin.QueryVolume(ctx, volumeName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Query volume %s error: %v", req.GetName(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = validateCapacity(ctx, req, vol)
	if err != nil {
		log.AddContext(ctx).Errorf("Validate capacity %s error: %v", req.GetName(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	accessibleTopologies := getAccessibleTopologies(ctx, req, selectBackend.Pools[0])
	attributes := getAttributes(req, vol, backendName)

	log.AddContext(ctx).Infof("Volume %s is created by manage", req.GetName())

	res := &csi.CreateVolumeResponse{
		Volume: getVolumeResponse(accessibleTopologies, attributes, backendName+"."+volumeName,
			req.GetCapacityRange().GetRequiredBytes()),
	}

	return res, nil
}

func validateCapacity(ctx context.Context, req *csi.CreateVolumeRequest, vol utils.Volume) error {
	actualCapacity := vol.GetSize()
	if actualCapacity == 0 {
		return errors.New("empty Size")
	}

	if actualCapacity != req.GetCapacityRange().RequiredBytes {
		return utils.Errorf(ctx, "the actual capacity %d is different from PVC storage size %d",
			actualCapacity, req.GetCapacityRange().RequiredBytes)
	}
	return nil
}

func processAnnotations(annotations map[string]string, req *csi.CreateVolumeRequest) error {
	fileSystemMode, systemModeOk := annotations[app.GetGlobalConfig().DriverName+annFileSystemMode]
	if systemModeOk && (fileSystemMode != "HyperMetro" && fileSystemMode != "local") {
		return errors.New("The value of filesystemMode can only be \"HyperMetro\" or \"local\".")
	}
	if systemModeOk {
		req.Parameters["fileSystemMode"] = fileSystemMode
	}

	volumeName, volumeNameOk := annotations[app.GetGlobalConfig().DriverName+annVolumeName]
	if volumeNameOk && volumeName == "" {
		return errors.New("The volume cannot be empty")
	}
	if volumeNameOk {
		req.Parameters["annVolumeName"] = volumeName
	}
	return nil
}

func getBackendFilesystemMode(ctx context.Context, bk *model.Backend, volName string) string {
	if protocol, ok := bk.Parameters["protocol"].(string); ok && protocol == plugin.ProtocolNfsPlus &&
		bk.Storage != constants.OceanStorDtree && bk.Storage != constants.FusionDTree {
		volume, err := bk.Plugin.QueryVolume(ctx, volName, map[string]interface{}{
			"description": "Query from Huawei Storage",
			"size":        int64(0),
		})
		if err != nil {
			log.AddContext(ctx).Errorf("query volume failed, volName: %s, err: %v", volName, err)
			return ""
		}
		log.AddContext(ctx).Debugf("controllerPublishVolume volume, volumeName: %s, volume: %+v", volName, volume)
		return volume.GetFilesystemMode()
	}
	return ""
}
