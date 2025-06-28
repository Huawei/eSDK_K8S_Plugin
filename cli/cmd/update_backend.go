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

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/cmd/options"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/helper"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/resources"
)

func registerUpdateBackendCmd() {
	options.NewFlagsOptions(updateBackendCmd).
		WithNameSpace(false).
		WithPassword(true).
		WithAuthenticationMode(false).
		WithParent(updateCmd)
}

var (
	updateBackendExample = helper.Examples(`
		# Update backend account information in default(huawei-csi) namespace
		oceanctl update backend <name>  --password

	    # Update backend account information in specified namespace
		oceanctl update backend <name> -n namespace --password

		# Update backend account information with ldap authentication mode in default(huawei-csi) namespace
		oceanctl update backend <name> --password --authenticationMode=ldap

		# Update backend account information with local authentication mode in default(huawei-csi) namespace
		oceanctl update backend <name> --password --authenticationMode=local

		# Update backend account information with ldap authentication mode in specified namespace
		oceanctl update backend <name> -n namespace --password --authenticationMode=ldap`)
)

var updateBackendCmd = &cobra.Command{
	Use:     "backend <name>",
	Short:   "Update a backend for Ocean Storage in Kubernetes",
	Example: updateBackendExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdateBackend(args)
	},
}

func runUpdateBackend(backendNames []string) error {
	res := resources.NewResourceBuilder().
		ResourceNames(string(client.Storagebackendclaim), backendNames...).
		NamespaceParam(config.Namespace).
		DefaultNamespace().
		Build()

	validator := resources.NewValidatorBuilder(res).ValidateNameIsExist().ValidateNameIsSingle().
		ValidateAuthenticationMode().Build()
	if err := validator.Validate(); err != nil {
		return helper.PrintlnError(err)
	}

	return resources.NewBackend(res).Update()
}
