/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"huawei-csi-driver/pkg/constants"
	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	// DefaultParallelCount defines default parallel count
	DefaultParallelCount int = 50

	// MaxParallelCount defines max parallel count
	MaxParallelCount int = 1000

	// MinParallelCount defines min parallel count
	MinParallelCount int = 20

	// GetInfoWaitInternal defines wait internal of getting info
	GetInfoWaitInternal = 10

	// QueryCountPerBatch defines query count for each circle of batch operation
	QueryCountPerBatch int = 100

	description string = "Created from huawei-csi for Kubernetes"

	defaultVStore   string = "System_vStore"
	defaultVStoreID string = "0"

	// IPLockErrorCode defines error code of ip lock
	IPLockErrorCode = 1077949071

	// UserOffline defines error code of user off line
	UserOffline = 1077949069

	// UserUnauthorized defines error code of user unauthorized
	UserUnauthorized = -401

	// SuccessCode defines error code of success
	SuccessCode = int64(0)

	// UrlNotFound defines error msg of url not found
	UrlNotFound = "404_NotFound"
)

var (
	// WrongPasswordErrorCodes user or password is incorrect
	WrongPasswordErrorCodes = []int64{1077987870, 1077949081, 1077949061}
	// AccountBeenLocked account been locked
	AccountBeenLocked = []int64{1077949070, 1077987871}
)

// BaseClientInterface defines interfaces for base client operations
type BaseClientInterface interface {
	ApplicationType
	Clone
	FC
	Filesystem
	FSSnapshot
	Host
	HyperMetro
	Iscsi
	Lun
	LunCopy
	LunSnapshot
	Mapping
	Qos
	Replication
	RoCE
	System
	VStore
	DTree
	OceanStorQuota
	Container
	LIF

	Call(ctx context.Context, method string, url string, data map[string]interface{}) (Response, error)
	SafeCall(ctx context.Context, method string, url string, data map[string]interface{}) (Response, error)
	BaseCall(ctx context.Context, method string, url string, data map[string]interface{}) (Response, error)
	SafeBaseCall(ctx context.Context, method string, url string, data map[string]interface{}) (Response, error)
	Get(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Post(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Put(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Delete(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	SafeDelete(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	GetRequest(ctx context.Context, method string, url string, data map[string]interface{}) (*http.Request, error)
	DuplicateClient() *BaseClient
	Login(ctx context.Context) error
	Logout(ctx context.Context)
	ReLogin(ctx context.Context) error
	GetBackendID() string
}

var (
	filterLog = map[string]map[string]bool{
		"POST": {
			"/xx/sessions": true,
		},
	}

	filterLogRegex = map[string][]string{
		"GET": {
			`/vstore_pair\?filter=ID`,
			`/FsHyperMetroDomain\?RUNNINGSTATUS=0`,
			`/remote_device`,
		},
	}

	debugLog = map[string]map[string]bool{
		"GET": {
			"/license/feature":       true,
			"/nfsservice":            true,
			"/storagepool":           true,
			`/vstore_pair\?RETYPE=1`: true,
			`/vstore?filter=NAME`:    true,
			`/container_pv`:          true,
			`/system`:                true,
		},
	}

	debugLogRegex = map[string][]string{
		"GET": {
			`/vstore_pair\?RETYPE=1`,
			`/vstore\?filter=NAME`,
			`/system`,
		},
	}

	// ClientSemaphore provides semaphore of client
	ClientSemaphore *utils.Semaphore
)

func isFilterLog(method, url string) bool {
	if filter, exist := filterLog[method]; exist && filter[url] {
		return true
	}

	if filter, exist := filterLogRegex[method]; exist {
		for _, k := range filter {
			match, err := regexp.MatchString(k, url)
			if err == nil && match {
				return true
			}
		}
	}

	return false
}

// BaseClient implements BaseClientInterface
type BaseClient struct {
	Client HTTP

	Url  string
	Urls []string

	User            string
	SecretNamespace string
	SecretName      string
	VStoreName      string
	VStoreID        string
	StorageVersion  string
	BackendID       string
	Storage         string
	CurrentSiteWwn  string
	CurrentLifWwn   string
	LastLif         string
	Product         string
	DeviceId        string
	Token           string

	SystemInfoRefreshing uint32
	ReLoginMutex         sync.Mutex
}

// HTTP defines for http request process
type HTTP interface {
	Do(req *http.Request) (*http.Response, error)
}

func newHTTPClientByBackendID(ctx context.Context, backendID string) (HTTP, error) {
	var defaultUseCert bool
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !defaultUseCert}},
		Timeout:   60 * time.Second,
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		log.AddContext(ctx).Errorf("create jar failed, error: %v", err)
		return client, err
	}

	useCert, certMeta, err := pkgUtils.GetCertSecretFromBackendID(ctx, backendID)
	if err != nil {
		log.AddContext(ctx).Errorf("get cert secret from backend [%v] failed, error: %v", backendID, err)
		return client, err
	}

	useCert, certPool, err := pkgUtils.GetCertPool(ctx, useCert, certMeta)
	if err != nil {
		log.AddContext(ctx).Errorf("get cert pool failed, error: %v", err)
		return client, err
	}

	client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !useCert, RootCAs: certPool}}
	client.Jar = jar
	return client, nil
}

