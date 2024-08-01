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
	"strings"
	"time"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/csi/app"

	// init the nfs connector
	_ "huawei-csi-driver/connector/nfs"
	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	defer utils.RecoverPanic(ctx)

	volumeId := req.GetVolumeId()
	log.AddContext(ctx).Infof("Start to stage volume %s", volumeId)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	var parameters = map[string]interface{}{}
	parameters = map[string]interface{}{
		"volumeUseMultiPath": d.useMultiPath,
		"scsiMultiPathType":  d.scsiMultiPathType,
		"nvmeMultiPathType":  d.nvmeMultiPathType,
	}
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
		log.AddContext(ctx).Infof("The access mode of volume %s is %s", volumeId, accessMode)

		if accessMode == "ReadOnly" {
			opts = append(opts, "ro")
		}

		parameters["targetPath"] = req.GetStagingTargetPath()
		parameters["fsType"] = mnt.GetFsType()
		parameters["mountFlags"] = strings.Join(opts, ",")
		parameters["accessMode"] = volumeAccessMode
		parameters["fsPermission"] = req.VolumeContext["fsPermission"]
	default:
		msg := fmt.Sprintf("Invalid volume capability.")
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}
	err := backend.Plugin.StageVolume(ctx, volName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Stage volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is staged", volumeId)
	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	targetPath := req.GetStagingTargetPath()

	log.AddContext(ctx).Infof("Start to unstage volume %s from %s", volumeId, targetPath)

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	parameters := map[string]interface{}{
		"targetPath": targetPath,
	}

	err := backend.Plugin.UnstageVolume(ctx, volName, parameters)
	if err != nil {
		log.AddContext(ctx).Errorf("Unstage volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is unstaged from %s", volumeId, targetPath)
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *Driver) NodePublishVolume(ctx context.Context,
	req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	sourcePath := req.GetStagingTargetPath()
	targetPath := req.GetTargetPath()

	log.AddContext(ctx).Infof("Start to node publish volume %s to %s", volumeId, targetPath)
	if req.GetVolumeCapability().GetBlock() != nil {
		// If the request is to publish raw block device then create symlink of the device
		// from the staging are to publish. Do not create fs and mount
		log.AddContext(ctx).Infoln("Creating symlink for the staged device on the node to publish")
		sourcePath = sourcePath + "/" + volumeId
		err := utils.CreateSymlink(ctx, sourcePath, targetPath)
		if err != nil {
			log.AddContext(ctx).Errorf("Failed to create symlink for the staging path [%v] to target path [%v]",
				sourcePath, targetPath)
			return nil, err
		}
		accessMode := utils.GetAccessModeType(req.GetVolumeCapability().GetAccessMode().GetMode())
		if accessMode == "ReadOnly" {
			_, err = utils.ExecShellCmd(ctx, "chmod 440 %s", targetPath)
			if err != nil {
				log.AddContext(ctx).Errorln("Unable to set ReadOnlyMany permission")
				return nil, err
			}
		}
		log.AddContext(ctx).Infof("Raw Block Volume %s is node published to %s", volumeId, targetPath)
		return &csi.NodePublishVolumeResponse{}, nil
	}

	opts := []string{"bind"}
	if req.GetReadonly() {
		opts = append(opts, "ro")
	}

	connectInfo := map[string]interface{}{
		"srcType":    connector.MountFSType,
		"sourcePath": sourcePath,
		"targetPath": targetPath,
		"mountFlags": strings.Join(opts, ","),
	}

	conn := connector.GetConnector(ctx, connector.NFSDriver)
	_, err := conn.ConnectVolume(ctx, connectInfo)
	if err != nil {
		log.AddContext(ctx).Errorf("Mount share %s to %s error: %v", sourcePath, targetPath, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	log.AddContext(ctx).Infof("Volume %s is node published to %s", volumeId, targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
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
			output, err := utils.ExecShellCmd(ctx, "umount %s", targetPath)
			if err != nil && !strings.Contains(output, "not mounted") {
				msg := fmt.Sprintf("umount %s for volume %s error: %s", targetPath, volumeId, output)
				log.AddContext(ctx).Errorln(msg)
				return nil, status.Error(codes.Internal, msg)
			}
		}
	} else {
		symLink, err := utils.IsPathSymlinkWithTimeout(targetPath, 10*time.Second)
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

func (d *Driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
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
			NodeId: string(nodeBytes),
		}, nil
	}

	// Get topology info from Node labels
	topology, err := d.k8sUtils.GetNodeTopology(ctx, d.nodeName)
	if err != nil {
		log.AddContext(ctx).Errorln(err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeGetInfoResponse{
		NodeId: string(nodeBytes),
		AccessibleTopology: &csi.Topology{
			Segments: topology,
		},
	}, nil
}

func (d *Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
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

func (d *Driver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		msg := fmt.Sprintf("no volume ID provided")
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	VolumePath := req.GetVolumePath()
	if len(VolumePath) == 0 {
		msg := fmt.Sprintf("no volume Path provided")
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	volumeMetrics, err := utils.GetVolumeMetrics(VolumePath)
	if err != nil {
		msg := fmt.Sprintf("get volume metrics failed, reason %v", volumeMetrics)
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

func (d *Driver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	log.AddContext(ctx).Infof("Start to node expand volume %s", req)
	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "no volume ID provided")
	}

	capacityRange := req.GetCapacityRange()
	if capacityRange == nil || capacityRange.RequiredBytes <= 0 {
		msg := "NodeExpandVolume CapacityRange must be provided"
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.InvalidArgument, msg)
	}

	volumePath := req.GetVolumePath()
	if volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "no volume path provided")
	}

	accessMode := utils.GetAccessModeType(req.GetVolumeCapability().GetAccessMode().GetMode())
	if accessMode == "ReadOnly" {
		log.AddContext(ctx).Warningf("The access mode of volume %s is %s", volumeId, accessMode)
		return &csi.NodeExpandVolumeResponse{}, nil
	}

	backendName, volName := utils.SplitVolumeId(volumeId)
	backend := backend.GetBackend(backendName)
	if backend == nil {
		msg := fmt.Sprintf("Backend %s doesn't exist", backendName)
		log.AddContext(ctx).Errorln(msg)
		return nil, status.Error(codes.Internal, msg)
	}

	var isBlock bool
	if req.GetVolumeCapability().GetBlock() != nil {
		isBlock = true
	}

	err := backend.Plugin.NodeExpandVolume(ctx, volName, volumePath, isBlock, capacityRange.RequiredBytes)
	if err != nil {
		log.AddContext(ctx).Errorf("Node expand volume %s error: %v", volName, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	log.AddContext(ctx).Infof("Finish node expand volume %s", volumeId)
	return &csi.NodeExpandVolumeResponse{}, nil
}
