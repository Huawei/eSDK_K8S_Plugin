/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package backend

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/cache"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName string = "backend_test.log"
)

var ctx = context.Background()

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	m.Run()
}

func TestAnalyzePools(t *testing.T) {
	tests := []struct {
		name      string
		backend   *model.Backend
		config    map[string]interface{}
		expectErr bool
	}{
		{"Normal",
			&model.Backend{Name: "testBackend1", Storage: "OceanStor-5000"},
			map[string]interface{}{"pools": []interface{}{"pool1", "pool2"}},
			false},
		{"NotHavePools",
			&model.Backend{Name: "testBackend1", Storage: "OceanStor-5000"},
			map[string]interface{}{"pools": []interface{}{""}},
			true},
		{"Normal9000",
			&model.Backend{Name: "testBackend1", Storage: "OceanStor-9000"},
			map[string]interface{}{"pools": []interface{}{"pool1", "pool2"}},
			false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzePools(tt.backend, tt.config); (got != nil) != tt.expectErr {
				t.Errorf("test analyzePools faild. got: %v expect: %v", got, tt.expectErr)
			}
		})
	}
}

func TestNewBackend(t *testing.T) {
	tests := []struct {
		name        string
		backendName string
		config      map[string]interface{}
		expectErr   bool
	}{
		{"Normal",
			"testBackend",
			map[string]interface{}{"storage": "oceanstor-san", "parameters": map[string]interface{}{}},
			false},
		{"storageEmpty",
			"testBackend",
			map[string]interface{}{"parameters": map[string]interface{}{}},
			true},
		{"parametersEmpty",
			"testBackend",
			map[string]interface{}{"storage": "oceanstor-san"},
			true},
		{"getTopoErr",
			"testBackend",
			map[string]interface{}{"storage": "oceanstor-san", "parameters": map[string]interface{}{},
				"supportedTopologies": "not list"},
			true},
		{"pluginNil",
			"testBackend",
			map[string]interface{}{"storage": "wrong-type", "parameters": map[string]interface{}{}},
			true},
		{"metroBackendEmpty",
			"testBackend",
			map[string]interface{}{"storage": "oceanstor-san", "parameters": map[string]interface{}{},
				"hyperMetroDomain": "testDomain"},
			true},
		{"metroDomainEmpty",
			"testBackend",
			map[string]interface{}{"storage": "oceanstor-san", "parameters": map[string]interface{}{},
				"metroBackend": "testMetroBackend"},
			true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewBackend(tt.backendName, tt.config); (err != nil) != tt.expectErr {
				t.Errorf("test NewBackend faild. err: %v expect: %v", err, tt.expectErr)
			}
		})
	}
}

func TestGetSupportedTopologies(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		expectErr bool
	}{
		{"Normal",
			map[string]interface{}{"supportedTopologies": []interface{}{
				map[string]interface{}{"Key1": "Val1", "Key2": "Val2"}}},
			false},
		{"TopoNotExist",
			map[string]interface{}{},
			false},
		{"TopoIsNotList",
			map[string]interface{}{"supportedTopologies": "testString"},
			true},
		{"TopoValIsNotMap",
			map[string]interface{}{"supportedTopologies": []interface{}{"testString"}},
			true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := getSupportedTopologies(tt.config); (err != nil) != tt.expectErr {
				t.Errorf("test getSupportedTopologies faild. err: %v expect: %v", err, tt.expectErr)
			}
		})
	}
}

