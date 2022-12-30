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
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)
	log.AddContext(ctx).Infof("Start to create volume %s", req.GetName())

	err := checkCreateVolumeRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	parameters, err := processCreateVolumeParameters(ctx, req)
	if err != nil {
		return nil, err
	}

	localPool, remotePool, err := backend.SelectStoragePool(ctx, req.GetCapacityRange().RequiredBytes, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot select pool for volume creation: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	processCreateVolumeParametersAfterSelect(parameters, localPool, remotePool)

	vol, err := localPool.Plugin.CreateVolume(ctx, req.GetName(), parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Create volume %s error: %v", req.GetName(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is created", req.GetName())
	return &csi.CreateVolumeResponse{
		Volume: makeCreateVolumeResponse(ctx, req, vol, localPool),
	}, nil
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

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (
	*csi.ControllerExpandVolumeResponse, error) {

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

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse, error) {

	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "Not implemented")
}

func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (
	*csi.ControllerGetCapabilitiesResponse, error) {

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

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (
	*csi.CreateSnapshotResponse, error) {

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

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (
	*csi.DeleteSnapshotResponse, error) {

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
