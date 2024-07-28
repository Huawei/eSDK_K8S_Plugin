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

// Package driver provides csi driver with controller, node, identity services
package driver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/backend/plugin"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// CreateVolume used to create volume
func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)
	log.AddContext(ctx).Infof("Start to create volume %s", req.GetName())

	err := checkCreateVolumeRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	annotations, err := app.GetGlobalConfig().K8sUtils.GetVolumeConfiguration(ctx, req.GetName())
	if err != nil {
		log.AddContext(ctx).Errorf("get pvc info failed, error: %v", err)
		return nil, status.Error(codes.FailedPrecondition, "PVC NotFound")
	}

	if err := processAnnotations(annotations, req); err != nil {
		log.AddContext(ctx).Errorf("process annotations error: %v", err)
		return nil, err
	}

	volumeName, volumeOk := annotations[app.GetGlobalConfig().DriverName+annManageVolumeName]
	backendName, backendOk := annotations[app.GetGlobalConfig().DriverName+annManageBackendName]
	if (!volumeOk && backendOk) || (volumeOk && !backendOk) {
		msg := fmt.Sprintf("The annotation with PVC %s is incorrect, both VolumeName [%s] and BackendName [%s] "+
			"should configure.", req.GetName(), volumeName, backendName)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.FailedPrecondition, msg)
	} else if volumeOk && backendOk {
		// manage Volume
		return d.manageVolume(ctx, req, volumeName, backendName)
	}
	return d.createVolume(ctx, req)
}

// DeleteVolume used to delete volume
func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)
	volumeId := req.GetVolumeId()
	log.AddContext(ctx).Infof("Start to delete volume %s", volumeId)

	backendName, volName := utils.SplitVolumeId(volumeId)

	bk, err := d.backendSelector.SelectBackend(ctx, backendName)
	if bk == nil || err != nil {
		log.AddContext(ctx).Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: volume need to manually delete from array.", backendName)
		return &csi.DeleteVolumeResponse{}, nil
	}

	if bk.Storage == plugin.DTreeStorage {
		err = bk.Plugin.DeleteDTreeVolume(ctx, map[string]interface{}{
			"parentname": bk.Parameters["parentname"],
			"name":       volName,
		})
	} else {
		err = bk.Plugin.DeleteVolume(ctx, volName)
	}

	if err != nil {
		log.AddContext(ctx).Errorf("Delete volume %s error: %v", volumeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is deleted", volumeId)

	// Delete the topology after the volume is successfully deleted.
	// This prevents the DeleteLabel function from being repeatedly invoked when the volume fails to be deleted.
	go pkgUtils.DeletePVLabel(volumeId)

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerExpandVolume used to controller expand volume
func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (
	*csi.ControllerExpandVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)

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
	backend, err := d.backendSelector.SelectBackend(ctx, backendName)
	if backend == nil || err != nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.AddContext(ctx).Errorf(" %s, error: %v", msg, err)
		return nil, status.Error(codes.Internal, msg)
	}

	if support, err := isSupportExpandVolume(ctx, req, backend); !support {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var nodeExpansionRequired bool
	if backend.Storage == plugin.DTreeStorage {
		nodeExpansionRequired, err = backend.Plugin.ExpandDTreeVolume(ctx, map[string]interface{}{
			"name":           volName,
			"parentname":     backend.Parameters["parentname"],
			"spacehardquota": minSize,
		})
	} else {
		nodeExpansionRequired, err = backend.Plugin.ExpandVolume(ctx, volName, minSize)
	}
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

// ControllerPublishVolume used to controller publish volume
func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)

	nodeId := req.GetNodeId()
	volumeId := req.GetVolumeId()
	log.AddContext(ctx).Infof("Run controller publish volume %s to node %s", volumeId, nodeId)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend, err := d.backendSelector.SelectBackend(ctx, backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	var parameters map[string]interface{}

	err = json.Unmarshal([]byte(nodeId), &parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal node info of %s error: %v", nodeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	mappingInfo, err := backend.Plugin.AttachVolume(ctx, volName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("controller publish volume %s to node %s error: %v", volName, nodeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	publishInfo, err := json.Marshal(mappingInfo)
	if err != nil {
		log.AddContext(ctx).Errorf("controller publish json marshal error: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is controller published to node %s", volumeId, nodeId)
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"publishInfo":    string(publishInfo),
			"filesystemMode": getBackendFilesystemMode(ctx, backend, volName),
		},
	}, nil
}

// ControllerUnpublishVolume used to controller unpublish volume
func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)

	volumeId := req.GetVolumeId()
	nodeInfo := req.GetNodeId()

	log.AddContext(ctx).Infof("Start to controller unpublish volume %s from node %s", volumeId, nodeInfo)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend, err := d.backendSelector.SelectBackend(ctx, backendName)
	if backend == nil {
		log.AddContext(ctx).Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: volume %s need to manually detach from array.", backendName, volName)
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	var parameters map[string]interface{}

	err = json.Unmarshal([]byte(nodeInfo), &parameters)
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

// ValidateVolumeCapabilities used to validate volume capabilities
func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse, error) {

	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

// ListVolumes used to list volumes
func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

// GetCapacity used to get volume capacity
func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

// ControllerGetCapabilities used to controller get capabilities
func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (
	*csi.ControllerGetCapabilitiesResponse, error) {
	defer utils.RecoverPanic(ctx)

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

// CreateSnapshot used to create snapshot for volume
func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (
	*csi.CreateSnapshotResponse, error) {
	defer utils.RecoverPanic(ctx)

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
	backend, err := d.backendSelector.SelectBackend(ctx, backendName)
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
			CreationTime:   &timestamppb.Timestamp{Seconds: snapshot["CreationTime"].(int64)},
			ReadyToUse:     true,
		},
	}, nil
}

// DeleteSnapshot used to delete snapshot
func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (
	*csi.DeleteSnapshotResponse, error) {
	defer utils.RecoverPanic(ctx)

	snapshotId := req.GetSnapshotId()
	if snapshotId == "" {
		return nil, status.Error(codes.InvalidArgument, "Snapshot ID missing in request")
	}
	log.AddContext(ctx).Infof("Start to Delete snapshot %s.", snapshotId)

	backendName, snapshotParentId, snapshotName := utils.SplitSnapshotId(snapshotId)
	backend, err := d.backendSelector.SelectBackend(ctx, backendName)
	if backend == nil {
		log.AddContext(ctx).Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: snapshot need to manually delete from array.", backendName)
		return &csi.DeleteSnapshotResponse{}, nil
	}

	err = backend.Plugin.DeleteSnapshot(ctx, snapshotParentId, snapshotName)
	if err != nil {
		log.AddContext(ctx).Errorf("Delete snapshot %s error: %v", snapshotName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Finish to Delete snapshot %s", snapshotId)
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots used to list snapshots
func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetVolume is to get volume info, but unimplemented
func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (
	*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
