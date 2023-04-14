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
	"huawei-csi-driver/cli/cmd/options"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
	"huawei-csi-driver/cli/resources"
)

func init() {
	options.NewFlagsOptions(createBackendCmd).
		WithNameSpace(false).
		WithFilename(true).
		WithParent(CreateCmd)
}

var (
	createBackendExample = helper.Examples(`
		# Create a new backend in default(huawei-csi) namespace based on backend.yaml file
		oceanctl create backend -f /path/to/backend.yaml
		
		# Create a new backend in specified namespace based on backend.yaml file
		oceanctl create backend -f /path/to/backend.yaml -n <namespace>`)
)

var createBackendCmd = &cobra.Command{
	Use:     "backend",
	Short:   "Create a backend to Ocean Storage in Kubernetes",
	Example: createBackendExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreateBackend()
	},
}

func runCreateBackend() error {
	res := resources.NewResourceBuilder().
		ResourceTypes(string(client.Storagebackendclaim)).
		NamespaceParam(config.Namespace).
		DefaultNamespace().
		FileName(config.FileName).
		Build()

	return resources.NewBackend(res).Create()
}
