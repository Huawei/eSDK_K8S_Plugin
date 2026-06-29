/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2026. All rights reserved.
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

// Package modify contains claim resource controller definitions and synchronization functions
package modify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	storagev1 "k8s.io/api/storage/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	clientSet "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned/fake"
	backendInformers "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/informers/externalversions"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

func TestModifyClaimController_syncClaimWork_WhenGetClaimFromListerFailed(t *testing.T) {
	// arrange
	factory := backendInformers.NewSharedInformerFactory(&clientSet.Clientset{}, 10*time.Second)
	claimLister := factory.Xuanwu().V1().VolumeModifyClaims().Lister()
	ctrl := &VolumeModifyController{claimLister: claimLister}

	// mock
	patches := gomonkey.ApplyMethodReturn(claimLister, "Get", nil, errors.New("get claim error"))
	defer patches.Reset()

	// action
	err := ctrl.syncClaimWork(context.Background(), "test-name")

	// assert
	if err == nil {
		t.Errorf("TestModifyClaimController_syncClaimWork_WhenGetClaimFromListerFailed failed,"+
			" want an error:%v, but got nil", err)
	}
}

func TestModifyClaimController_syncClaimWork_WhenGetClaimNotFound(t *testing.T) {
	// arrange
	factory := backendInformers.NewSharedInformerFactory(&clientSet.Clientset{}, 10*time.Second)
	claimLister := factory.Xuanwu().V1().VolumeModifyClaims().Lister()
	ctrl := &VolumeModifyController{claimLister: claimLister}

	// mock
	patches := gomonkey.ApplyMethodReturn(claimLister, "Get", nil, errors.New("get claim error")).
		ApplyFuncReturn(apiErrors.IsNotFound, true)
	defer patches.Reset()

	// action
	err := ctrl.syncClaimWork(context.Background(), "test-name")

	// assert
	if err != nil {
		t.Errorf("TestModifyClaimController_syncClaimWork_WhenGetClaimNotFound failed,"+
			" want nil, but got %v", err)
	}
}

func TestModifyClaimController_setClaimFinalizers_WhenStatusIsNil(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	ctrl := &VolumeModifyController{clientSet: fakeClient}
	claim := &xuanwuv1.VolumeModifyClaim{}

	// action
	claim, err := ctrl.setClaimFinalizers(ctx, claim)

	// assert
	if err != nil {
		t.Errorf("TestModifyClaimController_setClaimFinalizers_WhenStatusIsNil failed, "+
			"want nil, but got %v", err)
	}

	if utils.Contains(claim.Finalizers, ProtectClaimFinalizer) {
		t.Errorf("TestModifyClaimController_setClaimFinalizers_WhenStatusIsNil failed, "+
			"want not finalzer, but got %v", claim.Finalizers)
	}
}

func TestModifyClaimController_setClaimFinalizers_WhenPhaseIsPending(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	ctrl := &VolumeModifyController{clientSet: fakeClient}
	claim := &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-name"},
		Status: xuanwuv1.VolumeModifyClaimStatus{
			Phase: xuanwuv1.VolumeModifyClaimPending,
		},
	}

	// mock
	_, err := fakeClient.XuanwuV1().VolumeModifyClaims().Create(ctx, claim, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("setClaimFinalizers mock create calim failed")
		return
	}

	// action
	claim, err = ctrl.setClaimFinalizers(context.Background(), claim)

	// assert
	if err != nil {
		t.Errorf("TestModifyClaimController_setClaimFinalizers_WhenPhaseIsPending failed, "+
			"want nil, but got %v", err)
	}

	if !utils.Contains(claim.Finalizers, ProtectClaimFinalizer) {
		t.Errorf("TestModifyClaimController_setClaimFinalizers_WhenPhaseIsPending failed, "+
			"want proctect finalzer, but got %v", claim.Finalizers)
	}

	// clean
	t.Cleanup(func() {
		if fakeClient.XuanwuV1().VolumeModifyClaims().Delete(ctx, claim.Name, metav1.DeleteOptions{}) != nil {
			t.Errorf("clean test data faild, claim name: %s", claim.Name)
		}
	})
}

