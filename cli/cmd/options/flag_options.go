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

package options

import (
	"github.com/spf13/cobra"

	"huawei-csi-driver/cli/config"
)

type FlagsOptions struct {
	cmd *cobra.Command
}

// NewFlagsOptions Construct a FlagsOptions instance that requires a cmd as a parameter
func NewFlagsOptions(cmd *cobra.Command) *FlagsOptions {
	return &FlagsOptions{cmd: cmd}
}

// WithParent This function will add a parent command
func (b *FlagsOptions) WithParent(parentCmd *cobra.Command) {
	parentCmd.AddCommand(b.cmd)
}

// WithNameSpace This function will add a namespace flag
// If required is true, namespace flag must be set
func (b *FlagsOptions) WithNameSpace(required bool) *FlagsOptions {
	b.cmd.PersistentFlags().StringVarP(&config.Namespace, "namespace", "n", "", "namespace of resources")
	if required {
		// Because only 'no such flag' error will be returned, and we have ensured
		// that the incoming parameters are correct, so no err will be handled.
		_ = b.cmd.MarkPersistentFlagRequired("namespace")
	}
	return b
}

// WithFilename This function will add a filename flag
// If required is true, filename flag must be set
func (b *FlagsOptions) WithFilename(required bool) *FlagsOptions {
	b.cmd.PersistentFlags().StringVarP(&config.FileName, "filename", "f", "", "path to yaml file")
	if required {
		// Because only 'no such flag' error will be returned, and we have ensured
		// that the incoming parameters are correct, so no err will be handled.
		_ = b.cmd.MarkPersistentFlagRequired("filename")
	}
	return b
}

// WithOutPutFormat this function will add an output format flag
func (b *FlagsOptions) WithOutPutFormat() *FlagsOptions {
	b.cmd.PersistentFlags().StringVarP(&config.OutputFormat, "output", "o", "", "output format. One of "+
		"json|yaml|wide (default)")
	return b
}

// WithDeleteAll this function will add a deleted all options
func (b *FlagsOptions) WithDeleteAll() *FlagsOptions {
	b.cmd.PersistentFlags().BoolVarP(&config.DeleteAll, "all", "", false, "Delete all backends")
	return b
}

// WithPassword this function will add a change password options
func (b *FlagsOptions) WithPassword(required bool) *FlagsOptions {
	b.cmd.PersistentFlags().BoolVarP(&config.ChangePassword, "password", "", false, "Update account password")
	if required {
		_ = b.cmd.MarkPersistentFlagRequired("password")
	}
	return b
}
