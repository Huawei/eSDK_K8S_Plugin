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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/cmd/options"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/helper"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/resources"
)

func registerDeleteCertCmd() {
	options.NewFlagsOptions(deleteSecretCmd).
		WithNameSpace(false).
		WithBackend(true).
		WithParent(deleteCmd)
}

var (
	deleteSecretExample = helper.Examples(`
		# Delete certificate of specified backend in default(huawei-csi) namespace
		oceanctl delete cert -b <backend-name>
		
		# Delete certificate of specified backend in specified namespace
		oceanctl delete cert -n <namespace> -b <backend-name>
	`)
)

var deleteSecretCmd = &cobra.Command{
	Use:     "cert",
	Short:   "Delete the certificate of specified backend in Kubernetes",
	Example: deleteSecretExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeleteCert()
	},
}

func runDeleteCert() error {
	res := resources.NewResourceBuilder().
		NamespaceParam(config.Namespace).
		DefaultNamespace().
		BoundBackend(config.Backend).
		Build()

	validator := resources.NewValidatorBuilder(res).ValidateBackend().Build()
	if err := validator.Validate(); err != nil {
		return helper.PrintlnError(err)
	}

	return resources.NewCert(res).Delete()
}