func TestModifyClaimController_setClaimCreating_WhenPhaseIsPending(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	ctrl := &VolumeModifyController{
		clientSet:     fakeClient,
		eventRecorder: record.NewFakeRecorder(1000),
	}
	claim := &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-name"},
		Status: xuanwuv1.VolumeModifyClaimStatus{
			Phase: xuanwuv1.VolumeModifyClaimPending,
		},
	}

	// mock
	_, err := fakeClient.XuanwuV1().VolumeModifyClaims().Create(ctx, claim, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("setClaimFinalizers mock create calim failed")
		return
	}

	// action
	claim, err = ctrl.setClaimCreating(ctx, claim)

	// assert
	if err != nil {
		t.Errorf("TestModifyClaimController_setClaimFinalizers_WhenPhaseIsPending failed, "+
			"want nil, but got %v", err)
	}

	if claim.Status.Phase != xuanwuv1.VolumeModifyClaimCreating {
		t.Errorf("TestModifyClaimController_setClaimCreating_WhenPhaseIsPending failed, "+
			"want creating, but got %s", claim.Status.Phase)
	}

	// clean
	t.Cleanup(func() {
		if fakeClient.XuanwuV1().VolumeModifyClaims().Delete(ctx, claim.Name, metav1.DeleteOptions{}) != nil {
			t.Errorf("clean test data faild, claim name: %s", claim.Name)
		}
	})
}

func TestDeleteContent_PhaseEmpty(t *testing.T) {
	// arrange
	ctx := context.Background()
	ctrl := &VolumeModifyController{}
	claim := &xuanwuv1.VolumeModifyClaim{
		Status: xuanwuv1.VolumeModifyClaimStatus{Phase: ""},
	}

	// act
	result, err := ctrl.deleteContent(ctx, claim)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, claim, result)
}

func TestDeleteContent_PhaseDeleting(t *testing.T) {
	// arrange
	ctx := context.Background()
	ctrl := &VolumeModifyController{}
	claim := &xuanwuv1.VolumeModifyClaim{
		Status: xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimDeleting},
	}

	// act
	result, err := ctrl.deleteContent(ctx, claim)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, claim, result)
}

func TestDeleteContent_PhaseRollback(t *testing.T) {
	// arrange
	ctx := context.Background()
	ctrl := &VolumeModifyController{}
	claim := &xuanwuv1.VolumeModifyClaim{
		Status: xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimRollback},
	}

	// act
	result, err := ctrl.deleteContent(ctx, claim)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, claim, result)
}

func TestDeleteContent_GetRefContentsError(t *testing.T) {
	// arrange
	ctx := context.Background()
	factory := backendInformers.NewSharedInformerFactory(&clientSet.Clientset{}, 10*time.Second)
	contentLister := factory.Xuanwu().V1().VolumeModifyContents().Lister()

	ctrl := &VolumeModifyController{contentLister: contentLister}
	claim := &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-claim"},
		Status:     xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimCompleted},
	}

	// mock
	patches := gomonkey.ApplyMethodReturn(contentLister, "List", nil, errors.New("lister error"))
	defer patches.Reset()

	// act
	result, err := ctrl.deleteContent(ctx, claim)

	// assert
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "query content list error")
}

func TestDeleteContent_FetchStorageClassError(t *testing.T) {
	// arrange
	ctx := context.Background()
	k8sClient := k8sfake.NewSimpleClientset()
	factory := backendInformers.NewSharedInformerFactory(&clientSet.Clientset{}, 10*time.Second)
	contentLister := factory.Xuanwu().V1().VolumeModifyContents().Lister()
	recorder := record.NewFakeRecorder(1000)

	ctrl := &VolumeModifyController{
		client:        k8sClient,
		contentLister: contentLister,
		eventRecorder: recorder,
	}
	claim := &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-claim"},
		Spec: xuanwuv1.VolumeModifyClaimSpec{
			Source: &xuanwuv1.VolumeModifySpecSource{Name: "test-sc"},
		},
		Status: xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimCompleted},
	}

	// act
	result, err := ctrl.deleteContent(ctx, claim)

	// assert
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "fetch storageclass test-sc error")

	warningEvent := <-recorder.Events
	assert.Contains(t, warningEvent, DeleteFailedReason)
}

