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

func registerCreateCertCmd() {
	options.NewFlagsOptions(createSecretCmd).
		WithNameSpace(false).
		WithFilename(true).
		WithBackend(true).
		WithParent(CreateCmd)
}

var (
	createSecretExample = helper.Examples(`
		# Create certificate in default(huawei-csi) namespace based on cert.crt file
		oceanctl create cert <name> -f /path/to/cert.crt -b <backend-name>
		
		# Create certificate in specified namespace based on cert.crt file
		oceanctl create cert <name> -f /path/to/cert.crt -n <namespace> -b <backend-name>

		# Create certificate in specified namespace based on cert.pem file
		oceanctl create cert <name> -f /path/to/cert.pem -n <namespace> -b <backend-name>
	`)
)

var createSecretCmd = &cobra.Command{
	Use:     "cert <name>",
	Short:   "Create a certificate for specified backend in Kubernetes",
	Example: createSecretExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreateCert(args)
	},
}

func runCreateCert(SecretNames []string) error {
	res := resources.NewResourceBuilder().
		ResourceNames(string(client.Secret), SecretNames...).
		ResourceTypes(string(client.Secret)).
		NamespaceParam(config.Namespace).
		DefaultNamespace().
		FileName(config.FileName).
		BoundBackend(config.Backend).
		Build()

	validator := resources.NewValidatorBuilder(res).ValidateNameIsExist().ValidateNameIsSingle().Build()
	if err := validator.Validate(); err != nil {
		return helper.PrintlnError(err)
	}

	return resources.NewCert(res).Create()
}
