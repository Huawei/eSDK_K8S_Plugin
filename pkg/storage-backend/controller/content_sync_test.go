package controller

import (
	"context"
	"testing"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/pkg/finalizers"
	"huawei-csi-driver/pkg/utils"
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
