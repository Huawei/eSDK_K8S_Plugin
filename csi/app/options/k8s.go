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

// Package options control the service configurations, include env and config
package options

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

type k8sOptions struct {
	namespace string
}

// NewK8sOptions Construct a NewK8sOptions instance
func NewK8sOptions() *k8sOptions {
	return &k8sOptions{
		namespace: constants.DefaultNamespace,
	}
}

// AddFlags add the connector flags
func (opt *k8sOptions) AddFlags(ff *flag.FlagSet) {
	opt.namespace = GetCurrentPodNameSpace()
}

// ApplyFlags assign the connector flags
func (opt *k8sOptions) ApplyFlags(cfg *config.AppConfig) {
	cfg.Namespace = opt.namespace
}

// GetCurrentPodNameSpace get current pod namespace from env first.
// If call os.Getenv() returns blank, the default namespace is used.
func GetCurrentPodNameSpace() string {
	if namespace := os.Getenv(constants.NamespaceEnv); namespace != "" {
		return namespace
	}
	logrus.Infof("Get current pod namespace failed, use default %s", constants.DefaultNamespace)
	return constants.DefaultNamespace
}
