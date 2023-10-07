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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CertConfig used to create a cert object
type CertConfig struct {
	Name      string
	Namespace string
	Cert      []byte
}

// CertShow the content echoed by executing the oceanctl get cert
type CertShow struct {
	Namespace    string `show:"NAMESPACE"`
	Name         string `show:"NAME"`
	BoundBackend string `show:"BOUNDBACKEND"`
}

// ToCertSecret convert CertSecretConfig to Secret resource
func (c *CertConfig) ToCertSecret() corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ApiVersion,
			Kind:       KindSecret,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.Name,
			Namespace: c.Namespace,
		},
		Data: map[string][]byte{
			"tls.crt": c.Cert,
		},
		Type: "Opaque",
	}
}
