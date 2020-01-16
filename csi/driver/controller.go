package driver

import (
	"context"
	"csi/backend"
	"encoding/json"
	"fmt"
	"utils"
	"utils/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()

	log.Infof("Start to create volume %s", name)

	capacityRange := req.GetCapacityRange()
	if capacityRange == nil || capacityRange.RequiredBytes <= 0 {
		msg := "CreateVolume CapacityRange must be provided"
		log.Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	parameters := utils.CopyMap(req.GetParameters())
	size := capacityRange.RequiredBytes
	parameters["size"] = capacityRange.RequiredBytes

	cloneFrom, exist := parameters["cloneFrom"].(string)
	if exist && cloneFrom != "" {
		parameters["backend"], parameters["cloneFrom"] = utils.SplitVolumeId(cloneFrom)
	}

	localPool, remotePool, err := backend.SelectStoragePool(size, parameters)
	if err != nil {
		log.Errorf("Cannot select pool for volume creation: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	parameters["storagepool"] = localPool.Name
	if remotePool != nil {
		parameters["metroDomain"] = backend.GetMetroDomain(remotePool.Parent)
		parameters["remoteStoragePool"] = remotePool.Name
	}

	volName, err := localPool.Plugin.CreateVolume(name, parameters)
	if err != nil {
		log.Errorf("Create volume %s error: %v", name, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Volume %s is created", name)

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      localPool.Parent + "." + volName,
			CapacityBytes: size,
		},
	}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeId := req.GetVolumeId()

	log.Infof("Start to delete volume %s", volumeId)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	err := backend.Plugin.DeleteVolume(volName)
	if err != nil {
		log.Errorf("Delete volume %s error: %v", volumeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Volume %s is deleted", volumeId)
	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	// Volume attachment will be done at node stage process
	return &csi.ControllerPublishVolumeResponse{}, nil
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	nodeInfo := req.GetNodeId()

	log.Infof("Start to controller unpublish volume %s from node %s", volumeId, nodeInfo)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	var parameters map[string]interface{}

	err := json.Unmarshal([]byte(nodeInfo), &parameters)
	if err != nil {
		log.Errorf("Unmarshal node info of %s error: %v", nodeInfo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = backend.Plugin.DetachVolume(volName, parameters)
	if err != nil {
		log.Errorf("Unpublish volume %s from node %s error: %v", volName, nodeInfo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Volume %s is controller unpublished from node %s", volumeId, nodeInfo)
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
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
