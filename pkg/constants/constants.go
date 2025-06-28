/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.

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

// Package constants is related with provider constants
package constants

import "errors"

// FileType defines file type
type FileType string

var (
	// CSIVersion defines version for CSI, and it will be assigned value dynamic at build time
	CSIVersion string

	// ProviderVersion defines provider version
	ProviderVersion = CSIVersion
)

const (
	// ProviderVendorName defines provider vendor name
	ProviderVendorName = "Huawei"
	// EndpointDirPermission defines permission of endpoint dir
	EndpointDirPermission = 0755

	// CentralizedStorageNas is the centralized storage nas type
	CentralizedStorageNas = "centralized-storage-nas"

	// LicenseValid is a valid license
	LicenseValid = 1
	// LicenseExpired is an expired license
	LicenseExpired = 2

	// NamespaceEnv is driver namespace env
	NamespaceEnv = "CSI_NAMESPACE"
	// DefaultNamespace is driver default namespace
	DefaultNamespace = "huawei-csi"

	// Ext2 defines the fileType ext2
	Ext2 FileType = "ext2"
	// Ext3 defines the fileType ext3
	Ext3 FileType = "ext3"
	// Ext4 defines the fileType ext4
	Ext4 FileType = "ext4"
	// Xfs defines the fileType xfs
	Xfs FileType = "xfs"

	// NodeNameEnv is defined in helm file
	NodeNameEnv = "CSI_NODENAME"

	// DefaultDriverName is default huawei csi driver name
	DefaultDriverName = "csi.huawei.com"
	// DefaultTopoDriverName is default topo driver name
	DefaultTopoDriverName = "cmi.huawei.com"

	// PVKind is defined by k8s
	PVKind = "PersistentVolume"
	// PodKind is defined by k8s
	PodKind = "Pod"

	// KubernetesV1 is kubernetes v1 api version
	KubernetesV1 = "v1"
	// XuanwuV1 is xuanwu v1 api version
	XuanwuV1 = "xuanwu.huawei.io/v1"

	// NotMountStr defines not mount str
	NotMountStr = "not mounted"

	// DefaultKubeletVolumeDevicesDirName default kubelet volumeDevice name
	DefaultKubeletVolumeDevicesDirName = "/volumeDevices/"

	// AllocationUnitBytes default is 512 Bytes, it is the allocation and capacity unit for OceanStor
	AllocationUnitBytes = 512
	// FusionAllocUnitBytes default is 1Mi, it is the allocation unit for FusionStorage
	FusionAllocUnitBytes = 1024 * 1024
	// FusionFileCapacityUnit default is 1024 Bytes, it is the capacity unit for FusionStorage
	FusionFileCapacityUnit int64 = 1024
	// FusionDTreeCapacityUnit default is 1 Bytes, it is the capacity unit for FusionStorage DTree quota
	FusionDTreeCapacityUnit int64 = 1

	// DefaultIntBase is the default value of int base
	DefaultIntBase = 10
	// DefaultIntBitSize is the default value of bit size
	DefaultIntBitSize = 64

	// DTreeParentKey is the key of DTree parent name parameter
	DTreeParentKey = "dTreeParentName"
	// DisableVerifyCapacityKey is the key of disableVerifyCapacity parameter
	DisableVerifyCapacityKey = "disableVerifyCapacity"
	// AdvancedOptionsKey is the key of advanced volume options parameter in StorageClass
	AdvancedOptionsKey = "advancedOptions"
	// ScVolumeNameKey is the key of volumeName in StorageClass
	ScVolumeNameKey = "volumeName"

	// PVCNameKey is the key of PVC name in CreateVolumeRequest parameters
	PVCNameKey = "csi.storage.k8s.io/pvc/name"
	// PVCNamespaceKey is the key of PVC namespace in CreateVolumeRequest parameters
	PVCNamespaceKey = "csi.storage.k8s.io/pvc/namespace"
	// PVNameKey is the key of PV name in CreateVolumeRequest parameters
	PVNameKey = "csi.storage.k8s.io/pv/name"

	// AuthenticationModeKey is the param for login backend
	AuthenticationModeKey = "authenticationMode"
	// AuthModeLocal is the login backend param
	AuthModeLocal = "local"
	// AuthModeLDAP is the login backend param
	AuthModeLDAP = "ldap"
	// AuthModeScopeLocal is the login backend scope param
	AuthModeScopeLocal = "0"
	// AuthModeScopeLDAP is the login backend scope param
	AuthModeScopeLDAP = "1"
)

var (
	// ErrTimeout defines the timeout error
	ErrTimeout = errors.New("timeout")
)

// DRCSIConfig contains storage normal configuration
type DRCSIConfig struct {
	Backends map[string]interface{} `json:"backends"`
}

// DRCSISecret contains storage sensitive configuration
type DRCSISecret struct {
	Secrets map[string]interface{} `json:"secrets"`
}
