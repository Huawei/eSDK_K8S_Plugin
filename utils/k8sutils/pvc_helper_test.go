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

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var logName = "pvc_helper_test.log"

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func initClient() *KubeClient {
	clientSet := fake.NewSimpleClientset()
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
	helper := initClient()

	// action
	err := initPVCAccessor(helper)

	// assert
	assert.NoError(t, err)
}

func TestActivate(t *testing.T) {
	// arrange
	helper := initClient()

	// action
	err := initPVCAccessor(helper)
	helper.Activate()
	defer helper.Deactivate()

	// assert
	assert.NoError(t, err)
}

func TestProcessPVC(t *testing.T) {
	obj := genFakePvc("fake-pvc")
	helper := initClient()
	helper.addPVC(obj)
	helper.deletePVC(obj)
	helper.updatePVC(nil, obj)
}

func TestGetVolumeConfiguration(t *testing.T) {
	// arrange
	helper := initClient()

	// action
	initPVCAccessor(helper)
	_, err := helper.GetVolumeConfiguration(context.TODO(), "fake-pvc")

	// assert
	if err == nil {
		t.Error("TestGetVolumeConfiguration failed")
	}
}
