/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

// Package constants is storage-related constants
package constants

// OceanstorVersion defines Oceantor storage's version
type OceanstorVersion string

// OceanDiskVersion defines OceanDisk storage's version
type OceanDiskVersion string

// IsDorado checks whether the version is Dorado
func (ver OceanstorVersion) IsDorado() bool {
	return ver == "Dorado"
}

// IsDoradoV6OrV7 checks whether the version is Dorado v6 or Dorado v7
func (ver OceanstorVersion) IsDoradoV6OrV7() bool {
	return ver.IsDoradoV6() || ver.IsDoradoV7()
}

// IsDoradoV6 checks whether the version is Dorado v6
func (ver OceanstorVersion) IsDoradoV6() bool {
	return ver == OceanStorDoradoV6
}

// IsDoradoV7 checks whether the version is Dorado v7
func (ver OceanstorVersion) IsDoradoV7() bool {
	return ver == OceanStorDoradoV7
}

const (
	// OceanStorDoradoV7 is dorado v7
	OceanStorDoradoV7 OceanstorVersion = "DoradoV7"
	// OceanStorDoradoV6 Dorado V6 and OceanStor V6 are exactly the same
	OceanStorDoradoV6 OceanstorVersion = "DoradoV6"
	// OceanStorDoradoV3 is dorado v3
	OceanStorDoradoV3 OceanstorVersion = "DoradoV3"
	// OceanStorV3 is oceanstor v3
	OceanStorV3 OceanstorVersion = "OceanStorV3"
	// OceanStorV5 is oceanstor v5
	OceanStorV5 OceanstorVersion = "OceanStorV5"
)

const (
	// DoradoV615 is Dorado V6.1.5
	DoradoV615 = "6.1.5"

	// MinVersionSupportNfsPlus version gte 6.1.7 support label function
	MinVersionSupportNfsPlus = "6.1.7"
	// OceanStor9000 storage type is oceanstor-9000
	OceanStor9000 = "oceanstor-9000"
	// OceanStorDtree storage type is oceanstor-dtree
	OceanStorDtree = "oceanstor-dtree"
	// OceanStorNas storage type is oceanstor-nas
	OceanStorNas = "oceanstor-nas"
	// OceanStorSan storage type is oceanstor-san
	OceanStorSan = "oceanstor-san"
	// OceandiskSan storage type is oceandisk-san
	OceandiskSan = "oceandisk-san"
	// FusionSan storage type is fusionstorage-san
	FusionSan = "fusionstorage-san"
	// FusionNas storage type is fusionstorage-nas
	FusionNas = "fusionstorage-nas"

	// CloneSpeedLevel1 means level1 of the clone speed
	CloneSpeedLevel1 = 1
	// CloneSpeedLevel2 means level2 of the clone speed
	CloneSpeedLevel2 = 2
	// CloneSpeedLevel3 means level3 of the clone speed
	CloneSpeedLevel3 = 3
	// CloneSpeedLevel4 means level4 of the clone speed
	CloneSpeedLevel4 = 4
)

// BackendCapability backend capability
type BackendCapability string

// SupportThin defines backend capability SupportThin
var SupportThin BackendCapability = "SupportThin"

// SupportThick defines backend capability SupportThick
var SupportThick BackendCapability = "SupportThick"

// SupportQoS defines backend capability SupportQoS
var SupportQoS BackendCapability = "SupportQoS"

// SupportQuota defines backend capability SupportQuota
var SupportQuota BackendCapability = "SupportQuota"

// SupportClone defines backend capability SupportClone
var SupportClone BackendCapability = "SupportClone"

// SupportMetro defines backend capability SupportMetro
var SupportMetro BackendCapability = "SupportMetro"

// SupportReplication defines backend capability SupportReplication
var SupportReplication BackendCapability = "SupportReplication"

// SupportApplicationType defines backend capability SupportApplicationType
var SupportApplicationType BackendCapability = "SupportApplicationType"

// SupportMetroNAS defines backend capability SupportMetroNAS
var SupportMetroNAS BackendCapability = "SupportMetroNAS"
