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

	"huawei-csi-driver/cli/client"
	"huawei-csi-driver/cli/cmd/options"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
	"huawei-csi-driver/cli/resources"
)

func registerCreateBackendCmd() {
	options.NewFlagsOptions(createBackendCmd).
		WithNameSpace(false).
		WithFilename(true).
		WithInputFileType().
		WithProvisioner().
		WithNotValidateName().
		WithParent(CreateCmd)
}

var (
	createBackendExample = helper.Examples(`
		# Create backend in default(huawei-csi) namespace based on backend.yaml file
		oceanctl create backend -f /path/to/backend.yaml -i yaml
		
		# Create backend in specified namespace based on backend.yaml file
		oceanctl create backend -f /path/to/backend.yaml -i yaml -n <namespace>

		# Create backend in specified namespace based on config.json file
		oceanctl create backend -f /path/to/configmap.json -i json -n <namespace>

		# Create backend with specified provisioner
		oceanctl create backend -f /path/to/backend.yaml -i yaml --provisioner=csi.huawei.com -n <namespace>

		# Create backend with not validate backend name
		oceanctl create backend -f /path/to/backend.yaml -i yaml --not-validate-name
	`)
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
		FileName(config.FileName).
		FileType(config.FileType).
		Build()

	return resources.NewBackend(res).Create()
}
