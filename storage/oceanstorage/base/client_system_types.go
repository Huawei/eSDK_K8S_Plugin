/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package base provide base operations for oceanstor and oceandisk storage
package base

// OceanstorSystem holds the system information of Oceanstor storage
type OceanstorSystem struct {
	CacheWriteQuota              string `json:"CACHEWRITEQUOTA"`
	ConfigModel                  string `json:"CONFIGMODEL"`
	Description                  string `json:"DESCRIPTION"`
	DomainName                   string `json:"DOMAINNAME"`
	FreeDisksCapacity            string `json:"FREEDISKSCAPACITY"`
	HealthStatus                 string `json:"HEALTHSTATUS"`
	HotSpareDisksCapacity        string `json:"HOTSPAREDISKSCAPACITY"`
	ID                           string `json:"ID"`
	Location                     string `json:"LOCATION"`
	MemberDisksCapacity          string `json:"MEMBERDISKSCAPACITY"`
	Name                         string `json:"NAME"`
	ProductMode                  string `json:"PRODUCTMODE"`
	ProductVersion               string `json:"PRODUCTVERSION"`
	RunningStatus                string `json:"RUNNINGSTATUS"`
	SectorSize                   string `json:"SECTORSIZE"`
	StoragePoolCapacity          string `json:"STORAGEPOOLCAPACITY"`
	StoragePoolFreeCapacity      string `json:"STORAGEPOOLFREECAPACITY"`
	StoragePoolHostSpareCapacity string `json:"STORAGEPOOLHOSTSPARECAPACITY"`
	StoragePoolRawCapacity       string `json:"STORAGEPOOLRAWCAPACITY"`
	StoragePoolUsedCapacity      string `json:"STORAGEPOOLUSEDCAPACITY"`
	ThickLunsAllocateCapacity    string `json:"THICKLUNSALLOCATECAPACITY"`
	ThickLunsUsedCapacity        string `json:"THICKLUNSUSEDCAPACITY"`
	ThinLunsAllocateCapacity     string `json:"THINLUNSALLOCATECAPACITY"`
	ThinLunsMaxCapacity          string `json:"THINLUNSMAXCAPACITY"`
	ThinLunsUsedCapacity         string `json:"THINLUNSUSEDCAPACITY"`
	TotalCapacity                string `json:"TOTALCAPACITY"`
	Type                         int    `json:"TYPE"`
	UnavailableDisksCapACITY     string `json:"UNAVAILABLEDISKSCAPACITY"`
	UsedCapacity                 string `json:"USEDCAPACITY"`
	VasaAlternateName            string `json:"VASA_ALTERNATE_NAME"`
	VasaSupportBlock             string `json:"VASA_SUPPORT_BLOCK"`
	VasaSupportFilesystem        string `json:"VASA_SUPPORT_FILESYSTEM"`
	VasaSupportProfile           string `json:"VASA_SUPPORT_PROFILE"`
	WriteThroughSw               string `json:"WRITETHROUGHSW"`
	WriteThroughTime             string `json:"WRITETHROUGHTIME"`
	MappedLunsCountCapacity      string `json:"mappedLunsCountCapacity"`
	PatchVersion                 string `json:"patchVersion"`
	PointRelease                 string `json:"pointRelease"`
	ProductModeString            string `json:"productModeString"`
	UnMappedLunsCountCapacity    string `json:"unMappedLunsCountCapacity"`
	UserFreeCapacity             string `json:"userFreeCapacity"`
	Wwn                          string `json:"wwn"`
	ApolloVersion                string `json:"apolloVersion"`
}
