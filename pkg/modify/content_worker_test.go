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
	"errors"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	appcfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/client/clientset/versioned/fake"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/k8sutils"
)

type mockModifyClient struct {
	modifyFunc func(ctx context.Context, in *drcsi.ModifyVolumeRequest) (*drcsi.ModifyVolumeResponse, error)
}

func (m *mockModifyClient) ModifyVolume(ctx context.Context, in *drcsi.ModifyVolumeRequest,
	opts ...grpc.CallOption) (*drcsi.ModifyVolumeResponse, error) {
	if m.modifyFunc != nil {
		return m.modifyFunc(ctx, in)
	}
	return nil, nil
}

func TestCallVolumeModify_WhenPhaseIsNotCreating(t *testing.T) {
	// arrange
	ctx := context.Background()
	ctrl := &VolumeModifyController{}
	content := &xuanwuv1.VolumeModifyContent{
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentPending,
		},
	}

	// act
	result, err := ctrl.callVolumeModify(ctx, content)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestCallVolumeModify_WhenModifyVolumeFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	recorder := record.NewFakeRecorder(1000)
	mockClient := &mockModifyClient{
		modifyFunc: func(ctx context.Context, in *drcsi.ModifyVolumeRequest) (*drcsi.ModifyVolumeResponse, error) {
			return nil, errors.New("modify volume error")
		},
	}
	ctrl := &VolumeModifyController{
		clientSet:     fakeClient,
		contentClient: fakeClient,
		modifyClient:  mockClient,
		eventRecorder: recorder,
	}
	content := &xuanwuv1.VolumeModifyContent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-content"},
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentCreating,
		},
		Spec: xuanwuv1.VolumeModifyContentSpec{
			VolumeHandle: "test-volume",
		},
	}

	// act
	result, err := ctrl.callVolumeModify(ctx, content)

	// assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "modify volume error")
}

func TestCallVolumeModify_WhenStorageIsOceanStorSan(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	recorder := record.NewFakeRecorder(1000)
	mockClient := &mockModifyClient{
		modifyFunc: func(ctx context.Context, in *drcsi.ModifyVolumeRequest) (*drcsi.ModifyVolumeResponse, error) {
			return &drcsi.ModifyVolumeResponse{
				VolumeAttributes: map[string]string{
					"storage":     constants.OceanStorSan,
					"needStaging": "true",
				},
			}, nil
		},
	}
	ctrl := &VolumeModifyController{
		clientSet:     fakeClient,
		contentClient: fakeClient,
		modifyClient:  mockClient,
		eventRecorder: recorder,
	}
	content := &xuanwuv1.VolumeModifyContent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-content"},
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentCreating,
		},
		Spec: xuanwuv1.VolumeModifyContentSpec{
			VolumeHandle: "test-volume",
		},
	}

	_, err := fakeClient.XuanwuV1().VolumeModifyContents().Create(ctx, content, metav1.CreateOptions{})
	assert.NoError(t, err)

	// act
	result, err := ctrl.callVolumeModify(ctx, content)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, xuanwuv1.VolumeModifyContentStaging, result.Status.Phase)
}

func TestCallVolumeModify_WhenOceanStorSanNoMapping(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	recorder := record.NewFakeRecorder(1000)
	mockClient := &mockModifyClient{
		modifyFunc: func(ctx context.Context, in *drcsi.ModifyVolumeRequest) (*drcsi.ModifyVolumeResponse, error) {
			return &drcsi.ModifyVolumeResponse{
				VolumeAttributes: map[string]string{
					"storage":     constants.OceanStorSan,
					"needStaging": "false",
				},
			}, nil
		},
	}
	ctrl := &VolumeModifyController{
		clientSet:     fakeClient,
		contentClient: fakeClient,
		modifyClient:  mockClient,
		eventRecorder: recorder,
	}
	content := &xuanwuv1.VolumeModifyContent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-content"},
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentCreating,
		},
		Spec: xuanwuv1.VolumeModifyContentSpec{
			VolumeHandle: "test-volume",
		},
	}

	_, err := fakeClient.XuanwuV1().VolumeModifyContents().Create(ctx, content, metav1.CreateOptions{})
	assert.NoError(t, err)

	// act
	result, err := ctrl.callVolumeModify(ctx, content)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, xuanwuv1.VolumeModifyContentCompleted, result.Status.Phase)
}

func TestCallVolumeModify_WhenStorageIsNotOceanStorSan(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	recorder := record.NewFakeRecorder(1000)
	mockClient := &mockModifyClient{
		modifyFunc: func(ctx context.Context, in *drcsi.ModifyVolumeRequest) (*drcsi.ModifyVolumeResponse, error) {
			return &drcsi.ModifyVolumeResponse{
				VolumeAttributes: map[string]string{
					"storage": "other-storage",
				},
			}, nil
		},
	}
	ctrl := &VolumeModifyController{
		clientSet:     fakeClient,
		contentClient: fakeClient,
		modifyClient:  mockClient,
		eventRecorder: recorder,
	}
	content := &xuanwuv1.VolumeModifyContent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-content"},
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentCreating,
		},
		Spec: xuanwuv1.VolumeModifyContentSpec{
			VolumeHandle: "test-volume",
		},
	}

	_, err := fakeClient.XuanwuV1().VolumeModifyContents().Create(ctx, content, metav1.CreateOptions{})
	assert.NoError(t, err)

	// act
	result, err := ctrl.callVolumeModify(ctx, content)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, xuanwuv1.VolumeModifyContentCompleted, result.Status.Phase)
}

