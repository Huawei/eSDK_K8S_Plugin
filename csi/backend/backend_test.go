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
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	. "github.com/smartystreets/goconvey/convey"

	xuanwuv1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/csi/app"
	cfg "huawei-csi-driver/csi/app/config"
	clientSet "huawei-csi-driver/pkg/client/clientset/versioned"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils/log"
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
		backend   *Backend
		config    map[string]interface{}
		expectErr bool
	}{
		{"Normal",
			&Backend{Name: "testBackend1", Storage: "OceanStor-5000"},
			map[string]interface{}{"pools": []interface{}{"pool1", "pool2"}},
			false},
		{"NotHavePools",
			&Backend{Name: "testBackend1", Storage: "OceanStor-5000"},
			map[string]interface{}{"pools": []interface{}{""}},
			true},
		{"Normal9000",
			&Backend{Name: "testBackend1", Storage: "OceanStor-9000"},
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
		backend    *Backend
		driverName string
		expectErr  bool
		expect     []map[string]string
	}{
		{"Normal",
			&Backend{Parameters: map[string]interface{}{"protocol": "iscsi"},
				SupportedTopologies: []map[string]string{}},
			"csi.huawei.com",
			false,
			[]map[string]string{{"topology.kubernetes.io/protocol.iscsi": "csi.huawei.com"}}},
		{"NotHaveProtocol",
			&Backend{Parameters: map[string]interface{}{}, SupportedTopologies: []map[string]string{}},
			"csi.huawei.com",
			true,
			[]map[string]string{}},
		{"SupportedTopoNotEmpty",
			&Backend{Parameters: map[string]interface{}{"protocol": "iscsi"},
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
		candidatePools []*StoragePool
		expectErr      bool
		expect         []*StoragePool
	}{
		{"Normal",
			"targetBackend",
			[]*StoragePool{{Parent: "targetBackend"}, {Parent: "otherBackend"}},
			false,
			[]*StoragePool{{Parent: "targetBackend"}}},
		{"NotSpecified",
			"",
			[]*StoragePool{{Parent: "targetBackend"}, {Parent: "otherBackend"}},
			false,
			[]*StoragePool{{Parent: "targetBackend"}, {Parent: "otherBackend"}},
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
		candidatePools []*StoragePool
		expect         []*StoragePool
	}{
		{"Normal",
			"targetPool",
			[]*StoragePool{{Name: "targetPool"}, {Name: "otherPool"}},
			[]*StoragePool{{Name: "targetPool"}}},
		{"NotSpecified",
			"",
			[]*StoragePool{{Name: "targetPool"}, {Name: "otherPool"}},
			[]*StoragePool{{Name: "targetPool"}, {Name: "otherPool"}},
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
		candidatePools []*StoragePool
		expect         []*StoragePool
	}{
		{"defaultVolumeType",
			"",
			[]*StoragePool{{Storage: "oceanstor-san"}, {Storage: "oceanstor-nas"},
				{Storage: "fusionstorage-san"}, {Storage: "fusionstorage-nas"}},
			[]*StoragePool{{Storage: "oceanstor-san"}, {Storage: "fusionstorage-san"}}},
		{"normalLun",
			"lun",
			[]*StoragePool{{Storage: "oceanstor-san"}, {Storage: "oceanstor-nas"}},
			[]*StoragePool{{Storage: "oceanstor-san"}}},
		{"normalFs",
			"fs",
			[]*StoragePool{{Storage: "oceanstor-san"}, {Storage: "oceanstor-nas"}},
			[]*StoragePool{{Storage: "oceanstor-nas"}}},
		{"oceanstor-9000",
			"fs",
			[]*StoragePool{{Storage: "oceanstor-san"}, {Storage: "oceanstor-nas"}, {Storage: "oceanstor-9000"}},
			[]*StoragePool{{Storage: "oceanstor-nas"}, {Storage: "oceanstor-9000"}}},
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
		candidatePools []*StoragePool
		expect         []*StoragePool
	}{
		{"default",
			"",
			[]*StoragePool{
				{Name: "pool1", Capabilities: map[string]interface{}{"SupportThin": true}},
				{Name: "pool2", Capabilities: map[string]interface{}{"SupportThin": false}}},
			[]*StoragePool{{Name: "pool1", Capabilities: map[string]interface{}{"SupportThin": true}}},
		},
		{"normalThin",
			"thin",
			[]*StoragePool{
				{Name: "pool1", Capabilities: map[string]interface{}{"SupportThin": true}},
				{Name: "pool2", Capabilities: map[string]interface{}{"SupportThin": false}}},
			[]*StoragePool{{Name: "pool1", Capabilities: map[string]interface{}{"SupportThin": true}}},
		},
		{"normalThick",
			"thick",
			[]*StoragePool{
				{Name: "pool1", Capabilities: map[string]interface{}{"SupportThick": true}},
				{Name: "pool2", Capabilities: map[string]interface{}{"SupportThick": false}}},
			[]*StoragePool{{Name: "pool1", Capabilities: map[string]interface{}{"SupportThick": true}}},
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
	stub := gostub.Stub(&csiBackends, map[string]*Backend{"testBackend1": {
		Name:         "testBackend1",
		MetroBackend: &Backend{Name: "TestMetroBackend1"}}})
	defer stub.Reset()

	hyperMetro := "true"
	candidatePools := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportMetro": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]interface{}{"SupportMetro": false},
			Parent:       "testBackend2"}}
	expect := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportMetro": true},
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
	stub := gostub.Stub(&csiBackends, map[string]*Backend{"testBackend1": {
		Name:         "testBackend1",
		MetroBackend: &Backend{Name: "TestMetroBackend1"}}})
	defer stub.Reset()

	hyperMetro := "false"
	candidatePools := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportMetro": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]interface{}{"SupportMetro": false},
			Parent:       "testBackend2"}}
	expect := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportMetro": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]interface{}{"SupportMetro": false},
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
	stub := gostub.Stub(&csiBackends, map[string]*Backend{"testBackend1": {
		Name:         "testBackend1",
		MetroBackend: &Backend{Name: "TestMetroBackend1"}}})
	defer stub.Reset()

	hyperMetro := "true"
	candidatePools := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportMetro": true},
			Parent:       "notExist"},
		{
			Name:         "pool2",
			Capabilities: map[string]interface{}{"SupportMetro": false},
			Parent:       "notExist"}}
	expect := []*StoragePool{}

	got, _ := filterByMetro(ctx, hyperMetro, candidatePools)
	if len(got) == 0 && len(expect) == 0 {
		return
	}
	if !reflect.DeepEqual(got, expect) {
		t.Errorf("test filterByMetro faild. got: %v, expect: %v", got, expect)
	}
}

