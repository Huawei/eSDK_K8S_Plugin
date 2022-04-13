/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package connector

import (
	"context"
	"fmt"
	"time"

	"huawei-csi-driver/utils/log"
)

const (
	FCDriver     = "fibreChannel"
	FCNVMeDriver = "FC-NVMe"
	ISCSIDriver  = "iSCSI"
	RoCEDriver   = "RoCE"
	LocalDriver  = "Local"
	NFSDriver    = "NFS"

	MountFSType    = "fs"
	MountBlockType = "block"

	deviceTypeSCSI = "SCSI"
	deviceTypeNVMe = "NVMe"

	flushMultiPathInternal = 20
	HCTLLength             = 4
	lengthOfUltraPathInfo  = 10
	splitDeviceLength      = 2
	expandVolumeTimeOut    = 120
	expandVolumeInternal   = 5
	deviceWWidLength       = 4
	halfMiDataLength       = 524288

	UltraPathCommand     = "ultraPath"
	UltraPathNVMeCommand = "ultraPath-NVMe"

	DMMultiPath              = "DM-multipath"
	HWUltraPath              = "HW-UltraPath"
	HWUltraPathNVMe          = "HW-UltraPath-NVMe"
	UnsupportedMultiPathType = "UnsupportedMultiPathType"

	VolumeNotFound       = "VolumeDeviceNotFound"
	VolumePathIncomplete = "VolumePathIncomplete"

	PingCommand = "ping -c 3 -i 0.001 -w 1 %s"
)

var (
	connectors        = map[string]Connector{}
	ScanVolumeTimeout = 3 * time.Second
)

type Connector interface {
	ConnectVolume(context.Context, map[string]interface{}) (string, error)
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

func GetConnector(ctx context.Context, cType string) Connector {
	if cnt, exist := connectors[cType]; exist {
		return cnt
	}

	log.AddContext(ctx).Errorf("%s is not registered to connector", cType)
	return nil
}

func RegisterConnector(cType string, cnt Connector) error {
	if _, exist := connectors[cType]; exist {
		return fmt.Errorf("connector %s already exists", cType)
	}

	connectors[cType] = cnt
	return nil
}
