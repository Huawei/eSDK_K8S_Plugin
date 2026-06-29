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

package handler

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
)

func TestBackendRegister_FetchAndRegisterAllBackend(t *testing.T) {
	// arrange
	instance := NewBackendRegister()
	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.fetchHandler), "FetchAllBackends",
		func(_ *BackendFetcher, ctx context.Context) ([]v1.StorageBackendContent, error) {
			return []v1.StorageBackendContent{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       v1.StorageBackendContentSpec{},
					Status:     &v1.StorageBackendContentStatus{},
				},
			}, nil
		})
	defer patches.Reset()

	// action
	instance.FetchAndRegisterAllBackend(context.Background())
}

func TestBackendRegister_FetchAndRegisterOneBackend(t *testing.T) {
	// arrange
	instance := NewBackendRegister()
	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.fetchHandler), "FetchBackendByName",
		func(*BackendFetcher, context.Context, string, bool) (*v1.StorageBackendContent, error) {
			return &v1.StorageBackendContent{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       v1.StorageBackendContentSpec{},
				Status:     &v1.StorageBackendContentStatus{},
			}, nil
		}).ApplyMethod(reflect.TypeOf(instance), "UpdateAndAddBackend",
		func(*BackendRegister, context.Context, v1.StorageBackendContent) (*model.Backend, error) {
			return &model.Backend{}, nil
		},
	)
	defer patches.Reset()

	// action
	_, err := instance.FetchAndRegisterOneBackend(context.Background(), "name", false)
	if err != nil {
		t.Errorf("FetchAndRegisterOneBackend want err is nil, but got error is %v", err)
	}
}

func TestBackendRegister_LoadOrRegisterOneBackend(t *testing.T) {
	// arrange
	instance := NewBackendRegister()
	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "Load",
		func(*CacheWrapper, string) (model.Backend, bool) {
			return model.Backend{}, true
		})
	defer patches.Reset()

	// action
	_, err := instance.LoadOrRegisterOneBackend(context.Background(), "name")
	if err != nil {
		t.Errorf("LoadOrRegisterOneBackend want err is nil, but got error is %v", err)
	}
}

func TestBackendRegister_UpdateAndAddBackend(t *testing.T) {
	// arrange
	instance := NewBackendRegister()
	sbct := v1.StorageBackendContent{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       v1.StorageBackendContentSpec{BackendClaim: "ns/test"},
		Status:     &v1.StorageBackendContentStatus{Online: true},
	}

	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "Load",
		func(*CacheWrapper, string) (model.Backend, bool) {
			return model.Backend{}, true
		})
	defer patches.Reset()

	// action
	_, err := instance.UpdateAndAddBackend(context.Background(), sbct)
	if err != nil {
		t.Errorf("UpdateAndAddBackend want err is nil, but got error is %v", err)
	}
}

func TestBackendRegister_RemoveRegisteredOneBackend(t *testing.T) {
	// arrange
	instance := NewBackendRegister()

	// mock
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "Delete",
		func(*CacheWrapper, context.Context, string) {})
	defer patches.Reset()

	// action - should not panic
	instance.RemoveRegisteredOneBackend(context.Background(), "test-backend")
}

func TestBackendRegister_LoadOrRebuildOneBackend_NeedRebuild(t *testing.T) {
	// arrange
	instance := NewBackendRegister()

	// mock - NeedRebuild returns true
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "Load",
		func(*CacheWrapper, string) (model.Backend, bool) {
			return model.Backend{Name: "test", ContentName: "old-content"}, true
		}).ApplyMethod(reflect.TypeOf(instance.cacheHandler), "Delete",
		func(*CacheWrapper, context.Context, string) {}).ApplyMethod(reflect.TypeOf(instance), "FetchAndRegisterOneBackend",
		func(*BackendRegister, context.Context, string, bool) (*model.Backend, error) {
			return &model.Backend{Name: "test", ContentName: "new-content"}, nil
		})
	defer patches.Reset()

	// action
	_, err := instance.LoadOrRebuildOneBackend(context.Background(), "test", "new-content")
	if err != nil {
		t.Errorf("LoadOrRebuildOneBackend want err is nil, but got error is %v", err)
	}
}

func TestBackendRegister_LoadOrRebuildOneBackend_NotExists(t *testing.T) {
	// arrange
	instance := NewBackendRegister()

	// mock - backend not in cache
	patches := gomonkey.ApplyMethod(reflect.TypeOf(instance.cacheHandler), "Load",
		func(*CacheWrapper, string) (model.Backend, bool) {
			return model.Backend{}, false
		}).ApplyMethod(reflect.TypeOf(instance), "FetchAndRegisterOneBackend",
		func(*BackendRegister, context.Context, string, bool) (*model.Backend, error) {
			return &model.Backend{Name: "test"}, nil
		})
	defer patches.Reset()

	// action
	_, err := instance.LoadOrRebuildOneBackend(context.Background(), "test", "content")
	if err != nil {
		t.Errorf("LoadOrRebuildOneBackend want err is nil, but got error is %v", err)
	}
}

func TestBackendRegister_UpdateOrRegisterOneBackend_Error(t *testing.T) {
	// arrange
	instance := NewBackendRegister()
	sbct := &v1.StorageBackendContent{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       v1.StorageBackendContentSpec{BackendClaim: "ns/test"},
		Status:     &v1.StorageBackendContentStatus{Online: true},
	}

	// mock - SplitMetaNamespaceKey returns error
	patches := gomonkey.ApplyFuncReturn(pkgUtils.SplitMetaNamespaceKey,
		"", "", errors.New("split error"))
	defer patches.Reset()

	// action
	err := instance.UpdateOrRegisterOneBackend(context.Background(), sbct)
	if err == nil {
		t.Errorf("UpdateOrRegisterOneBackend want err is not nil, but got nil")
	}
}
