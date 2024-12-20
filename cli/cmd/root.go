/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

// cmd defines commands of oceanctl.
package cmd

import (
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/cmd/options"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// RootCmd is a root command of oceanctl.
var RootCmd = &cobra.Command{
	SilenceUsage:      true,
	Use:               "oceanctl",
	Short:             "A CLI tool for Ocean Storage in Kubernetes",
	CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := startLogging()
		if err != nil {
			return err
		}
		return discoverOperating()
	},
}

// Execute runs the root command
func Execute() error {
	registerRootCmd()
	registerCollectCmd()
	registerCollectLogsCmd()
	registerCreateCmd()
	registerCreateBackendCmd()
	registerCreateCertCmd()
	registerDeleteCmd()
	registerDeleteBackendCmd()
	registerDeleteCertCmd()
	registerGetCmd()
	registerGetBackendCmd()
	registerGetCertCmd()
	registerUpdateCmd()
	registerUpdateBackendCmd()
	registerUpdateCertCmd()
	registerVersionCmd()

	return RootCmd.Execute()
}

func registerRootCmd() {
	options.NewFlagsOptions(RootCmd).WithLogDir()
}

func discoverOperating() error {
	absPath, err := filepath.Abs(config.LogDir)
	if err != nil {
		return err
	}

	clientName, err := client.DiscoverKubernetesCLI(path.Join(absPath, config.DefaultLogName))
	if err != nil {
		return err
	}

	kubernetesClient, err := client.LoadSupportedClient(clientName)
	if err != nil {
		return err
	}

	config.Client = kubernetesClient
	return nil
}

// startLogging used to start logging.
// Since the cli tool does not need to specify a log configuration, the default values are used here.
func startLogging() error {
	if config.LogDir == "" {
		config.LogDir = config.DefaultLogDir
	}
	logRequest := &log.Config{
		LogName:       config.DefaultLogName,
		LogFileSize:   config.DefaultLogSize,
		LoggingModule: config.DefaultLogModule,
		LogLevel:      config.DefaultLogLevel,
		LogFileDir:    config.LogDir,
		MaxBackups:    config.DefaultLogMaxBackups,
	}
	return log.InitLogging(logRequest)
}
