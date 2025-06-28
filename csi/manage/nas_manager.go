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

package manage

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/connector"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// NasManager implements VolumeManager interface
type NasManager struct {
	storage         string
	protocol        string
	portals         []string
	metroPortals    []string
	dTreeParentName string
	Conn            connector.VolumeConnector
}

// StageVolume stage volume
func (m *NasManager) StageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) error {
	if m.storage == constants.OceanStorDtree || m.storage == constants.FusionDTree {
		log.AddContext(ctx).Infoln("dtree needn't to stage volume")
		return nil
	}

	parameters, err := BuildParameters(
		WithProtocol(m.protocol),
		WithPortals(req.PublishContext, m.protocol, m.portals, m.metroPortals),
		WithVolumeCapability(ctx, req),
	)
	if err != nil {
		return utils.Errorf(ctx, "build nas parameters failed, error: %v", err)
	}

	_, volumeName := utils.SplitVolumeId(req.GetVolumeId())
	if volumeName == "" {
		return utils.Errorf(ctx, "volume name is blank, volumeId: %s", req.GetVolumeId())
	}

	sourcePathPrefix, err := generatePathPrefixByProtocol(m.protocol, m.portals)
	if err != nil {
		return utils.Errorf(ctx, "generate path prefix failed, error: %v", err)
	}

	// concatenate the prefix and volume name.
	sourcePath := sourcePathPrefix + volumeName

	connectInfo := map[string]interface{}{
		"srcType":    connector.MountFSType,
		"sourcePath": sourcePath,
		"targetPath": parameters["targetPath"],
		"mountFlags": parameters["mountFlags"],
		"protocol":   parameters["protocol"],
		"portals":    parameters["portals"],
	}

	return Mount(ctx, connectInfo)
}

// UnStageVolume for nas volumes, unstage is only umount the staging target path
func (m *NasManager) UnStageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) error {
	if m.storage == constants.OceanStorDtree || m.storage == constants.FusionDTree {
		log.AddContext(ctx).Infoln("dtree needn't to unstage volume")
		return nil
	}

	return Unmount(ctx, req.GetStagingTargetPath())
}

// ExpandVolume for nas volumes, nodeExpandVolume is not required, because the NodeExpandionRequired field
// returned by ControllerExpandVolume is equal to false
func (m *NasManager) ExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) error {
	log.AddContext(ctx).Infof("start to node expand nas volume, volumeId: %s", req.VolumeId)
	return nil
}

// UnStageWithWwn for nas volumes, unstage is only umount the staging target path
func (m *NasManager) UnStageWithWwn(ctx context.Context, wwn, volumeId string) error {
	log.AddContext(ctx).Infof("start to unstage nas volume with wwn, wwn: %s, volumeId: %s", wwn, volumeId)
	return nil
}
