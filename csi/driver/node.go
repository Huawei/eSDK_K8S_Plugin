/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"huawei-csi-driver/connector"
	_ "huawei-csi-driver/connector/nfs" // init the nfs connector
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/manage"
	"huawei-csi-driver/pkg/constants"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
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

	if !strings.Contains(targetPath, app.GetGlobalConfig().KubeletVolumeDevicesDirName) {
		log.AddContext(ctx).Infof("Unmounting the targetPath [%s]", targetPath)
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
				return nil, err
			}
		}
	} else {
		symLink, err := utils.IsPathSymlinkWithTimeout(targetPath, checkSymlinkPathTimeout)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to Access path %s, error: %v", targetPath, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
		if symLink {
			log.AddContext(ctx).Infof("Removing the symlink [%s]", targetPath)
			err := utils.RemoveSymlink(ctx, targetPath)
			if err != nil {
				log.AddContext(ctx).Errorf("Failed to remove symlink for target path [%v]", targetPath)
				return nil, err
			}
		}
	}

	log.AddContext(ctx).Infof("Volume %s is node unpublished from %s", volumeId, targetPath)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetInfo used to get node info
func (d *CsiDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	defer utils.RecoverPanic(ctx)
	hostname, err := utils.GetHostName(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Cannot get current host's hostname")
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
		msg := fmt.Sprintf("no volume ID provided")
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	volumePath := req.GetVolumePath()
	if len(volumePath) == 0 {
		msg := fmt.Sprintf("no volume Path provided")
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	volumeMetrics, err := utils.GetVolumeMetrics(volumePath)
	if err != nil {
		msg := fmt.Sprintf("get volume metrics failed, reason %v", err)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	volumeAvailable, ok := volumeMetrics.Available.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics available %v is invalid", volumeMetrics.Available)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	volumeCapacity, ok := volumeMetrics.Capacity.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics capacity %v is invalid", volumeMetrics.Capacity)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	volumeUsed, ok := volumeMetrics.Used.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics used %v is invalid", volumeMetrics.Used)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	volumeInodesFree, ok := volumeMetrics.InodesFree.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics inodesFree %v is invalid", volumeMetrics.InodesFree)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	volumeInodes, ok := volumeMetrics.Inodes.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics inodes %v is invalid", volumeMetrics.Inodes)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	volumeInodesUsed, ok := volumeMetrics.InodesUsed.AsInt64()
	if !ok {
		msg := fmt.Sprintf("Volume metrics inodesUsed %v is invalid", volumeMetrics.InodesUsed)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Errorf(codes.Internal, msg)
	}

	response := &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Available: volumeAvailable,
				Total:     volumeCapacity,
				Used:      volumeUsed,
				Unit:      csi.VolumeUsage_BYTES,
			},
			{
				Available: volumeInodesFree,
				Total:     volumeInodes,
				Used:      volumeInodesUsed,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
	}
	return response, nil
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
