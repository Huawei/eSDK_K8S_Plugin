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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	sessionUrl    = "/rest/plat/smapp/v1/sessions"
	systemInfoUrl = "/rest/productmgmt/v1/system-info"
	storageUrl    = "/rest/storagemgmt/v1/storages"
	taskUrl       = "/rest/taskmgmt/v1/tasks/"
	dmeGrantType  = "password"
)

const (
	// DefaultParallelCount defines default parallel count
	DefaultParallelCount int = 5

	// MaxParallelCount defines max parallel count
	MaxParallelCount int = 30

	// MinParallelCount defines min parallel count
	MinParallelCount int = 1
)

var (
	filterLog = map[string]map[string]bool{
		http.MethodPut: {
			sessionUrl: true,
		},
	}
	debugLog = map[string]map[string]bool{
		http.MethodGet: {
			systemInfoUrl: true,
		},
		http.MethodPost: {
			storagePoolUrl:          true,
			batchQueryFilesystemUrl: true,
		},
	}

	debugLogRegex = map[string][]string{
		http.MethodGet: {taskUrl},
	}
)

func isFilterLog(method, url string) bool {
	filter, exist := filterLog[method]
	return exist && filter[url]
}

// Task defines the task info of asynchronous call
type Task struct {
	ID       string `json:"id"`
	Name     string `json:"name_en"`
	ParentID string `json:"parent_id"`
	Status   int    `json:"status"`
	Detail   string `json:"detail_en"`
}

// BaseClientInterface defines interfaces for base client call
type BaseClientInterface interface {
	Call(ctx context.Context, method string, url string, data any) ([]byte, error)
	Login(ctx context.Context) error
	Logout(ctx context.Context)
	ReLogin(ctx context.Context) error
	SetSystemInfo(ctx context.Context, sn string) error
	ValidateLogin(ctx context.Context) error
	GetTaskInfos(ctx context.Context, taskID string) ([]*Task, error)
	GetStorageID() string
	GetBackendID() string
	GetDeviceSN() string
}

// BaseClient defines client implements the base client interface
type BaseClient struct {
	client storage.HTTP
	url    string
	urls   []string

	user            string
	secretNamespace string
	secretName      string
	backendID       string
	storageID       string
	deviceSN        string
	token           string

	reLoginMutex     sync.Mutex
	requestSemaphore *utils.Semaphore
}

// LoginResponse is the response of get login request
type LoginResponse struct {
	AccessSession string `json:"accessSession"`
}

// SystemInfoResponse is the response of get system info request
type SystemInfoResponse struct {
	Version string `json:"version"`
	SN      string `json:"sn"`
}

// StorageDeviceResp is the response of get storage info request
type StorageDeviceResp struct {
	Total int64 `json:"total"`
	Datas []struct {
		ID string `json:"id"`
		SN string `json:"sn"`
	} `json:"datas"`
}

// TaskBatchResponse is the response of get tasks request
type TaskBatchResponse struct {
	Total int64   `json:"total"`
	Tasks []*Task `json:"tasks"`
}

// LoginParam defines dme login param
type LoginParam struct {
	UserName  string `json:"userName"`
	Value     string `json:"value"`
	GrantType string `json:"grantType"`
}

func (cli *BaseClient) getRequestParams(authInfo *pkgUtils.BackendAuthInfo) (*LoginParam, error) {
	cli.user = authInfo.User
	data := &LoginParam{
		UserName:  authInfo.User,
		Value:     authInfo.Password,
		GrantType: dmeGrantType,
	}
	authInfo.Password = ""
	return data, nil
}

