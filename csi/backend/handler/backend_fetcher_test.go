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
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "huawei-csi-driver/client/apis/xuanwu/v1"
	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	pkgUtils "huawei-csi-driver/pkg/utils"
)

func TestBackendFetcher_FetchAllBackends(t *testing.T) {
	// arrange
	instance := NewBackendFetcher()

	// mock
	patches := gomonkey.ApplyFunc(pkgUtils.ListContent, func(ctx context.Context,
		client clientSet.Interface) (*v1.StorageBackendContentList, error) {
		return &v1.StorageBackendContentList{
			Items: []v1.StorageBackendContent{
				{
					TypeMeta:   metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{},
					Spec:       v1.StorageBackendContentSpec{},
					Status: &v1.StorageBackendContentStatus{
						Pools: nil,
						Capabilities: map[string]bool{
							"SupportThin": true,
						},
						Specification: nil,
						ConfigmapMeta: "",
						SecretMeta:    "",
						Online:        true,
					},
				},
			},
		}, nil
	})
	defer patches.Reset()

	// action
	backends, err := instance.FetchAllBackends(context.Background())
	if err != nil {
		t.Errorf("FetchAllBackends want err is nil, but got = %v", err)
		return
	}
	if len(backends) != 1 {
		t.Errorf("FetchAllBackends want one backend, but got = %+v", backends)
	}
}

func TestBackendFetcher_FetchOnlineBackendByName(t *testing.T) {
	// arrange
	instance := NewBackendFetcher()

	// mock
	patches := gomonkey.ApplyFunc(pkgUtils.GetContentByClaimMeta, func(ctx context.Context,
		claimNameMeta string) (*v1.StorageBackendContent, error) {
		return &v1.StorageBackendContent{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       v1.StorageBackendContentSpec{},
			Status: &v1.StorageBackendContentStatus{
				Pools:         nil,
				Capabilities:  nil,
				Specification: nil,
				ConfigmapMeta: "",
				SecretMeta:    "",
				Online:        true,
			},
		}, nil
	})
	defer patches.Reset()

	// action
	backend, err := instance.FetchBackendByName(context.Background(), "", false)
	if err != nil {
		t.Errorf("FetchAllBackends want err is nil, but got = %v", err)
		return
	}
	if backend == nil {
		t.Error("FetchAllBackends want one backend, but not found any backend")
	}
}