func newHTTPClientByCertMeta(ctx context.Context, useCert bool, certMeta string) (HTTP, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.AddContext(ctx).Errorf("create jar failed, error: %v", err)
		return nil, err
	}

	useCert, certPool, err := pkgUtils.GetCertPool(ctx, useCert, certMeta)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !useCert, RootCAs: certPool},
		},
		Jar:     jar,
		Timeout: 60 * time.Second,
	}, nil
}

// Response defines response of request
type Response struct {
	Error map[string]interface{} `json:"error"`
	Data  interface{}            `json:"data,omitempty"`
}

// AssertErrorCode asserts if error code represents success
func (resp *Response) AssertErrorCode() error {
	val, exists := resp.Error["code"]
	if !exists {
		return fmt.Errorf("error code not exists, data: %+v", *resp)
	}

	code, ok := val.(float64)
	if !ok {
		return fmt.Errorf("code is not float64, data: %+v", *resp)
	}

	if int64(code) != SuccessCode {
		return fmt.Errorf("error code is not success, data: %+v", *resp)
	}

	return nil
}

// GetData converts interface{} type data to specific type
func (resp *Response) GetData(val any) error {
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal data, error %w", err)
	}

	err = json.Unmarshal(data, &val)
	if err != nil {
		return fmt.Errorf("failed to unmarshal data as %T, error: %w", val, err)
	}

	return nil
}

// NewClientConfig stores the information needed to create a new OceanStor client
type NewClientConfig struct {
	Urls            []string
	User            string
	SecretName      string
	SecretNamespace string
	VstoreName      string
	ParallelNum     string
	BackendID       string
	UseCert         bool
	CertSecretMeta  string
	Storage         string
	Name            string
}

// NewClient inits a new base client
func NewClient(ctx context.Context, param *NewClientConfig) (*BaseClient, error) {
	var err error
	var parallelCount int

	if len(param.ParallelNum) > 0 {
		parallelCount, err = strconv.Atoi(param.ParallelNum)
		if err != nil || parallelCount > MaxParallelCount || parallelCount < MinParallelCount {
			log.AddContext(ctx).Warningf("The config parallelNum %d is invalid, set it to the default value %d",
				parallelCount, DefaultParallelCount)
			parallelCount = DefaultParallelCount
		}
	} else {
		parallelCount = DefaultParallelCount
	}

	log.AddContext(ctx).Infof("Init parallel count is %d", parallelCount)
	ClientSemaphore = utils.NewSemaphore(parallelCount)

	httpClient, err := newHTTPClientByCertMeta(ctx, param.UseCert, param.CertSecretMeta)
	if err != nil {
		log.AddContext(ctx).Errorf("new http client by cert meta failed, err is %v", err)
		return nil, err
	}

	return &BaseClient{
		Urls:            param.Urls,
		User:            param.User,
		Storage:         param.Storage,
		SecretName:      param.SecretName,
		SecretNamespace: param.SecretNamespace,
		VStoreName:      param.VstoreName,
		Client:          httpClient,
		BackendID:       param.BackendID,
	}, nil
}

