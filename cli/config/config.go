/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2025. All rights reserved.
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

// Package config defines the global configurations for oceanctl
package config

import (
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

const (
	// DefaultMaxClientThreads default max client threads of common backend
	DefaultMaxClientThreads = "30"

	// DMEDefaultMaxClientThreads default max client threads of DME backend
	DMEDefaultMaxClientThreads = "5"

	// DefaultUidLength default uid length
	DefaultUidLength = 10

	// DefaultNamespace default namespace
	DefaultNamespace = "huawei-csi"

	// DefaultProvisioner default driver name
	DefaultProvisioner = "csi.huawei.com"

	// DefaultInputFormat default input format
	DefaultInputFormat = "yaml"

	// DefaultLogName default log file name
	DefaultLogName = "oceanctl-log"

	// DefaultLogSize default log file size
	DefaultLogSize = "20M"

	// DefaultLogModule default log file module
	DefaultLogModule = "file"

	// DefaultLogLevel default log file level
	DefaultLogLevel = "info"

	// DefaultLogMaxBackups default log file max backups
	DefaultLogMaxBackups = 9

	// DefaultLogDir default log dir
	DefaultLogDir = "/var/log/huawei"

	// DefaultMaxNodeThreads default max Node Threads num
	DefaultMaxNodeThreads = 50
)

var (
	// SupportedFormats supported output format
	SupportedFormats = []string{"json", "wide", "yaml"}
)

var (
	// CliVersion oceanctl version
	CliVersion = constants.CSIVersion

	// Namespace the value of namespace flag, set by options.WithNameSpace().
	Namespace string

	// OutputFormat the value of output format flag, set by options.WithOutPutFormat().
	OutputFormat string

	// FileName the value of filename flag, set by options.WithFilename().
	FileName string

	// FileType the value of input format flag, set by options.WithInputFileType().
	FileType string

	// DeleteAll the value of all flag, set by options.DeleteAll().
	DeleteAll bool

	// ChangePassword the value of password flag, set by options.WithPassword().
	ChangePassword bool

	// Provisioner the value of password flag, set by options.WithProvisioner().
	Provisioner string

	// NotValidateName the value of validate flag, set by options.WithNotValidateName
	NotValidateName bool

	// Backend the value of backend flag, set by options.WithBackend().
	Backend string

	// Client when the discoverOperating() function executes successfully, this field will be set.
	Client client.KubernetesClient

	// IsAllNodes the value of allNodes flag, set by options.WithAllNodes().
	IsAllNodes bool

	// NodeName the value of nodeName flag, set by options.WithNodeName()
	NodeName string

	// LogDir the value of log-dir flag, set by options.WithLogDir()
	LogDir string

	// MaxNodeThreads the value of threads-max flag, set by options.WithMaxThreads()
	MaxNodeThreads int

	// AuthenticationMode the value of authenticationMode flag, set by options.WithAuthenticationMode().
	AuthenticationMode string
)
