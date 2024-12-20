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
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/helper"
	xuanwuV1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Cert is the cert resource
type Cert struct {
	// resource of request
	resource *Resource
}

// NewCert initialize a Cert instance
func NewCert(resource *Resource) *Cert {
	return &Cert{resource: resource}
}

// Get query Cert resources
func (c *Cert) Get() error {
	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	claim, err := storageBackendClaimClient.QueryByName(c.resource.namespace, c.resource.backend)
	if err != nil {
		return err
	}

	if claim.Name == "" {
		helper.PrintNotFoundBackend(c.resource.backend)
		return nil
	}

	_, certSecretName := helper.SplitQualifiedName(claim.Spec.CertSecret)

	if certSecretName == "" {
		helper.PrintNoResourceCert(claim.Name, claim.Namespace)
		return nil
	}

	secretClient := client.NewCommonCallHandler[corev1.Secret](config.Client)
	secret, err := secretClient.QueryByName(c.resource.namespace, certSecretName)
	if err != nil {
		return err
	}

	shows := fetchCertShows([]corev1.Secret{secret}, c.resource.namespace, claim.Name)
	helper.PrintSecret(shows, []string{}, helper.PrintWithTable[CertShow])
	return nil
}

// Delete cert resource
func (c *Cert) Delete() error {
	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	oldClaim, err := storageBackendClaimClient.QueryByName(c.resource.namespace, c.resource.backend)
	if err != nil {
		return err
	}

	if oldClaim.Name == "" {
		helper.PrintNotFoundBackend(c.resource.backend)
		return nil
	}

	_, certSecretName := helper.SplitQualifiedName(oldClaim.Spec.CertSecret)

	if certSecretName == "" {
		helper.PrintNoResourceCert(oldClaim.Name, oldClaim.Namespace)
		return nil
	}

	newClaim := oldClaim.DeepCopy()
	newClaim.Spec.UseCert = false
	newClaim.Spec.CertSecret = ""
	if err = storageBackendClaimClient.Update(*newClaim); err != nil {
		return err
	}

	secretClient := client.NewCommonCallHandler[corev1.Secret](config.Client)
	secret, err := secretClient.QueryByName(c.resource.namespace, certSecretName)
	if err != nil {
		return err
	}

	if err := deleteSecretResources(secret); err != nil {
		return helper.LogErrorf("delete cert reference resource failed, error: %v", err)
	}

	helper.PrintOperateResult("cert", "deleted", secret.Name)
	return nil
}

// Update update Cert
func (c *Cert) Update() error {
	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	claim, err := storageBackendClaimClient.QueryByName(c.resource.namespace, c.resource.backend)
	if err != nil {
		return err
	}

	if claim.Name == "" {
		helper.PrintNotFoundBackend(c.resource.backend)
		return nil
	}

	_, certSecretName := helper.SplitQualifiedName(claim.Spec.CertSecret)

	if certSecretName == "" {
		helper.PrintNoResourceCert(claim.Name, claim.Namespace)
		return nil
	}

	certConfig, err := c.LoadCertFile()
	if err != nil {
		return helper.LogErrorf("load cert failed: error: %v", err)
	}

	secretClient := client.NewCommonCallHandler[corev1.Secret](config.Client)
	oldSecret, err := secretClient.QueryByName(c.resource.namespace, certSecretName)
	if err != nil {
		return err
	}

	newSecret := oldSecret.DeepCopy()
	newSecret.Data = map[string][]byte{
		"tls.crt": certConfig.Cert,
	}

	if err = secretClient.Update(*newSecret); err != nil {
		return helper.LogErrorf("apply cert failed, error: %v", err)
	}
	helper.PrintOperateResult("cert", "updated", newSecret.Name)
	return nil
}

// Create create Cert
func (c *Cert) Create() error {
	certConfig, err := c.LoadCertFile()
	if err != nil {
		return helper.LogErrorf("load cert failed: error: %v", err)
	}
	certConfig.Name = c.resource.names[0]

	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	oldClaim, err := storageBackendClaimClient.QueryByName(c.resource.namespace, c.resource.backend)
	if err != nil {
		return err
	}

	if oldClaim.Name == "" {
		helper.PrintNotFoundBackend(c.resource.backend)
		return nil
	}

	if oldClaim.Spec.CertSecret != "" {
		return fmt.Errorf("a cert already exists on the backend [%s]", oldClaim.Name)
	}

	// create secret resource
	secretClient := client.NewCommonCallHandler[corev1.Secret](config.Client)
	if err = secretClient.Create(certConfig.ToCertSecret()); err != nil {
		return err
	}

	newClaim := oldClaim.DeepCopy()
	newClaim.Spec.UseCert = true
	newClaim.Spec.CertSecret = path.Join(newClaim.Namespace, certConfig.Name)

	if err = storageBackendClaimClient.Update(*newClaim); err != nil {
		if err := secretClient.DeleteByNames(newClaim.Namespace, certConfig.Name); err != nil {
			log.Errorf("delete new created cert failed, error: %v", err)
		}
		return err
	}

	// print create result
	helper.PrintOperateResult("cert", "created", certConfig.Name)
	return nil
}

// LoadCertFile to load cert file from disk
func (c *Cert) LoadCertFile() (*CertConfig, error) {
	certData, err := os.ReadFile(c.resource.fileName)
	if err != nil {
		return nil, err
	}

	return c.LoadCertsFromDate(certData)
}

// LoadCertsFromDate load cert from bytes
func (c *Cert) LoadCertsFromDate(Data []byte) (*CertConfig, error) {
	return &CertConfig{
		Namespace: c.resource.namespace,
		Cert:      Data,
	}, nil
}

func deleteSecretResources(secret corev1.Secret) error {
	referenceResources := []string{
		path.Join(string(client.Secret), secret.Name),
	}

	_, err := config.Client.DeleteResourceByQualifiedNames(referenceResources, secret.Namespace)
	return err
}

func fetchCertShows(secrets []corev1.Secret, namespace, backend string) []CertShow {
	result := make([]CertShow, 0)
	for _, secret := range secrets {
		result = append(result, CertShow{Name: secret.Name, Namespace: namespace, BoundBackend: backend})
	}
	return result
}