// Call provides call for restful request
func (cli *BaseClient) Call(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (Response, error) {
	var r Response
	var err error

	r, err = cli.BaseCall(ctx, method, url, data)
	if !needReLogin(r, err) {
		return r, err
	}

	// Current connection fails, try to relogin to other Urls if exist,
	// if relogin success, resend the request again.
	log.AddContext(ctx).Infof("Try to relogin and resend request method: %s, Url: %s", method, url)
	err = cli.ReLogin(ctx)
	if err != nil {
		return r, err
	}

	// If the logical port changes from storage A to storage B, the system information must be updated.
	if err = cli.SetSystemInfo(ctx); err != nil {
		log.AddContext(ctx).Errorf("after relogin, can't get system info, error: %v", err)
		return r, err
	}

	return cli.BaseCall(ctx, method, url, data)
}

// SafeCall provides call for restful request
func (cli *BaseClient) SafeCall(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (Response, error) {
	var r Response
	var err error

	r, err = cli.SafeBaseCall(ctx, method, url, data)
	if !needReLogin(r, err) {
		return r, err
	}

	// Current connection fails, try to relogin to other Urls if exist,
	// if relogin success, resend the request again.
	log.AddContext(ctx).Infof("Try to re-login and resend request method: %s, Url: %s", method, url)
	err = cli.ReLogin(ctx)
	if err != nil {
		return r, err
	}

	// If the logical port changes from storage A to storage B, the system information must be updated.
	if err = cli.SetSystemInfo(ctx); err != nil {
		log.AddContext(ctx).Errorf("after re-login, can't get system info, error: %v", err)
		return r, err
	}

	return cli.SafeBaseCall(ctx, method, url, data)
}

// needReLogin determine if it is necessary to log in to the storage again
func needReLogin(r Response, err error) bool {
	var unconnected, unauthorized, offline bool
	if err != nil && err.Error() == "unconnected" {
		unconnected = true
	}

	if r.Error != nil {
		if code, ok := r.Error["code"].(float64); ok {
			unauthorized = int64(code) == UserUnauthorized
			offline = int64(code) == UserOffline
		}
	}
	return unconnected || unauthorized || offline
}

// GetRequest return the request info
func (cli *BaseClient) GetRequest(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (*http.Request, error) {
	var req *http.Request
	var err error

	reqUrl := cli.Url
	if cli.DeviceId != "" {
		reqUrl += "/" + cli.DeviceId
	}
	reqUrl += url

	var reqBody io.Reader

	if data != nil {
		reqBytes, err := json.Marshal(data)
		if err != nil {
			log.AddContext(ctx).Errorf("json.Marshal data %v error: %v", data, err)
			return req, err
		}
		reqBody = bytes.NewReader(reqBytes)
	}

	req, err = http.NewRequest(method, reqUrl, reqBody)
	if err != nil {
		log.AddContext(ctx).Errorf("Construct http request error: %s", err.Error())
		return req, err
	}

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")

	if cli.Token != "" {
		req.Header.Set("iBaseToken", cli.Token)
	}

	return req, nil
}

// BaseCall provides base call for request
func (cli *BaseClient) BaseCall(ctx context.Context,
	method string,
	url string,
	data map[string]interface{}) (Response, error) {
	var r Response
	var req *http.Request
	var err error

	if cli.Client == nil {
		errMsg := "http client is nil"
		log.AddContext(ctx).Errorf("Failed to send request method: %s, url: %s, error: %s", method, url, errMsg)
		return r, errors.New(errMsg)
	}

	reqUrl := cli.Url
	reqUrl += url

	if url != "/xx/sessions" && url != "/sessions" {
		cli.ReLoginMutex.Lock()
		req, err = cli.GetRequest(ctx, method, url, data)
		cli.ReLoginMutex.Unlock()
	} else {
		req, err = cli.GetRequest(ctx, method, url, data)
	}

	if err != nil {
		return r, err
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Request method: %s, Url: %s, body: %v", method, req.URL, data))

	ClientSemaphore.Acquire()
	defer ClientSemaphore.Release()

	resp, err := cli.Client.Do(req)
	if err != nil {
		log.AddContext(ctx).Errorf("Send request method: %s, Url: %s, error: %v", method, req.URL, err)
		return r, errors.New("unconnected")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.AddContext(ctx).Errorf("Read response data error: %v", err)
		return r, err
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Response method: %s, Url: %s, body: %s", method, req.URL, body))

	err = json.Unmarshal(body, &r)
	if err != nil {
		log.AddContext(ctx).Errorf("json.Unmarshal data %s error: %v", body, err)
		return r, err
	}

	return r, nil
}

// SafeBaseCall provides base call for request
func (cli *BaseClient) SafeBaseCall(ctx context.Context,
	method string,
	url string,
	data map[string]interface{}) (Response, error) {
	var req *http.Request
	var err error

	if cli.Client == nil {
		return Response{}, fmt.Errorf("failed to send request method: %s, url: %s,"+
			" cause by client not init", method, url)
	}

	reqUrl := cli.Url
	reqUrl += url

	if url != "/xx/sessions" && url != "/sessions" {
		cli.ReLoginMutex.Lock()
		req, err = cli.GetRequest(ctx, method, url, data)
		cli.ReLoginMutex.Unlock()
	} else {
		req, err = cli.GetRequest(ctx, method, url, data)
	}

	if err != nil || req == nil {
		return Response{}, fmt.Errorf("get request failed, error: %w", err)
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Request method: %s, Url: %s, body: %v", method, req.URL, data))

	if ClientSemaphore != nil {
		ClientSemaphore.Acquire()
		defer ClientSemaphore.Release()
	}

	return cli.safeDoCall(ctx, method, url, req)
}

func (cli *BaseClient) safeDoCall(ctx context.Context, method string, url string, req *http.Request) (Response, error) {
	// check whether the logical port is changed from A to B before invoking.
	// The possible cause is that other invoking operations are performed for re-login.
	isNotSessionUrl := url != "/xx/sessions" && url != "/sessions"
	if isNotSessionUrl && cli.CurrentLifWwn != "" {
		if cli.systemInfoRefreshing() {
			return Response{}, fmt.Errorf("querying lif and system information... Please wait")
		}

		if cli.CurrentLifWwn != cli.CurrentSiteWwn {
			currentPort := cli.GetCurrentLif(ctx)
			log.AddContext(ctx).Errorf("current logical port [%s] is not running on own site, "+
				"currentLifWwn: %s, currentSiteWwn: %s", currentPort, cli.CurrentLifWwn, cli.CurrentSiteWwn)
			return Response{}, fmt.Errorf("current logical port [%s] is not running on own site", currentPort)
		}
	}

	resp, err := cli.Client.Do(req)
	if err != nil {
		log.AddContext(ctx).Errorf("Send request method: %s, Url: %s, error: %v", method, req.URL, err)
		return Response{}, errors.New("unconnected")
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.AddContext(ctx).Infof("read close resp failed, error %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response data error: %w", err)
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Response method: %s, Url: %s, body: %s", method, req.URL, body))

	var r Response
	err = json.Unmarshal(body, &r)
	if err != nil {
		return Response{}, fmt.Errorf("json.Unmarshal data %s error: %w", body, err)
	}

	return r, nil
}

// Get provides http request of GET method
func (cli *BaseClient) Get(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.Call(ctx, "GET", url, data)
}

// Post provides http request of POST method
func (cli *BaseClient) Post(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.Call(ctx, "POST", url, data)
}

// Put provides http request of PUT method
func (cli *BaseClient) Put(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.Call(ctx, "PUT", url, data)
}

// Delete provides http request of DELETE method
func (cli *BaseClient) Delete(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.Call(ctx, "DELETE", url, data)
}

// SafeDelete provides http request of DELETE method
func (cli *BaseClient) SafeDelete(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.SafeCall(ctx, "DELETE", url, data)
}

// DuplicateClient clone a base client from origin client
func (cli *BaseClient) DuplicateClient() *BaseClient {
	dup := *cli

	dup.Urls = make([]string, len(cli.Urls))
	copy(dup.Urls, cli.Urls)

	dup.Client = nil

	return &dup
}

// ValidateLogin validates the login info
func (cli *BaseClient) ValidateLogin(ctx context.Context) error {
	var resp Response
	var err error

	password, err := utils.GetPasswordFromSecret(ctx, cli.SecretName, cli.SecretNamespace)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"username": cli.User,
		"password": password,
		"scope":    "0",
	}

	if len(cli.VStoreName) > 0 && cli.VStoreName != defaultVStore {
		data["vstorename"] = cli.VStoreName
	}

	cli.DeviceId = ""
	cli.Token = ""
	for i, url := range cli.Urls {
		cli.Url = url + "/deviceManager/rest"

		log.AddContext(ctx).Infof("Try to login %s", cli.Url)
		resp, err = cli.BaseCall(ctx, "POST", "/xx/sessions", data)
		if err == nil {
			/* Sort the login Url to the last slot of san addresses, so that
			   if this connection error, next time will try other Url first. */
			cli.Urls[i], cli.Urls[len(cli.Urls)-1] = cli.Urls[len(cli.Urls)-1], cli.Urls[i]
			break
		} else if err.Error() != "unconnected" {
			log.AddContext(ctx).Errorf("Login %s error", cli.Url)
			break
		}

		log.AddContext(ctx).Warningf("Login %s error due to connection failure, gonna try another Url",
			cli.Url)
	}

	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("validate login %s error: %+v", cli.Url, resp)
	}

	cli.setDeviceIdFromRespData(ctx, resp)

	log.AddContext(ctx).Infof("Validate login %s success", cli.Url)
	return nil
}

func (cli *BaseClient) setDeviceIdFromRespData(ctx context.Context, resp Response) {
	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		log.AddContext(ctx).Warningf("convert response data to map[string]interface{} failed, data type: [%T]",
			resp.Data)
	}

	cli.DeviceId, ok = respData["deviceid"].(string)
	if !ok {
		log.AddContext(ctx).Warningf("not found deviceId, response data is: [%v]", respData["deviceid"])
	}

	if _, exists := respData["iBaseToken"]; !exists {
		log.AddContext(ctx).Warningf("not found iBaseToken, response data is: [%v]", resp.Data)
	}
	cli.Token, ok = respData["iBaseToken"].(string)
	if !ok {
		log.AddContext(ctx).Warningf("convert iBaseToken to string error, data type: [%T]",
			respData["iBaseToken"])
	}
}

// Login login and set data from response
func (cli *BaseClient) Login(ctx context.Context) error {
	var resp Response
	var err error

	cli.Client, err = newHTTPClientByBackendID(ctx, cli.BackendID)
	if err != nil {
		log.AddContext(ctx).Errorf("new http client by backend %s failed, err is %v", cli.BackendID, err)
		return err
	}

	data, err := cli.getRequestParams(ctx, cli.BackendID)
	if err != nil {
		return err
	}

	cli.DeviceId = ""
	cli.Token = ""
	for i, url := range cli.Urls {
		cli.Url = url + "/deviceManager/rest"

		log.AddContext(ctx).Infof("Try to login %s", cli.Url)
		resp, err = cli.BaseCall(ctx, "POST", "/xx/sessions", data)
		if err == nil {
			/* Sort the login Url to the last slot of san addresses, so that
			   if this connection error, next time will try other Url first. */
			cli.Urls[i], cli.Urls[len(cli.Urls)-1] = cli.Urls[len(cli.Urls)-1], cli.Urls[i]
			break
		} else if err.Error() != "unconnected" {
			log.AddContext(ctx).Errorf("Login %s error", cli.Url)
			break
		}

		log.AddContext(ctx).Warningf("Login %s error due to connection failure, gonna try another Url", cli.Url)
	}

	if err != nil {
		return err
	}

	errCode, _ := resp.Error["code"].(float64)
	if code := int64(errCode); code != 0 {
		msg := fmt.Sprintf("Login %s error: %+v", cli.Url, resp)
		if utils.Contains(WrongPasswordErrorCodes, code) || utils.Contains(AccountBeenLocked, code) ||
			code == IPLockErrorCode {
			if err := pkgUtils.SetStorageBackendContentOnlineStatus(ctx, cli.BackendID, false); err != nil {
				msg = msg + fmt.Sprintf("\nSetStorageBackendContentOffline [%s] failed. error: %v", cli.BackendID, err)
			}
		}
		return errors.New(msg)
	}

	if err = cli.setDataFromRespData(ctx, resp); err != nil {
		cli.Logout(ctx)
		setErr := pkgUtils.SetStorageBackendContentOnlineStatus(ctx, cli.BackendID, false)
		if setErr != nil {
			log.AddContext(ctx).Errorf("SetStorageBackendContentOffline [%s] failed. error: %v", cli.BackendID, setErr)
		}
		return err
	}
	return nil
}

func (cli *BaseClient) setDataFromRespData(ctx context.Context, resp Response) error {
	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("convert resp.Data to map[string]interface{} failed,"+
			" data type: [%T]", resp.Data))
	}
	cli.DeviceId, ok = respData["deviceid"].(string)
	if !ok {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("convert respData[\"deviceid\"]: [%v] to string failed",
			respData["deviceid"]))
	}
	cli.Token, ok = respData["iBaseToken"].(string)
	if !ok {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("convert respData[\"iBaseToken\"]: [%T] to string failed",
			respData["iBaseToken"]))
	}

	vStoreName, exist := respData["vstoreName"].(string)
	vStoreID, idExist := respData["vstoreId"].(string)
	if !exist && !idExist {
		log.AddContext(ctx).Infof("storage client login response vstoreName is empty, set it to default %s",
			defaultVStore)
		cli.VStoreName = defaultVStore
	} else if exist {
		cli.VStoreName = vStoreName
	}

	if !idExist {
		log.AddContext(ctx).Infof("storage client login response vstoreID is empty, set it to default %s",
			defaultVStoreID)
		cli.VStoreID = defaultVStoreID
	} else {
		cli.VStoreID = vStoreID
	}

	log.AddContext(ctx).Infof("Login %s success", cli.Url)
	return nil
}

