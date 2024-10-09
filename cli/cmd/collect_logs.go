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

package cmd

import (
	"github.com/spf13/cobra"

	"huawei-csi-driver/cli/cmd/options"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
	"huawei-csi-driver/cli/resources"
)

func registerCollectLogsCmd() {
	options.NewFlagsOptions(collectLogsCmd).
		WithNameSpace(true).
		WithAllNodes().
		WithNodeName().
		WithMaxThreads().
		WithParent(collectCmd)
}

var (
	collectLogsExample = helper.Examples(`
		# Collect logs of all nodes in specified namespace
		oceanctl collect logs -n <namespace>

		# Collect logs of specified node in specified namespace
		oceanctl collect logs -n <namespace> -N <node>

		# Collect logs of all nodes in specified namespace
		oceanctl collect logs -n <namespace> -a

		# Collect logs of all nodes in specified namespace with a maximum of 50 nodes collected at the same time
		oceanctl collect logs -n <namespace> -a --threads-max=50

		# Collect logs of specified node in specified namespace
		oceanctl collect logs -n <namespace> -N <node> -a`)
)

var collectLogsCmd = &cobra.Command{
	Use:     "logs",
	Short:   "Collect logs of one or more nodes in specified namespace in Kubernetes",
	Example: collectLogsExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCollectLogs()
	},
}

func runCollectLogs() error {
	res := resources.NewResourceBuilder().
		AllNodes(config.IsAllNodes).
		NodeName(config.NodeName).
		NamespaceParam(config.Namespace).
		MaxNodeThreads(config.MaxNodeThreads).
		Build()

	return resources.NewLogs(res).Collect()
}
