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

package app

import "sync"

var once sync.Once

func Builder() *Config {
	once.Do(func() {
		globalCfg = &Config{}
	})
	return globalCfg
}

// WithVolumeUseMultipath Used to set whether to use multipath
func (c *Config) WithVolumeUseMultipath(volumeUseMultipath bool) *Config {
	c.VolumeUseMultiPath = volumeUseMultipath
	return c
}

// WithScsiMultipathType Used to set the multipath type used by the scsi protocol
func (c *Config) WithScsiMultipathType(scsiMultipathType string) *Config {
	c.ScsiMultiPathType = scsiMultipathType
	return c
}

// WithNvmeMultipathType Used to set the multipath type used by the nvme protocol
func (c *Config) WithNvmeMultipathType(nvmeMultipathType string) *Config {
	c.NvmeMultiPathType = nvmeMultipathType
	return c
}

// WithAllPathOnline Used to check the number of paths for DM-multipath aggregation
func (c *Config) WithAllPathOnline(allPathOnline bool) *Config {
	c.AllPathOnline = allPathOnline
	return c
}

//Build Used to set global configuration
func (c *Config) Build() {
	globalCfg = c
}