// Logout logout
func (cli *BaseClient) Logout(ctx context.Context) {
	resp, err := cli.BaseCall(ctx, "DELETE", "/sessions", nil)
	if err != nil {
		log.AddContext(ctx).Warningf("Logout %s error: %v", cli.Url, err)
		return
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		log.AddContext(ctx).Warningf("Logout %s error: %d", cli.Url, code)
		return
	}

	log.AddContext(ctx).Infof("Logout %s success", cli.Url)
}

// ReLogin logout and login again
func (cli *BaseClient) ReLogin(ctx context.Context) error {
	oldToken := cli.Token

	cli.ReLoginMutex.Lock()
	defer cli.ReLoginMutex.Unlock()

	if cli.Token != "" && oldToken != cli.Token {
		// Coming here indicates other thread had already done relogin, so no need to relogin again
		return nil
	} else if cli.Token != "" {
		cli.Logout(ctx)
	}

	err := cli.Login(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Try to relogin error: %v", err)
		return err
	}

	return nil
}

// GetBackendID get backend id of client
func (cli *BaseClient) GetBackendID() string {
	return cli.BackendID
}

func (cli *BaseClient) getResponseDataMap(ctx context.Context, data interface{}) (map[string]interface{}, error) {
	respData, ok := data.(map[string]interface{})
	if !ok {
		return nil, utils.Errorf(ctx, "the response data is not a map[string]interface{}")
	}
	return respData, nil
}

