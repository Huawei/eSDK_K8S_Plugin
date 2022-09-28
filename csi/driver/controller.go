/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	RWX        = "ReadWriteMany"
	Block      = "Block"
	FileSystem = "FileSystem"
)

var nfsProtocolMap = map[string]string{
	// nfsvers=3.0 is not support
	"nfsvers=3":   "nfs3",
	"nfsvers=4":   "nfs4",
	"nfsvers=4.0": "nfs4",
	"nfsvers=4.1": "nfs41",
}

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)

	volumeName := req.GetName()
	log.AddContext(ctx).Infof("Start to create volume %s", volumeName)

	capacityRange := req.GetCapacityRange()
	if capacityRange == nil || capacityRange.RequiredBytes <= 0 {
		msg := "CreateVolume CapacityRange must be provided"
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	parameters := utils.CopyMap(req.GetParameters())
	err := d.checkStorageClassParameters(ctx, parameters)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	size := capacityRange.RequiredBytes
	parameters["size"] = capacityRange.RequiredBytes

	cloneFrom, exist := parameters["cloneFrom"].(string)
	if exist && cloneFrom != "" {
		parameters["backend"], parameters["cloneFrom"] = utils.SplitVolumeId(cloneFrom)
	}

	// process volume content source. snapshot or clone
	err = d.processVolumeContentSource(ctx, req, parameters)
	if err != nil {
		return nil, err
	}

	// process accessibility requirements. Topology
	d.processAccessibilityRequirements(ctx, req, parameters)
	err = d.processNFSProtocol(ctx, req, parameters)
	if err != nil {
		return nil, err
	}

	msg := d.validateModeAndType(req, parameters)
	if msg != "" {
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	localPool, remotePool, err := backend.SelectStoragePool(ctx, size, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot select pool for volume creation: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	parameters["storagepool"] = localPool.Name
	if remotePool != nil {
		parameters["metroDomain"] = backend.GetMetroDomain(remotePool.Parent)
		parameters["vStorePairID"] = backend.GetMetrovStorePairID(remotePool.Parent)
		parameters["remoteStoragePool"] = remotePool.Name
	}

	parameters["accountName"] = backend.GetAccountName(localPool.Parent)

	vol, err := localPool.Plugin.CreateVolume(ctx, volumeName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Create volume %s error: %v", volumeName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	volume, err := d.getCreatedVolume(ctx, req, vol, localPool)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is created", volumeName)
	return &csi.CreateVolumeResponse{
		Volume: volume,
	}, nil
}

func (d *Driver) checkStorageClassParameters(ctx context.Context, parameters map[string]interface{}) error {
	// check fsPermission parameter in sc
	err := d.checkFsPermission(ctx, parameters)
	if err != nil {
		return err
	}

	return nil
}

func (d *Driver) checkFsPermission(ctx context.Context, parameters map[string]interface{}) error {
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

func (d *Driver) getCreatedVolume(ctx context.Context, req *csi.CreateVolumeRequest, vol utils.Volume,
	pool *backend.StoragePool) (*csi.Volume, error) {
	contentSource := req.GetVolumeContentSource()
	size := req.GetCapacityRange().GetRequiredBytes()

	accessibleTopologies := make([]*csi.Topology, 0)
	if req.GetAccessibilityRequirements() != nil &&
		len(req.GetAccessibilityRequirements().GetRequisite()) != 0 {
		supportedTopology := pool.GetSupportedTopologies(ctx)
		if len(supportedTopology) > 0 {
			for _, segment := range supportedTopology {
				accessibleTopologies = append(accessibleTopologies, &csi.Topology{Segments: segment})
			}
		}
	}

	volName := vol.GetVolumeName()
	attributes := map[string]string{
		"backend":      pool.Parent,
		"name":         volName,
		"fsPermission": req.Parameters["fsPermission"],
	}

	if lunWWN, err := vol.GetLunWWN(); err == nil {
		attributes["lunWWN"] = lunWWN
	}

	csiVolume := &csi.Volume{
		VolumeId:           pool.Parent + "." + volName,
		CapacityBytes:      size,
		VolumeContext:      attributes,
		AccessibleTopology: accessibleTopologies,
	}

	if contentSource != nil {
		csiVolume.ContentSource = contentSource
	}

	return csiVolume, nil
}

func (d *Driver) processVolumeContentSource(ctx context.Context, req *csi.CreateVolumeRequest,
	parameters map[string]interface{}) error {
	contentSource := req.GetVolumeContentSource()
	if contentSource != nil {
		if contentSnapshot := contentSource.GetSnapshot(); contentSnapshot != nil {
			sourceSnapshotId := contentSnapshot.GetSnapshotId()
			sourceBackendName, snapshotParentId, sourceSnapshotName := utils.SplitSnapshotId(sourceSnapshotId)
			parameters["sourceSnapshotName"] = sourceSnapshotName
			parameters["snapshotParentId"] = snapshotParentId
			parameters["backend"] = sourceBackendName
			log.AddContext(ctx).Infof("Start to create volume from snapshot %s", sourceSnapshotName)
		} else if contentVolume := contentSource.GetVolume(); contentVolume != nil {
			sourceVolumeId := contentVolume.GetVolumeId()
			sourceBackendName, sourceVolumeName := utils.SplitVolumeId(sourceVolumeId)
			parameters["sourceVolumeName"] = sourceVolumeName
			parameters["backend"] = sourceBackendName
			log.AddContext(ctx).Infof("Start to create volume from volume %s", sourceVolumeName)
		} else {
			log.AddContext(ctx).Errorf("The source %s is not snapshot either volume", contentSource)
			return status.Error(codes.InvalidArgument, "no source ID provided is invalid")
		}
	}

	return nil
}

func (d *Driver) processAccessibilityRequirements(ctx context.Context, req *csi.CreateVolumeRequest,
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

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeId := req.GetVolumeId()

	log.AddContext(ctx).Infof("Start to delete volume %s", volumeId)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		log.AddContext(ctx).Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: volume need to manually delete from array.", backendName)
		return &csi.DeleteVolumeResponse{}, nil
	}

	err := backend.Plugin.DeleteVolume(ctx, volName)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete volume %s error: %v", volumeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is deleted", volumeId)
	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "no volume ID provided")
	}

	log.AddContext(ctx).Infof("Start to controller expand volume %s", volumeId)
	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "no capacity range provided")
	}

	minSize := req.GetCapacityRange().GetRequiredBytes()
	maxSize := req.GetCapacityRange().GetLimitBytes()
	if 0 < maxSize && maxSize < minSize {
		return nil, status.Error(codes.InvalidArgument, "limitBytes is smaller than requiredBytes")
	}

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	if support, err := isSupportExpandVolume(ctx, req, backend); !support {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	nodeExpansionRequired, err := backend.Plugin.ExpandVolume(ctx, volName, minSize)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand volume %s error: %v", volumeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is expanded to %d, nodeExpansionRequired %t", volName, minSize, nodeExpansionRequired)
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         minSize,
		NodeExpansionRequired: nodeExpansionRequired,
	}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse, error) {
	// Volume attachment will be done at node stage process
	log.AddContext(ctx).Infof("Run controller publish volume %s from node %s",
		req.GetVolumeId(), req.GetNodeId())
	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	nodeInfo := req.GetNodeId()

	log.AddContext(ctx).Infof("Start to controller unpublish volume %s from node %s", volumeId, nodeInfo)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		log.AddContext(ctx).Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: volume %s need to manually detach from array.", backendName, volName)
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	var parameters map[string]interface{}

	err := json.Unmarshal([]byte(nodeInfo), &parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal node info of %s error: %v", nodeInfo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = backend.Plugin.DetachVolume(ctx, volName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Unpublish volume %s from node %s error: %v", volName, nodeInfo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is controller unpublished from node %s", volumeId, nodeInfo)
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
					},
				},
			},
			{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	volumeId := req.GetSourceVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	snapshotName := req.GetName()
	if snapshotName == "" {
		return nil, status.Error(codes.InvalidArgument, "Snapshot Name missing in request")
	}
	log.AddContext(ctx).Infof("Start to Create snapshot %s for volume %s", snapshotName, volumeId)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	snapshot, err := backend.Plugin.CreateSnapshot(ctx, volName, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Create snapshot %s error: %v", snapshotName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Finish to Create snapshot %s for volume %s", snapshotName, volumeId)
	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SizeBytes:      snapshot["SizeBytes"].(int64),
			SnapshotId:     backendName + "." + snapshot["ParentID"].(string) + "." + snapshotName,
			SourceVolumeId: volumeId,
			CreationTime:   &timestamp.Timestamp{Seconds: snapshot["CreationTime"].(int64)},
			ReadyToUse:     true,
		},
	}, nil
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	snapshotId := req.GetSnapshotId()
	if snapshotId == "" {
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID missing in request")
	}
	log.AddContext(ctx).Infof("Start to Delete snapshot %s.", snapshotId)

	backendName, snapshotParentId, snapshotName := utils.SplitSnapshotId(snapshotId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		log.AddContext(ctx).Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: snapshot need to manually delete from array.", backendName)
		return &csi.DeleteSnapshotResponse{}, nil
	}

	err := backend.Plugin.DeleteSnapshot(ctx, snapshotParentId, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete snapshot %s error: %v", snapshotName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Finish to Delete snapshot %s", snapshotId)
	return &csi.DeleteSnapshotResponse{}, nil
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetVolume is to get volume info, but unimplemented
func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (
	*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) validateModeAndType(req *csi.CreateVolumeRequest, parameters map[string]interface{}) string {
	// validate volumeMode and volumeType
	volumeCapabilities := req.GetVolumeCapabilities()
	if volumeCapabilities == nil {
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

	if volumeMode == Block && parameters["volumeType"] == "fs" {
		return "VolumeMode is block but volumeType is fs. Please check the storage class"
	}

	if accessMode == RWX && volumeMode == FileSystem && parameters["volumeType"] == "lun" {
		return "If volumeType in the sc.yaml file is set to \"lun\" and volumeMode in the pvc.yaml file is " +
			"set to \"Filesystem\", accessModes in the pvc.yaml file cannot be set to \"ReadWriteMany\"."
	}

	return ""
}

func isSupportExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest, b *backend.Backend) (
	bool, error) {
	if b.Storage == "fusionstorage-nas" || b.Storage == "oceanstor-nas" {
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
		return false, utils.Errorf(ctx, "The PVC %s accessModes is \"ReadOnlyMany\", no need to expand volume.",
			req.GetVolumeId())
	}

	return true, nil
}

func (d *Driver) processNFSProtocol(ctx context.Context, req *csi.CreateVolumeRequest,
	parameters map[string]interface{}) error {
	for _, v := range req.GetVolumeCapabilities() {
		for _, mountFlag := range v.GetMount().GetMountFlags() {
			err := d.addNFSProtocol(ctx, mountFlag, parameters)
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

func (d *Driver) addNFSProtocol(ctx context.Context, mountFlag string, parameters map[string]interface{}) error {
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
