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

	"huawei-csi-driver/connector/nvme"
)

type Manager interface {
	StageVolume(context.Context, *csi.NodeStageVolumeRequest) error
	UnStageVolume(context.Context, *csi.NodeUnstageVolumeRequest) error
	ExpandVolume(context.Context, *csi.NodeExpandVolumeRequest) error
	UnStageWithWwn(ctx context.Context, wwn, volumeId string) error
}

// ControllerPublishInfo context passed by ControllerPublishVolume
// VolumeUseMultiPath is required, and if it is equal true, then MultiPathType is required
// iscsi protocol: TgtPortals, TgtIQNs, TgtHostLUNs, TgtLunWWN is required
// fc protocol: TgtLunWWN, TgtWWNs, TgtHostLUNs is required
// fc-nvme protocol: PortWWNList, TgtLunGuid is required
// roce protocol: TgtPortals, TgtLunGuid is required
// scsi protocol: TgtLunWWN is required
type ControllerPublishInfo struct {
	TgtLunWWN          string             `json:"tgtLunWWN"`
	TgtPortals         []string           `json:"tgtPortals"`
	TgtIQNs            []string           `json:"tgtIQNs"`
	TgtHostLUNs        []string           `json:"tgtHostLUNs"`
	TgtLunGuid         string             `json:"tgtLunGuid"`
	TgtWWNs            []string           `json:"tgtWWNs"`
	PortWWNList        []nvme.PortWWNPair `json:"portWWNList"`
	VolumeUseMultiPath bool               `json:"volumeUseMultiPath"`
	MultiPathType      string             `json:"multiPathType"`
}

// BackendConfig backend configuration
type BackendConfig struct {
	protocol string
	portals  []string
}