func TestFilterByReplicationNormal(t *testing.T) {
	stub := gostub.Stub(&csiBackends, map[string]*Backend{"testBackend1": {
		Name:           "testBackend1",
		ReplicaBackend: &Backend{Name: "TestMetroBackend1"}}})
	defer stub.Reset()

	replication := "true"
	candidatePools := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportReplication": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]interface{}{"SupportReplication": false},
			Parent:       "testBackend2"}}
	expect := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportReplication": true},
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
	stub := gostub.Stub(&csiBackends, map[string]*Backend{"testBackend1": {
		Name:           "testBackend1",
		ReplicaBackend: &Backend{Name: "TestMetroBackend1"}}})
	defer stub.Reset()

	replication := "false"
	candidatePools := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportReplication": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]interface{}{"SupportReplication": false},
			Parent:       "testBackend2"}}
	expect := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportReplication": true},
			Parent:       "testBackend1"},
		{
			Name:         "pool2",
			Capabilities: map[string]interface{}{"SupportReplication": false},
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
	stub := gostub.Stub(&csiBackends, map[string]*Backend{"testBackend1": {
		Name:           "testBackend1",
		ReplicaBackend: &Backend{Name: "TestMetroBackend1"}}})
	defer stub.Reset()

	replication := "true"
	candidatePools := []*StoragePool{
		{
			Name:         "pool1",
			Capabilities: map[string]interface{}{"SupportReplication": true},
			Parent:       "notExist"},
		{
			Name:         "pool2",
			Capabilities: map[string]interface{}{"SupportReplication": false},
			Parent:       "notExist"}}
	expect := []*StoragePool{}

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
		candidatePools []*StoragePool
		expect         int64
	}{
		{"Normal",
			"nfs3",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportNFS3": true}}},
			1},
		{"NormalMulti",
			"nfs4",
			[]*StoragePool{
				{Capabilities: map[string]interface{}{"SupportNFS4": true}},
				{Capabilities: map[string]interface{}{"SupportNFS4": true}}},
			2},
		{"NFS41NotSupport",
			"nfs41",
			[]*StoragePool{
				{Capabilities: map[string]interface{}{"SupportNFS41": true}},
				{Capabilities: map[string]interface{}{"SupportNFS41": false}}},
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
		candidatePools []*StoragePool
		expect         int64
	}{
		{"Normal",
			"source",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportClone": true}}},
			1},
		{"NormalMulti",
			"source",
			[]*StoragePool{
				{Capabilities: map[string]interface{}{"SupportClone": true}},
				{Capabilities: map[string]interface{}{"SupportClone": true}}},
			2},
		{"HasNotSupportClone",
			"source",
			[]*StoragePool{
				{Capabilities: map[string]interface{}{"SupportClone": true}},
				{Capabilities: map[string]interface{}{"SupportClone": false}}},
			1},
		{"AllNotSupportClone",
			"source",
			[]*StoragePool{
				{Capabilities: map[string]interface{}{"SupportClone": false}},
				{Capabilities: map[string]interface{}{"SupportClone": false}},
				{Capabilities: map[string]interface{}{"SupportClone": false}}},
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
		candidatePools []*StoragePool
		expect         int64
	}{
		{"NormalThin",
			1024,
			"thin",
			[]*StoragePool{
				{Capabilities: map[string]interface{}{"SupportThin": true}},
				{Capabilities: map[string]interface{}{"SupportThin": true}}},
			2},
		{"NormalThick",
			1024,
			"thick",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportThick": true, "FreeCapacity": int64(1025)}}},
			1},
		{"NormalThinIsEmpty",
			1024,
			"",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportThin": true}}},
			1},
		{"NotHasSupportThinParam",
			1024,
			"thin",
			[]*StoragePool{{Capabilities: map[string]interface{}{}}},
			0},
		{"NotSupportThin",
			1024,
			"thin",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportThin": false}}},
			0},
		{"SizeInsufficient",
			1024,
			"thick",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportThick": true, "FreeCapacity": int64(1023)}}},
			0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterByCapacity(tt.requestSize, tt.allocType, tt.candidatePools); int64(len(got)) != tt.expect {
				t.Errorf("test filterByCapacity faild. got: %v expect: %v", len(got), tt.expect)
			}
		})
	}
}

