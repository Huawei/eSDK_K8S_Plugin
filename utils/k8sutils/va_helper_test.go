/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

type fakeKubeClient struct {
	client    *KubeClient
	clientSet *fake.Clientset
	factory   informers.SharedInformerFactory
	stopCh    chan struct{}
}

func buildFakeKubeClientWithVA() *fakeKubeClient {
	clientSet := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(clientSet, 0)
	stopCh := make(chan struct{})

	pvAccessor, _ := NewResourceAccessor[*corev1.PersistentVolume](
		factory.Core().V1().PersistentVolumes().Informer(),
		WithIndexers[*corev1.PersistentVolume](cache.Indexers{volumeIdIndex: func(obj any) ([]string, error) {
			pv, ok := obj.(*corev1.PersistentVolume)
			if !ok {
				return []string{}, nil
			}
			if pv.Spec.CSI == nil {
				return []string{}, nil
			}
			return []string{pv.Spec.CSI.VolumeHandle}, nil
		}}),
	)

	vaAccessor, _ := NewResourceAccessor[*storagev1.VolumeAttachment](
		factory.Storage().V1().VolumeAttachments().Informer(),
		WithIndexers[*storagev1.VolumeAttachment](cache.Indexers{pvNameIndex: func(obj any) ([]string, error) {
			va, ok := obj.(*storagev1.VolumeAttachment)
			if !ok {
				return []string{}, nil
			}
			if va.Spec.Source.PersistentVolumeName == nil {
				return []string{}, nil
			}
			return []string{*va.Spec.Source.PersistentVolumeName}, nil
		}}),
	)

	client := &KubeClient{
		clientSet:       clientSet,
		informerFactory: factory,
		pvAccessor:      pvAccessor,
		vaAccessor:      vaAccessor,
	}

	factory.Start(stopCh)
	cache.WaitForCacheSync(stopCh,
		factory.Core().V1().PersistentVolumes().Informer().HasSynced,
		factory.Storage().V1().VolumeAttachments().Informer().HasSynced,
	)

	return &fakeKubeClient{
		client:    client,
		clientSet: clientSet,
		factory:   factory,
		stopCh:    stopCh,
	}
}

func createPVWithVolumeId(clientSet *fake.Clientset, pvName, volumeId string) {
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: pvName},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					VolumeHandle: volumeId,
				},
			},
		},
	}
	clientSet.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{})
	time.Sleep(50 * time.Millisecond)
}

func createVA(clientSet *fake.Clientset, vaName, pvName, nodeName string) {
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: vaName},
		Spec: storagev1.VolumeAttachmentSpec{
			NodeName: nodeName,
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName},
		},
		Status: storagev1.VolumeAttachmentStatus{
			AttachmentMetadata: map[string]string{},
		},
	}
	clientSet.StorageV1().VolumeAttachments().Create(context.Background(), va, metav1.CreateOptions{})
	time.Sleep(50 * time.Millisecond)
}

func TestGetVAsByPVName_AccessorNil(t *testing.T) {
	// arrange
	client := &KubeClient{vaAccessor: nil}

	// action
	vaList, err := client.GetVAsByPVName("test-pv")

	// assert
	assert.Nil(t, vaList)
	assert.ErrorContains(t, err, "VolumeAttachment accessor is not initialized")
}

func TestGetVAsByPVName_Success(t *testing.T) {
	// arrange
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)
	createVA(fakeClient.clientSet, "va-test", "test-pv", "node1")

	// action
	vaList, err := fakeClient.client.GetVAsByPVName("test-pv")

	// assert
	assert.NoError(t, err)
	assert.Len(t, vaList, 1)
	assert.Equal(t, "va-test", vaList[0].Name)
	assert.Equal(t, "node1", vaList[0].Spec.NodeName)
}

func TestGetVAsByPVName_NotFound(t *testing.T) {
	// arrange
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	// action
	vaList, err := fakeClient.client.GetVAsByPVName("nonexistent-pv")

	// assert
	assert.Empty(t, vaList)
	assert.Error(t, err)
}

