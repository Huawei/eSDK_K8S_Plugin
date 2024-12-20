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
// package for backend cache test
package cache

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/prashantv/gostub"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "backend.cache.log"
	Base10  = 10
)

func TestMain(m *testing.M) {
	stubs := gostub.StubFunc(&app.GetGlobalConfig, config.MockCompletedConfig())
	defer stubs.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

// TestBackendCache_Store test function for store
func TestBackendCache_Store(t *testing.T) {
	// arrange
	ctx := context.TODO()
	var tests = []struct {
		backend model.Backend
	}{
		{
			backend: model.Backend{
				Name:    "backend1",
				Storage: "storage1",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend2",
				Storage: "storage2",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend3",
				Storage: "storage3",
			},
		},
	}

	// act
	for _, tt := range tests {
		BackendCacheProvider.Store(ctx, tt.backend.Name, tt.backend)
	}

	// assert
	for _, tt := range tests {
		newBackend, exists := BackendCacheProvider.Load(tt.backend.Name)
		if !exists {
			t.Errorf("TestBackendCache_Clear failed, want exists: true, but got %v", exists)
		}
		if newBackend.Name != tt.backend.Name || newBackend.Storage != tt.backend.Storage {
			t.Errorf("TestBackendCache_Store failed, wantBackend: [%+v], but find backend :[%+v]",
				tt.backend, newBackend)
		}
	}

	newBackendCount := BackendCacheProvider.Count()
	if len(tests) != newBackendCount {
		t.Errorf("TestBackendCache_Store failed, wantBackendCount: [%v], but find backendCount :[%v]",
			len(tests), newBackendCount)
	}

	// cleanup
	BackendCacheProvider.Clear(ctx)
}

// TestBackendCache_Load test function for load
func TestBackendCache_Load(t *testing.T) {
	// arrange
	ctx := context.TODO()
	backend := model.Backend{
		Name:    "backend" + strconv.FormatInt(rand.Int63n(time.Now().Unix()), Base10),
		Storage: "backend" + strconv.FormatInt(rand.Int63n(time.Now().Unix()), Base10),
	}

	// act
	BackendCacheProvider.Store(ctx, backend.Name, backend)

	// assert
	newBackend, exists := BackendCacheProvider.Load(backend.Name)
	if !exists {
		t.Errorf("TestBackendCache_Clear failed, want exists: true, but got %v", exists)
	}
	if backend.Name != newBackend.Name || backend.Storage != newBackend.Storage {
		t.Errorf("TestBackendCache_Load failed, wantBackend: [%+v], but find backend :[%+v]", backend, newBackend)
	}

	// cleanup
	BackendCacheProvider.Clear(ctx)
}

// TestBackendCache_Delete test function for delete
func TestBackendCache_Delete(t *testing.T) {
	// arrange
	ctx := context.TODO()
	var tests = []struct {
		backend model.Backend
	}{
		{
			backend: model.Backend{
				Name:    "backend1",
				Storage: "storage1",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend2",
				Storage: "storage2",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend3",
				Storage: "storage3",
			},
		},
	}

	// act
	for _, tt := range tests {
		BackendCacheProvider.Store(ctx, tt.backend.Name, tt.backend)
	}

	BackendCacheProvider.Delete(ctx, tests[0].backend.Name)

	// assert
	for idx, tt := range tests {
		newBackend, exists := BackendCacheProvider.Load(tt.backend.Name)
		if idx == 0 {
			if exists {
				t.Errorf("TestBackendCache_Delete failed, wantBackend: [%+v], but find backend :[%+v]",
					nil, newBackend)
			}
			continue
		}
		if newBackend.Name != tt.backend.Name || newBackend.Storage != tt.backend.Storage {
			t.Errorf("TestBackendCache_Delete failed, wantBackend: [%+v], but find backend :[%+v]",
				tt.backend, newBackend)
		}
	}

	newBackendCount := BackendCacheProvider.Count()
	if len(tests)-1 != newBackendCount {
		t.Errorf("TestBackendCache_Delete failed, wantBackendCount: [%v], but find backendCount :[%v]",
			len(tests)-1, newBackendCount)
	}

	// cleanup
	BackendCacheProvider.Clear(ctx)
}

// TestBackendCache_Clear test function for clear
func TestBackendCache_Clear(t *testing.T) {
	// arrange
	ctx := context.TODO()
	var tests = []struct {
		backend model.Backend
	}{
		{
			backend: model.Backend{
				Name:    "backend1",
				Storage: "storage1",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend2",
				Storage: "storage2",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend3",
				Storage: "storage3",
			},
		},
	}

	// act
	for _, tt := range tests {
		BackendCacheProvider.Store(ctx, tt.backend.Name, tt.backend)
	}

	// assert
	for _, tt := range tests {
		newBackend, exists := BackendCacheProvider.Load(tt.backend.Name)
		if !exists {
			t.Errorf("TestBackendCache_Clear failed, want exists: true, but got %v", exists)
		}
		if newBackend.Name != tt.backend.Name || newBackend.Storage != tt.backend.Storage {
			t.Errorf("TestBackendCache_Clear failed, wantBackend: [%+v], but find backend :[%+v]",
				tt.backend, newBackend)
		}
	}

	// act
	BackendCacheProvider.Clear(ctx)

	// assert
	backendCount := BackendCacheProvider.Count()
	if backendCount != 0 {
		t.Errorf("TestBackendCache_Clear failed, wantBackendCount: [%v], but find backend count:[%v]",
			0, backendCount)
	}

	// cleanup
	BackendCacheProvider.Clear(ctx)
}

// TestBackendCache_List test function for list
func TestBackendCache_List(t *testing.T) {
	// arrange
	ctx := context.TODO()
	BackendCacheProvider.Clear(ctx)
	var tests = []struct {
		backend model.Backend
	}{
		{
			backend: model.Backend{
				Name:    "backend1",
				Storage: "storage1",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend2",
				Storage: "storage2",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend3",
				Storage: "storage3",
			},
		},
	}

	// act
	for _, tt := range tests {
		BackendCacheProvider.Store(ctx, tt.backend.Name, tt.backend)
	}

	backendList := BackendCacheProvider.List(ctx)

	// assert
	if len(backendList) != len(tests) {
		t.Errorf("TestBackendCache_List failed, wantBackendCount: [%v], but find backend count :[%v]",
			len(tests), len(backendList))
	}
	var matchBackendCount int
	for _, backend := range backendList {
		for _, test := range tests {
			if backend.Name == test.backend.Name {
				matchBackendCount++
			}
			if backend.Name == test.backend.Name && backend.Storage != test.backend.Storage {
				t.Errorf("TestBackendCache_List failed, wantBackend: [%+v], but find backend  :[%+v]",
					test.backend, backend)
			}
		}
	}
	if matchBackendCount != len(tests) {
		t.Errorf("TestBackendCache_List failed, want match backend count:[%v], actual match backend count:[%v]",
			len(tests), matchBackendCount)
	}

	// cleanup
	BackendCacheProvider.Clear(ctx)
}

// TestBackendCache_Count test function for count
func TestBackendCache_Count(t *testing.T) {
	// arrange
	ctx := context.TODO()
	var tests = []struct {
		backend model.Backend
	}{
		{
			backend: model.Backend{
				Name:    "backend1",
				Storage: "storage1",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend2",
				Storage: "storage2",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend3",
				Storage: "storage3",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend4",
				Storage: "storage4",
			},
		},
		{
			backend: model.Backend{
				Name:    "backend5",
				Storage: "storage5",
			},
		},
	}

	// act
	for _, test := range tests {
		BackendCacheProvider.Store(ctx, test.backend.Name, test.backend)
	}
	backendCount := BackendCacheProvider.Count()

	// assert
	if backendCount != len(tests) {
		t.Errorf("TestBackendCache_Count failed, want match backend count:[%v], actual match backend count:[%v]",
			len(tests), backendCount)
	}

	// clear
	BackendCacheProvider.Clear(ctx)
}
