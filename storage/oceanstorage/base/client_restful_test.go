/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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

// Package base provide base operations for oceanstor base storage
package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "baseTest.log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	m.Run()
}

func TestRestClient_Call_BaseCallError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	ctx := context.Background()
	method := "GET"
	url := "/test"
	data := make(map[string]interface{})

	// mock
	wantErr := fmt.Errorf("base call error")
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(cli, "BaseCall", Response{}, wantErr)

	// act
	gotResp, gotErr := cli.Call(ctx, method, url, data)

	// assert
	assert.Error(t, gotErr)
	assert.Equal(t, wantErr, gotErr)
	assert.Empty(t, gotResp)

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestRestClient_Call_ReLoginFailure(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	ctx := context.Background()
	method := "GET"
	url := "/test"
	data := make(map[string]interface{})

	// mock
	wantErr := fmt.Errorf("relogin failed")
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(cli, "BaseCall", Response{}, errors.New(Unconnected)).
		ApplyMethodReturn(cli, "ReLogin", wantErr)

	// act
	gotResp, gotErr := cli.Call(ctx, method, url, data)

	// assert
	assert.Error(t, gotErr)
	assert.Equal(t, wantErr, gotErr)
	assert.Empty(t, gotResp)

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestRestClient_Call_Success(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	ctx := context.Background()
	method := "GET"
	url := "/test"
	data := make(map[string]interface{})

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(cli, "BaseCall", Response{}, nil).
		ApplyMethodReturn(cli, "ReLogin", nil)

	// act
	gotResp, gotErr := cli.Call(ctx, method, url, data)

	// assert
	assert.Nil(t, gotErr)
	assert.Equal(t, Response{}, gotResp)

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestRestClient_Call_RetrySuccess(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	ctx := context.Background()
	method := "GET"
	url := "/test"
	data := make(map[string]interface{})

	// mock first basecall returns 401, second success
	mock := gomonkey.NewPatches()
	mock.ApplyMethodSeq(cli, "BaseCall", []gomonkey.OutputCell{
		{Values: gomonkey.Params{Response{}, errors.New(Unconnected)}},
		{Values: gomonkey.Params{Response{}, nil}},
	}).ApplyMethodReturn(cli, "ReLogin", nil)

	// act
	_, gotErr := cli.Call(ctx, method, url, data)

	// assert
	assert.NoError(t, gotErr)

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

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
	wantResponse := Response{
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
	wantResponse := Response{
		Error: make(map[string]interface{}),
		Data:  "",
	}

	responseByte, err := json.Marshal(wantResponse)
	if err != nil {
		return
	}

	requestTime := 10
	wantAvailablePermits := MaxStorageThreads - requestTime
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
	getAvailablePermits := RequestSemaphoreMap[UninitializedStorage].AvailablePermits()
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
	wantResponse := Response{
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
		if cur > int32(MaxStorageThreads) {
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
	assert.Equal(t, currentConcurrent.Load(), int32(0))
	assert.Equal(t, errCount.Load(), int32(0))

	// cleanup
	t.Cleanup(func() {
		mockServer1.Close()
	})
}

func TestRestClient_BaseCall_IsReachMaxConcurrency(t *testing.T) {
	// arrange
	wantResponse := Response{
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
		if cur > int32(MaxStorageThreads-1) {
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
	assert.Equal(t, currentConcurrent.Load(), int32(0))
	assert.Greater(t, errCount.Load(), int32(0))

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
	wantResponse := Response{
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
	assert.Equal(t, currentConcurrent.Load(), int32(0))
	assert.Equal(t, errCount.Load(), int32(0))

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
	wantResponse := Response{
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
	assert.Equal(t, currentConcurrent.Load(), int32(0))
	assert.Greater(t, errCount.Load(), int32(0))

	// cleanup
	t.Cleanup(func() {
		mockServer1.Close()
	})
}

func TestRestClient_BaseCall_ClientNil(t *testing.T) {
	// arrange
	cli := &RestClient{Client: nil}
	ctx := context.Background()
	method := "GET"
	url := "/test"
	data := make(map[string]interface{})

	wantErr := errors.New("http client is nil")

	// act
	gotResp, gotErr := cli.BaseCall(ctx, method, url, data)

	// assert
	assert.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), wantErr.Error())
	assert.Empty(t, gotResp)
}

func TestRestClient_BaseCall_RequestSemaphoreNil(t *testing.T) {
	// arrange
	cli := &RestClient{
		Client: &http.Client{},
	}
	ctx := context.Background()
	method := "GET"
	url := "/test"
	data := make(map[string]interface{})

	// mock
	mock := gomonkey.NewPatches()
	mock.ApplyMethodReturn(cli, "GetRequest", &http.Request{}, nil)

	// act
	gotResp, gotErr := cli.BaseCall(ctx, method, url, data)

	// assert
	assert.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), "request semaphore is nil")
	assert.Empty(t, gotResp)

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestRestClient_loginCall_AllUrlsUnconnected(t *testing.T) {
	// arrange
	cli := &RestClient{
		Urls: []string{"url1", "url2"},
	}
	ctx := context.Background()
	data := make(map[string]interface{})

	mock := gomonkey.NewPatches()
	mock.ApplyMethodSeq(cli, "BaseCall", []gomonkey.OutputCell{
		{Values: gomonkey.Params{Response{}, errors.New(Unconnected)}},
		{Values: gomonkey.Params{Response{}, errors.New(Unconnected)}},
	})

	originalUrls := cli.Urls

	// act
	gotResp, gotErr := cli.loginCall(ctx, data)

	// assert
	assert.Error(t, gotErr)
	assert.Contains(t, gotErr.Error(), Unconnected)
	assert.Equal(t, originalUrls, cli.Urls)
	assert.Empty(t, gotResp)

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestRestClient_loginCall_NonConnectionError(t *testing.T) {
	// arrange
	cli := &RestClient{
		Urls: []string{"url1", "url2", "url3"},
	}
	ctx := context.Background()
	data := make(map[string]interface{})

	mock := gomonkey.NewPatches()
	mock.ApplyMethodSeq(cli, "BaseCall", []gomonkey.OutputCell{
		{Values: gomonkey.Params{Response{}, errors.New(Unconnected)}},
		{Values: gomonkey.Params{Response{}, errors.New("auth failed")}},
	})

	// act
	gotResp, gotErr := cli.loginCall(ctx, data)

	// assert
	assert.Error(t, gotErr)
	assert.Equal(t, "auth failed", gotErr.Error())
	assert.Len(t, cli.Urls, 3)
	assert.Empty(t, gotResp)

	// cleanup
	t.Cleanup(func() {
		mock.Reset()
	})
}

func TestRestClient_loginCall_SuccessOnSecondUrl(t *testing.T) {
	// arrange
	cli := &RestClient{
		Urls: []string{"url1", "url2", "url3"},
	}
	ctx := context.Background()
	data := make(map[string]interface{})
	successResp := Response{Data: "success"}

	mock := gomonkey.NewPatches()
	defer mock.Reset()
	mock.ApplyMethodSeq(cli, "BaseCall", []gomonkey.OutputCell{
		{Values: gomonkey.Params{Response{}, errors.New(Unconnected)}},
		{Values: gomonkey.Params{successResp, nil}},
	})

	expectedOrder := []string{"url1", "url3", "url2"}

	// act
	gotResp, gotErr := cli.loginCall(ctx, data)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, successResp, gotResp)
	assert.Equal(t, expectedOrder, cli.Urls)
}

func TestRestClient_ValidateLogin_GetPasswordError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	wantErr := errors.New("password error")

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, wantErr)

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	assert.ErrorContains(t, gotErr, wantErr.Error())
}

func TestRestClient_ValidateLogin_AllUrlUnconnected(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	cli.Urls = []string{"url1", "url2"}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, nil).
		ApplyMethodReturn(cli, "BaseCall", Response{}, errors.New(Unconnected))

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	assert.ErrorContains(t, gotErr, Unconnected)
}

func TestRestClient_ValidateLogin_ResponseFormatError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, nil).
		ApplyMethodReturn(cli, "BaseCall", Response{Data: "invalid_data_format"}, nil)

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	assert.ErrorContains(t, gotErr, "format login response data error")
}

func TestRestClient_ValidateLogin_NonConnectionError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	cli.Urls = []string{"url1", "url2"}
	wantErr := errors.New("login error")

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, nil).
		ApplyMethodReturn(cli, "BaseCall", Response{}, wantErr)

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	assert.ErrorIs(t, gotErr, wantErr)
}

