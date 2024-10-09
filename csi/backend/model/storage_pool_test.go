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

// package for backend model test
package model

import (
	"context"
	"reflect"
	"testing"

	"github.com/prashantv/gostub"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/csi/app/config"
	"huawei-csi-driver/utils/log"
)

const (
	logName = "storage.pool.log"
)

func TestMain(m *testing.M) {
	stubs := gostub.StubFunc(&app.GetGlobalConfig, config.MockCompletedConfig())
	defer stubs.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

// TestStoragePool_UpdatePoolBySBCT test
func TestStoragePool_UpdatePoolBySBCT(t *testing.T) {
	// arrange
	ctx := context.Background()
	pool := &StoragePool{
		Name:   "pool1",
		Parent: "backend1",
	}
	capacities := map[string]string{
		string(xuanwuv1.FreeCapacity):  "1",
		string(xuanwuv1.UsedCapacity):  "2",
		string(xuanwuv1.TotalCapacity): "3",
	}
	content := &xuanwuv1.StorageBackendContent{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec:       xuanwuv1.StorageBackendContentSpec{},
		Status: &xuanwuv1.StorageBackendContentStatus{
			ContentName:     "",
			VendorName:      "",
			ProviderVersion: "",
			Pools: []xuanwuv1.Pool{
				{
					Name:       "pool1",
					Capacities: capacities,
				},
			},
			Capabilities: map[string]bool{
				"SupportThin":  true,
				"SupportThick": false,
			},
		},
	}

	// act
	pool.UpdatePoolBySBCT(ctx, content)

	// assert
	if !reflect.DeepEqual(pool.GetCapacities(), content.Status.Pools[0].Capacities) {
		t.Errorf("TestStoragePool_UpdatePoolBySBCT failed, pool: %+v, wantCapacities: %+v, "+
			"acutallCapacities: %+v", pool, content.Status.Pools[0].Capacities, pool.GetCapacities())
	}
	if !reflect.DeepEqual(pool.GetCapabilities(), content.Status.Capabilities) {
		t.Errorf("TestStoragePool_UpdatePoolBySBCT failed, pool: %+v, wantCapabilities: %+v,"+
			" acutallCapabilities: %+v", pool, content.Status.Capabilities, pool.GetCapabilities())
	}
}

// TestStoragePool_ChangeCapacities test
func TestStoragePool_ChangeCapacities(t *testing.T) {
	// arrange
	ctx := context.Background()
	pool := &StoragePool{
		Name:         "pool1",
		Storage:      "",
		Parent:       "backend1",
		Capabilities: nil,
		Capacities:   nil,
		Plugin:       nil,
	}
	poolCapacities := map[string]string{
		string(xuanwuv1.FreeCapacity):  "1",
		string(xuanwuv1.UsedCapacity):  "2",
		string(xuanwuv1.TotalCapacity): "3",
	}

	// act
	pool.UpdateCapacities(ctx, poolCapacities)

	// assert
	if !reflect.DeepEqual(pool.GetCapacities(), poolCapacities) {
		t.Errorf("TestStoragePool_UpdatePoolCapabilities failed, pool: %+v, wantCapacities: %+v, "+
			"acutallCapacities: %+v", pool, poolCapacities, pool.GetCapacities())
	}
}

// TestStoragePool_UpdatePoolCapabilities test
func TestStoragePool_UpdatePoolCapabilities(t *testing.T) {
	// arrange
	ctx := context.Background()
	pool := &StoragePool{
		Name:         "pool1",
		Storage:      "",
		Parent:       "backend1",
		Capabilities: nil,
		Capacities: map[string]string{
			string(xuanwuv1.FreeCapacity):  "1",
			string(xuanwuv1.UsedCapacity):  "2",
			string(xuanwuv1.TotalCapacity): "3",
		},
		Plugin: nil,
	}

	backendCapabilities := map[string]bool{
		"SupportThin":            true,
		"SupportThick":           false,
		"SupportQoS":             false,
		"SupportMetro":           true,
		"SupportReplication":     false,
		"SupportApplicationType": true,
		"SupportClone":           true,
		"SupportMetroNAS":        false,
	}

	// act
	pool.UpdateCapabilities(ctx, backendCapabilities)

	// assert
	if !reflect.DeepEqual(pool.GetCapabilities(), backendCapabilities) {
		t.Errorf("TestStoragePool_UpdatePoolCapabilities failed, pool: %+v, wantCapabilities: %+v,"+
			" acutallCapabilities: %+v", pool, backendCapabilities, pool.GetCapabilities())
	}
}
