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

// Package app get all configs for the service
package app

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/options"
)

const (
	huaweiCSIService = "huawei-csi"
)

var globalCfg *cfg.CompletedConfig

// NewCommand parses the configuration
func NewCommand() *cobra.Command {
	flagSet := flag.NewFlagSet(huaweiCSIService, flag.ContinueOnError)
	if err := flagSet.Parse(os.Args); err != nil {
		logrus.Fatalf("Parse flag error: %v", err)
	}

	return commandFunc(flagSet)
}

func commandFunc(flagSet *flag.FlagSet) *cobra.Command {
	optManager := options.NewOptionsManager()
	runFunc := func(cmd *cobra.Command, args []string) {
		if err := flagSet.Parse(args); err != nil {
			logrus.Fatalf("Failed to parse args, error: %v", err)
		}

		cmdArgs := flagSet.Args()
		if len(cmdArgs) > 0 {
			logrus.Fatalf("Unknown command, %v", cmdArgs)
		}

		envCfg, err := optManager.Config()
		if err != nil {
			logrus.Fatalf("Failed to get configuration, %v", err)
		}

		globalCfg, err = envCfg.Complete()
		if err != nil {
			logrus.Fatalf("Failed to get all configuration, %v", err)
		}
		globalCfg.Print()
	}

	optManager.AddFlags(flagSet)
	return &cobra.Command{
		Use:                huaweiCSIService,
		Long:               "",
		DisableFlagParsing: true,
		Run:                runFunc,
	}
}

// GetGlobalConfig used to get global configuration
var GetGlobalConfig = func() *cfg.CompletedConfig {
	return globalCfg
}