func (cli *BaseClient) getResponseDataList(ctx context.Context, data interface{}) ([]interface{}, error) {
	respData, ok := data.([]interface{})
	if !ok {
		return nil, utils.Errorf(ctx, "the response data is not a []interface{}")
	}
	return respData, nil
}

func (cli *BaseClient) getCountFromResponse(ctx context.Context, data interface{}) (int64, error) {
	respData, err := cli.getResponseDataMap(ctx, data)
	if err != nil {
		return 0, err
	}
	countStr, ok := respData["COUNT"].(string)
	if !ok {
		return 0, utils.Errorf(ctx, "The COUNT is not in respData %v", respData)
	}
	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (cli *BaseClient) getSystemUTCTime(ctx context.Context) (int64, error) {
	resp, err := cli.Get(ctx, "/system_utc_time", nil)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return 0, utils.Errorf(ctx, "get system UTC time error: %d", code)
	}

	if resp.Data == nil {
		return 0, utils.Errorf(ctx, "can not get the system UTC time")
	}

	respData, err := cli.getResponseDataMap(ctx, resp.Data)
	if err != nil {
		return 0, err
	}

	utcTime, ok := respData["CMO_SYS_UTC_TIME"].(string)
	if !ok {
		return 0, utils.Errorf(ctx, "The CMO_SYS_UTC_TIME is not in respData %v", respData)
	}

	time, err := strconv.ParseInt(utcTime, 10, 64)
	if err != nil {
		return 0, err
	}
	return time, nil
}

