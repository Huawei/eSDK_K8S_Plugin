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
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	_ "github.com/Huawei/eSDK_K8S_Plugin/v4/connector/nfs" // init the nfs connector
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/manage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/retry"
)

const checkSymlinkPathTimeout = 10 * time.Second

// NodeStageVolume used to stage volume
func (d *CsiDriver) NodeStageVolume(ctx context.Context,
	req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)

	volumeId := req.GetVolumeId()
	log.AddContext(ctx).Infof("Start to stage volume %s", volumeId)
	backendName, volName := utils.SplitVolumeId(volumeId)

	manager, err := manage.NewManager(ctx, backendName)
	if err != nil {
		log.AddContext(ctx).Errorf("Stage init manager fail, backend: %s, error: %v", backendName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = manager.StageVolume(ctx, req)
	if err != nil {
		log.AddContext(ctx).Errorf("Stage volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is staged", volumeId)
	return &csi.NodeStageVolumeResponse{}, nil
}

// NodeUnstageVolume used to unstage volume
func (d *CsiDriver) NodeUnstageVolume(ctx context.Context,
	req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)
	volumeId := req.GetVolumeId()
	targetPath := req.GetStagingTargetPath()

	log.AddContext(ctx).Infof("Start to unstage volume %s from %s", volumeId, targetPath)
	backendName, volName := utils.SplitVolumeId(volumeId)

	manager, err := manage.NewManager(ctx, backendName)
	if err != nil {
		log.AddContext(ctx).Errorf("UnStage init manager fail, backend: %s, error: %v", backendName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = manager.UnStageVolume(ctx, req)
	if err != nil {
		log.AddContext(ctx).Errorf("UnStage volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is unstaged from %s", volumeId, targetPath)
	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodePublishVolume used to node publish volume
func (d *CsiDriver) NodePublishVolume(ctx context.Context,
	req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)
	volumeId := req.GetVolumeId()
	targetPath := req.GetTargetPath()

	log.AddContext(ctx).Infof("Start to node publish volume %s to %s", volumeId, targetPath)
	if req.GetVolumeCapability().GetBlock() != nil {
		if err := manage.PublishBlock(ctx, req); err != nil {
			log.AddContext(ctx).Errorf("publish block volume fail, volume: %s, error: %v", volumeId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		if err := manage.PublishFilesystem(ctx, req); err != nil {
			log.AddContext(ctx).Errorf("publish filesystem volume fail, volume: %s, error: %v", volumeId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	log.AddContext(ctx).Infof("Volume %s is node published from %s", volumeId, targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume used to node unpublish volume
func (d *CsiDriver) NodeUnpublishVolume(ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)

	volumeId := req.GetVolumeId()
	targetPath := req.GetTargetPath()

	log.AddContext(ctx).Infof("Start to node unpublish volume %s from %s", volumeId, targetPath)

	mounted, err := connector.MountPathIsExist(ctx, targetPath)
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to get mount point [%s], error: %v", targetPath, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if mounted {
		umountRes, err := utils.ExecShellCmd(ctx, "umount %s", targetPath)
		if err != nil && !strings.Contains(umountRes, constants.NotMountStr) {
			log.AddContext(ctx).Errorf("umount %s for volume %s msg:%s error: %s", targetPath, volumeId,
				umountRes, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	log.AddContext(ctx).Infof("remove target path %s", targetPath)
	const attempts = 3
	if err := retry.Attempts(attempts).Period(time.Second).Do(func() error {
		err := os.Remove(targetPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		return nil
	}); err != nil {
		log.AddContext(ctx).Errorf("Failed to delete the target [%v]", targetPath)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is node unpublished from %s", volumeId, targetPath)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetInfo used to get node info
func (d *CsiDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	defer utils.RecoverPanic(ctx)
	hostname, err := utils.GetHostName(ctx)
	if err != nil {
		log.AddContext(ctx).Errorln("Cannot get current host's hostname")
		return nil, status.Error(codes.Internal, err.Error())
	}

	node := map[string]interface{}{
		"HostName": hostname,
	}

	nodeBytes, err := json.Marshal(node)
	if err != nil {
		log.AddContext(ctx).Errorf("Marshal node info of %s error: %v", nodeBytes, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.AddContext(ctx).Infof("Get NodeId %s", nodeBytes)

	if d.nodeName == "" {
		return &csi.NodeGetInfoResponse{
			NodeId:            string(nodeBytes),
			MaxVolumesPerNode: int64(app.GetGlobalConfig().MaxVolumesPerNode),
		}, nil
	}

	// Get topology info from Node labels
	topology, err := d.k8sUtils.GetNodeTopology(ctx, d.nodeName)
	if err != nil {
		log.AddContext(ctx).Errorln(err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeGetInfoResponse{
		NodeId:            string(nodeBytes),
		MaxVolumesPerNode: int64(app.GetGlobalConfig().MaxVolumesPerNode),
		AccessibleTopology: &csi.Topology{
			Segments: topology,
		},
	}, nil
}

// NodeGetCapabilities used to get node capabilities
func (d *CsiDriver) NodeGetCapabilities(ctx context.Context,
	req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	defer utils.RecoverPanic(ctx)
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
					},
				},
			},
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},
		},
	}, nil
}

// NodeGetVolumeStats used to get node volume status
func (d *CsiDriver) NodeGetVolumeStats(ctx context.Context,
	req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	defer utils.RecoverPanic(ctx)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		log.AddContext(ctx).Errorln("no volume ID provided")
		return nil, status.Error(codes.InvalidArgument, "no volume ID provided")
	}

	volumePath := req.GetVolumePath()
	if len(volumePath) == 0 {
		log.AddContext(ctx).Errorln("no volume Path provided")
		return nil, status.Error(codes.InvalidArgument, "no volume Path provided")
	}

	isBlock, err := utils.IsBlockDevice(volumePath)
	if err != nil {
		log.AddContext(ctx).Errorf("check block device for volume %s failed: %v", volumeID, err)
		return nil, status.Errorf(codes.Internal, "check block device for volume %s failed: %v", volumeID, err)
	}

	if isBlock {
		return d.getBlockVolumeStats(ctx, volumeID, volumePath)
	}
	return d.getFilesystemVolumeStats(ctx, volumePath)
}

func (d *CsiDriver) getBlockVolumeStats(ctx context.Context, volumeID, volumePath string) (
	*csi.NodeGetVolumeStatsResponse, error) {
	blockSize, err := utils.GetBlockDeviceSize(volumePath)
	if err != nil {
		log.AddContext(ctx).Errorf("get block size for volume %s failed: %v", volumeID, err)
		return nil, status.Errorf(codes.Internal, "get block size for volume %s failed: %v", volumeID, err)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{{Total: blockSize, Unit: csi.VolumeUsage_BYTES}},
	}, nil
}

func (d *CsiDriver) getFilesystemVolumeStats(ctx context.Context, volumePath string) (
	*csi.NodeGetVolumeStatsResponse, error) {
	volumeMetrics, err := utils.GetVolumeMetrics(volumePath)
	if err != nil {
		log.AddContext(ctx).Errorf("get volume metrics failed: %v", err)
		return nil, status.Errorf(codes.Internal, "get volume metrics failed: %v", err)
	}

	bytesUsage, err := d.extractBytesUsage(ctx, volumeMetrics)
	if err != nil {
		return nil, err
	}

	inodesUsage, err := d.extractInodesUsage(ctx, volumeMetrics)
	if err != nil {
		return nil, err
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{bytesUsage, inodesUsage},
	}, nil
}

func (d *CsiDriver) extractBytesUsage(ctx context.Context, vm *utils.VolumeMetrics) (*csi.VolumeUsage, error) {
	available, ok := vm.Available.AsInt64()
	if !ok {
		log.AddContext(ctx).Errorf("Volume metrics available %v is invalid", vm.Available)
		return nil, status.Errorf(codes.Internal, "Volume metrics available %v is invalid", vm.Available)
	}

	capacity, ok := vm.Capacity.AsInt64()
	if !ok {
		log.AddContext(ctx).Errorf("Volume metrics capacity %v is invalid", vm.Capacity)
		return nil, status.Errorf(codes.Internal, "Volume metrics capacity %v is invalid", vm.Capacity)
	}

	used, ok := vm.Used.AsInt64()
	if !ok {
		log.AddContext(ctx).Errorf("Volume metrics used %v is invalid", vm.Used)
		return nil, status.Errorf(codes.Internal, "Volume metrics used %v is invalid", vm.Used)
	}

	return &csi.VolumeUsage{
		Available: available,
		Total:     capacity,
		Used:      used,
		Unit:      csi.VolumeUsage_BYTES,
	}, nil
}

func (d *CsiDriver) extractInodesUsage(ctx context.Context, vm *utils.VolumeMetrics) (*csi.VolumeUsage, error) {
	inodesFree, ok := vm.InodesFree.AsInt64()
	if !ok {
		log.AddContext(ctx).Errorf("Volume metrics inodesFree %v is invalid", vm.InodesFree)
		return nil, status.Errorf(codes.Internal, "Volume metrics inodesFree %v is invalid", vm.InodesFree)
	}

	inodes, ok := vm.Inodes.AsInt64()
	if !ok {
		log.AddContext(ctx).Errorf("Volume metrics inodes %v is invalid", vm.Inodes)
		return nil, status.Errorf(codes.Internal, "Volume metrics inodes %v is invalid", vm.Inodes)
	}

	inodesUsed, ok := vm.InodesUsed.AsInt64()
	if !ok {
		log.AddContext(ctx).Errorf("Volume metrics inodesUsed %v is invalid", vm.InodesUsed)
		return nil, status.Errorf(codes.Internal, "Volume metrics inodesUsed %v is invalid", vm.InodesUsed)
	}

	return &csi.VolumeUsage{
		Available: inodesFree,
		Total:     inodes,
		Used:      inodesUsed,
		Unit:      csi.VolumeUsage_INODES,
	}, nil
}

// NodeExpandVolume used to node expand volume
func (d *CsiDriver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (
	*csi.NodeExpandVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)

	log.AddContext(ctx).Infof("Start to node expand volume %s", req)
	volumeId := req.GetVolumeId()

	backendName, volName := utils.SplitVolumeId(volumeId)
	manager, err := manage.NewManager(ctx, backendName)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand init manager fail, backend: %s, error: %v", backendName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = manager.ExpandVolume(ctx, req)
	if err != nil {
		log.AddContext(ctx).Errorf("Expand volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.AddContext(ctx).Infof("Finish node expand volume %s", volumeId)
	return &csi.NodeExpandVolumeResponse{}, nil
}
