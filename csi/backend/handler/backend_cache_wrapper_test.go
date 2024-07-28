/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

// Package handler contains all helper functions with backend process
package handler

import (
	"context"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/app/config"
	"huawei-csi-driver/csi/backend"
	"huawei-csi-driver/csi/backend/model"
	"huawei-csi-driver/csi/backend/plugin"
	"huawei-csi-driver/utils/log"
)

const (
	logName = "backend-handler.log"
)

func TestMain(m *testing.M) {
	stubs := gostub.StubFunc(&app.GetGlobalConfig, config.MockCompletedConfig())
	defer stubs.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestCacheWrapper_AddBackendToCache(t *testing.T) {
	// arrange
	instance := NewCacheWrapper()
	sbct := v1.StorageBackendContent{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       v1.StorageBackendContentSpec{},
		Status:     nil,
	}

	// mock
	patches := gomonkey.ApplyFunc(backend.BuildBackend, func(context.Context,
		v1.StorageBackendContent) (*model.Backend, error) {
		return &model.Backend{Plugin: &plugin.OceanstorNasPlugin{}}, nil
	})

	defer patches.Reset()

	// action
	_, err := instance.AddBackendToCache(context.Background(), sbct)

	// assert
	if err != nil {
		t.Errorf("AddBackendToCache want err is nil, but got = %v", err)
	}
}

func TestCacheWrapper_LoadCacheBackendTopologies(t *testing.T) {
	// arrange
	instance := NewCacheWrapper()

	// action
	topologies := instance.LoadCacheBackendTopologies(context.Background(), "notFoundName")

	// assert
	if len(topologies) != 0 {
		t.Errorf("AddBackendToCache want empty, but got = %v", topologies)
	}
}

func TestCacheWrapper_LoadCacheStoragePools(t *testing.T) {
	// arrange
	instance := NewCacheWrapper()

	// action
	pools := instance.LoadCacheStoragePools(context.Background())

	// assert
	if len(pools) != 0 {
		t.Errorf("LoadCacheStoragePools want empty, but got = %v", pools)
	}
}
