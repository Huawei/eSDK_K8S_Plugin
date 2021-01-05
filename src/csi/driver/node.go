package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Huawei/eSDK_K8S_Plugin/src/csi/backend"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	volumeId := req.GetVolumeId()

	log.Infof("Start to stage volume %s", volumeId)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	mnt := req.GetVolumeCapability().GetMount()

	parameters := map[string]interface{}{
		"targetPath": req.GetStagingTargetPath(),
		"fsType":     mnt.GetFsType(),
		"mountFlags": strings.Join(mnt.GetMountFlags(), ","),
	}

	err := backend.Plugin.StageVolume(volName, parameters)
	if err != nil {
		log.Errorf("Stage volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Volume %s is staged", volumeId)
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	targetPath := req.GetStagingTargetPath()

	log.Infof("Start to unstage volume %s from %s", volumeId, targetPath)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	parameters := map[string]interface{}{
		"targetPath": targetPath,
	}

	err := backend.Plugin.UnstageVolume(volName, parameters)
	if err != nil {
		log.Errorf("Unstage volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Volume %s is unstaged from %s", volumeId, targetPath)
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	sourcePath := req.GetStagingTargetPath()
	targetPath := req.GetTargetPath()

	log.Infof("Start to node publish volume %s to %s", volumeId, targetPath)

	opts := []string{"bind"}
	if req.GetReadonly() {
		opts = append(opts, "ro")
	}

	output, err := utils.ExecShellCmd("mount -o %s %s %s", strings.Join(opts, ","), sourcePath, targetPath)
	if err != nil {
		msg := fmt.Sprintf("Bind mount %s to %s error: %s", sourcePath, targetPath, output)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	log.Infof("Volume %s is node published to %s", volumeId, targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	targetPath := req.GetTargetPath()

	log.Infof("Start to node unpublish volume %s from %s", volumeId, targetPath)

	output, err := utils.ExecShellCmd("umount %s", targetPath)
	if err != nil && !strings.Contains(output, "not mounted") {
		msg := fmt.Sprintf("umount %s for volume %s error: %s", targetPath, volumeId, output)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	log.Infof("Volume %s is node unpublished from %s", volumeId, targetPath)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *Driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	hostname, err := utils.GetHostName()
	if err != nil {
		log.Errorf("Cannot get current host's hostname")
		return nil, status.Error(codes.Internal, err.Error())
	}

	node := map[string]interface{}{
		"HostName": hostname,
	}

	nodeBytes, err := json.Marshal(node)
	if err != nil {
		log.Errorf("Marshal node info of %s error: %v", nodeBytes, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.Infof("Get NodeId %s", nodeBytes)
	return &csi.NodeGetInfoResponse{
		NodeId: string(nodeBytes),
	}, nil
}

func (d *Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			&csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
			&csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			&csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		msg := fmt.Sprintf("no volume ID provided")
		log.Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	VolumePath := req.GetVolumePath()
	if len(VolumePath) == 0 {
		msg := fmt.Sprintf("no volume Path provided")
		log.Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	volumeMetrics, err := utils.GetVolumeMetrics(VolumePath)
	if err != nil {
		msg := fmt.Sprintf("get volume metrics failed, reason %v", volumeMetrics)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	volumeAvailable, ok := volumeMetrics.Available.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics available %v is invalid", volumeMetrics.Available)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	volumeCapacity, ok := volumeMetrics.Capacity.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics capacity %v is invalid", volumeMetrics.Capacity)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	volumeUsed, ok := volumeMetrics.Used.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics used %v is invalid", volumeMetrics.Used)
		log.Errorln(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	volumeInodesFree, ok := volumeMetrics.InodesFree.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics inodesFree %v is invalid", volumeMetrics.InodesFree)
		log.Errorln(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	volumeInodes, ok := volumeMetrics.Inodes.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics inodes %v is invalid", volumeMetrics.Inodes)
		log.Errorln(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	volumeInodesUsed, ok := volumeMetrics.InodesUsed.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics inodesUsed %v is invalid", volumeMetrics.InodesUsed)
		log.Errorln(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	response := &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			&csi.VolumeUsage{
				Available: volumeAvailable,
				Total:     volumeCapacity,
				Used:      volumeUsed,
				Unit:      csi.VolumeUsage_BYTES,
			},
			&csi.VolumeUsage{
				Available: volumeInodesFree,
				Total:     volumeInodes,
				Used:      volumeInodesUsed,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
	}
	return response, nil
}

func (d *Driver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	log.Infof("Start to node expand volume %s", req)
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "no volume ID provided")
	}

	volumePath := req.GetVolumePath()
	if volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "no volume path provided")
	}

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}
	err := backend.Plugin.NodeExpandVolume(volName, volumePath)
	if err != nil {
		log.Errorf("Node expand volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &csi.NodeExpandVolumeResponse{}, nil
}