func TestWaitVolumeStaged_WhenPhaseIsNotStaging(t *testing.T) {
	// arrange
	ctx := context.Background()
	ctrl := &VolumeModifyController{}
	content := &xuanwuv1.VolumeModifyContent{
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentCreating,
		},
	}

	// act
	result, err := ctrl.waitVolumeStaged(ctx, content)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestWaitVolumeStaged_WhenGetVAsByPVNameFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	recorder := record.NewFakeRecorder(1000)

	k8sClient := &k8sutils.KubeClient{}
	completedConfig := &appcfg.CompletedConfig{
		K8sUtils: k8sClient,
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()

	patches.ApplyFuncReturn(app.GetGlobalConfig, completedConfig)
	patches.ApplyMethodReturn(&k8sutils.KubeClient{}, "GetVAsByPVName",
		nil, errors.New("get VAs error"))

	ctrl := &VolumeModifyController{
		clientSet:     fakeClient,
		contentClient: fakeClient,
		eventRecorder: recorder,
	}
	content := &xuanwuv1.VolumeModifyContent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-content"},
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentStaging,
		},
		Spec: xuanwuv1.VolumeModifyContentSpec{
			PVName: "test-pv",
		},
	}

	// act
	result, err := ctrl.waitVolumeStaged(ctx, content)

	// assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get volumeAttachments by pv")
}

func TestWaitVolumeStaged_WhenScanNotFinished(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	recorder := record.NewFakeRecorder(1000)
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 1*time.Second)
	contentQueue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter,
		workqueue.RateLimitingQueueConfig{Name: "test"})

	k8sClient := &k8sutils.KubeClient{}
	completedConfig := &appcfg.CompletedConfig{
		K8sUtils: k8sClient,
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()

	patches.ApplyFuncReturn(app.GetGlobalConfig, completedConfig)

	va := []*storagev1.VolumeAttachment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-va",
				Labels: map[string]string{
					constants.RescanLabelKey: "true",
				},
			},
		},
	}
	patches.ApplyMethodReturn(&k8sutils.KubeClient{}, "GetVAsByPVName", va, nil)

	ctrl := &VolumeModifyController{
		clientSet:     fakeClient,
		contentClient: fakeClient,
		contentQueue:  contentQueue,
		eventRecorder: recorder,
	}
	content := &xuanwuv1.VolumeModifyContent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-content"},
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentStaging,
		},
		Spec: xuanwuv1.VolumeModifyContentSpec{
			PVName: "test-pv",
		},
	}

	// act
	result, err := ctrl.waitVolumeStaged(ctx, content)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, content, result)
}

func TestWaitVolumeStaged_WhenScanFinished(t *testing.T) {
	// arrange
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	recorder := record.NewFakeRecorder(1000)
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 1*time.Second)
	contentQueue := workqueue.NewRateLimitingQueueWithConfig(rateLimiter,
		workqueue.RateLimitingQueueConfig{Name: "test"})

	k8sClient := &k8sutils.KubeClient{}
	completedConfig := &appcfg.CompletedConfig{
		K8sUtils: k8sClient,
	}

	patches := gomonkey.NewPatches()
	defer patches.Reset()

	patches.ApplyFuncReturn(app.GetGlobalConfig, completedConfig)

	va := []*storagev1.VolumeAttachment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-va",
				Labels: map[string]string{
					constants.RescanLabelKey: "false",
				},
			},
		},
	}
	patches.ApplyMethodReturn(&k8sutils.KubeClient{}, "GetVAsByPVName", va, nil)

	ctrl := &VolumeModifyController{
		clientSet:     fakeClient,
		contentClient: fakeClient,
		contentQueue:  contentQueue,
		eventRecorder: recorder,
	}
	content := &xuanwuv1.VolumeModifyContent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-content"},
		Status: xuanwuv1.VolumeModifyContentStatus{
			Phase: xuanwuv1.VolumeModifyContentStaging,
		},
		Spec: xuanwuv1.VolumeModifyContentSpec{
			PVName: "test-pv",
		},
	}

	_, err := fakeClient.XuanwuV1().VolumeModifyContents().Create(ctx, content, metav1.CreateOptions{})
	assert.NoError(t, err)

	// act
	result, err := ctrl.waitVolumeStaged(ctx, content)

	// assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, xuanwuv1.VolumeModifyContentCompleted, result.Status.Phase)
}
