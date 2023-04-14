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
	"encoding/json"
	"errors"
	"fmt"

	k8string "k8s.io/utils/strings"

	"huawei-csi-driver/cli/client"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
	xuanwuV1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/utils/log"
)

type Backend struct {
	// resource of request
	resource *Resource

	// backend and reference resource's name
	backendName   string
	configMapName string
	secretName    string

	// create backend needs yaml files
	backendYaml   helper.YamlValues
	configMapYaml string
	secretYaml    string
	sbcYaml       string

	// backend config fields
	maxClientThreads string
}

// NewBackend initialize a Backend instance
func NewBackend(resource *Resource) *Backend {
	return &Backend{resource: resource}
}

// Get query backend resources
func (b *Backend) Get() ([]byte, error) {
	return config.Client.GetResource(b.resource.names, b.resource.namespace, b.resource.output,
		client.Storagebackendclaim)
}

// Create used to create a backend
func (b *Backend) Create() error {
	if err := b.parseBackendYaml(); err != nil {
		return helper.LogErrorf("parse backend yaml failed, error: %v", err)
	}

	if err := b.checkBackendExist(); err != nil {
		return helper.LogErrorf("check backend exists failed, error: %v", err)
	}

	if err := b.deleteBackedReferenceResources(); err != nil {
		return helper.LogErrorf("delete backend reference resources failed, error: %v", err)
	}

	if err := b.GenerateNeedsYaml(); err != nil {
		return helper.LogErrorf("generate create backend needs yaml files failed, error: %v", err)
	}

	if err := b.executeCreate(b.toYamlFiles()); err != nil {
		if err := b.deleteBackedReferenceResources(); err != nil {
			log.Errorf("execute create revert failed, error: %v", err)
		}
		return helper.LogErrorf("execute create backend failed, error: %v", err)
	}

	helper.PrintOperateResult([]string{b.backendName}, "backend", "created")
	return nil
}

func (b *Backend) Delete() error {
	claims, err := b.GetStorageBackendClaims()
	if err != nil {
		return helper.PrintlnError(err)
	}

	var deleteResult []string
	for _, claim := range claims {
		qualifiedNames := buildQualifiedNameByClaim(claim)
		if err := deleteResourceByQualifiedNames(qualifiedNames, claim.Namespace); err != nil {
			return helper.LogErrorf("delete backend reference resource failed, error: %v", err)
		}
		deleteResult = append(deleteResult, claim.Name)
	}

	helper.PrintOperateResult(deleteResult, "backend", "deleted")
	return nil
}

func (b *Backend) Update() error {
	// Query whether the storageBackendClaim exists, and print an error if it does not exist
	b.backendName = b.resource.names[0]
	claims, err := b.GetStorageBackendClaims()
	if err != nil {
		return helper.PrintlnError(err)
	}
	claim := claims[0]

	// Create a new secret with an uid name
	secretConfig, err := b.CreateSecretWithUid(err, claims)
	if err != nil {
		return err
	}

	// Update storageBackendClaim
	if err := b.updateStorageBackendClaim(claim, secretConfig); err != nil {
		return err
	}

	// Update successful, delete the old secret referenced with the storageBackendClaim
	_, oldSecretName := k8string.SplitQualifiedName(claim.Spec.SecretMeta)
	if err := deleteSecret(oldSecretName, claim.Namespace); err != nil {
		log.Errorf("delete secret failed, error: %v", err)
	}

	// print update result
	helper.PrintOperateResult([]string{b.backendName}, "backend", "updated")
	return nil
}

// CreateSecretWithUid create secret with uid
func (b *Backend) CreateSecretWithUid(err error, claims []xuanwuV1.StorageBackendClaim) (helper.SecretConfig, error) {
	// Generate new secret yaml with an uid name
	secretConfig, err := b.toSecretConfig()
	if err != nil {
		return helper.SecretConfig{}, err
	}

	secretConfig.Name = helper.AppendUid(claims[0].Name, config.DefaultUidLength)
	newSecretYaml := helper.GenerateSecretYaml(secretConfig)

	// Create a secret with an uid name
	err = config.Client.OperateResourceByYaml(newSecretYaml, client.Create, false)
	if err != nil {
		return helper.SecretConfig{}, err
	}
	return secretConfig, nil
}

