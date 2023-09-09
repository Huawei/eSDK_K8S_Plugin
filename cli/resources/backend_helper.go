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
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8string "k8s.io/utils/strings"

	"huawei-csi-driver/cli/helper"
	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
)

const (
	ApiVersion              = "v1"
	XuanWuApiVersion        = "xuanwu.huawei.io/v1"
	KindSecret              = "Secret"
	KindConfigMap           = "ConfigMap"
	KindStorageBackendClaim = "StorageBackendClaim"
	YamlSeparator           = "---"
)

// BackendConfiguration backend config
type BackendConfiguration struct {
	Name                string                   `json:"name,omitempty" yaml:"name"`
	NameSpace           string                   `json:"namespace,omitempty" yaml:"namespace"`
	Storage             string                   `json:"storage,omitempty" yaml:"storage"`
	VstoreName          string                   `json:"vstoreName,omitempty" yaml:"vstoreName"`
	AccountName         string                   `json:"accountName,omitempty" yaml:"accountName"`
	Urls                []string                 `json:"urls,omitempty" yaml:"urls"`
	Pools               []string                 `json:"pools,omitempty" yaml:"pools"`
	MetrovStorePairID   string                   `json:"metrovStorePairID,omitempty" yaml:"metrovStorePairID"`
	MetroBackend        string                   `json:"metroBackend,omitempty" yaml:"metroBackend"`
	SupportedTopologies []map[string]interface{} `json:"supportedTopologies,omitempty" yaml:"supportedTopologies"`
	MaxClientThreads    string                   `json:"maxClientThreads,omitempty" yaml:"maxClientThreads"`
	Configured          bool                     `json:"-" yaml:"configured"`
	Provisioner         string                   `json:"provisioner,omitempty" yaml:"provisioner"`
	Parameters          struct {
		Protocol   string                                `json:"protocol,omitempty" yaml:"protocol"`
		ParentName string                                `json:"parentname" yaml:"parentname"`
		Portals    []string                              `json:"portals,omitempty" yaml:"portals"`
		Alua       []map[string][]map[string]interface{} `json:"ALUA,omitempty" yaml:"ALUA"`
	} `json:"parameters,omitempty" yaml:"parameters"`
}

// BackendShowWide the content echoed by executing the oceanctl get backend -o wide
type BackendShowWide struct {
	Namespace                 string `show:"NAMESPACE"`
	Name                      string `show:"NAME"`
	Protocol                  string `show:"PROTOCOL"`
	StorageType               string `show:"STORAGETYPE"`
	Sn                        string `show:"SN"`
	Status                    string `show:"STATUS"`
	Online                    string `show:"ONLINE"`
	Url                       string `show:"Url"`
	VendorName                string `show:"VENDORNAME"`
	StorageBackendContentName string `show:"STORAGEBACKENDCONTENTNAME"`
}

// BackendShow the content echoed by executing the oceanctl get backend
type BackendShow struct {
	Namespace   string `show:"NAMESPACE"`
	Name        string `show:"NAME"`
	Protocol    string `show:"PROTOCOL"`
	StorageType string `show:"STORAGETYPE"`
	Sn          string `show:"SN"`
	Status      string `show:"STATUS"`
	Online      string `show:"ONLINE"`
	Url         string `show:"Url"`
}

// BackendConfigShow the content echoed by executing the oceanctl create backend
type BackendConfigShow struct {
	Number     string `show:"NUMBER"`
	Configured string `show:"CONFIGURED"`
	Name       string `show:"NAME"`
	Storage    string `show:"STORAGE"`
	Urls       string `show:"URLS"`
}

// StorageBackendClaimConfig used to create a storageBackendClaim object
type StorageBackendClaimConfig struct {
	Name             string
	Namespace        string
	ConfigmapMeta    string
	SecretMeta       string
	MaxClientThreads string
	Provisioner      string
}

