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

	"huawei-csi-driver/cli/cmd/options"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
	"huawei-csi-driver/cli/resources"
)

func init() {
	options.NewFlagsOptions(updateSecretCmd).
		WithNameSpace(false).
		WithFilename(true).
		WithBackend(true).
		WithParent(updateCmd)
}

var (
	updateSecretExample = helper.Examples(`
		# Update certificate of specified backend in default(huawei-csi) namespace based on cert.crt file
		oceanctl update cert -b <backend-name> -f /path/to/cert.crt

		# Update certificate of specified backend in specified namespace based on cert.pem file
		oceanctl update cert -b <backend-name> -n namespace -f /path/to/cert.pem

		# Update certificate of specified backend in specified namespace based on cert.crt file
		oceanctl update cert -b <backend-name> -n namespace -f /path/to/cert.crt
 	`)
)

var updateSecretCmd = &cobra.Command{
	Use:     "cert",
	Short:   "Update the certificate of specified backend in Kubernetes",
	Example: updateSecretExample,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUpdateCert()
	},
}

func runUpdateCert() error {
	res := resources.NewResourceBuilder().
		NamespaceParam(config.Namespace).
		DefaultNamespace().
		FileName(config.FileName).
		BoundBackend(config.Backend).
		Build()

	validator := resources.NewValidatorBuilder(res).ValidateBackend().Build()
	if err := validator.Validate(); err != nil {
		return helper.PrintlnError(err)
	}

	return resources.NewCert(res).Update()
}