func TestAddProtocolTopology(t *testing.T) {
	tests := []struct {
		name       string
		backend    *model.Backend
		driverName string
		expectErr  bool
		expect     []map[string]string
	}{
		{"Normal",
			&model.Backend{Parameters: map[string]interface{}{"protocol": "iscsi"},
				SupportedTopologies: []map[string]string{}},
			"csi.huawei.com",
			false,
			[]map[string]string{{"topology.kubernetes.io/protocol.iscsi": "csi.huawei.com"}}},
		{"NotHaveProtocol",
			&model.Backend{Parameters: map[string]interface{}{}, SupportedTopologies: []map[string]string{}},
			"csi.huawei.com",
			true,
			[]map[string]string{}},
		{"SupportedTopoNotEmpty",
			&model.Backend{Parameters: map[string]interface{}{"protocol": "iscsi"},
				SupportedTopologies: []map[string]string{{"key1": "val1"}}},
			"csi.huawei.com",
			false,
			[]map[string]string{{"key1": "val1"},
				{"topology.kubernetes.io/protocol.iscsi": "csi.huawei.com", "key1": "val1"},
				{"topology.kubernetes.io/protocol.iscsi": "csi.huawei.com"},
			}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := addProtocolTopology(tt.backend, tt.driverName)
			if (err != nil) != tt.expectErr || !reflect.DeepEqual(tt.backend.SupportedTopologies, tt.expect) {
				t.Errorf("test addProtocolTopology faild. got: %v, expect: %v, err: %v, expectErr: %v",
					tt.backend.SupportedTopologies, tt.expect, err, tt.expectErr)
			}
		})
	}
}

func TestFilterByBackendName(t *testing.T) {
	tests := []struct {
		name           string
		backendName    string
		candidatePools []*model.StoragePool
		expectErr      bool
		expect         []*model.StoragePool
	}{
		{"Normal",
			"targetBackend",
			[]*model.StoragePool{{Parent: "targetBackend"}, {Parent: "otherBackend"}},
			false,
			[]*model.StoragePool{{Parent: "targetBackend"}}},
		{"NotSpecified",
			"",
			[]*model.StoragePool{{Parent: "targetBackend"}, {Parent: "otherBackend"}},
			false,
			[]*model.StoragePool{{Parent: "targetBackend"}, {Parent: "otherBackend"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := filterByBackendName(ctx, tt.backendName, tt.candidatePools)
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("test filterByBackendName faild. got: %v, expect: %v", got, tt.expect)
			}
		})
	}
}

func TestFilterByStoragePool(t *testing.T) {
	tests := []struct {
		name           string
		poolName       string
		candidatePools []*model.StoragePool
		expect         []*model.StoragePool
	}{
		{"Normal",
			"targetPool",
			[]*model.StoragePool{{Name: "targetPool"}, {Name: "otherPool"}},
			[]*model.StoragePool{{Name: "targetPool"}}},
		{"NotSpecified",
			"",
			[]*model.StoragePool{{Name: "targetPool"}, {Name: "otherPool"}},
			[]*model.StoragePool{{Name: "targetPool"}, {Name: "otherPool"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := filterByStoragePool(ctx, tt.poolName, tt.candidatePools)
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("test filterByStoragePool faild. got: %v, expect: %v", got, tt.expect)
			}
		})
	}
}

func TestFilterByVolumeType(t *testing.T) {
	tests := []struct {
		name           string
		volumeType     string
		candidatePools []*model.StoragePool
		expect         []*model.StoragePool
	}{
		{"defaultVolumeType",
			"",
			[]*model.StoragePool{{Storage: "oceanstor-san"}, {Storage: "oceanstor-nas"},
				{Storage: "fusionstorage-san"}, {Storage: "fusionstorage-nas"}},
			[]*model.StoragePool{{Storage: "oceanstor-san"}, {Storage: "fusionstorage-san"}}},
		{"normalLun",
			"lun",
			[]*model.StoragePool{{Storage: "oceanstor-san"}, {Storage: "oceanstor-nas"}},
			[]*model.StoragePool{{Storage: "oceanstor-san"}}},
		{"normalFs",
			"fs",
			[]*model.StoragePool{{Storage: "oceanstor-san"}, {Storage: "oceanstor-nas"}},
			[]*model.StoragePool{{Storage: "oceanstor-nas"}}},
		{"oceanstor-9000",
			"fs",
			[]*model.StoragePool{{Storage: "oceanstor-san"}, {Storage: "oceanstor-nas"},
				{Storage: "oceanstor-9000"}},
			[]*model.StoragePool{{Storage: "oceanstor-nas"}, {Storage: "oceanstor-9000"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterByVolumeType(ctx, tt.volumeType, tt.candidatePools)
			if err != nil {
				t.Errorf("test filterByVolumeType faild. err: %v", err)
			}
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("test filterByVolumeType faild. got: %v, expect: %v", got, tt.expect)
			}
		})
	}
}