// SecretConfig used to create a secret object
type SecretConfig struct {
	Name      string
	Namespace string
	User      string
	Pwd       string
}

// ConfigMapConfig used to create a configmap object
type ConfigMapConfig struct {
	Name      string
	Namespace string
	JsonData  string
}

// ShowWithContentOption set StorageBackendContent value for BackendShowWide
func (b *BackendShowWide) ShowWithContentOption(content xuanwuv1.StorageBackendContent) *BackendShowWide {
	b.StorageBackendContentName = content.Name
	if content.Status != nil {
		b.Online = strconv.FormatBool(content.Status.Online)
		b.VendorName = content.Status.VendorName
		b.Sn = content.Status.SN
	}
	return b
}

// ShowWithConfigOption set BackendConfiguration value for BackendShowWide
func (b *BackendShowWide) ShowWithConfigOption(configuration BackendConfiguration) *BackendShowWide {
	b.Url = strings.Join(configuration.Urls, "\n")
	return b
}

// ShowWithClaimOption set StorageBackendClaim value for BackendShowWide
func (b *BackendShowWide) ShowWithClaimOption(claim xuanwuv1.StorageBackendClaim) *BackendShowWide {
	b.Namespace = claim.Namespace
	b.Name = claim.Name
	if claim.Status != nil {
		b.StorageType = claim.Status.StorageType
		b.Protocol = claim.Status.Protocol
		b.Status = string(claim.Status.Phase)
	}
	return b
}

// ToBackendShow convert BackendShowWide to BackendShow
func (b *BackendShowWide) ToBackendShow() BackendShow {
	return BackendShow{
		Namespace:   b.Namespace,
		Name:        b.Name,
		Protocol:    b.Protocol,
		StorageType: b.StorageType,
		Sn:          b.Sn,
		Status:      b.Status,
		Online:      b.Online,
		Url:         b.Url,
	}
}

// ToStorageBackendClaimConfig covert backend to StorageBackendClaimConfig
func (b *BackendConfiguration) ToStorageBackendClaimConfig() StorageBackendClaimConfig {
	return StorageBackendClaimConfig{
		Name:             b.Name,
		Namespace:        b.NameSpace,
		ConfigmapMeta:    k8string.JoinQualifiedName(b.NameSpace, b.Name),
		SecretMeta:       k8string.JoinQualifiedName(b.NameSpace, b.Name),
		MaxClientThreads: b.MaxClientThreads,
		Provisioner:      b.Provisioner,
	}
}

// ToConfigMapConfig convert backend to helper.ConfigMapConfig
func (b *BackendConfiguration) ToConfigMapConfig() (ConfigMapConfig, error) {
	config := struct {
		Backends BackendConfiguration `json:"backends"`
	}{*b}

	output, err := json.MarshalIndent(&config, "", "  ")
	if err != nil {
		return ConfigMapConfig{}, helper.LogErrorf(" json.MarshalIndent failed: %v", err)
	}

	return ConfigMapConfig{
		Name:      b.Name,
		Namespace: b.NameSpace,
		JsonData:  string(output),
	}, nil
}

// ToSecretConfig convert backend to helper.SecretConfig
// If start stdin failed, an error will return.
func (b *BackendConfiguration) ToSecretConfig() (SecretConfig, error) {
	userName, password, err := helper.StartStdInput()
	if err != nil {
		return SecretConfig{}, err
	}

	return SecretConfig{
		Name:      b.Name,
		Namespace: b.NameSpace,
		User:      userName,
		Pwd:       password,
	}, nil
}

// ToConfigMap convert ConfigMapConfig to  ConfigMap resource
func (c *ConfigMapConfig) ToConfigMap() corev1.ConfigMap {
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ApiVersion,
			Kind:       KindConfigMap,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
		Data: map[string]string{
			"csi.json": c.JsonData,
		},
	}
}

