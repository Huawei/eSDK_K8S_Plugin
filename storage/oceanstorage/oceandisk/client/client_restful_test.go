/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package client provides oceandisk storage client
package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
)

func TestBaseClient_GetRequest_Success(t *testing.T) {
	// arrange
	method := "GET"
	url := "/mockUrl"
	data := map[string]interface{}{}
	client, _ := NewRestClient(context.Background(), &NewClientConfig{})

	// act
	getRequest, getErr := client.GetRequest(context.TODO(), method, url, data)

	// assert
	if getErr != nil || getRequest == nil {
		t.Errorf("TestBaseClient_GetRequest_Success failed, "+
			"wantErr = nil, gotErr = %v, wantRequest != nil, gotRequest = %v", getErr, getRequest)
	}
}

func TestBaseClient_GetRequest_JsonMarshalFailed(t *testing.T) {
	// arrange
	method := "GET"
	url := "/mockUrl"
	data := map[string]interface{}{}
	client, _ := NewRestClient(context.Background(), &NewClientConfig{})
	wantErr := errors.New("json error")

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyFunc(json.Marshal, func(v any) ([]byte, error) {
		return nil, wantErr
	})

	// act
	_, getErr := client.GetRequest(context.TODO(), method, url, data)

	// assert
	if !reflect.DeepEqual(getErr, wantErr) {
		t.Errorf("TestBaseClient_GetRequest_JsonMarshalFailed failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_GetRequest_newRequestFailed(t *testing.T) {
	// arrange
	method := "GET"
	url := "/mockUrl"
	data := map[string]interface{}{}
	client, _ := NewRestClient(context.Background(), &NewClientConfig{})
	wantErr := errors.New("new request error")

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyFunc(http.NewRequest, func(method, url string, body io.Reader) (*http.Request, error) {
		return nil, wantErr
	})

	// act
	_, getErr := client.GetRequest(context.TODO(), method, url, data)

	// assert
	if !reflect.DeepEqual(getErr, wantErr) {
		t.Errorf("TestBaseClient_GetRequest_newRequestFailed failed, "+
			"wantErr = %v, gotErr = %v", wantErr, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestBaseClient_BaseCall_Success(t *testing.T) {
	// arrange
	method := "GET"
	data := map[string]interface{}{}
	mockClient, _ := NewRestClient(context.Background(), &NewClientConfig{})
	wantResponse := base.Response{
		Error: make(map[string]interface{}),
		Data:  "",
	}

	responseByte, err := json.Marshal(wantResponse)
	if err != nil {
		return
	}

	// mock
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(responseByte)
		if err != nil {
			return
		}
	}))

	getResponse, getErr := mockClient.BaseCall(context.TODO(), method, mockServer.URL, data)

	// assert
	if !reflect.DeepEqual(getResponse, wantResponse) || getErr != nil {
		t.Errorf("TestBaseClient_BaseCall_Success failed, "+
			"wantRes = %v, getRes = %v, wantErr = nil, gotErr = %v", wantResponse, getResponse, getErr)
	}

	// cleanup
	t.Cleanup(func() {
		mockServer.Close()
	})
}

func TestRestClient_BaseCall_Concurrency(t *testing.T) {
	// arrange
	method := "GET"
	data := map[string]interface{}{}
	client, _ := NewRestClient(context.Background(), &NewClientConfig{})
	wantResponse := base.Response{
		Error: make(map[string]interface{}),
		Data:  "",
	}

	responseByte, err := json.Marshal(wantResponse)
	if err != nil {
		return
	}

	requestTime := 10
	wantAvailablePermits := base.MaxStorageThreads - requestTime
	wg := sync.WaitGroup{}
	wg.Add(requestTime)

	// mock
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wg.Done()
		wg.Wait()
		time.Sleep(100 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(responseByte)
		if err != nil {
			return
		}
	}))

	// action
	for i := 0; i < requestTime; i++ {
		go func() { _, _ = client.BaseCall(context.TODO(), method, mockServer.URL, data) }()
	}
	wg.Wait()

	// assert
	getAvailablePermits := base.RequestSemaphoreMap[base.UninitializedStorage].AvailablePermits()
	if getAvailablePermits != wantAvailablePermits {
		t.Errorf("TestRestClient_BaseCall_Concurrency failed, "+
			"wantAvailablePermits = %d, getAvailablePermits = %d", wantAvailablePermits, getAvailablePermits)
	}

	// cleanup
	t.Cleanup(func() {
		mockServer.Close()
	})
}

