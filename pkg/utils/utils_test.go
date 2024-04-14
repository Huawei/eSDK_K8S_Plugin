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
	"fmt"
	"reflect"
	"testing"
	"time"

	//"huawei-csi-driver/utils/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/utils/log"
)

const (
	logDir  = "/var/log/xuanwu/"
	logName = "utils.log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestStoreObjectUpdate(t *testing.T) {
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-content",
			Namespace: "test-ns",
		},
	}
	fakeStore := cache.NewStore(cache.DeletionHandlingMetaNamespaceKeyFunc)

	if got, err := StoreObjectUpdate(context.TODO(), fakeStore, fakeContent,
		"storageBackendContent"); !got || err != nil {
		t.Errorf("StoreObjectUpdate failed, got: %v, error: %v", got, err)
	}
}

func TestStorageBackendClaimKey(t *testing.T) {
	fakeClaim := &xuanwuv1.StorageBackendClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-claim",
			Namespace: "test-ns",
		},
	}

	got := StorageBackendClaimKey(fakeClaim)
	if got != fmt.Sprintf("%s/%s", fakeClaim.Namespace, fakeClaim.Name) {
		t.Errorf("StorageBackendClaimKey failed, got: %s", got)
	}
}

func TestGenDynamicContentName(t *testing.T) {
	fakeClaim := &xuanwuv1.StorageBackendClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-claim",
			Namespace: "test-ns",
		},
	}

	got := GenDynamicContentName(fakeClaim)
	if got != fmt.Sprintf("content-%s", fakeClaim.UID) {
		t.Errorf("GenDynamicContentName failed, got: %s", got)
	}
}

func TestIsClaimBoundContentFalse(t *testing.T) {
	fakeClaim := &xuanwuv1.StorageBackendClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-claim",
			Namespace: "test-ns",
		},
		Status: &xuanwuv1.StorageBackendClaimStatus{},
	}

	if IsClaimBoundContent(fakeClaim) {
		t.Error("IsClaimBoundContent test failed")
	}
}

func TestNeedAddClaimBoundFinalizers(t *testing.T) {
	fakeClaim := &xuanwuv1.StorageBackendClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-claim",
			Namespace: "test-ns",
		},
		Status: &xuanwuv1.StorageBackendClaimStatus{
			BoundContentName: "fake-content",
		},
	}

	if !NeedAddClaimBoundFinalizers(fakeClaim) {
		t.Error("NeedAddClaimBoundFinalizers test failed")
	}
}

func TestNeedRemoveClaimBoundFinalizers(t *testing.T) {
	now := metav1.NewTime(time.Now())
	fakeClaim := &xuanwuv1.StorageBackendClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "fake-storage-claim",
			Namespace:         "test-ns",
			Finalizers:        []string{ClaimBoundFinalizer},
			DeletionTimestamp: &now,
		},
		Status: &xuanwuv1.StorageBackendClaimStatus{
			BoundContentName: "fake-content",
		},
	}

	if !NeedRemoveClaimBoundFinalizers(fakeClaim) {
		t.Error("NeedRemoveClaimBoundFinalizers test failed")
	}
}

func TestIsClaimReady(t *testing.T) {
	fakeClaim := &xuanwuv1.StorageBackendClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-claim",
			Namespace: "test-ns",
		},
		Status: &xuanwuv1.StorageBackendClaimStatus{
			Phase: xuanwuv1.BackendBound,
		},
	}

	if !IsClaimReady(fakeClaim) {
		t.Error("IsClaimReady test failed")
	}
}

func TestIsContentRegisterTrue(t *testing.T) {
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-content",
			Namespace: "test-ns",
		},
		Spec: xuanwuv1.StorageBackendContentSpec{
			Parameters:    nil,
			ConfigmapMeta: "",
			SecretMeta:    "",
		},
	}

	if !IsContentReady(context.TODO(), fakeContent) {
		t.Error("IsContentReady test failed")
	}
}

