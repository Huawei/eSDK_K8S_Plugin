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

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/finalizers"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
)

func TestUpdateContentAddFinalizerFailed(t *testing.T) {
	ctrl := initController()
	ctrl.claimQueue.Add("fake-claim")
	err := ctrl.updateContent(context.TODO(), newContent(
		xuanwuv1.StorageBackendContentSpec{
			Provider:     "fake-provider",
			BackendClaim: "fake-claim"}))
	if err == nil {
		t.Errorf("TestUpdateContentAddFinalizerFailed failed, error %v", err)
	}
}

func TestUpdateContentWithoutFinalizer(t *testing.T) {
	ctrl := initController()
	ctrl.claimQueue.Add("fake-claim")
	fakeContent := newContent(
		xuanwuv1.StorageBackendContentSpec{
			Provider:     "fake-provider",
			BackendClaim: "fake-claim"})
	finalizers.SetFinalizer(fakeContent, utils.ContentBoundFinalizer)
	err := ctrl.updateContent(context.TODO(), fakeContent)
	if err != nil {
		t.Errorf("TestUpdateContentWithoutFinalizer failed, error %v", err)
	}
}

func TestNeedUpdateClaimStatus(t *testing.T) {
	ctrl := initController()
	fakeContent := newContent(xuanwuv1.StorageBackendContentSpec{Provider: "fake-provider"})
	fakeClaim := newClaim(xuanwuv1.StorageBackendClaimSpec{})
	if ctrl.needUpdateClaimStatus(fakeClaim, fakeContent) {
		t.Error("TestNeedUpdateClaimStatus failed, want false")
	}
}

func TestNeedUpdateClaimStatusTrue(t *testing.T) {
	ctrl := initController()
	fakeContent := newContent(xuanwuv1.StorageBackendContentSpec{Provider: "fake-provider"})
	fakeContent.Status = &xuanwuv1.StorageBackendContentStatus{ContentName: "fake-content-name"}
	fakeClaim := newClaim(xuanwuv1.StorageBackendClaimSpec{})
	if !ctrl.needUpdateClaimStatus(fakeClaim, fakeContent) {
		t.Error("TestNeedUpdateClaimStatusTrue failed, want true")
	}
}