func TestRestClient_BaseCall_ExceedMaxConcurrency(t *testing.T) {
	// arrange
	wantResponse := base.Response{
		Error: make(map[string]interface{}),
		Data:  "",
	}

	responseByte, err := json.Marshal(wantResponse)
	if err != nil {
		return
	}

	var currentConcurrent atomic.Int32
	var errCount atomic.Int32
	wg := sync.WaitGroup{}
	wg.Add(100 * DefaultParallelCount)

	// mock
	mockServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		cur := currentConcurrent.Add(1)
		if cur > int32(base.MaxStorageThreads) {
			errCount.Add(1)
		}
		time.Sleep(10 * time.Millisecond)
		currentConcurrent.Add(-1)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(responseByte)
		if err != nil {
			return
		}
	}))

	// action
	for num := 0; num < 100; num++ {
		go func() {
			client, _ := NewRestClient(context.Background(), &NewClientConfig{})
			for i := 0; i < DefaultParallelCount; i++ {
				go func() {
					_, _ = client.BaseCall(context.TODO(), "GET",
						mockServer1.URL, map[string]interface{}{})
				}()
			}
		}()
	}

	//assert
	wg.Wait()
	require.Equal(t, currentConcurrent.Load(), int32(0))
	require.Equal(t, errCount.Load(), int32(0))

	// cleanup
	t.Cleanup(func() {
		mockServer1.Close()
	})
}

func TestRestClient_BaseCall_IsReachMaxConcurrency(t *testing.T) {
	// arrange
	wantResponse := base.Response{
		Error: make(map[string]interface{}),
		Data:  "",
	}

	responseByte, err := json.Marshal(wantResponse)
	if err != nil {
		return
	}

	var currentConcurrent atomic.Int32
	var errCount atomic.Int32
	wg := sync.WaitGroup{}
	wg.Add(100 * DefaultParallelCount)

	// mock
	mockServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		cur := currentConcurrent.Add(1)
		if cur > int32(base.MaxStorageThreads-1) {
			errCount.Add(1)
		}
		time.Sleep(10 * time.Millisecond)
		currentConcurrent.Add(-1)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(responseByte)
		if err != nil {
			return
		}
	}))

	// action
	for num := 0; num < 100; num++ {
		go func() {
			client, _ := NewRestClient(context.Background(), &NewClientConfig{})
			for i := 0; i < DefaultParallelCount; i++ {
				go func() {
					_, _ = client.BaseCall(context.TODO(), "GET",
						mockServer1.URL, map[string]interface{}{})
				}()
			}
		}()
	}

	//assert
	wg.Wait()
	require.Equal(t, currentConcurrent.Load(), int32(0))
	require.Greater(t, errCount.Load(), int32(0))

	// cleanup
	t.Cleanup(func() {
		mockServer1.Close()
	})
}

func TestRestClient_BaseCall_ExceedMaxBackendConcurrency(t *testing.T) {
	// arrange
	method := "GET"
	data := map[string]interface{}{}
	var currentConcurrent atomic.Int32
	var errCount atomic.Int32
	maxConcurrent := 10
	client, _ := NewRestClient(context.Background(), &NewClientConfig{ParallelNum: strconv.Itoa(int(maxConcurrent))})
	wantResponse := base.Response{
		Error: make(map[string]interface{}),
		Data:  "",
	}

	responseByte, err := json.Marshal(wantResponse)
	if err != nil {
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(maxConcurrent * 2)

	// mock
	mockServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		cur := currentConcurrent.Add(1)
		if cur > int32(maxConcurrent) {
			errCount.Add(1)
		}
		time.Sleep(10 * time.Millisecond)
		currentConcurrent.Add(-1)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(responseByte)
		if err != nil {
			return
		}
	}))

	// action
	for i := 0; i < maxConcurrent*2; i++ {
		go func() { _, _ = client.BaseCall(context.TODO(), method, mockServer1.URL, data) }()
	}

	//assert
	wg.Wait()
	require.Equal(t, currentConcurrent.Load(), int32(0))
	require.Equal(t, errCount.Load(), int32(0))

	// cleanup
	t.Cleanup(func() {
		mockServer1.Close()
	})
}

func TestRestClient_BaseCall_IsReachMaxBackendConcurrency(t *testing.T) {
	// arrange
	method := "GET"
	data := map[string]interface{}{}
	var currentConcurrent atomic.Int32
	var errCount atomic.Int32
	maxConcurrent := 10
	client, _ := NewRestClient(context.Background(), &NewClientConfig{ParallelNum: strconv.Itoa(int(maxConcurrent))})
	wantResponse := base.Response{
		Error: make(map[string]interface{}),
		Data:  "",
	}

	responseByte, err := json.Marshal(wantResponse)
	if err != nil {
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(maxConcurrent * 10)

	// mock
	mockServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		cur := currentConcurrent.Add(1)
		if cur > int32(maxConcurrent-1) {
			errCount.Add(1)
		}
		time.Sleep(10 * time.Millisecond)
		currentConcurrent.Add(-1)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(responseByte)
		if err != nil {
			return
		}
	}))

	// action
	for i := 0; i < maxConcurrent*10; i++ {
		go func() { _, _ = client.BaseCall(context.TODO(), method, mockServer1.URL, data) }()
	}

	//assert
	wg.Wait()
	require.Equal(t, currentConcurrent.Load(), int32(0))
	require.Greater(t, errCount.Load(), int32(0))

	// cleanup
	t.Cleanup(func() {
		mockServer1.Close()
	})
}