// updateStorageBackendClaim update storageBackendClaim with new secret.
// If update failed, restore and delete new secret.
func (b *Backend) updateStorageBackendClaim(claim xuanwuV1.StorageBackendClaim, secretConfig helper.SecretConfig) error {
	claimConfig := helper.StorageBackendClaimConfig{
		Name:             claim.Name,
		Namespace:        claim.Namespace,
		ConfigmapMeta:    claim.Spec.ConfigMapMeta,
		SecretMeta:       k8string.JoinQualifiedName(claim.Namespace, secretConfig.Name),
		MaxClientThreads: claim.Spec.MaxClientThreads,
	}
	// Apply storageBackendClaim with a new secret
	if err := applyStorageBackendClaim(claimConfig); err != nil {
		// Apply failed.
		// First, restore the secret referenced with the storageBackendClaim.
		claimConfig.SecretMeta = claim.Spec.SecretMeta
		if err := applyStorageBackendClaim(claimConfig); err != nil {
			log.Errorf("apply storageBackendClaim failed, error: %v", err)
		}
		// Next, delete the new secret.
		if err := deleteSecret(secretConfig.Name, claim.Namespace); err != nil {
			log.Errorf("delete secret failed, error: %v", err)
		}
		// Finally, return apply failed error.
		return err
	}
	return nil
}

func buildQualifiedNameByClaim(claim xuanwuV1.StorageBackendClaim) []string {
	_, secretName := k8string.SplitQualifiedName(claim.Spec.SecretMeta)
	_, configmapName := k8string.SplitQualifiedName(claim.Spec.ConfigMapMeta)

	return []string{
		k8string.JoinQualifiedName(string(client.Storagebackendclaim), claim.Name),
		k8string.JoinQualifiedName(string(client.Secret), secretName),
		k8string.JoinQualifiedName(string(client.ConfigMap), configmapName),
	}
}

// GetStorageBackendClaims used to get storageBackendClaims
func (b *Backend) GetStorageBackendClaims() ([]xuanwuV1.StorageBackendClaim, error) {
	b.resource.Output("json")
	out, _ := b.Get()

	var claims xuanwuV1.StorageBackendClaimList
	if err := b.jsonToStorageBackendClaims(out, &claims); err != nil {
		log.Errorf("parse json to storageBackendClaims failed, error: %v", err)
		return nil, fmt.Errorf("Get resources failed in %s namespace.", b.resource.namespace)
	}

	if len(claims.Items) == 0 {
		return nil, fmt.Errorf("No resources found in %s namespace.", b.resource.namespace)
	}
	return claims.Items, nil
}

// jsonToStorageBackendClaims convert json data to v1.StorageBackendClaimList
func (b *Backend) jsonToStorageBackendClaims(out []byte, claims *xuanwuV1.StorageBackendClaimList) error {
	jsonData := string(out)
	if !(b.resource.selectAll || len(b.resource.resources) > 1) {
		jsonData = "{ \"items\":[" + jsonData + "]}"
	}
	return json.Unmarshal([]byte(jsonData), claims)
}

// parseBackendYaml parse backend yaml,
func (b *Backend) parseBackendYaml() error {
	yamlValue, err := helper.ReadYamlFile(b.resource.fileName)
	if err != nil {
		return helper.LogErrorf("read yaml file failed, error: %v", err)
	}
	b.backendYaml = yamlValue

	// if name doesn't was specified in backend yaml, will return an error
	backendName, ok := yamlValue["name"].(string)
	if !ok {
		return errors.New("name must was specified in backend yaml")
	}
	b.backendName = backendName
	b.configMapName = backendName
	b.secretName = backendName

	// Because yaml cannot express the difference between string and numeric types,
	// it is necessary to determine its exact type
	b.maxClientThreads = config.DefaultMaxClientThreads
	if maxClientThreads, ok := yamlValue["maxClientThreads"]; !ok {
		b.maxClientThreads = helper.ParseNumericString(maxClientThreads)
	}

	if namespace, ok := yamlValue["namespace"].(string); ok {
		if b.resource.namespace == "" || config.Namespace == "" {
			b.resource.namespace = namespace
		}
		delete(yamlValue, "namespace")
	}

	return nil
}