func TestRestClient_ValidateLogin_StatusCodeError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	cli.Urls = []string{"url1", "url2"}
	wantCode := float64(1)
	wantMsg := "internal error"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, nil).
		ApplyMethodReturn(cli, "BaseCall", Response{
			Error: map[string]interface{}{
				"code":        wantCode,
				"description": wantMsg,
			}}, nil)

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	assert.ErrorContains(t, gotErr, "error code: 1")
	assert.ErrorContains(t, gotErr, wantMsg)
}

func TestRestClient_setDeviceIdFromRespData_TypeConversionError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	resp := Response{
		Data: map[string]interface{}{
			"deviceid":   123,
			"iBaseToken": true,
		},
	}

	// act
	cli.setDeviceIdFromRespData(context.Background(), resp)

	// assert
	assert.Empty(t, cli.DeviceId)
	assert.Empty(t, cli.Token)
}

func TestRestClient_getRequestParams(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	vstore := "test"
	cli.VStoreName = vstore
	password := "pwd"
	scope := "0"
	backendID := "0"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromBackendID, &pkgUtils.BackendAuthInfo{
		Password: password,
		Scope:    scope,
	}, nil)

	// act
	gotdata, gotErr := cli.getRequestParams(context.Background(), backendID)

	// assert
	assert.NoError(t, gotErr)
	assert.Equal(t, gotdata["password"], password)
	assert.Equal(t, gotdata["scope"], scope)
	assert.Equal(t, gotdata["vstorename"], vstore)
}
