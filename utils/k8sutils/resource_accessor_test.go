/*
 *
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestResourceAccessor_GetByIndex_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(fakeClient, 0)
	FactoryCh := make(chan struct{})
	go factory.Start(FactoryCh)
	defer close(FactoryCh)
	var wg sync.WaitGroup
	wg.Add(1)
	testPvcKey := func(obj any) ([]string, error) {
		defer wg.Done()
		pvc, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			return nil, errors.New("obj is not of type *corev1.PersistentVolumeClaim")
		}

		return []string{pvc.Name}, nil
	}
	accessor, _ := NewResourceAccessor[*corev1.PersistentVolumeClaim](
		factory.Core().V1().PersistentVolumeClaims().Informer(),
		WithIndexers[*corev1.PersistentVolumeClaim](cache.Indexers{"test-index": testPvcKey}))
	fakeClient.CoreV1().PersistentVolumeClaims(corev1.NamespaceDefault).
		Create(ctx, genFakePvc("fake-pvc"), metav1.CreateOptions{})

	// action
	wg.Wait()
	pvc, err := accessor.GetByIndex("test-index", "fake-pvc")

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, pvc)
	assert.Equal(t, pvc.Name, "fake-pvc")
}
