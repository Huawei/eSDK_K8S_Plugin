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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/cmd/options"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/helper"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/resources"
)

func registerDeleteBackendCmd() {
	options.NewFlagsOptions(deleteBackendCmd).
		WithNameSpace(false).
		WithDeleteAll().
		WithParent(deleteCmd)
}

var (
	deleteBackendExample = helper.Examples(`
		# Delete a backend in default(huawei-csi) namespace
		oceanctl delete backend <name> 
		
		# Delete specified backends in specified namespace
		oceanctl delete backend <name...> -n <namespace>

        # Delete all backends in specified namespace
		oceanctl delete backend -n <namespace> --all`)
)

var deleteBackendCmd = &cobra.Command{
	Use:     "backend <name>",
	Short:   "Delete one or all backends from Ocean Storage in Kubernetes",
	Example: deleteBackendExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeleteBackends(args)
	},
}

func runDeleteBackends(backendNames []string) error {
	res := resources.NewResourceBuilder().
		ResourceNames(string(client.Storagebackendclaim), backendNames...).
		NamespaceParam(config.Namespace).
		DefaultNamespace().
		SelectAll(config.DeleteAll).
		Build()

	validator := resources.NewValidatorBuilder(res).ValidateSelector().Build()
	if err := validator.Validate(); err != nil {
		return helper.PrintlnError(err)
	}

	return resources.NewBackend(res).Delete()
}
