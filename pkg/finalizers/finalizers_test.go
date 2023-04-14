/*
 Copyright (c) Huawei Technologies Co., Ltd. 2022-2022. All rights reserved.

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

// Package finalizers used add/remove finalizer from object
package finalizers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
)

const (
	contentBoundFinalizer = "storagebackend.xuanwu.huawei.io/storagebackendcontent-bound-protection"
)

func TestContainsFinalizer(t *testing.T) {
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "fake-storage-content",
			Namespace:  "test-ns",
			Finalizers: []string{contentBoundFinalizer},
		},
	}

	if !ContainsFinalizer(fakeContent, contentBoundFinalizer) {
		t.Error("ContainsFinalizer test failed")
	}
}

func TestSetFinalizer(t *testing.T) {
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-content",
			Namespace: "test-ns",
		},
	}

	SetFinalizer(fakeContent, contentBoundFinalizer)
}

func TestRemoveFinalizer(t *testing.T) {
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "fake-storage-content",
			Namespace:  "test-ns",
			Finalizers: []string{contentBoundFinalizer},
		},
	}

	RemoveFinalizer(fakeContent, contentBoundFinalizer)
}