func TestGetVA_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientSet := fake.NewSimpleClientset()
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-test"},
	}
	_, _ = clientSet.StorageV1().VolumeAttachments().Create(ctx, va, metav1.CreateOptions{})
	client := &KubeClient{clientSet: clientSet}

	// action
	got, err := client.GetVA(ctx, "va-test")

	// assert
	assert.NoError(t, err)
	assert.Equal(t, "va-test", got.Name)
}

func TestGetVA_NotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientSet := fake.NewSimpleClientset()
	client := &KubeClient{clientSet: clientSet}

	// action
	_, err := client.GetVA(ctx, "nonexistent")

	// assert
	assert.Error(t, err)
}

func TestUpdateVA_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientSet := fake.NewSimpleClientset()
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-test"},
	}
	_, _ = clientSet.StorageV1().VolumeAttachments().Create(ctx, va, metav1.CreateOptions{})
	client := &KubeClient{clientSet: clientSet}

	// action
	va.Labels = map[string]string{"key": "value"}
	updated, err := client.UpdateVA(ctx, va)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, "value", updated.Labels["key"])
}

func TestUpdateVAStatus_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientSet := fake.NewSimpleClientset()
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-test"},
		Status:     storagev1.VolumeAttachmentStatus{Attached: false},
	}
	_, _ = clientSet.StorageV1().VolumeAttachments().Create(ctx, va, metav1.CreateOptions{})
	client := &KubeClient{clientSet: clientSet}

	// action
	va.Status.Attached = true
	updated, err := client.UpdateVAStatus(ctx, va)

	// assert
	assert.NoError(t, err)
	assert.True(t, updated.Status.Attached)
}

