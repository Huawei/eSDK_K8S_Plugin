package controller

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/pkg/utils"
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
