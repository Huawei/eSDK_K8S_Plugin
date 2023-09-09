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

package config

import (
	"huawei-csi-driver/cli/client"
)

const (
	//CliVersion oceanctl version
	CliVersion = "v4.1.1"

	// DefaultMaxClientThreads default max client threads
	DefaultMaxClientThreads = "30"

	// DefaultUidLength default uid length
	DefaultUidLength = 10

	// DefaultNamespace default namespace
	DefaultNamespace = "huawei-csi"

	// DefaultProvisioner default driver name
	DefaultProvisioner = "csi.huawei.com"

	// DefaultInputFormat default input format
	DefaultInputFormat = "yaml"
)

var (
	// SupportedFormats supported output format
	SupportedFormats = []string{"json", "wide", "yaml"}
)

var (
	// Namespace the value of namespace flag, set by options.WithNameSpace().
	Namespace string

	// OutputFormat the value of output format flag, set by options.WithOutPutFormat().
	OutputFormat string

	// FileName the value of filename flag, set by options.WithFilename().
	FileName string

	// FileType the value of input format flag, set by options.WithInputFileType().
	FileType string

	//DeleteAll the value of all flag, set by options.DeleteAll().
	DeleteAll bool

	// ChangePassword the value of password flag, set by options.WithPassword().
	ChangePassword bool

	// Provisioner the value of password flag, set by options.WithProvisioner().
	Provisioner string

	// NotValidateName the value of validate flag, set by options.WithNotValidateName
	NotValidateName bool

	// Client when the discoverOperating() function executes successfully, this field will be set.
	Client client.KubernetesClient
)
