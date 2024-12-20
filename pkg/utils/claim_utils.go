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

// GetClaim used to get claim by xuanwu client
func GetClaim(ctx context.Context, client clientSet.Interface, storageBackend *xuanwuv1.StorageBackendClaim) (
	*xuanwuv1.StorageBackendClaim, error) {

	log.AddContext(ctx).Debugf("Start to get claim %s.", StorageBackendClaimKey(storageBackend))
	defer log.AddContext(ctx).Debugf("Finished get claim %s.", StorageBackendClaimKey(storageBackend))

	return client.XuanwuV1().StorageBackendClaims(storageBackend.Namespace).Get(
		ctx, storageBackend.Name, metav1.GetOptions{})
}

// UpdateClaim used to update claim by xuanwu client
func UpdateClaim(ctx context.Context, client clientSet.Interface, storageBackend *xuanwuv1.StorageBackendClaim) (
	*xuanwuv1.StorageBackendClaim, error) {

	log.AddContext(ctx).Infof("Start to update claim %s.", StorageBackendClaimKey(storageBackend))
	defer log.AddContext(ctx).Infof("Finished update claim %s.", StorageBackendClaimKey(storageBackend))

	return client.XuanwuV1().StorageBackendClaims(storageBackend.Namespace).Update(
		ctx, storageBackend, metav1.UpdateOptions{})
}

// UpdateClaimStatus used to update claim status by xuanwu client
func UpdateClaimStatus(ctx context.Context, client clientSet.Interface, storageBackend *xuanwuv1.StorageBackendClaim) (
	*xuanwuv1.StorageBackendClaim, error) {

	log.AddContext(ctx).Infof("Start to update claim %s with status %s.",
		StorageBackendClaimKey(storageBackend), storageBackend.Status)
	defer log.AddContext(ctx).Infof("Finished update claim %s with status %s.",
		StorageBackendClaimKey(storageBackend), storageBackend.Status)

	return client.XuanwuV1().StorageBackendClaims(storageBackend.Namespace).UpdateStatus(
		ctx, storageBackend, metav1.UpdateOptions{})
}

// ListClaim used to list claims by xuanwu client
func ListClaim(ctx context.Context, client clientSet.Interface, namespace string) (*xuanwuv1.StorageBackendClaimList, error) {
	log.AddContext(ctx).Infoln("Start to list claims.")
	defer log.AddContext(ctx).Infoln("Finished list claims.")

	return client.XuanwuV1().StorageBackendClaims(namespace).List(context.TODO(), metav1.ListOptions{})
}
