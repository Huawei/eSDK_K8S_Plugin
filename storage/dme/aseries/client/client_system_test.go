/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.
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

// Package client provides DME A-series storage client
package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeSysJSON(t *testing.T, w http.ResponseWriter, data string) {
	t.Helper()
	_, err := w.Write([]byte(data))
	assert.NoError(t, err)
}

func TestSystemClient_GetHyperScalePoolByName_Success(t *testing.T) {
	// arrange
	successResp := `
		{
			"total": 1,
			"data": [
				{
					"id": "aaa",
					"name": "bbb",
					"total_capacity": 4194304,
					"capacity_usage": 0.13,
					"free_capacity": 4188568.95488
				}
			]
		}
	`

	// Mock
	systemCli1 := SystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	pool1, err := systemCli1.GetHyperScalePoolByName(context.Background(), "bbb")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, pool1.ID, "aaa")

	// Mock
	systemCli2 := SystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	pool2, err := systemCli2.GetHyperScalePoolByName(context.Background(), "ccc")

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, pool2)
}

func TestSystemClient_GetHyperScalePoolByName_Empty(t *testing.T) {
	// Arrange - query returns total=0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"total":0,"data":[]}`)
	}))
	defer mockServer.Close()

	systemCli := SystemClient{BaseClientInterface: getMockClientWithServer(mockServer.URL)}

	// Action
	pool, err := systemCli.GetHyperScalePoolByName(context.Background(), "aaa")

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, pool)
}

func TestSystemClient_QueryZoneBySN(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/rest/storageclusterservice/v1/zones/query", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"total":1,"datas":[`+
			`{"id":"zone-cmdb-id","native_id":"native-id-123","name":"zone1","sn":"SN123"}]}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}
	zone, err := cli.QueryZoneBySN(context.Background(), "SN123")
	assert.NoError(t, err)
	assert.NotNil(t, zone)
	assert.Equal(t, "zone-cmdb-id", zone.ID)
	assert.Equal(t, "native-id-123", zone.NativeID)
}

func TestSystemClient_QueryZoneBySN_NotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"total":0,"datas":[]}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}
	zone, err := cli.QueryZoneBySN(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Nil(t, zone)
}

func TestSystemClient_QueryVstores(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/rest/fileservice/v1/vstores/query", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"total":1,"vstores":[`+
			`{"id":"vstore-id-1","raw_id":"raw-id-1","name":"System_vStore"}]}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}
	vstores, err := cli.QueryVstores(context.Background(), &VstoreQueryParams{
		Name:      "System_vStore",
		StorageID: "storage-id",
		ZoneID:    "zone-id",
	})
	assert.NoError(t, err)
	assert.Len(t, vstores, 1)
	assert.Equal(t, "vstore-id-1", vstores[0].ID)
	assert.Equal(t, "raw-id-1", vstores[0].RawID)
}

func TestSystemClient_QueryVstores_NotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"total":0,"vstores":[]}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}
	vstores, err := cli.QueryVstores(context.Background(), &VstoreQueryParams{Name: "nonexistent"})
	assert.Error(t, err)
	assert.Nil(t, vstores)
}

func TestSystemClient_GetStoragePoolByName(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/rest/storagemgmt/v1/storagepools/query", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"total":1,"datas":[{"id":"pool-id-1","name":"pool1","raw_id":"1"}]}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}
	pool, err := cli.GetStoragePoolByName(context.Background(), "pool1", "zone-id")
	assert.NoError(t, err)
	assert.NotNil(t, pool)
	assert.Equal(t, "pool1", pool.Name)
	assert.Equal(t, "1", pool.RawID)
}

func TestSystemClient_GetStoragePoolByName_NotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"total":1,"datas":[{"id":"pool-id-1","name":"other-pool","raw_id":"2"}]}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}
	pool, err := cli.GetStoragePoolByName(context.Background(), "nonexistent", "zone-id")
	assert.NoError(t, err)
	assert.Nil(t, pool)
}

func TestHyperScalePool_GetRawID(t *testing.T) {
	// Arrange
	pool := &HyperScalePool{ID: "id-1", RawId: "raw-1", Name: "pool1"}

	// Action
	rawID := pool.GetRawID()

	// Assert
	assert.Equal(t, "raw-1", rawID)
}

func TestSystemClient_QueryZoneBySN_Error(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}

	// Action
	zone, err := cli.QueryZoneBySN(context.Background(), "SN123")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, zone)
	assert.Contains(t, err.Error(), "query zone by SN SN123 failed")
}

func TestSystemClient_QueryVstores_Error(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}

	// Action
	vstores, err := cli.QueryVstores(context.Background(), &VstoreQueryParams{Name: "vstore1"})

	// Assert
	assert.Error(t, err)
	assert.Nil(t, vstores)
	assert.Contains(t, err.Error(), "query vstores failed")
}

func TestStoragePool_GetRawID(t *testing.T) {
	// Arrange
	pool := &StoragePool{ID: "id-1", RawID: "raw-1", Name: "pool1"}

	// Action
	rawID := pool.GetRawID()

	// Assert
	assert.Equal(t, "raw-1", rawID)
}

func TestSystemClient_GetStoragePools_Error(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}

	// Action
	pools, err := cli.GetStoragePools(context.Background(), "zone-id")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, pools)
	assert.Contains(t, err.Error(), "get storage pools failed")
}

func TestSystemClient_GetStoragePoolByName_Error(t *testing.T) {
	// Arrange
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeSysJSON(t, w, `{"error_code":"500","error_msg":"internal error"}`)
	}))
	defer mockServer.Close()

	baseCli := getMockClient(200, "")
	baseCli.url = mockServer.URL
	baseCli.client = &http.Client{}
	cli := &SystemClient{BaseClientInterface: baseCli}

	// Action
	pool, err := cli.GetStoragePoolByName(context.Background(), "pool1", "zone-id")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "get storage pool by name pool1 failed")
}
