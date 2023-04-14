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
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type NasManager struct {
	protocol string
	portal   string
	Conn     connector.Connector
}

// NewNasManager build a nas manager instance according to the protocol
func NewNasManager(ctx context.Context, protocol, portal string) (Manager, error) {
	return &NasManager{
		protocol: protocol,
		portal:   portal,
		Conn:     connector.GetConnector(ctx, connector.NFSDriver),
	}, nil
}

// StageVolume stage volume
func (m *NasManager) StageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) error {
	parameters, err := BuildParameters(
		WithProtocol(m.protocol),
		WithVolumeCapability(ctx, req),
	)
	if err != nil {
		log.AddContext(ctx).Errorf("build nas parameters filed, error: %v", err)
		return err
	}

	volumeId := req.GetVolumeId()
	_, volumeName := utils.SplitVolumeId(volumeId)
	if volumeName == "" {
		return utils.Errorf(ctx, "volume name is blank, volumeId: %s", volumeId)
	}

	sourcePath := m.portal + ":/" + volumeName
	if m.protocol == "dpc" {
		sourcePath = "/" + volumeName
	}

	connectInfo := map[string]interface{}{
		"srcType":    connector.MountFSType,
		"sourcePath": sourcePath,
		"targetPath": parameters["targetPath"],
		"mountFlags": parameters["mountFlags"],
		"protocol":   parameters["protocol"],
	}

	return Mount(ctx, connectInfo)
}

// UnStageVolume for nas volumes, unstage is only umount the staging target path
func (m *NasManager) UnStageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) error {
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