func TestUpdateVAsWithHostMap_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")
	createVA(fakeClient.clientSet, "va-test", "test-pv", "node1")

	hostMap := map[string]map[string]interface{}{
		"node1": {"wwn": "fake-wwn"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.NoError(t, err)

	updated, err := fakeClient.client.GetVA(ctx, "va-test")
	assert.NoError(t, err)
	assert.Equal(t, "true", updated.Labels[constants.RescanLabelKey])

	expectedBytes, err := json.Marshal(hostMap["node1"])
	assert.NoError(t, err)
	assert.Equal(t, string(expectedBytes), updated.Status.AttachmentMetadata["publishInfo"])
}

func TestUpdateVAsWithHostMap_PVNotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	hostMap := map[string]map[string]interface{}{
		"node1": {"wwn": "fake-wwn"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "nonexistent-vol", hostMap)

	// assert
	assert.Error(t, err)
	assert.ErrorContains(t, err, "update VAs failed while getting pvList")
}

func TestUpdateVAsWithHostMap_VANotFoundForHost(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")
	createVA(fakeClient.clientSet, "va-test", "test-pv", "other-node")

	hostMap := map[string]map[string]interface{}{
		"node1": {"wwn": "fake-wwn"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.ErrorContains(t, err, "can not find the VA")
}

func TestUpdateVAsWithHostMap_GetVAsByPVNameError(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")
	fakeClient.client.vaAccessor = nil

	hostMap := map[string]map[string]interface{}{
		"node1": {"key": "val"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.ErrorContains(t, err, "get vaList by pv name")
}

func TestUpdateVAsWithHostMap_MarshalError(t *testing.T) {
	// arrange
	ctx := context.Background()
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	vaList := []*storagev1.VolumeAttachment{
		{Spec: storagev1.VolumeAttachmentSpec{NodeName: "node1"}},
	}
	patches.ApplyMethodReturn(&KubeClient{}, "GetVAsByPVName",
		vaList, nil)
	patches.ApplyFuncReturn(json.Marshal, nil, fmt.Errorf("marshal error"))

	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")

	hostMap := map[string]map[string]interface{}{
		"node1": {"key": "val"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.ErrorContains(t, err, "marshal mappingInfo")
}

func TestUpdateVAsWithHostMap_UpdatePublishInfoError(t *testing.T) {
	// arrange
	ctx := context.Background()
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	vaList := []*storagev1.VolumeAttachment{
		{Spec: storagev1.VolumeAttachmentSpec{NodeName: "node1"}},
	}
	patches.ApplyMethodReturn(&KubeClient{}, "GetVAsByPVName",
		vaList, nil)
	patches.ApplyMethodReturn(&KubeClient{}, "GetVA",
		nil, fmt.Errorf("get va error"))

	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")

	hostMap := map[string]map[string]interface{}{
		"node1": {"key": "val"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.ErrorContains(t, err, "update VA publish info")
}

func TestUpdateVAsWithHostMap_MultipleHostsInHostMap(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")
	createVA(fakeClient.clientSet, "va-node1", "test-pv", "node1")
	createVA(fakeClient.clientSet, "va-node2", "test-pv", "node2")

	hostMap := map[string]map[string]interface{}{
		"node1": {"wwn": "wwn-1"},
		"node2": {"wwn": "wwn-2"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.NoError(t, err)

	va1, err := fakeClient.client.GetVA(ctx, "va-node1")
	assert.NoError(t, err)
	assert.Equal(t, "true", va1.Labels[constants.RescanLabelKey])
	expectedBytes1, err := json.Marshal(hostMap["node1"])
	assert.NoError(t, err)
	assert.Equal(t, string(expectedBytes1), va1.Status.AttachmentMetadata["publishInfo"])

	va2, err := fakeClient.client.GetVA(ctx, "va-node2")
	assert.NoError(t, err)
	assert.Equal(t, "true", va2.Labels[constants.RescanLabelKey])
	expectedBytes2, err := json.Marshal(hostMap["node2"])
	assert.NoError(t, err)
	assert.Equal(t, string(expectedBytes2), va2.Status.AttachmentMetadata["publishInfo"])
}

func TestUpdateVAsWithHostMap_EmptyHostMap(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")
	createVA(fakeClient.clientSet, "va-test", "test-pv", "node1")

	hostMap := map[string]map[string]interface{}{}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.NoError(t, err)
}

func TestUpdateVAsWithHostMap_MultiplePVsForSameVolumeId(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "pv-1", "vol-123")
	createPVWithVolumeId(fakeClient.clientSet, "pv-2", "vol-123")
	createVA(fakeClient.clientSet, "va-pv1-node1", "pv-1", "node1")
	createVA(fakeClient.clientSet, "va-pv2-node1", "pv-2", "node1")

	hostMap := map[string]map[string]interface{}{
		"node1": {"wwn": "fake-wwn"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.NoError(t, err)

	va1, err := fakeClient.client.GetVA(ctx, "va-pv1-node1")
	assert.NoError(t, err)
	assert.Equal(t, "true", va1.Labels[constants.RescanLabelKey])

	va2, err := fakeClient.client.GetVA(ctx, "va-pv2-node1")
	assert.NoError(t, err)
	assert.Equal(t, "true", va2.Labels[constants.RescanLabelKey])
}

func TestUpdateVAsWithHostMap_UpdateScanLabelError(t *testing.T) {
	// arrange
	ctx := context.Background()
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-test"},
		Spec:       storagev1.VolumeAttachmentSpec{NodeName: "node1"},
		Status:     storagev1.VolumeAttachmentStatus{AttachmentMetadata: map[string]string{}},
	}
	patches.ApplyMethodReturn(&KubeClient{}, "GetVAsByPVName",
		[]*storagev1.VolumeAttachment{va}, nil)
	patches.ApplyMethodReturn(&KubeClient{}, "GetVA",
		va, nil)
	patches.ApplyMethodReturn(&KubeClient{}, "UpdateVAStatus",
		va, nil)
	patches.ApplyMethodReturn(&KubeClient{}, "UpdateVA",
		nil, fmt.Errorf("update va label error"))

	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")

	hostMap := map[string]map[string]interface{}{
		"node1": {"key": "val"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.ErrorContains(t, err, "update VA label")
}

func TestUpdateVAsWithHostMap_FirstHostSuccessSecondVANotFound(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := buildFakeKubeClientWithVA()
	defer close(fakeClient.stopCh)

	createPVWithVolumeId(fakeClient.clientSet, "test-pv", "vol-123")
	createVA(fakeClient.clientSet, "va-node1", "test-pv", "node1")

	hostMap := map[string]map[string]interface{}{
		"node1": {"wwn": "wwn-1"},
		"node2": {"wwn": "wwn-2"},
	}

	// action
	err := fakeClient.client.UpdateVAsWithHostMap(ctx, "vol-123", hostMap)

	// assert
	assert.ErrorContains(t, err, "can not find the VA")
}
