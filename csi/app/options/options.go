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

// Package options control the service configurations, include env and config
package options

import (
	"flag"
	"fmt"
	"strings"

	"huawei-csi-driver/csi/app/config"
)

type optionsManager struct {
	logOption       *loggingOptions
	connectorOption *connectorOptions
	serviceOption   *serviceOptions
	k8sOption       *k8sOptions
	extenderOption  *extenderOptions
}

// NewOptionsManager return options manager
func NewOptionsManager() *optionsManager {
	return &optionsManager{
		logOption:       NewLoggingOptions(),
		connectorOption: NewConnectorOptions(),
		serviceOption:   NewServiceOptions(),
		k8sOption:       NewK8sOptions(),
		extenderOption:  NewExtenderOptions(),
	}
}

// AddFlags add the flags
func (opt *optionsManager) AddFlags(ff *flag.FlagSet) {
	opt.logOption.AddFlags(ff)
	opt.connectorOption.AddFlags(ff)
	opt.serviceOption.AddFlags(ff)
	opt.k8sOption.AddFlags(ff)
	opt.extenderOption.AddFlags(ff)
}

// ApplyFlags assign the flags
func (opt *optionsManager) ApplyFlags(cfg *config.AppConfig) {
	opt.logOption.ApplyFlags(cfg)
	opt.connectorOption.ApplyFlags(cfg)
	opt.serviceOption.ApplyFlags(cfg)
	opt.k8sOption.ApplyFlags(cfg)
	opt.extenderOption.ApplyFlags(cfg)
}

// ValidateFlags validate the flags
func (opt *optionsManager) ValidateFlags() error {
	errs := make([]error, 0)
	errs = append(errs, opt.logOption.ValidateFlags()...)
	errs = append(errs, opt.connectorOption.ValidateFlags()...)
	errs = append(errs, opt.serviceOption.ValidateFlags()...)

	if len(errs) == 0 {
		return nil
	}

	msg := make([]string, 0)
	for _, err := range errs {
		msg = append(msg, err.Error())
	}
	return fmt.Errorf(strings.Join(msg, ";"))
}

// Config set all configuration
func (opt *optionsManager) Config() (*config.AppConfig, error) {
	if err := opt.ValidateFlags(); err != nil {
		return nil, err
	}

	cfg := config.AppConfig{}
	opt.ApplyFlags(&cfg)
	return &cfg, nil
}
