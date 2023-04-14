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
	options.NewFlagsOptions(getBackendCmd).
		WithNameSpace(false).
		WithOutPutFormat().
		WithParent(getCmd)
}

var (
	getBackendExample = helper.Examples(`
		# List all backend in specified namespace
		oceanctl get backend -n <namespace>

		# List all backend in specified namespace with more information (such as storageType)
		oceanctl get backend -n <namespace> -owide 
		
		# List a single backend with JSON output format in default(huawei-csi) namespace
		oceanctl get backend <name> -o json`)
)

var getBackendCmd = &cobra.Command{
	Use:     "backend [<name>...]",
	Short:   "Get one or more backends from Ocean Storage in Kubernetes",
	Example: getBackendExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGetBackend(args)
	},
}

func runGetBackend(backendNames []string) error {
	res := resources.NewResourceBuilder().
		ResourceNames(string(client.Storagebackendclaim), backendNames...).
		NamespaceParam(config.Namespace).
		DefaultNamespace().
		Output(config.OutputFormat).
		Build()

	validator := resources.NewValidatorBuilder(res).ValidateOutputFormat().Build()
	if err := validator.Validate(); err != nil {
		return helper.PrintlnError(err)
	}

	out, err := resources.NewBackend(res).Get()
	if err != nil {
		return helper.PrintError(err)
	}

	helper.PrintResult(string(out))
	return nil
}
