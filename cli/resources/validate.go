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

package resources

import (
	"fmt"
	"strings"

	"k8s.io/utils/strings/slices"

	"huawei-csi-driver/cli/config"
)

type Validator struct {
	errs     []error
	resource *Resource
}

type ValidatorBuilder struct {
	*Validator
}

// Validate used to validate whether the command params are legal
func (v *Validator) Validate() error {
	if len(v.errs) != 0 {
		return v.errs[0]
	}
	return nil
}

// NewValidatorBuilder initialize a ValidatorBuilder instance
func NewValidatorBuilder(resource *Resource) *ValidatorBuilder {
	return &ValidatorBuilder{
		Validator: &Validator{
			resource: resource,
		},
	}
}

// Build used to convert ValidatorBuilder to Validator
func (b *ValidatorBuilder) Build() *Validator {
	return b.Validator
}

// ValidateSelector used to validate selector. For example, the following operations are illegal
// oceanctl delete backend
// oceanctl delete backend <name> --all
func (b *ValidatorBuilder) ValidateSelector() *ValidatorBuilder {
	if b.resource.selectAll && len(b.resource.names) != 0 {
		b.errs = append(b.errs, fmt.Errorf("name cannot be provided when a selector is specified"))
		return b
	}

	if !b.resource.selectAll && len(b.resource.names) == 0 {
		b.errs = append(b.errs, fmt.Errorf("resources were provided, but no name or selector was specified"))
	}

	return b
}

// ValidateNameIsExist used to validate resource names is exists. For example, the following operations are illegal
// oceanctl update backend
func (b *ValidatorBuilder) ValidateNameIsExist() *ValidatorBuilder {
	if len(b.resource.names) == 0 {
		b.errs = append(b.errs, fmt.Errorf("resources were provided, but no name was specified"))
	}
	return b
}

// ValidateNameIsSingle used to validate resource names is single. For example, the following operations are illegal
// oceanctl update backend <name-1> <name-2>
func (b *ValidatorBuilder) ValidateNameIsSingle() *ValidatorBuilder {
	if len(b.resource.names) > 1 {
		b.errs = append(b.errs, fmt.Errorf("only one resource name should be provide"))
	}
	return b
}

// ValidateOutputFormat used to validate resource names is single.For example, the following operations are illegal
// oceanctl get backend <name-1> -o xml
func (b *ValidatorBuilder) ValidateOutputFormat() *ValidatorBuilder {
	if b.resource.output == "" {
		return b
	}
	if !slices.Contains(config.SupportedFormats, b.resource.output) {
		b.errs = append(b.errs, fmt.Errorf("unable to match a printer suitable for the output format %s, "+
			"allowed formats are: %v", b.resource.output, strings.Join(config.SupportedFormats, ", ")))
	}

	return b
}
