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

const (
	// OceanStorDoradoV6 Dorado V6 and OceanStor V6 are exactly the same
	OceanStorDoradoV6 = "DoradoV6"
	// OceanStorDoradoV3 is dorado v3
	OceanStorDoradoV3 = "DoradoV3"
	// OceanStorV3 is oceanstor v3
	OceanStorV3 = "OceanStorV3"
	// OceanStorV5 is oceanstor v5
	OceanStorV5 = "OceanStorV5"

	// MinVersionSupportLabel version gte 6.1.7 support label function
	MinVersionSupportLabel = "6.1.7"
)

// BackendCapability backend capability
type BackendCapability string

var SupportThin BackendCapability = "SupportThin"
var SupportThick BackendCapability = "SupportThick"
var SupportQoS BackendCapability = "SupportQoS"
var SupportQuota BackendCapability = "SupportQuota"
var SupportClone BackendCapability = "SupportClone"
var SupportMetro BackendCapability = "SupportMetro"
var SupportReplication BackendCapability = "SupportReplication"
var SupportApplicationType BackendCapability = "SupportApplicationType"
var SupportMetroNAS BackendCapability = "SupportMetroNAS"
var SupportLabel BackendCapability = "SupportLabel"
