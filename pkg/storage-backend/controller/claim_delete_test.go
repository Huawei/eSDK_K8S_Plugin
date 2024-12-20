/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

package controller

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
)

func TestDeleteStorageBackendClaim(t *testing.T) {
	fakeClaim := newClaim(xuanwuv1.StorageBackendClaimSpec{})
	fakeClaim.Status = &xuanwuv1.StorageBackendClaimStatus{
		BoundContentName: "",
	}
	ctrl := initController()
	if err := ctrl.deleteStorageBackendClaim(context.TODO(), fakeClaim); err != nil {
		t.Errorf("TestDeleteStorageBackendClaim failed, error %v", err)
	}
}

func TestProcessWithDeletionTimeStamp(t *testing.T) {
	removePatch := gomonkey.ApplyFunc(utils.NeedRemoveClaimBoundFinalizers, func(storageBackend *xuanwuv1.StorageBackendClaim) bool {
		return false
	})
	defer removePatch.Reset()

	fakeClaim := newClaim(xuanwuv1.StorageBackendClaimSpec{})
	fakeClaim.Status = &xuanwuv1.StorageBackendClaimStatus{
		BoundContentName: "fake-content",
	}
	ctrl := initController()
	if err := ctrl.processWithDeletionTimeStamp(context.TODO(), fakeClaim); err != nil {
		t.Errorf("TestProcessWithDeletionTimeStamp failed, error %v", err)
	}
}

func TestGetContentFromStoreNotExist(t *testing.T) {
	ctrl := initController()
	if content, err := ctrl.getContentFromStore("fake-content"); content != nil || err != nil {
		t.Errorf("TestGetContentFromStoreNotExist failed, content %v, error %v", content, err)
	}
}

func TestGetContentFromStoreExist(t *testing.T) {
	ctrl := initController()
	fakeContent := newContent(xuanwuv1.StorageBackendContentSpec{})
	err := ctrl.contentStore.Add(fakeContent)
	if err != nil {
		return
	}

	if content, err := ctrl.getContentFromStore("fake-content"); content != nil || err != nil {
		t.Errorf("TestGetContentFromStoreExist failed, content %v, error %v", content, err)
	}
}

func TestRemoveContentFinalizer(t *testing.T) {
	fakeContent := newContent(xuanwuv1.StorageBackendContentSpec{})
	ctrl := initController()
	if err := ctrl.removeContentFinalizer(context.TODO(), fakeContent); err == nil {
		t.Errorf("TestGetContentFromStoreNotExist failed, error %v", err)
	}
}