// checkBackendExist used to check whether the backend exists.
// If it exists, an AlreadyExists error is returned.
// If it doesn't exist, nothing is done.
func (b *Backend) checkBackendExist() error {
	if exist, err := config.Client.CheckResourceExist(b.backendName, b.resource.namespace,
		client.Storagebackendclaim); err != nil {
		return err
	} else if exist {
		msg := fmt.Sprintf("Error from server (AlreadyExists): error when creating "+
			"storagebackendclaim: storagebackendclaim %s already exists in %s", b.backendName, b.resource.namespace)
		return errors.New(msg)
	}
	return nil
}

// deleteBackedReferenceResources delete backend reference resource, e.g. configmap and secret.
func (b *Backend) deleteBackedReferenceResources() error {
	qualifiedNames := buildQualifiedNameByBackend(b)
	return deleteResourceByQualifiedNames(qualifiedNames, b.resource.namespace)
}

// buildQualifiedNameByBackend build backend reference resource qualified name.
func buildQualifiedNameByBackend(b *Backend) []string {
	return []string{
		k8string.JoinQualifiedName(string(client.ConfigMap), b.backendName),
		k8string.JoinQualifiedName(string(client.Secret), b.backendName),
	}
}

// deleteResourceByQualifiedNames delete resource by qualified names.
// the qualified name format is resourceType/resourceName
func deleteResourceByQualifiedNames(qualifiedNames []string, namespace string) error {
	if _, err := config.Client.DeleteResourceByQualifiedNames(qualifiedNames, namespace); err != nil {
		return err
	}
	return nil
}

// GenerateNeedsYaml used to prepare yaml required to create the backend, including configmap yaml, secret yaml and
// storageBackendClaim yaml.
func (b *Backend) GenerateNeedsYaml() error {
	b.configMapYaml = helper.GenerateConfigMapYaml(b.toConfigMapConfig())
	b.sbcYaml = helper.GenerateStorageBackendClaimYaml(b.toStorageBackendClaimConfig())
	secretConfig, err := b.toSecretConfig()
	if err != nil {
		return err
	}
	b.secretYaml = helper.GenerateSecretYaml(secretConfig)
	return nil
}

// toStorageBackendClaimConfig covert backend to helper.StorageBackendClaimConfig
func (b *Backend) toStorageBackendClaimConfig() helper.StorageBackendClaimConfig {
	return helper.StorageBackendClaimConfig{
		Name:             b.backendName,
		Namespace:        b.resource.namespace,
		ConfigmapMeta:    k8string.JoinQualifiedName(b.resource.namespace, b.configMapName),
		SecretMeta:       k8string.JoinQualifiedName(b.resource.namespace, b.secretName),
		MaxClientThreads: b.maxClientThreads,
	}
}

// toConfigMapConfig convert backend to helper.ConfigMapConfig
func (b *Backend) toConfigMapConfig() helper.ConfigMapConfig {
	jsonData := b.backendYaml.ToPrettyJson()

	return helper.ConfigMapConfig{
		Name:      b.backendName,
		Namespace: b.resource.namespace,
		JsonData:  jsonData,
	}
}

// toSecretConfig convert backend to helper.SecretConfig
// If start stdin failed, an error will return.
func (b *Backend) toSecretConfig() (helper.SecretConfig, error) {
	userName, password, err := helper.StartStdInput()
	if err != nil {
		return helper.SecretConfig{}, err
	}

	return helper.SecretConfig{
		Name:      b.backendName,
		Namespace: b.resource.namespace,
		User:      userName,
		Pwd:       password,
	}, nil
}

// executeCreate used to execute create task, if create resource failed,
// an error calling the CreateResource() function will be returned
func (b *Backend) executeCreate(yamlFiles []string) error {
	for _, yaml := range yamlFiles {
		if err := config.Client.OperateResourceByYaml(yaml, client.Create, false); err != nil {
			return err
		}
	}
	return nil
}

// toYamlFiles used to build yaml files required to create a backend
func (b *Backend) toYamlFiles() []string {
	return []string{b.configMapYaml, b.secretYaml, b.sbcYaml}
}

func applyStorageBackendClaim(claimConfig helper.StorageBackendClaimConfig) error {
	claimYaml := helper.GenerateStorageBackendClaimYaml(claimConfig)
	return config.Client.OperateResourceByYaml(claimYaml, client.Apply, false)
}

func deleteSecret(name, namespace string) error {
	qualifiedNames := []string{k8string.JoinQualifiedName(string(client.Secret), name)}
	if _, err := config.Client.DeleteResourceByQualifiedNames(qualifiedNames, namespace); err != nil {
		return err
	}
	return nil
}
