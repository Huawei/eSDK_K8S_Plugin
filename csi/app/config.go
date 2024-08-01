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

var globalCfg *Config

// Config contains the configurations from env
type Config struct {
	VolumeUseMultiPath bool
	ScsiMultiPathType  string
	NvmeMultiPathType  string
	AllPathOnline      bool

	// kubeletVolumeDevicesDirName, default is /volumeDevices/
	KubeletVolumeDevicesDirName string
}

// GetGlobalConfig used to get global configuration
func GetGlobalConfig() *Config {
	return globalCfg
}