func (cli *BaseClient) getRequest(ctx context.Context,
	method string, url string, data any) (*http.Request, error) {
	var (
		req *http.Request
		err error
	)
	reqUrl := cli.url + url
	var reqBody io.Reader
	if data != nil {
		reqBytes, err := json.Marshal(data)
		if err != nil {
			log.AddContext(ctx).Errorf("Json.Marshal data %v error: %v", data, err)
			return nil, err
		}
		reqBody = bytes.NewReader(reqBytes)
	}

	req, err = http.NewRequest(method, reqUrl, reqBody)
	if err != nil {
		log.AddContext(ctx).Errorf("Construct http request error: %v", err)
		return nil, err
	}

	if req == nil || req.Header == nil {
		log.AddContext(ctx).Errorln("Construct http request error: request header init failed")
		return nil, errors.New("construct http request error: request header init failed")
	}

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")

	if cli.token != "" {
		req.Header.Set("X-Auth-Token", cli.token)
	}
	return req, nil
}

// NewBaseClient inits a new base client
func NewBaseClient(ctx context.Context, param *storage.NewClientConfig) (*BaseClient, error) {
	var err error
	var parallelCount int

	parallelCount, err = strconv.Atoi(param.ParallelNum)
	if err != nil || parallelCount > MaxParallelCount || parallelCount < MinParallelCount {
		log.AddContext(ctx).Warningf("The config parallelNum %d is invalid, set it to the default value %d",
			parallelCount, DefaultParallelCount)
		parallelCount = DefaultParallelCount
	}

	httpClient, err := storage.NewHTTPClientByCertMeta(ctx, param.UseCert, param.CertSecretMeta)
	if err != nil {
		log.AddContext(ctx).Errorf("New http client by cert meta failed, err is %v", err)
		return nil, err
	}

	return &BaseClient{
		client:           httpClient,
		urls:             param.Urls,
		user:             param.User,
		secretNamespace:  param.SecretNamespace,
		secretName:       param.SecretName,
		backendID:        param.BackendID,
		requestSemaphore: utils.NewSemaphore(parallelCount),
	}, nil
}

// ReLogin logout and login again
func (cli *BaseClient) ReLogin(ctx context.Context) error {
	oldToken := cli.token

	cli.reLoginMutex.Lock()
	defer cli.reLoginMutex.Unlock()

	if cli.token != "" && oldToken != cli.token {
		// Coming here indicates other thread had already done relogin, so no need to relogin again
		return nil
	} else if cli.token != "" {
		cli.Logout(ctx)
	}

	err := cli.Login(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Try to relogin error: %v", err)
		return err
	}

	return nil
}

// SetSystemInfo init system info of client
func (cli *BaseClient) SetSystemInfo(ctx context.Context, sn string) error {
	systemInfoResp, err := gracefulCall[SystemInfoResponse](ctx, cli, http.MethodGet, systemInfoUrl, nil)
	if err != nil {
		return fmt.Errorf("get system info failed: %w", err)
	}
	cli.deviceSN = systemInfoResp.SN

	deviceResp, err := gracefulCall[StorageDeviceResp](ctx, cli, http.MethodGet, storageUrl, nil)
	if err != nil {
		return fmt.Errorf("get storage device info failed: %w", err)
	}
	for _, device := range deviceResp.Datas {
		if device.SN == sn {
			cli.storageID = device.ID
			break
		}
	}
	if cli.storageID == "" {
		return fmt.Errorf("get storageID failed, sn:%s", sn)
	}

	return nil
}

func (cli *BaseClient) doLogin(ctx context.Context, authInfo *pkgUtils.BackendAuthInfo) error {
	data, err := cli.getRequestParams(authInfo)
	if err != nil {
		return fmt.Errorf("get reuqest failed while login, error : %v", err)
	}

	for _, url := range cli.urls {
		cli.url = url
		log.AddContext(ctx).Infof("try to login %s", cli.url)
		resp, reqErr := gracefulCallAndMarshal[LoginResponse](ctx, cli, http.MethodPut, sessionUrl, data)
		if reqErr == nil {
			cli.token = resp.AccessSession
			return nil
		} else {
			err = reqErr
		}
	}
	return err
}

// ValidateLogin validates the login info
func (cli *BaseClient) ValidateLogin(ctx context.Context) error {
	authInfo, err := pkgUtils.GetAuthInfoFromSecret(ctx, cli.secretName, cli.secretNamespace)
	if err != nil {
		log.AddContext(ctx).Errorf("Get auth info failed: %v", err)
		return err
	}
	return cli.doLogin(ctx, authInfo)
}

