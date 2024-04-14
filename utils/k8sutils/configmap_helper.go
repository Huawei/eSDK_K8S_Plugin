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

	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigmapOps defines interfaces required by configmap
type ConfigmapOps interface {
	// CreateConfigmap creates the given configmap
	CreateConfigmap(context.Context, *coreV1.ConfigMap) (*coreV1.ConfigMap, error)
	// GetConfigmap gets the configmap object given its name and namespace
	GetConfigmap(context.Context, string, string) (*coreV1.ConfigMap, error)
	// UpdateConfigmap update the configmap object given its name and namespace
	UpdateConfigmap(context.Context, *coreV1.ConfigMap) (*coreV1.ConfigMap, error)
	// DeleteConfigmap delete the configmap object given its name and namespace
	DeleteConfigmap(context.Context, *coreV1.ConfigMap) error
}

// CreateConfigmap creates the given configmap
func (k *KubeClient) CreateConfigmap(ctx context.Context, configmap *coreV1.ConfigMap) (*coreV1.ConfigMap, error) {
	return k.clientSet.CoreV1().ConfigMaps(configmap.Namespace).Create(
		ctx, configmap, metaV1.CreateOptions{})
}

// GetConfigmap gets the configmap object given its name and namespace
func (k *KubeClient) GetConfigmap(ctx context.Context, name, namespace string) (*coreV1.ConfigMap, error) {
	return k.clientSet.CoreV1().ConfigMaps(namespace).Get(ctx, name, metaV1.GetOptions{})
}

// UpdateConfigmap update configmap
func (k *KubeClient) UpdateConfigmap(ctx context.Context, configmap *coreV1.ConfigMap) (*coreV1.ConfigMap, error) {
	return k.clientSet.CoreV1().ConfigMaps(configmap.Namespace).Update(ctx, configmap, metaV1.UpdateOptions{})
}

// DeleteConfigmap delete configmap
func (k *KubeClient) DeleteConfigmap(ctx context.Context, configmap *coreV1.ConfigMap) error {
	return k.clientSet.CoreV1().ConfigMaps(configmap.Namespace).Delete(ctx, configmap.Name, metaV1.DeleteOptions{})
}
