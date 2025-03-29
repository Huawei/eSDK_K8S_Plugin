/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var logName = "pvc_helper_test.log"

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func initClient(clientSet kubernetes.Interface) *KubeClient {
	helper := KubeClient{
		clientSet:         clientSet,
		informersStopChan: make(chan struct{}),
		volumeNamePrefix:  "pvc",
		informerFactory:   informers.NewSharedInformerFactory(clientSet, 0),
	}
	return &helper
}

func genFakePvc(name string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		TypeMeta:   metav1.TypeMeta{Kind: "PersistentVolumeClaim"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.PersistentVolumeClaimSpec{
			Resources: v1.VolumeResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceStorage: {},
				}},
		},
	}
}

func TestInitPVCAccessor(t *testing.T) {
	// arrange
	helper := initClient(fake.NewSimpleClientset())

	// action
	err := initPVCAccessor(helper)

	// assert
	assert.NoError(t, err)
}

func TestActivate(t *testing.T) {
	// arrange
	helper := initClient(fake.NewSimpleClientset())

	// action
	err := initPVCAccessor(helper)
	helper.Activate()
	defer helper.Deactivate()

	// assert
	assert.NoError(t, err)
}

func TestProcessPVC(t *testing.T) {
	obj := genFakePvc("fake-pvc")
	helper := initClient(fake.NewSimpleClientset())
	helper.addPVC(obj)
	helper.deletePVC(obj)
	helper.updatePVC(nil, obj)
}

func TestGetVolumeConfiguration(t *testing.T) {
	// arrange
	helper := initClient(fake.NewSimpleClientset())

	// action
	err := initPVCAccessor(helper)
	_, err = helper.GetVolumeConfiguration(context.TODO(), "fake-pvc")

	// assert
	if err == nil {
		t.Error("TestGetVolumeConfiguration failed")
	}
}

func Test_GetPvcCache_OverWrite(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientSet := fake.NewSimpleClientset()
	client := initClient(clientSet)
	err := initPVCAccessor(client)
	client.Activate()
	h1 := newTestPvcAnnoHelper("1")
	h2 := newTestPvcAnnoHelper("2")
	h3 := newTestPvcAnnoHelper("3")
	h4 := newTestPvcAnnoHelper("4")

	// action
	_, err = clientSet.CoreV1().PersistentVolumeClaims(h1.ns).Create(ctx, h1.fakePvc(), metav1.CreateOptions{})
	time.Sleep(time.Millisecond)
	anno1, err1 := client.GetVolumeConfiguration(context.Background(), h1.volume)
	_, err = clientSet.CoreV1().PersistentVolumeClaims(h2.ns).Create(ctx, h2.fakePvc(), metav1.CreateOptions{})
	time.Sleep(time.Millisecond)
	anno2, err2 := client.GetVolumeConfiguration(context.Background(), h2.volume)

	_, err = clientSet.CoreV1().PersistentVolumeClaims(h3.ns).Create(ctx, h3.fakePvc(), metav1.CreateOptions{})
	_, err = clientSet.CoreV1().PersistentVolumeClaims(h4.ns).Create(ctx, h4.fakePvc(), metav1.CreateOptions{})
	time.Sleep(time.Millisecond)
	anno3, err3 := client.GetVolumeConfiguration(context.Background(), h3.volume)
	anno4, err4 := client.GetVolumeConfiguration(context.Background(), h4.volume)

	// assert
	require.NoError(t, err)
	h1.assertAnnoResult(t, err1, anno1)
	h2.assertAnnoResult(t, err2, anno2)
	h3.assertAnnoResult(t, err3, anno3)
	h4.assertAnnoResult(t, err4, anno4)
}

type testPvcAnnoHelper struct {
	no     string
	ns     string
	volume string
}

func newTestPvcAnnoHelper(no string) *testPvcAnnoHelper {
	return &testPvcAnnoHelper{
		no:     no,
		ns:     "ns-" + no,
		volume: "pvc-uid-" + no,
	}
}

func (helper *testPvcAnnoHelper) fakePvc() *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pvc-name",
			Namespace:   helper.ns,
			UID:         "uid-" + types.UID(helper.no),
			Annotations: map[string]string{"key" + helper.no: "value" + helper.no},
		},
	}
}

func (helper *testPvcAnnoHelper) assertAnnoResult(t *testing.T, err error, anno map[string]string) {
	require.NoError(t, err)
	val, ok := anno["key"+helper.no]
	require.True(t, ok)
	require.Equal(t, "value"+helper.no, val)
}