func TestFilterByAllocType(t *testing.T) {
	tests := []struct {
		name           string
		allocType      string
		candidatePools []*model.StoragePool
		expect         []*model.StoragePool
	}{
		{"default",
			"",
			[]*model.StoragePool{
				{Name: "pool1", Capabilities: map[string]bool{"SupportThin": true}},
				{Name: "pool2", Capabilities: map[string]bool{"SupportThin": false}}},
			[]*model.StoragePool{{Name: "pool1", Capabilities: map[string]bool{"SupportThin": true}}},
		},
		{"normalThin",
			"thin",
			[]*model.StoragePool{
				{Name: "pool1", Capabilities: map[string]bool{"SupportThin": true}},
				{Name: "pool2", Capabilities: map[string]bool{"SupportThin": false}}},
			[]*model.StoragePool{{Name: "pool1", Capabilities: map[string]bool{"SupportThin": true}}},
		},
		{"normalThick",
			"thick",
			[]*model.StoragePool{
				{Name: "pool1", Capabilities: map[string]bool{"SupportThick": true}},
				{Name: "pool2", Capabilities: map[string]bool{"SupportThick": false}}},
			[]*model.StoragePool{{Name: "pool1", Capabilities: map[string]bool{"SupportThick": true}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := filterByAllocType(ctx, tt.allocType, tt.candidatePools)
			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("test filterByAllocType faild. got: %v, expect: %v", got, tt.expect)
			}
		})
	}
}

func TestFilterByMetroNormal(t *testing.T) {
	load := gomonkey.ApplyMethod(reflect.TypeOf(&cache.BackendCache{}), "Load",
		func(_ *cache.BackendCache, backendName string) (model.Backend, bool) {
			return model.Backend{
				Name:         "testBackend1",
				MetroBackend: &model.Backend{Name: "TestMetroBackend1"}}, true
		})
	defer load.Reset()

	hyperMetro := "true"
	candidatePools := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportMetro": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]bool{"SupportMetro": false},
			Parent:       "testBackend2"}}
	expect := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportMetro": true},
			Parent:       "testBackend1"}}

	got, _ := filterByMetro(ctx, hyperMetro, candidatePools)
	if len(got) == 0 && len(expect) == 0 {
		return
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("test filterByMetro faild. got: %v, expect: %v", got, expect)
	}
}

func TestFilterByMetroNotHyperMetro(t *testing.T) {
	load := gomonkey.ApplyMethod(reflect.TypeOf(&cache.BackendCache{}), "Load",
		func(_ *cache.BackendCache, backendName string) (model.Backend, bool) {
			return model.Backend{
				Name:         "testBackend1",
				MetroBackend: &model.Backend{Name: "TestMetroBackend1"}}, true
		})
	defer load.Reset()

	hyperMetro := "false"
	candidatePools := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportMetro": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]bool{"SupportMetro": false},
			Parent:       "testBackend2"}}
	expect := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportMetro": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]bool{"SupportMetro": false},
			Parent:       "testBackend2"}}

	got, _ := filterByMetro(ctx, hyperMetro, candidatePools)
	if len(got) == 0 && len(expect) == 0 {
		return
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("test filterByMetro faild. got: %v, expect: %v", got, expect)
	}
}

func TestFilterByMetroParentNotExist(t *testing.T) {
	load := gomonkey.ApplyMethod(reflect.TypeOf(&cache.BackendCache{}), "Load",
		func(_ *cache.BackendCache, backendName string) (model.Backend, bool) {
			return model.Backend{}, false
		})
	defer load.Reset()

	hyperMetro := "true"
	candidatePools := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportMetro": true},
			Parent:       "notExist"},
		{
			Name:         "pool2",
			Capabilities: map[string]bool{"SupportMetro": false},
			Parent:       "notExist"}}
	expect := []*model.StoragePool{}

	got, _ := filterByMetro(ctx, hyperMetro, candidatePools)
	if len(got) == 0 && len(expect) == 0 {
		return
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("test filterByMetro faild. got: %v, expect: %v", got, expect)
	}
}