func (cli *BaseClient) getObjByvStoreName(objList []interface{}) map[string]interface{} {
	for _, data := range objList {
		obj, ok := data.(map[string]interface{})
		if !ok || obj == nil {
			continue
		}

		vStoreName, ok := obj["vstoreName"].(string)
		if !ok {
			vStoreName = defaultVStore
		}

		if vStoreName == cli.GetvStoreName() {
			return obj
		}
		continue

	}
	return nil
}

func (cli *BaseClient) getObj(ctx context.Context, url string, start, end int, filterLog bool) (
	[]map[string]interface{}, error) {
	objUrl := fmt.Sprintf("%s?range=[%d-%d]", url, start, end)
	resp, err := cli.Get(ctx, objUrl, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get batch obj list error: %d", code)
	}

	if !filterLog {
		log.AddContext(ctx).Infoln("There is no obj in storage.")
	}

	if resp.Data == nil {
		return nil, nil
	}

	var objList []map[string]interface{}
	respData, ok := resp.Data.([]interface{})
	if !ok {
		return nil, errors.New("convert resp.Data to []interface{} failed")
	}
	for _, i := range respData {
		device, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert resp.Data to map[string]interface{} failed")
			continue
		}
		objList = append(objList, device)
	}
	return objList, nil
}