// Login login and set data from response
func (cli *BaseClient) Login(ctx context.Context) error {
	client, err := storage.NewHTTPClientByBackendID(ctx, cli.backendID)
	if err != nil {
		return fmt.Errorf("new http client by backend %s failed, err is %v", cli.backendID, err)
	}
	cli.client = client
	authInfo, err := pkgUtils.GetAuthInfoFromBackendID(ctx, cli.backendID)
	if err != nil {
		log.AddContext(ctx).Errorf("Get auth info failed: %v", err)
		return err
	}
	err = cli.doLogin(ctx, authInfo)
	if err == nil {
		return nil
	}
	cli.Logout(ctx)
	if setErr := pkgUtils.SetStorageBackendContentOnlineStatus(ctx, cli.GetBackendID(), false); setErr != nil {
		return fmt.Errorf("login failed: %w\nSetStorageBackendContentOffline [%s] failed. error: %v",
			err, cli.GetBackendID(), setErr)
	}
	return err
}

// Logout delete the session
func (cli *BaseClient) Logout(ctx context.Context) {
	_, err := cli.Call(ctx, http.MethodDelete, sessionUrl, nil)
	if err != nil {
		log.AddContext(ctx).Errorf("Logout failed, error: %v", err)
	}
}

// Call provides call for restful request
func (cli *BaseClient) Call(ctx context.Context, method string, url string, data any) ([]byte, error) {
	var (
		req *http.Request
		err error
	)
	if cli.client == nil {
		errMsg := "http client is nil"
		log.AddContext(ctx).Errorf("Failed to send request method: %s, url: %s, error: %s", method, url, errMsg)
		return nil, errors.New(errMsg)
	}

	if url != sessionUrl {
		cli.reLoginMutex.Lock()
		req, err = cli.getRequest(ctx, method, url, data)
		cli.reLoginMutex.Unlock()
	} else {
		req, err = cli.getRequest(ctx, method, url, data)
	}
	if err != nil {
		return nil, err
	}

	if req == nil {
		return nil, errors.New("req is nil")
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Request method: %s, Url: %s, body: %v", method, req.URL, data))

	if cli.requestSemaphore == nil {
		return nil, errors.New("request semaphore is nil")
	}

	cli.requestSemaphore.Acquire()
	defer cli.requestSemaphore.Release()

	key := storage.UninitializedStorage
	if storage.RequestSemaphoreMap[cli.GetDeviceSN()] != nil {
		key = cli.GetDeviceSN()
	}

	storage.RequestSemaphoreMap[key].Acquire()
	defer storage.RequestSemaphoreMap[key].Release()

	resp, err := cli.client.Do(req)
	if err != nil {
		log.AddContext(ctx).Errorf("Send request method: %s, Url: %s, error: %v", method, req.URL, err)
		return nil, errors.New(storage.Unconnected)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.AddContext(ctx).Errorf("Read response data error: %v", err)
		return nil, err
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Response method: %s, url: %s, body: %s", method, req.URL, respBody))

	return respBody, nil
}

// GetTaskInfos gets task infos by task id
func (cli *BaseClient) GetTaskInfos(ctx context.Context, taskID string) ([]*Task, error) {
	reqUrl := taskUrl + taskID
	resp, err := gracefulCall[[]*Task](ctx, cli, http.MethodGet, reqUrl, nil)
	if err != nil {
		log.AddContext(ctx).Errorf("Get task infos failed, taskID: %s, err is %v", taskID, err)
		return nil, err
	}
	return *resp, nil
}

// GetStorageID get storage id of client
func (cli *BaseClient) GetStorageID() string {
	return cli.storageID
}

// GetBackendID get backend id of client
func (cli *BaseClient) GetBackendID() string {
	return cli.backendID
}

// GetDeviceSN get device sn of client
func (cli *BaseClient) GetDeviceSN() string {
	return cli.deviceSN
}