func TestFilterByReplicationNormal(t *testing.T) {
	load := gomonkey.ApplyMethod(reflect.TypeOf(&cache.BackendCache{}), "Load",
		func(_ *cache.BackendCache, backendName string) (model.Backend, bool) {
			return model.Backend{
				Name:           "testBackend1",
				ReplicaBackend: &model.Backend{Name: "TestMetroBackend1"}}, true
		})
	defer load.Reset()

	replication := "true"
	candidatePools := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportReplication": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]bool{"SupportReplication": false},
			Parent:       "testBackend2"}}
	expect := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportReplication": true},
			Parent:       "testBackend1"}}

	got, _ := filterByReplication(ctx, replication, candidatePools)
	if len(got) == 0 && len(expect) == 0 {
		return
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("test filterByReplication faild. got: %v, expect: %v", got, expect)
	}
}

func TestFilterByReplicationWithFalse(t *testing.T) {
	load := gomonkey.ApplyMethod(reflect.TypeOf(&cache.BackendCache{}), "Load",
		func(_ *cache.BackendCache, backendName string) (model.Backend, bool) {
			return model.Backend{
				Name:         "testBackend1",
				MetroBackend: &model.Backend{Name: "TestMetroBackend1"}}, true
		})
	defer load.Reset()

	replication := "false"
	candidatePools := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportReplication": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]bool{"SupportReplication": false},
			Parent:       "testBackend2"}}
	expect := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportReplication": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]bool{"SupportReplication": false},
			Parent:       "testBackend2"}}

	got, _ := filterByReplication(ctx, replication, candidatePools)
	if len(got) == 0 && len(expect) == 0 {
		return
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("test filterByReplication faild. got: %v, expect: %v", got, expect)
	}
}

func TestFilterByReplicationParentNotExist(t *testing.T) {
	load := gomonkey.ApplyMethod(reflect.TypeOf(&cache.BackendCache{}), "Load",
		func(_ *cache.BackendCache, backendName string) (model.Backend, bool) {
			return model.Backend{
				Name:         "testBackend1",
				MetroBackend: &model.Backend{Name: "TestMetroBackend1"}}, true
		})
	defer load.Reset()

	replication := "true"
	candidatePools := []*model.StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]bool{"SupportReplication": true},
			Parent:       "notExist"},
		{
			Name:         "pool2",
			Capabilities: map[string]bool{"SupportReplication": false},
			Parent:       "notExist"}}
	expect := []*model.StoragePool{}

	got, _ := filterByReplication(ctx, replication, candidatePools)
	if len(got) == 0 && len(expect) == 0 {
		return
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("test filterByReplication faild. got: %v, expect: %v", got, expect)
	}
}

func TestFilterByNFSProtocol(t *testing.T) {
	tests := []struct {
		name           string
		nfsProtocol    string
		candidatePools []*model.StoragePool
		expect         int64
	}{
		{"Normal",
			"nfs3",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportNFS3": true}}},
			1},
		{"NormalMulti",
			"nfs4",
			[]*model.StoragePool{
				{Capabilities: map[string]bool{"SupportNFS4": true}},
				{Capabilities: map[string]bool{"SupportNFS4": true}}},
			2},
		{"NFS41NotSupport",
			"nfs41",
			[]*model.StoragePool{
				{Capabilities: map[string]bool{"SupportNFS41": true}},
				{Capabilities: map[string]bool{"SupportNFS41": false}}},
			1},
		{"ProtocolEmpty",
			"",
			nil,
			0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := filterByNFSProtocol(ctx, tt.nfsProtocol, tt.candidatePools); int64(len(got)) != tt.expect {
				t.Errorf("test filterByNFSProtocol faild. got: %v expect: %v", len(got), tt.expect)
			}
		})
	}
}