func TestIsContentRegisterFalse(t *testing.T) {
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-content",
			Namespace: "test-ns",
		},
		Spec: xuanwuv1.StorageBackendContentSpec{
			Parameters:    map[string]string{},
			ConfigmapMeta: "testConfigMap",
		},
		Status: &xuanwuv1.StorageBackendContentStatus{
			ContentName:     "",
			ProviderVersion: "",
		},
	}

	if IsContentReady(context.TODO(), fakeContent) {
		t.Error("IsContentReady test failed")
	}
}

func TestNeedAddContentBoundFinalizers(t *testing.T) {
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-content",
			Namespace: "test-ns",
		},
	}

	if !NeedAddContentBoundFinalizers(fakeContent) {
		t.Error("NeedAddContentBoundFinalizers test failed")
	}
}

func TestNeedRemoveContentBoundFinalizers(t *testing.T) {
	now := metav1.NewTime(time.Now())
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "fake-storage-content",
			Namespace:         "test-ns",
			Finalizers:        []string{ContentBoundFinalizer},
			DeletionTimestamp: &now,
		},
	}

	if !NeedRemoveContentBoundFinalizers(fakeContent) {
		t.Error("NeedAddContentBoundFinalizers test failed")
	}
}

func TestSplitMetaNamespaceKey(t *testing.T) {
	if ns, name, err := SplitMetaNamespaceKey("fake-ns/fake-name"); ns != "fake-ns" ||
		name != "fake-name" || err != nil {
		t.Errorf("SplitMetaNamespaceKey test failed, got namespace: %s, name: %s", ns, name)
	}
}

func TestGenObjectMetaKey(t *testing.T) {
	fakeContent := &xuanwuv1.StorageBackendContent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-content",
			Namespace: "test-ns",
		},
	}

	if meta, err := GenObjectMetaKey(fakeContent); meta != "test-ns/fake-storage-content" || err != nil {
		t.Errorf("GenObjectMetaKey test failed, got metakey: %s", meta)
	}
}

func TestErrorln(t *testing.T) {
	if err := Errorln(context.TODO(), "test"); err == nil {
		t.Errorf("TestErrorln test failed, error %v", err)
	}
}

func TestErrorf(t *testing.T) {
	if err := Errorf(context.TODO(), "test"); err == nil {
		t.Errorf("TestErrorf test failed, error %v", err)
	}
}

func TestNeedChangeContent(t *testing.T) {
	fakeClaim := &xuanwuv1.StorageBackendClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-claim",
			Namespace: "test-ns",
		},
		Spec: xuanwuv1.StorageBackendClaimSpec{
			SecretMeta: "new-secret",
		},
		Status: &xuanwuv1.StorageBackendClaimStatus{
			BoundContentName: "fake-content",
			SecretMeta:       "fake-secret",
		},
	}

	if !NeedChangeContent(fakeClaim) {
		t.Errorf("testNeedChangeContent test failed")
	}
}

func TestGetNameSpaceFromEnv(t *testing.T) {
	xuanwuNamespace := "xuanwu"
	ns := GetNameSpaceFromEnv("", xuanwuNamespace)
	if ns != xuanwuNamespace {
		t.Error("TestGetNameSpaceFromEnv test failed")
	}
}

// TestConvertToMapValueX test convert map value
func TestConvertToMapValueX(t *testing.T) {
	// arrange
	ctx := context.Background()
	poolCapabilities := make(map[string]interface{})
	capability := map[string]interface{}{
		string(xuanwuv1.FreeCapacity):  int64(100),
		string(xuanwuv1.TotalCapacity): int64(100),
		string(xuanwuv1.UsedCapacity):  int64(100),
	}
	poolCapabilities["pool1"] = capability
	poolCapabilities["pool2"] = capability

	// act
	poolCapacibilityMap := ConvertToMapValueX[map[string]interface{}](ctx, poolCapabilities)

	// assert
	if !reflect.DeepEqual(poolCapacibilityMap["pool1"], capability) {
		t.Errorf("ConvertToMapValueX map[string]interface{} from %+v to %+v failed",
			poolCapabilities["pool1"], capability)
	}
}
