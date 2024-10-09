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

package manage

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"huawei-csi-driver/connector"
	"huawei-csi-driver/csi/backend/plugin"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

// NasManager implements VolumeManager interface
type NasManager struct {
	protocol        string
	portals         []string
	metroPortals    []string
	dTreeParentName string
	Conn            connector.VolumeConnector
}

// NewNasManager build a nas manager instance according to the protocol
func NewNasManager(ctx context.Context,
	protocol, dTreeParentName string, portals, metroPortals []string) (VolumeManager, error) {
	return &NasManager{
		protocol:        protocol,
		portals:         portals,
		metroPortals:    metroPortals,
		dTreeParentName: dTreeParentName,
		Conn:            getConnectorByProtocol(ctx, protocol),
	}, nil
}

// StageVolume stage volume
func (m *NasManager) StageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) error {
	if m.dTreeParentName != "" {
		log.AddContext(ctx).Infoln("dtree needn't to stage volume")
		return nil
	}

	parameters, err := BuildParameters(
		WithProtocol(m.protocol),
		WithPortals(req.PublishContext, m.protocol, m.portals, m.metroPortals),
		WithVolumeCapability(ctx, req),
	)
	if err != nil {
		log.AddContext(ctx).Errorf("build nas parameters failed, error: %v", err)
		return err
	}

	_, volumeName := utils.SplitVolumeId(req.GetVolumeId())
	if volumeName == "" {
		return utils.Errorf(ctx, "volume name is blank, volumeId: %s", req.GetVolumeId())
	}

	var sourcePath string
	switch m.protocol {
	case plugin.ProtocolDpc:
		sourcePath = "/" + volumeName
	case plugin.ProtocolNfs, plugin.ProtocolNfsPlus:
		sourcePath = m.portals[0] + ":/" + volumeName
	default:
		return pkgUtils.Errorf(ctx, "stage volume protocol is invalid, protocol: %s, param: %+v",
			m.protocol, parameters)
	}

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
	if m.dTreeParentName != "" {
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