// ToSecret convert SecretConfig to Secret resource
func (c *SecretConfig) ToSecret() corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ApiVersion,
			Kind:       KindSecret,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
		StringData: map[string]string{
			"password": c.Pwd,
			"user":     c.User,
		},
		Type: "Opaque",
	}
}

// ToStorageBackendClaim convert StorageBackendClaimConfig to Secret StorageBackendClaim
func (c *StorageBackendClaimConfig) ToStorageBackendClaim() xuanwuv1.StorageBackendClaim {
	return xuanwuv1.StorageBackendClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: XuanWuApiVersion,
			Kind:       KindStorageBackendClaim,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
		Spec: xuanwuv1.StorageBackendClaimSpec{
			Provider:         c.Provisioner,
			ConfigMapMeta:    c.ConfigmapMeta,
			SecretMeta:       c.SecretMeta,
			MaxClientThreads: c.MaxClientThreads,
		},
	}
}

// LoadBackendsFromJson load backend from json bytes
func LoadBackendsFromJson(jsonData []byte) (map[string]*BackendConfiguration, error) {
	result := make(map[string]*BackendConfiguration)

	configmap := corev1.ConfigMap{}
	err := json.Unmarshal(jsonData, &configmap)
	if err != nil {
		return result, err
	}

	return LoadBackendsFromConfigMap(configmap)
}

// LoadBackendsFromConfigMap load backend from configmap resource
func LoadBackendsFromConfigMap(configmap corev1.ConfigMap) (map[string]*BackendConfiguration, error) {
	result := make(map[string]*BackendConfiguration)
	jsonStr, ok := configmap.Data["csi.json"]
	if !ok {
		return result, errors.New("not found csi.json config")
	}

	backendContent, err := AnalyseBackendExist(jsonStr)
	if err != nil {
		return nil, err
	}

	var backends []*BackendConfiguration
	if _, ok = backendContent.([]interface{}); ok {
		backends, err = LoadMultipleBackendFromConfigmap(jsonStr)
	} else {
		backends, err = LoadSingleBackendFromConfigmap(jsonStr)
	}
	if err != nil {
		return nil, err
	}

	for _, backend := range backends {
		result[backend.Name] = backend
	}
	return result, nil
}

//AnalyseBackendExist analyse backend,an error is returned if backends not exist
func AnalyseBackendExist(jsonStr string) (interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, err
	}
	backendContent, ok := config["backends"]
	if !ok {
		return nil, errors.New("not found backends config")
	}
	return backendContent, nil
}

// LoadSingleBackendFromConfigmap load single backend
func LoadSingleBackendFromConfigmap(jsonStr string) ([]*BackendConfiguration, error) {
	config := struct {
		Backends *BackendConfiguration `json:"backends"`
	}{}
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, err
	}

	return []*BackendConfiguration{config.Backends}, nil
}

// LoadMultipleBackendFromConfigmap load multiple backend
func LoadMultipleBackendFromConfigmap(jsonStr string) ([]*BackendConfiguration, error) {
	config := struct {
		Backends []*BackendConfiguration `json:"backends"`
	}{}
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, err
	}

	return config.Backends, nil
}

// LoadBackendsFromYaml load backend from yaml
func LoadBackendsFromYaml(yamlData []byte) (map[string]*BackendConfiguration, error) {
	cleanYamlData := strings.Trim(strings.TrimSpace(string(yamlData)), YamlSeparator)
	decoder := yaml.NewDecoder(bytes.NewReader([]byte(cleanYamlData)))

	var backends = map[string]*BackendConfiguration{}
	config := &BackendConfiguration{}
	err := decoder.Decode(config)
	for err == nil {
		if !reflect.DeepEqual(*config, BackendConfiguration{}) {
			backends[config.Name] = config
		}
		config = &BackendConfiguration{}
		err = decoder.Decode(config)
	}

	if !errors.Is(err, io.EOF) {
		return backends, err
	}
	return backends, nil
}
