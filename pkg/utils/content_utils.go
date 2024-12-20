/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package utils to provide utils for storageBackend
package utils

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	clientSet "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// CreateContent used to create content by xuanwu client
func CreateContent(ctx context.Context, client clientSet.Interface, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	log.AddContext(ctx).Infof("Start to create content with content %s.", content.Name)
	defer log.AddContext(ctx).Infof("Finished create content with content %s.", content.Name)

	return client.XuanwuV1().StorageBackendContents().Create(ctx, content, metav1.CreateOptions{})
}

// DeleteContent used to delete content by xuanwu client
func DeleteContent(ctx context.Context, client clientSet.Interface, contentName string) error {
	log.AddContext(ctx).Infof("Start to delete content with content %s.", contentName)
	defer log.AddContext(ctx).Infof("Finished delete content with content %s.", contentName)

	return client.XuanwuV1().StorageBackendContents().Delete(ctx, contentName, metav1.DeleteOptions{})
}

// GetContent used to get content by xuanwu client
func GetContent(ctx context.Context, client clientSet.Interface, contentName string) (
	*xuanwuv1.StorageBackendContent, error) {

	log.AddContext(ctx).Debugf("Start to get content with content %s.", contentName)
	defer log.AddContext(ctx).Debugf("Finished get content with content %s.", contentName)

	return client.XuanwuV1().StorageBackendContents().Get(ctx, contentName, metav1.GetOptions{})
}

// UpdateContent used to update content by xuanwu client
func UpdateContent(ctx context.Context, client clientSet.Interface, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	log.AddContext(ctx).Infof("Start to update content with content %s.", content.Name)
	defer log.AddContext(ctx).Infof("Finished update content with content %s.", content.Name)

	return client.XuanwuV1().StorageBackendContents().Update(ctx, content, metav1.UpdateOptions{})
}

// UpdateContentStatus used to update content status by xuanwu client
func UpdateContentStatus(ctx context.Context, client clientSet.Interface, content *xuanwuv1.StorageBackendContent) (
	*xuanwuv1.StorageBackendContent, error) {

	log.AddContext(ctx).Debugf("Start to update content status with content %s, status %v.",
		content.Name, content.Status)
	defer log.AddContext(ctx).Debugf("Finished update content status with content %s, status %v.",
		content.Name, content.Status)

	return client.XuanwuV1().StorageBackendContents().UpdateStatus(ctx, content, metav1.UpdateOptions{})
}

// ListContent used to list contents by xuanwu client
func ListContent(ctx context.Context, client clientSet.Interface) (*xuanwuv1.StorageBackendContentList, error) {
	log.AddContext(ctx).Debugln("Start to list contents.")
	defer log.AddContext(ctx).Debugf("Finished list contents.")

	return client.XuanwuV1().StorageBackendContents().List(ctx, metav1.ListOptions{})
}
