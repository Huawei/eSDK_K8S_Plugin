/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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

package k8sutils

import (
	"errors"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
)

// ConfigOption defines the functional option type for client configuration.
type ConfigOption func(*rest.Config)

// QPS sets the QPS limit for the REST config.
func QPS(qps float32) ConfigOption {
	return func(config *rest.Config) {
		config.QPS = qps
	}
}

// Burst sets the Burst limit for the REST config.
func Burst(burst int) ConfigOption {
	return func(config *rest.Config) {
		config.Burst = burst
	}
}

// BuildConfig builds a REST config from kubeConfig and applies the given options.
func BuildConfig(kubeConfig string, opts ...ConfigOption) (*rest.Config, error) {
	var config *rest.Config
	var err error
	if kubeConfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, errors.New("config is nil")
	}
	for _, opt := range opts {
		opt(config)
	}
	return config, nil
}

// NewBackendUtils creates a new backend clientset with the given kubeConfig and options.
func NewBackendUtils(kubeConfig string, opts ...ConfigOption) (versioned.Interface, error) {
	config, err := BuildConfig(kubeConfig, opts...)
	if err != nil {
		return nil, err
	}
	return versioned.NewForConfig(config)
}