func TestFilterBySupportClone(t *testing.T) {
	tests := []struct {
		name           string
		cloneSource    string
		candidatePools []*model.StoragePool
		expect         int64
	}{
		{"Normal",
			"source",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportClone": true}}},
			1},
		{"NormalMulti",
			"source",
			[]*model.StoragePool{
				{Capabilities: map[string]bool{"SupportClone": true}},
				{Capabilities: map[string]bool{"SupportClone": true}}},
			2},
		{"HasNotSupportClone",
			"source",
			[]*model.StoragePool{
				{Capabilities: map[string]bool{"SupportClone": true}},
				{Capabilities: map[string]bool{"SupportClone": false}}},
			1},
		{"AllNotSupportClone",
			"source",
			[]*model.StoragePool{
				{Capabilities: map[string]bool{"SupportClone": false}},
				{Capabilities: map[string]bool{"SupportClone": false}},
				{Capabilities: map[string]bool{"SupportClone": false}}},
			0},
		{"cloneSourceEmpty",
			"",
			nil,
			0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := filterBySupportClone(ctx, tt.cloneSource, tt.candidatePools); int64(len(got)) != tt.expect {
				t.Errorf("test filterBySupportClone faild. got: %v expect: %v", len(got), tt.expect)
			}
		})
	}
}

func TestFilterByCapacity(t *testing.T) {
	tests := []struct {
		name           string
		requestSize    int64
		allocType      string
		candidatePools []*model.StoragePool
		expect         int64
	}{
		{"NormalThin",
			1024,
			"thin",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportThin": true}},
				{Capabilities: map[string]bool{"SupportThin": true}}}, 2},
		{"NormalThick",
			1024,
			"thick",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportThick": true},
				Capacities: map[string]string{"FreeCapacity": "1025"},
			}},
			1},
		{"NormalThinIsEmpty",
			1024,
			"",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportThin": true}}},
			1},
		{"NotHasSupportThinParam",
			1024,
			"thin",
			[]*model.StoragePool{{Capabilities: map[string]bool{}}},
			0},
		{"NotSupportThin",
			1024,
			"thin",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportThin": false}}},
			0},
		{"SizeInsufficient",
			1024,
			"thick",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportThick": true},
				Capacities: map[string]string{"FreeCapacity": "1023"}}}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterByCapacity(tt.requestSize, tt.allocType, tt.candidatePools); int64(len(got)) != tt.expect {
				t.Errorf("test FilterByCapacity faild. got: %v expect: %v", len(got), tt.expect)
			}
		})
	}
}

func TestWeightByFreeCapacity(t *testing.T) {
	tests := []struct {
		name           string
		candidatePools []*model.StoragePool
		expect         *model.StoragePool
	}{
		{"Normal",
			[]*model.StoragePool{{Capacities: map[string]string{"FreeCapacity": "1024"}}},
			&model.StoragePool{Capacities: map[string]string{"FreeCapacity": "1024"}},
		},
		{"NormalMulti",
			[]*model.StoragePool{{Capacities: map[string]string{"FreeCapacity": "1024"}},
				{Capacities: map[string]string{"FreeCapacity": "4096"}},
				{Capacities: map[string]string{"FreeCapacity": "2048"}}},
			&model.StoragePool{Capacities: map[string]string{"FreeCapacity": "4096"}},
		},
		{
			"InputNil",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weightByFreeCapacity(tt.candidatePools); !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("test weightByFreeCapacity faild. got: %v expect: %v", got, tt.expect)
			}
		})
	}
}

func TestFilterByApplicationType(t *testing.T) {
	tests := []struct {
		name           string
		appType        string
		candidatePools []*model.StoragePool
		expect         int64
	}{
		{"Normal",
			"SQL_Server_OLAP",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportApplicationType": true}}},
			1,
		},
		{"NormalMulti",
			"SQL_Server_OLAP",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportApplicationType": true}},
				{Capabilities: map[string]bool{"SupportApplicationType": true}},
				{Capabilities: map[string]bool{"SupportApplicationType": true}}},
			3,
		},
		{
			"AppTypeEmpty",
			"",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportApplicationType": true}},
				{Capabilities: map[string]bool{"SupportApplicationType": false}},
				{Capabilities: map[string]bool{"SupportApplicationType": false}}},
			3,
		},
		{
			"SomeNotSupport",
			"SQL_Server_OLAP",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportApplicationType": false}},
				{Capabilities: map[string]bool{"SupportApplicationType": true}},
				{Capabilities: map[string]bool{"SupportApplicationType": false}}},
			1,
		},
		{
			"AllNotSupport",
			"SQL_Server_OLAP",
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportApplicationType": false}},
				{Capabilities: map[string]bool{"SupportApplicationType": false}},
				{Capabilities: map[string]bool{"SupportApplicationType": false}}},
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := filterByApplicationType(ctx, tt.appType, tt.candidatePools); int64(len(got)) != tt.expect {
				t.Errorf("test filterByApplicationType faild. got: %v expect: %v", got, tt.expect)
			}
		})
	}
}

