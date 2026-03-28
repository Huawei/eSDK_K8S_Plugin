/*
 *
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

const (
	fakePv = "fake-pv"
)

func TestKubeClient_GetVolumeAttrsByVolumeId_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(fakeClient, 0)
	factoryCh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	testPvcKey := func(obj any) ([]string, error) {
		defer wg.Done()
		pv, ok := obj.(*corev1.PersistentVolume)
		if !ok {
			return nil, errors.New("obj is not of type *corev1.PersistentVolume")
		}

		return []string{pv.Name}, nil
	}

	// mock
	accessor, _ := NewResourceAccessor[*corev1.PersistentVolume](
		factory.Core().V1().PersistentVolumes().Informer(),
		WithIndexers[*corev1.PersistentVolume](cache.Indexers{volumeIdIndex: testPvcKey}))
	factory.Start(factoryCh)
	defer close(factoryCh)
	fakeClient.CoreV1().PersistentVolumes().
		Create(ctx, genFakePv("fake-pv"), metav1.CreateOptions{})
	client := &KubeClient{pvAccessor: accessor}

	// action
	wg.Wait()
	_, err := client.GetVolumeAttrsByVolumeId("fake-pv")

	// assert
	assert.NoError(t, err)
}

func TestKubeClient_GetDTreeParentNameByVolumeId_NoParentNameField(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(fakeClient, 0)
	factoryCh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	testPvcKey := func(obj any) ([]string, error) {
		defer wg.Done()
		pv, ok := obj.(*corev1.PersistentVolume)
		if !ok {
			return nil, errors.New("obj is not of type *corev1.PersistentVolume")
		}

		return []string{pv.Name}, nil
	}

	// mock
	accessor, _ := NewResourceAccessor[*corev1.PersistentVolume](
		factory.Core().V1().PersistentVolumes().Informer(),
		WithIndexers[*corev1.PersistentVolume](cache.Indexers{volumeIdIndex: testPvcKey}))
	factory.Start(factoryCh)
	defer close(factoryCh)
	fakeClient.CoreV1().PersistentVolumes().
		Create(ctx, genFakePv("fake-pv"), metav1.CreateOptions{})
	client := &KubeClient{pvAccessor: accessor}

	// action
	wg.Wait()
	parent, err := client.GetDTreeParentNameByVolumeId("fake-pv")

	// assert
	assert.ErrorContains(t, err, "does not exist")
	assert.Equal(t, "", parent)
}

func TestKubeClient_GetKvCacheStoreIdByVolumeId_NoKvCacheStoreIdField(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(fakeClient, 0)
	factoryCh := make(chan struct{})
	notifyCh := make(chan struct{}, 1)
	testPvcKey := func(obj any) ([]string, error) {
		defer func() {
			select {
			case notifyCh <- struct{}{}:
			default:
			}
		}()

		pv, ok := obj.(*corev1.PersistentVolume)
		if !ok {
			return nil, errors.New("obj is not of type *corev1.PersistentVolume")
		}

		return []string{pv.Name}, nil
	}
	wantErr := fmt.Errorf("volume attrs %q does not exist", fakePv)

	// mock
	accessor, _ := NewResourceAccessor[*corev1.PersistentVolume](factory.Core().V1().PersistentVolumes().Informer(),
		WithIndexers[*corev1.PersistentVolume](cache.Indexers{volumeIdIndex: testPvcKey}))
	factory.Start(factoryCh)
	defer close(factoryCh)
	fakeClient.CoreV1().PersistentVolumes().
		Create(ctx, genFakePv(fakePv), metav1.CreateOptions{})
	client := &KubeClient{pvAccessor: accessor}

	// action
	<-notifyCh
	kvcacheStoreId, err := client.GetKvCacheStoreIdByVolumeId(fakePv)

	// assert
	assert.EqualError(t, err, wantErr.Error())
	assert.Equal(t, "", kvcacheStoreId)
}

func genFakePv(name string) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		TypeMeta:   metav1.TypeMeta{Kind: "PersistentVolume"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{},
			},
		},
	}
}

func TestKubeClient_GetKvCacheStoreIdByVolumeId_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(fakeClient, 0)
	factoryCh := make(chan struct{})

	notifyCh := make(chan struct{}, 1)
	testPvcKey := func(obj any) ([]string, error) {
		defer func() {
			select {
			case notifyCh <- struct{}{}:
			default:
			}
		}()

		pv, ok := obj.(*corev1.PersistentVolume)
		if !ok {
			return nil, errors.New("obj is not of type *corev1.PersistentVolume")
		}

		return []string{pv.Name}, nil
	}
	expectedStoreId := "test-store-id-123"

	// mock
	accessor, _ := NewResourceAccessor[*corev1.PersistentVolume](factory.Core().V1().PersistentVolumes().Informer(),
		WithIndexers[*corev1.PersistentVolume](cache.Indexers{volumeIdIndex: testPvcKey}))

	factory.Start(factoryCh)
	defer close(factoryCh)

	fakeClient.CoreV1().PersistentVolumes().
		Create(ctx, genFakePvWithKvcacheStoreId(expectedStoreId), metav1.CreateOptions{})

	client := &KubeClient{pvAccessor: accessor}

	// action
	<-notifyCh
	cache.WaitForCacheSync(factoryCh, factory.Core().V1().PersistentVolumes().Informer().HasSynced)
	parent, err := client.GetKvCacheStoreIdByVolumeId(fakePv)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, expectedStoreId, parent)
}

func genFakePvWithKvcacheStoreId(expectedStoreId string) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-pv",
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					VolumeHandle: fakePv,
					VolumeAttributes: map[string]string{
						constants.KvCacheStoreId: expectedStoreId,
					},
				},
			},
		},
	}
}

func TestKubeClient_GetKvCacheStoreIdByVolumeId_GetVolumeAttrsByVolumeIdField(t *testing.T) {
	// arrange
	client := &KubeClient{}
	getVolumeAttrsByVolumeIdErr := errors.New("test error")
	wantErr := fmt.Errorf("failed to get volume attributes by volume id %q: %w", fakePv, getVolumeAttrsByVolumeIdErr)

	// mock
	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodReturn(&KubeClient{}, "GetVolumeAttrsByVolumeId",
		nil, errors.New("test error"))

	// action
	kvcacheStoreId, err := client.GetKvCacheStoreIdByVolumeId(fakePv)

	// assert
	assert.EqualError(t, err, wantErr.Error())
	assert.Equal(t, "", kvcacheStoreId)
}
