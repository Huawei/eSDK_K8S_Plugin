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

// Package k8sutils provides Kubernetes utilities
package k8sutils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type secretOps interface {
	// GetSecret get secret
	GetSecret(ctx context.Context, secretName, namespace string) (*corev1.Secret, error)
	// CreateSecret create secret
	CreateSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error)
	// UpdateSecret update secret
	UpdateSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error)
	// DeleteSecret delete secret
	DeleteSecret(ctx context.Context, secretName, namespace string) error
}

// GetSecret get secret
func (k *KubeClient) GetSecret(ctx context.Context, secretName, namespace string) (*corev1.Secret, error) {
	return k.clientSet.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
}

// CreateSecret create secret
func (k *KubeClient) CreateSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
	return k.clientSet.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
}

// UpdateSecret update secret
func (k *KubeClient) UpdateSecret(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
	return k.clientSet.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
}

// DeleteSecret delete secret
func (k *KubeClient) DeleteSecret(ctx context.Context, secretName, namespace string) error {
	return k.clientSet.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
}