func (cli *BaseClient) getBatchObjs(ctx context.Context, url string, filterLog bool) ([]map[string]interface{}, error) {
	rangeStart := 0
	var objList []map[string]interface{}
	for true {
		rangeEnd := rangeStart + QueryCountPerBatch
		objs, err := cli.getObj(ctx, url, rangeStart, rangeEnd, filterLog)
		if err != nil {
			return nil, err
		}

		if objs == nil {
			break
		}

		objList = append(objList, objs...)
		if len(objs) < QueryCountPerBatch {
			break
		}
		rangeStart = rangeEnd + QueryCountPerBatch
	}
	return objList, nil
}

func (cli *BaseClient) getRequestParams(ctx context.Context, backendID string) (map[string]interface{}, error) {
	password, err := pkgUtils.GetPasswordFromBackendID(ctx, backendID)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"username": cli.User,
		"password": password,
		"scope":    "0",
	}

	if len(cli.VStoreName) > 0 && cli.VStoreName != defaultVStore {
		data["vstorename"] = cli.VStoreName
	}

	return data, err
}

// SetSystemInfo set system info
// the mutex lock is required for re-login. Therefore, the internal query of the login interface cannot be performed.
func (cli *BaseClient) SetSystemInfo(ctx context.Context) error {
	log.AddContext(ctx).Infof("set backend [%s] system info is refreshing", cli.BackendID)
	atomic.StoreUint32(&cli.SystemInfoRefreshing, 1)
	defer func() {
		log.AddContext(ctx).Infof("set backend [%s] system info are refreshed", cli.BackendID)
		atomic.StoreUint32(&cli.SystemInfoRefreshing, 0)
	}()

	err := cli.setBaseInfo(ctx)
	if err != nil {
		return err
	}
	cli.setLifInfo(ctx)

	log.AddContext(ctx).Infof("backend type [%s], backend [%s], storage product [%s], storage version [%s], "+
		"current site wwn [%s], current lif [%s], current lif wwn [%s]", cli.Storage,
		cli.BackendID, cli.Product, cli.StorageVersion, cli.CurrentSiteWwn, cli.GetCurrentLif(ctx), cli.CurrentLifWwn)
	return nil
}

func (cli *BaseClient) setBaseInfo(ctx context.Context) error {
	system, err := cli.GetSystem(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("get system info failed, error: %v", err)
		return err
	}

	cli.Product, err = utils.GetProductVersion(system)
	if err != nil {
		log.AddContext(ctx).Errorf("get product version failed, error: %v", err)
		return err
	}

	storagePointVersion, ok := system["pointRelease"].(string)
	if ok {
		cli.StorageVersion = storagePointVersion
	}

	currentSiteWwn, ok := system["wwn"].(string)
	if ok {
		cli.CurrentSiteWwn = currentSiteWwn
	}

	return nil
}

func (cli *BaseClient) setLifInfo(ctx context.Context) {
	if cli.Product != constants.OceanStorDoradoV6 || cli.Storage != constants.OceanStorNas {
		log.AddContext(ctx).Infof("backend type [%s], name [%s], version [%s], not need query lif", cli.Storage,
			cli.BackendID, cli.StorageVersion)
		return
	}

	currentLif := cli.GetCurrentLif(ctx)
	if cli.LastLif == currentLif {
		log.AddContext(ctx).Infof("backend [%s] last lif [%s], current lif [%s], not change",
			cli.BackendID, cli.LastLif, currentLif)
		return
	}

	port, err := cli.GetLogicPort(ctx, currentLif)
	if err != nil {
		log.AddContext(ctx).Errorf("get logic port failed, error: %v", err)
		return
	}

	cli.LastLif = currentLif
	cli.CurrentLifWwn = port.HomeSiteWwn
}

func (cli *BaseClient) systemInfoRefreshing() bool {
	return atomic.LoadUint32(&cli.SystemInfoRefreshing) == 1
}
