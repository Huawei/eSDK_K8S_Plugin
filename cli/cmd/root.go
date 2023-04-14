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

package command

import (
	"github.com/spf13/cobra"

	"huawei-csi-driver/cli/client"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/utils/log"
)

var RootCmd = &cobra.Command{
	SilenceUsage:      true,
	Use:               "oceanctl",
	Short:             "A CLI tool for Ocean Storage in Kubernetes",
	CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		startLogging()
		return discoverOperating()
	},
}

func discoverOperating() error {
	clientName, err := client.DiscoverKubernetesCLI()
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
// Since the cli tool does not need to specify a log configuration, the default values are used here
func startLogging() {
	logRequest := &log.LoggingRequest{
		LogName:       "oceanctl-log",
		LogFileSize:   "20M",
		LoggingModule: "file",
		LogLevel:      "info",
		LogFileDir:    "/var/log/huawei",
		MaxBackups:    9,
	}
	_ = log.InitLogging(logRequest)
}
