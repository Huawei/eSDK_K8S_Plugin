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

package connector

import (
	"context"
	"fmt"

	"huawei-csi-driver/utils/log"
)

const (
	// FCDriver name string
	FCDriver = "fibreChannel"
	// FCNVMeDriver name string
	FCNVMeDriver = "FC-NVMe"
	// ISCSIDriver name string
	ISCSIDriver = "iSCSI"
	// RoCEDriver name string
	RoCEDriver = "RoCE"
	// LocalDriver name string
	LocalDriver = "Local"
	// NFSDriver name string
	NFSDriver = "NFS"
	// NFSPlusDriver name string
	NFSPlusDriver = "NFS+"

	// MountFSType file system type
	MountFSType = "fs"
	// MountBlockType block type
	MountBlockType = "block"

	deviceTypeSCSI = "SCSI"
	deviceTypeNVMe = "NVMe"

	flushMultiPathInternal = 20
	// HCTLLength length of HCTL
	HCTLLength            = 4
	lengthOfUltraPathInfo = 10
	splitDeviceLength     = 2
	expandVolumeTimeOut   = 120
	expandVolumeInternal  = 5
	deviceWWidLength      = 4
	halfMiDataLength      = 524288

	// UltraPathCommand ultra-path name string
	UltraPathCommand = "ultraPath"
	// UltraPathNVMeCommand ultra-path-NVMe name string
	UltraPathNVMeCommand = "ultraPath-NVMe"

	// DMMultiPath DM-multipath name string
	DMMultiPath = "DM-multipath"
	// HWUltraPath HW-UltraPath name string
	HWUltraPath = "HW-UltraPath"
	// HWUltraPathNVMe HW-UltraPath-NVMe name string
	HWUltraPathNVMe = "HW-UltraPath-NVMe"
	// UnsupportedMultiPathType multi-path type not supported
	UnsupportedMultiPathType = "UnsupportedMultiPathType"

	// VolumeNotFound the message of volume not found error
	VolumeNotFound = "VolumeDeviceNotFound"
	// VolumePathIncomplete the message of volume path incomplete
	VolumePathIncomplete = "VolumePathIncomplete"

	// PingCommand "ping" command format string
	PingCommand = "ping -c 3 -i 0.001 -w 1 %s"
)

var (
	// connectors is the global map
	connectors = map[string]Connector{}
)

// Connector defines the behavior that the connector should have
type Connector interface {
	// ConnectVolume to mount the source to target path, the source path can be block or nfs
	// Example:
	//    mount /dev/sdb /<target-path>
	//    mount <source-path> /<target-path>
	ConnectVolume(context.Context, map[string]interface{}) (string, error)
	// DisConnectVolume to unmount the target path
	DisConnectVolume(context.Context, string) error
}

// DisConnectInfo defines the fields of disconnect volume
type DisConnectInfo struct {
	Conn   Connector
	TgtLun string
}

// ConnectInfo defines the fields of connect volume
type ConnectInfo struct {
	Conn        Connector
	MappingInfo map[string]interface{}
}

// GetConnector can get a connector by its type from the global connector map
func GetConnector(ctx context.Context, cType string) Connector {
	if cnt, exist := connectors[cType]; exist {
		return cnt
	}

	log.AddContext(ctx).Errorf("%s is not registered to connector", cType)
	return nil
}

// RegisterConnector is used to register the specific Connector to the global connector map
func RegisterConnector(cType string, cnt Connector) error {
	if _, exist := connectors[cType]; exist {
		return fmt.Errorf("connector %s already exists", cType)
	}

	connectors[cType] = cnt
	return nil
}
