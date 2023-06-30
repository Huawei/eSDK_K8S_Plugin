/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.

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

type FileType string

const (
	ProviderVersion       = "4.1.0"
	ProviderVendorName    = "Huawei"
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

	// Ext2 list the fileType
	Ext2 FileType = "ext2"
	Ext3 FileType = "ext3"
	Ext4 FileType = "ext4"
	Xfs  FileType = "xfs"
)

// DRCSIConfig contains storage normal configuration
type DRCSIConfig struct {
	Backends map[string]interface{} `json:"backends"`
}

// DRCSISecret contains storage sensitive configuration
type DRCSISecret struct {
	Secrets map[string]interface{} `json:"secrets"`
}