func TestWeightByFreeCapacity(t *testing.T) {
	tests := []struct {
		name           string
		candidatePools []*StoragePool
		expect         *StoragePool
	}{
		{"Normal",
			[]*StoragePool{{Capabilities: map[string]interface{}{"FreeCapacity": int64(1024)}}},
			&StoragePool{Capabilities: map[string]interface{}{"FreeCapacity": int64(1024)}},
		},
		{"NormalMulti",
			[]*StoragePool{{Capabilities: map[string]interface{}{"FreeCapacity": int64(1024)}},
				{Capabilities: map[string]interface{}{"FreeCapacity": int64(4096)}},
				{Capabilities: map[string]interface{}{"FreeCapacity": int64(2048)}}},
			&StoragePool{Capabilities: map[string]interface{}{"FreeCapacity": int64(4096)}},
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
		candidatePools []*StoragePool
		expect         int64
	}{
		{"Normal",
			"SQL_Server_OLAP",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportApplicationType": true}}},
			1,
		},
		{"NormalMulti",
			"SQL_Server_OLAP",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportApplicationType": true}},
				{Capabilities: map[string]interface{}{"SupportApplicationType": true}},
				{Capabilities: map[string]interface{}{"SupportApplicationType": true}}},
			3,
		},
		{
			"AppTypeEmpty",
			"",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportApplicationType": true}},
				{Capabilities: map[string]interface{}{"SupportApplicationType": false}},
				{Capabilities: map[string]interface{}{"SupportApplicationType": false}}},
			3,
		},
		{
			"SomeNotSupport",
			"SQL_Server_OLAP",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportApplicationType": false}},
				{Capabilities: map[string]interface{}{"SupportApplicationType": true}},
				{Capabilities: map[string]interface{}{"SupportApplicationType": false}}},
			1,
		},
		{
			"AllNotSupport",
			"SQL_Server_OLAP",
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportApplicationType": false}},
				{Capabilities: map[string]interface{}{"SupportApplicationType": false}},
				{Capabilities: map[string]interface{}{"SupportApplicationType": false}}},
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
		candidatePools []*StoragePool
		expect         int64
		expectErr      bool
	}{
		{"NormalSoftQuota",
			`{"spaceQuota": "softQuota", "gracePeriod": 100}`,
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportQuota": true}}},
			1,
			false,
		},
		{"NormalHardQuota",
			`{"spaceQuota": "hardQuota"}`,
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportQuota": true}}},
			1,
			false,
		},
		{"NegativePeriod",
			`{"spaceQuota": "hardQuota", "gracePeriod": -1}`,
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportQuota": true}}},
			0,
			true,
		},
		{"ExceedsTheMaximumPeriod",
			`{"spaceQuota": "hardQuota", "gracePeriod": 4294967295}`,
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportQuota": true}}},
			0,
			true,
		},
		{"WrongType",
			`{"spaceQuota": "WrongType"`,
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportQuota": true}}},
			0,
			true,
		},
		{"HardWithPeriod",
			`{"spaceQuota": "hardQuota", "gracePeriod": 10}`,
			[]*StoragePool{{Capabilities: map[string]interface{}{"SupportQuota": true}}},
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

func mockStoragePool(features ...string) []*StoragePool {
	var pools []*StoragePool
	for i, feature := range features {
		pool := &StoragePool{
			Name:         fmt.Sprintf("pool-%d", i),
			Capabilities: map[string]interface{}{feature: true},
			Storage:      "oceanstor-nas",
		}
		pools = append(pools, pool)
	}

	return pools
}

func mockStorageBackend(storage string, pool []*StoragePool) *Backend {
	return &Backend{
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

func TestRegisterAllBackend(t *testing.T) {
	Convey("List claim failed", t, func() {
		m := gomonkey.ApplyFunc(pkgUtils.ListClaim,
			func(ctx context.Context, client clientSet.Interface, namespace string) (
				*xuanwuv1.StorageBackendClaimList, error) {
				return &xuanwuv1.StorageBackendClaimList{}, errors.New("mock list claim failed")
			})
		defer m.Reset()

		So(RegisterAllBackend(ctx), ShouldBeError)
	})

	Convey("Get content failed", t, func() {
		m := gomonkey.ApplyFunc(pkgUtils.ListClaim,
			func(ctx context.Context, client clientSet.Interface, namespace string) (
				*xuanwuv1.StorageBackendClaimList, error) {
				claim := &xuanwuv1.StorageBackendClaim{
					Status: &xuanwuv1.StorageBackendClaimStatus{
						BoundContentName: "mock content name",
					},
				}
				return &xuanwuv1.StorageBackendClaimList{
					Items: []xuanwuv1.StorageBackendClaim{*claim},
				}, nil
			})
		defer m.Reset()

		m.ApplyFunc(pkgUtils.GetContent,
			func(ctx context.Context, client clientSet.Interface, contentName string) (
				*xuanwuv1.StorageBackendContent, error) {
				return &xuanwuv1.StorageBackendContent{}, errors.New("mock get content failed")
			})

		So(RegisterAllBackend(ctx), ShouldBeNil)
	})
}
