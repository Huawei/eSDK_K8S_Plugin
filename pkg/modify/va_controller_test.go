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

package modify

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	storageV1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/manage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/scanner"
)

func TestNewVAController(t *testing.T) {
	// arrange
	kubeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)
	vaInformer := informerFactory.Storage().V1().VolumeAttachments()

	request := VAControllerRequest{
		KubeClient:    kubeClient,
		EventRecorder: nil,
		VaInformer:    vaInformer,
		HostName:      "test-node",
	}

	// act
	ctrl := NewVAController(request)

	// assert
	assert.NotNil(t, ctrl)
	assert.NotNil(t, ctrl.vaQueue)
}

func TestVAController_Run_Success(t *testing.T) {
	// arrange
	kubeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)
	vaInformer := informerFactory.Storage().V1().VolumeAttachments()

	request := VAControllerRequest{
		KubeClient:    kubeClient,
		EventRecorder: nil,
		VaInformer:    vaInformer,
		HostName:      "test-node",
	}
	ctrl := NewVAController(request)

	ctx := context.Background()
	stopCh := make(chan struct{})

	informerFactory.Start(stopCh)
	require.True(t, cache.WaitForCacheSync(stopCh, vaInformer.Informer().HasSynced))

	// act
	go ctrl.Run(ctx, 1, stopCh)
	time.Sleep(100 * time.Millisecond)

	// assert
	select {
	case <-stopCh:
	default:
	}

	// cleanup
	close(stopCh)
}

func TestVAController_enqueueVA(t *testing.T) {
	// arrange
	kubeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)
	vaInformer := informerFactory.Storage().V1().VolumeAttachments()

	request := VAControllerRequest{
		KubeClient:    kubeClient,
		EventRecorder: nil,
		VaInformer:    vaInformer,
		HostName:      "test-node",
	}
	ctrl := NewVAController(request)

	va := &storageV1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-va",
			Labels: map[string]string{
				constants.RescanLabelKey: "true",
			},
		},
		Spec: storageV1.VolumeAttachmentSpec{
			NodeName: "test-node",
		},
	}

	// act
	ctrl.enqueueVA(va)

	// assert
	assert.Equal(t, 1, ctrl.vaQueue.Len())
}

func TestVAController_enqueueVA_NoRescanLabel(t *testing.T) {
	// arrange
	kubeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)
	vaInformer := informerFactory.Storage().V1().VolumeAttachments()

	request := VAControllerRequest{
		KubeClient:    kubeClient,
		EventRecorder: nil,
		VaInformer:    vaInformer,
		HostName:      "test-node",
	}
	ctrl := NewVAController(request)

	va := &storageV1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-va",
			Labels: map[string]string{},
		},
		Spec: storageV1.VolumeAttachmentSpec{
			NodeName: "test-node",
		},
	}

	// act
	ctrl.enqueueVA(va)

	// assert
	assert.Equal(t, 0, ctrl.vaQueue.Len())
}

func TestVAController_processNextVAItem_Shutdown(t *testing.T) {
	// arrange
	kubeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)
	vaInformer := informerFactory.Storage().V1().VolumeAttachments()

	request := VAControllerRequest{
		KubeClient:    kubeClient,
		EventRecorder: nil,
		VaInformer:    vaInformer,
		HostName:      "test-node",
	}
	ctrl := NewVAController(request)
	ctrl.vaQueue.ShutDown()

	ctx := context.Background()

	// act
	result := ctrl.processNextVAItem(ctx)

	// assert
	assert.False(t, result)
}

