package driver

import (
	"context"
	"csi/backend"
	"encoding/json"
	"fmt"
	"utils"
	"utils/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/timestamp"
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

	// process volume content source
	err := d.processVolumeContentSource(req, parameters)
	if err != nil {
		return nil, err
	}

	// process accessibility requirements
	d.processAccessibilityRequirements(req, parameters)

	localPool, remotePool, err := backend.SelectStoragePool(size, parameters)
	if err != nil {
		log.Errorf("Cannot select pool for volume creation: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	parameters["storagepool"] = localPool.Name
	if remotePool != nil {
		parameters["metroDomain"] = backend.GetMetroDomain(remotePool.Parent)
		parameters["vStorePairID"] = backend.GetMetrovStorePairID(remotePool.Parent)
		parameters["remoteStoragePool"] = remotePool.Name
	}

	volName, err := localPool.Plugin.CreateVolume(name, parameters)
	if err != nil {
		log.Errorf("Create volume %s error: %v", name, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	volume, err := d.getCreatedVolume(req, volName, localPool)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.Infof("Volume %s is created", name)
	return &csi.CreateVolumeResponse{
		Volume: volume,
	}, nil
}

func (d *Driver) getCreatedVolume(req *csi.CreateVolumeRequest, volName string, pool *backend.StoragePool) (*csi.Volume, error) {
	contentSource := req.GetVolumeContentSource()
	size := req.GetCapacityRange().GetRequiredBytes()

	accessibleTopologies := make([]*csi.Topology, 0)
	if req.GetAccessibilityRequirements() != nil &&
		len(req.GetAccessibilityRequirements().GetRequisite()) != 0 {
		supportedTopology := pool.GetSupportedTopologies()
		if len(supportedTopology) > 0 {
			for _, segment := range supportedTopology {
				accessibleTopologies = append(accessibleTopologies, &csi.Topology{Segments: segment})
			}
		}
	}

	if contentSource != nil {
		attributes := map[string]string{
			"backend": pool.Parent,
			"name":    volName,
		}

		return &csi.Volume{
			VolumeId:           pool.Parent + "." + volName,
			CapacityBytes:      size,
			VolumeContext:      attributes,
			ContentSource:      contentSource,
			AccessibleTopology: accessibleTopologies,
		}, nil
	}

	return &csi.Volume{
		VolumeId:           pool.Parent + "." + volName,
		CapacityBytes:      size,
		AccessibleTopology: accessibleTopologies,
	}, nil
}

func (d *Driver) processVolumeContentSource(req *csi.CreateVolumeRequest, parameters map[string]interface{}) error {
	contentSource := req.GetVolumeContentSource()
	if contentSource != nil {
		if contentSnapshot := contentSource.GetSnapshot(); contentSnapshot != nil {
			sourceSnapshotId := contentSnapshot.GetSnapshotId()
			sourceBackendName, snapshotParentId, sourceSnapshotName := utils.SplitSnapshotId(sourceSnapshotId)
			parameters["sourceSnapshotName"] = sourceSnapshotName
			parameters["snapshotParentId"] = snapshotParentId
			parameters["backend"] = sourceBackendName
			log.Infof("Start to create volume from snapshot %s", sourceSnapshotName)
		} else if contentVolume := contentSource.GetVolume(); contentVolume != nil {
			sourceVolumeId := contentVolume.GetVolumeId()
			sourceBackendName, sourceVolumeName := utils.SplitVolumeId(sourceVolumeId)
			parameters["sourceVolumeName"] = sourceVolumeName
			parameters["backend"] = sourceBackendName
			log.Infof("Start to create volume from volume %s", sourceVolumeName)
		} else {
			log.Errorf("The source %s is not snapshot either volume", contentSource)
			return status.Error(codes.InvalidArgument, "no source ID provided is invalid")
		}
	}

	return nil
}

func (d *Driver) processAccessibilityRequirements(req *csi.CreateVolumeRequest, parameters map[string]interface{}) {
	accessibleTopology := req.GetAccessibilityRequirements()
	if accessibleTopology == nil {
		log.Infoln("Empty accessibility requirements in create volume request")
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

	log.Infof("accessibility Requirements in create volume %+v", parameters[backend.Topology])
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeId := req.GetVolumeId()

	log.Infof("Start to delete volume %s", volumeId)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		log.Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: volume need to manually delete from array.", backendName)
		return &csi.DeleteVolumeResponse{}, nil
	}

	err := backend.Plugin.DeleteVolume(volName)
	if err != nil {
		log.Errorf("Delete volume %s error: %v", volumeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Volume %s is deleted", volumeId)
	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "no volume ID provided")
	}

	log.Infof("Start to controller expand volume %s", volumeId)
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
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	nodeExpansionRequired, err := backend.Plugin.ExpandVolume(volName, minSize)
	if err != nil {
		log.Errorf("Expand volume %s error: %v", volumeId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Volume %s is expanded to %d, nodeExpansionRequired %t", volName, minSize, nodeExpansionRequired)
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         minSize,
		NodeExpansionRequired: nodeExpansionRequired,
	}, nil
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
		log.Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: volume %s need to manually detach from array.", backendName, volName)
		return &csi.ControllerUnpublishVolumeResponse{}, nil
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
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
					},
				},
			},
			&csi.ControllerServiceCapability{
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
	log.Infof("Start to Create snapshot %s for volume %s", snapshotName, volumeId)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	snapshot, err := backend.Plugin.CreateSnapshot(volName, snapshotName)
	if err != nil {
		log.Errorf("Create snapshot %s error: %v", snapshotName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Finish to Create snapshot %s for volume %s", snapshotName, volumeId)
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
	log.Infof("Start to Delete snapshot %s.", snapshotId)

	backendName, snapshotParentId, snapshotName := utils.SplitSnapshotId(snapshotId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		log.Warningf("Backend %s doesn't exist. Ignore this request and return success. "+
			"CAUTION: snapshot need to manually delete from array.", backendName)
		return &csi.DeleteSnapshotResponse{}, nil
	}

	err := backend.Plugin.DeleteSnapshot(snapshotParentId, snapshotName)
	if err != nil {
		log.Errorf("Delete snapshot %s error: %v", snapshotName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Finish to Delete snapshot %s", snapshotId)
	return &csi.DeleteSnapshotResponse{}, nil
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