func TestFilterByStorageQuota(t *testing.T) {
	tests := []struct {
		name           string
		storageQuota   string
		candidatePools []*model.StoragePool
		expect         int64
		expectErr      bool
	}{
		{"NormalSoftQuota",
			`{"spaceQuota": "softQuota", "gracePeriod": 100}`,
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportQuota": true}}},
			1,
			false,
		},
		{"NormalHardQuota",
			`{"spaceQuota": "hardQuota"}`,
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportQuota": true}}},
			1,
			false,
		},
		{"NegativePeriod",
			`{"spaceQuota": "hardQuota", "gracePeriod": -1}`,
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportQuota": true}}},
			0,
			true,
		},
		{"ExceedsTheMaximumPeriod",
			`{"spaceQuota": "hardQuota", "gracePeriod": 4294967295}`,
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportQuota": true}}},
			0,
			true,
		},
		{"WrongType",
			`{"spaceQuota": "WrongType"`,
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportQuota": true}}},
			0,
			true,
		},
		{"HardWithPeriod",
			`{"spaceQuota": "hardQuota", "gracePeriod": 10}`,
			[]*model.StoragePool{{Capabilities: map[string]bool{"SupportQuota": true}}},
			0,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterByStorageQuota(ctx, tt.storageQuota, tt.candidatePools)
			if int64(len(got)) != tt.expect || (err != nil) != tt.expectErr {
				t.Errorf("test filterByStorageQuota faild. got: %v expect: %v", got, tt.expect)
			}
		})
	}
}

func mockStoragePool(features ...string) []*model.StoragePool {
	var pools []*model.StoragePool
	for i, feature := range features {
		pool := &model.StoragePool{
			Name:         fmt.Sprintf("pool-%d", i),
			Capabilities: map[string]bool{feature: true},
			Storage:      "oceanstor-nas",
		}
		pools = append(pools, pool)
	}

	return pools
}

func mockStorageBackend(storage string, pool []*model.StoragePool) *model.Backend {
	return &model.Backend{
		Name:       "mock-backend",
		Storage:    storage,
		Available:  true,
		Plugin:     nil,
		Pools:      pool,
		Parameters: nil,
	}
}

func TestInValidBackendName(t *testing.T) {
	features := []string{"SupportThin", "SupportQoS"}
	mockBackend := mockStorageBackend("oceanstor-nas", mockStoragePool(features...))
	parameters := map[string]interface{}{"backend": "fake-backend"}
	err := ValidateBackend(ctx, mockBackend, parameters)
	if err == nil {
		t.Error("test inValidBackendName error")
	}
}

func TestInValidVolumeType(t *testing.T) {
	features := []string{"SupportThin", "SupportQoS"}
	mockBackend := mockStorageBackend("oceanstor-nas", mockStoragePool(features...))
	parameters := map[string]interface{}{"volumeType": "lun"}
	err := ValidateBackend(ctx, mockBackend, parameters)
	if err == nil {
		t.Error("test inValidVolumeType error")
	}
}

func TestValidateBackend(t *testing.T) {
	features := []string{"SupportThin", "SupportQoS"}
	mockBackend := mockStorageBackend("oceanstor-nas", mockStoragePool(features...))
	parameters := map[string]interface{}{"volumeType": "fs"}
	err := ValidateBackend(ctx, mockBackend, parameters)
	if err != nil {
		t.Errorf("test validateBackend error %v", err)
	}
}

func TestUpdateSelectPool(t *testing.T) {
	// arrange
	var (
		requestSize = int64(16106127360)
		parameters  = map[string]any{"allocType": "thick"}
		selectPool  = &model.StoragePool{Capacities: map[string]string{"FreeCapacity": "1138971639808"}}

		expectedCapacity = "1122865512448" // 1138971639808 - 16106127360
	)

	// act
	updateSelectPool(requestSize, parameters, selectPool)

	// assert
	require.Equal(t, expectedCapacity, selectPool.Capacities["FreeCapacity"])
}