func TestVAController_handleVA_Success(t *testing.T) {
	// arrange
	kubeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)
	vaInformer := informerFactory.Storage().V1().VolumeAttachments()

	request := VAControllerRequest{
		KubeClient:    kubeClient,
		EventRecorder: nil,
		VaInformer:    vaInformer,
		HostName:      "test-node",
	}
	ctrl := NewVAController(request)

	publishInfo := manage.ControllerPublishInfo{
		TgtIQNs:     []string{"iqn.2026-06.com.huawei:eSDK_K8S_Plugin"},
		TgtPortals:  []string{"192.168.1.1"},
		TgtHostLUNs: []string{"0"},
		TgtLunWWN:   "wwn.123456789",
	}

	publishInfoBytes, err := json.Marshal(publishInfo)
	assert.NoError(t, err)
	va := &storageV1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-va",
			Labels: map[string]string{
				constants.RescanLabelKey: "true",
			},
		},
		Spec: storageV1.VolumeAttachmentSpec{
			NodeName: "test-node",
		},
		Status: storageV1.VolumeAttachmentStatus{
			AttachmentMetadata: map[string]string{
				"publishInfo": string(publishInfoBytes),
			},
		},
	}

	// mock
	stopCh := make(chan struct{})
	informerFactory.Start(stopCh)
	_, err = kubeClient.StorageV1().VolumeAttachments().Create(context.Background(), va, metav1.CreateOptions{})
	assert.NoError(t, err)
	require.True(t, cache.WaitForCacheSync(stopCh, vaInformer.Informer().HasSynced))
	mock := gomonkey.NewPatches().ApplyMethodReturn(&scanner.ISCSIScanner{}, "Scan", nil)
	defer mock.Reset()

	// act
	err = ctrl.handleVA(context.Background(), va)

	// assert
	assert.Nil(t, err)
	close(stopCh)
}

func TestVAController_handleVA_ScanError(t *testing.T) {
	// arrange
	kubeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)
	vaInformer := informerFactory.Storage().V1().VolumeAttachments()

	request := VAControllerRequest{
		KubeClient:    kubeClient,
		EventRecorder: record.NewFakeRecorder(1000),
		VaInformer:    vaInformer,
		HostName:      "test-node",
	}
	ctrl := NewVAController(request)

	publishInfo := manage.ControllerPublishInfo{
		TgtIQNs:     []string{"iqn.2026-06.com.huawei:eSDK_K8S_Plugin"},
		TgtPortals:  []string{"192.168.1.1"},
		TgtHostLUNs: []string{"0"},
		TgtLunWWN:   "wwn.123456789",
	}
	publishInfoBytes, err := json.Marshal(publishInfo)
	assert.NoError(t, err)

	va := &storageV1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-va",
			Labels: map[string]string{
				constants.RescanLabelKey: "true",
			},
		},
		Spec: storageV1.VolumeAttachmentSpec{
			NodeName: "test-node",
		},
		Status: storageV1.VolumeAttachmentStatus{
			AttachmentMetadata: map[string]string{
				"publishInfo": string(publishInfoBytes),
			},
		},
	}

	scanErr := fmt.Errorf("scan volume failed")
	mock := gomonkey.NewPatches().ApplyFuncReturn(filepath.Glob, []string{"path1"}, nil).
		ApplyMethodReturn(&scanner.ISCSIScanner{}, "Scan", scanErr)
	defer mock.Reset()

	// act
	err = ctrl.handleVA(context.Background(), va)

	// assert
	assert.NotNil(t, err)
	assert.ErrorContains(t, err, "scan volume failed")
}

func TestVAController_handleVA_PublishInfoNotFound(t *testing.T) {
	// arrange
	kubeClient := fake.NewClientset()
	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)
	vaInformer := informerFactory.Storage().V1().VolumeAttachments()

	request := VAControllerRequest{
		KubeClient:    kubeClient,
		EventRecorder: nil,
		VaInformer:    vaInformer,
		HostName:      "test-node",
	}
	ctrl := NewVAController(request)

	va := &storageV1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-va",
		},
		Spec: storageV1.VolumeAttachmentSpec{
			NodeName: "test-node",
		},
		Status: storageV1.VolumeAttachmentStatus{
			AttachmentMetadata: map[string]string{},
		},
	}

	// act
	err := ctrl.handleVA(context.Background(), va)

	// assert
	assert.NotNil(t, err)
	assert.ErrorContains(t, err, "publishInfo not found")
}