func TestDeleteContent_LunTypeCreatingCannotDelete(t *testing.T) {
	// arrange
	ctx := context.Background()
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-sc"},
		Parameters: map[string]string{"volumeType": "lun"},
	}
	k8sClient := k8sfake.NewSimpleClientset(sc)
	factory := backendInformers.NewSharedInformerFactory(&clientSet.Clientset{}, 10*time.Second)
	contentLister := factory.Xuanwu().V1().VolumeModifyContents().Lister()
	recorder := record.NewFakeRecorder(1000)

	ctrl := &VolumeModifyController{
		client:        k8sClient,
		contentLister: contentLister,
		eventRecorder: recorder,
	}
	claim := &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-claim"},
		Spec: xuanwuv1.VolumeModifyClaimSpec{
			Source: &xuanwuv1.VolumeModifySpecSource{Name: "test-sc"},
		},
		Status: xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimCreating},
	}

	// act
	result, err := ctrl.deleteContent(ctx, claim)

	// assert
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "can not delete claim test-claim while creating with lun type")

	warningEvent := <-recorder.Events
	assert.Contains(t, warningEvent, DeleteFailedReason)
}

func TestDeleteContent_NonLunTypeCreatingRollback(t *testing.T) {
	// arrange
	ctx := context.Background()
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-sc"},
		Parameters: map[string]string{"volumeType": "fs"},
	}
	k8sClient := k8sfake.NewSimpleClientset(sc)
	csClient := fake.NewSimpleClientset()
	factory := backendInformers.NewSharedInformerFactory(&clientSet.Clientset{}, 10*time.Second)
	contentLister := factory.Xuanwu().V1().VolumeModifyContents().Lister()
	claimLister := factory.Xuanwu().V1().VolumeModifyClaims().Lister()
	recorder := record.NewFakeRecorder(1000)

	ctrl := &VolumeModifyController{
		client:        k8sClient,
		clientSet:     csClient,
		contentClient: csClient,
		contentLister: contentLister,
		claimLister:   claimLister,
		eventRecorder: recorder,
	}
	claim := &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-claim"},
		Spec: xuanwuv1.VolumeModifyClaimSpec{
			Source: &xuanwuv1.VolumeModifySpecSource{Name: "test-sc"},
		},
		Status: xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimCreating},
	}

	_, err := csClient.XuanwuV1().VolumeModifyClaims().Create(ctx, &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-claim"},
		Status:     xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimCreating},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(contentLister, "List", []*xuanwuv1.VolumeModifyContent{}, nil)
	patches.ApplyMethodReturn(claimLister, "Get", claim, nil)

	// act
	result, err := ctrl.deleteContent(ctx, claim)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, xuanwuv1.VolumeModifyClaimRollback, result.Status.Phase)
}

func TestDeleteContent_CompletedPhaseDeleting(t *testing.T) {
	// arrange
	ctx := context.Background()
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: "test-sc"},
		Parameters: map[string]string{"volumeType": "fs"},
	}
	k8sClient := k8sfake.NewSimpleClientset(sc)
	csClient := fake.NewSimpleClientset()
	factory := backendInformers.NewSharedInformerFactory(&clientSet.Clientset{}, 10*time.Second)
	contentLister := factory.Xuanwu().V1().VolumeModifyContents().Lister()
	claimLister := factory.Xuanwu().V1().VolumeModifyClaims().Lister()
	recorder := record.NewFakeRecorder(1000)

	ctrl := &VolumeModifyController{
		client:        k8sClient,
		clientSet:     csClient,
		contentClient: csClient,
		contentLister: contentLister,
		claimLister:   claimLister,
		eventRecorder: recorder,
	}
	claim := &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-claim"},
		Spec: xuanwuv1.VolumeModifyClaimSpec{
			Source: &xuanwuv1.VolumeModifySpecSource{Name: "test-sc"},
		},
		Status: xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimCompleted},
	}

	_, err := csClient.XuanwuV1().VolumeModifyClaims().Create(ctx, &xuanwuv1.VolumeModifyClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "test-claim"},
		Status:     xuanwuv1.VolumeModifyClaimStatus{Phase: xuanwuv1.VolumeModifyClaimCompleted},
	}, metav1.CreateOptions{})
	assert.NoError(t, err)

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(contentLister, "List", []*xuanwuv1.VolumeModifyContent{}, nil)
	patches.ApplyMethodReturn(claimLister, "Get", claim, nil)

	// act
	result, err := ctrl.deleteContent(ctx, claim)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, xuanwuv1.VolumeModifyClaimDeleting, result.Status.Phase)
}
