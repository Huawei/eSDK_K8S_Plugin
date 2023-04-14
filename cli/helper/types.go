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

package helper

// BackendYaml represents the yaml files required to create a backend
type BackendYaml struct {
	SecretYaml, ConfigMapYaml, SbcYaml string
}

// StorageBackendClaimConfig used to create a storageBackendClaim yaml
type StorageBackendClaimConfig struct {
	Name             string `yaml:"STORAGE_BACKEND_NAME"`
	Namespace        string `yaml:"NAMESPACE"`
	ConfigmapMeta    string `yaml:"BACKEND_CONFIGMAP_META"`
	SecretMeta       string `yaml:"BACKEND_SECRET_META"`
	MaxClientThreads string `yaml:"BACKEND_MAX_CLIENT"`
}

// SecretConfig used to create a secret yaml
type SecretConfig struct {
	Name      string `yaml:"SECRET_NAME"`
	Namespace string `yaml:"NAMESPACE"`
	User      string `yaml:"BACKEND_USER"`
	Pwd       string `yaml:"BACKEND_PWD"`
}

// ConfigMapConfig used to create a configmap yaml
type ConfigMapConfig struct {
	Name      string `yaml:"CONFIGMAP_NAME"`
	Namespace string `yaml:"NAMESPACE"`
	JsonData  string `yaml:"BACKEND_CONFIGMAP_DATA"`
}

// BackendConfig backend config
type BackendConfig struct {
	Name             string `json:"name"`
	MaxClientThreads string `json:"maxClientThreads"`
}
