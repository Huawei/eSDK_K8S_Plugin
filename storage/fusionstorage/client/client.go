/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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

// Package client provides fusion storage client
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"sync"
	"time"

	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/fusionstorage/types"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	noAuthenticated int64 = 10000003
	offLineCodeInt  int64 = 1077949069

	// Cause of pacific storage does not have a determined parallel count,
	// we set default parallel count to 30, which is consistent with the previous default value in our document.
	// This configuration has run smoothly for a long time.
	defaultParallelCount int = 30
	maxParallelCount     int = 30
	minParallelCount     int = 1

	loginFailed         = 1077949061
	loginFailedWithArg  = 1077987870
	userPasswordInvalid = 1073754390

	// IPLock defines error code of ip lock
	IPLock = 1077949071

	unconnectedError   = "unconnected"
	defaultHttpTimeout = 60 * time.Second
)

var (
	filterLog = map[string]map[string]bool{
		"POST": {
			"/dsware/service/v1.3/sec/login":     true,
			"/dsware/service/v1.3/sec/keepAlive": true,
		},
	}

	debugLog = map[string]map[string]bool{
		"GET": {
			"/dsware/service/v1.3/storagePool":        true,
			"/dfv/service/obsPOE/accounts":            true,
			"/api/v2/nas_protocol/nfs_service_config": true,
		},
	}

	debugLogRegex = map[string][]string{}
)

func isFilterLog(method, url string) bool {
	filter, exist := filterLog[method]
	return exist && filter[url]
}

// IRestClient defines all methods for fusion storage client
type IRestClient interface {
	Host
	Iscsi
	Namespace
	Qos
	Quota
	Snapshot
	System
	Volume
	DTree

	ValidateLogin(ctx context.Context) error
	Login(ctx context.Context) error
	SetAccountId(ctx context.Context) error
	Logout(ctx context.Context)
	ReLogin(ctx context.Context) error
	KeepAlive(ctx context.Context)
}

// RestClient implement fusion storage client
type RestClient struct {
	url             string
	user            string
	secretNamespace string
	secretName      string
	backendID       string
	useCert         bool
	certSecretMeta  string

	accountName string
	accountId   int

	authToken string
	client    *http.Client

	reloginMutex     sync.Mutex
	RequestSemaphore *utils.Semaphore
}

// NewClientConfig stores the information needed to create a new FusionStorage client
type NewClientConfig struct {
	Url             string
	User            string
	SecretName      string
	SecretNamespace string
	ParallelNum     string
	BackendID       string
	AccountName     string
	UseCert         bool
	CertSecretMeta  string
}

// NewClient used to init a new fusion storage client
func NewClient(ctx context.Context, clientConfig *NewClientConfig) *RestClient {
	var err error
	var parallelCount int

	parallelCount, err = strconv.Atoi(clientConfig.ParallelNum)
	if err != nil || parallelCount > maxParallelCount || parallelCount < minParallelCount {
		log.Infof("The config parallelNum %d is invalid, set it to the default value %d",
			parallelCount, defaultParallelCount)
		parallelCount = defaultParallelCount
	}

	log.AddContext(ctx).Infof("Init parallel count is %d", parallelCount)
	return &RestClient{
		url:              clientConfig.Url,
		user:             clientConfig.User,
		secretName:       clientConfig.SecretName,
		secretNamespace:  clientConfig.SecretNamespace,
		backendID:        clientConfig.BackendID,
		accountName:      clientConfig.AccountName,
		useCert:          clientConfig.UseCert,
		certSecretMeta:   clientConfig.CertSecretMeta,
		RequestSemaphore: utils.NewSemaphore(parallelCount),
	}
}

// NewIRestClient returns a RestClient as IRestClient
func NewIRestClient(ctx context.Context, clientConfig *NewClientConfig) IRestClient {
	return NewClient(ctx, clientConfig)
}

// ValidateLogin try to login fusion storage by secret
func (cli *RestClient) ValidateLogin(ctx context.Context) error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.AddContext(ctx).Errorf("create jar failed, error: %v", err)
		return err
	}

	useCert, certPool, err := pkgUtils.GetCertPool(ctx, cli.useCert, cli.certSecretMeta)
	if err != nil {
		return err
	}

	cli.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !useCert, RootCAs: certPool},
		},
		Jar:     jar,
		Timeout: defaultHttpTimeout,
	}

	log.AddContext(ctx).Infof("Try to login %s.", cli.url)

	authInfo, err := pkgUtils.GetAuthInfoFromSecret(ctx, cli.secretName, cli.secretNamespace)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"userName": authInfo.User,
		"password": authInfo.Password,
	}
	authInfo.Password = ""

	_, resp, err := cli.baseCall(ctx, "POST", "/dsware/service/v1.3/sec/login", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("validate login %s error: %+v", cli.url, resp)
	}

	log.AddContext(ctx).Infof("Validate login [%s] success", cli.url)
	return nil
}

// Login try to login fusion storage by backend id
func (cli *RestClient) Login(ctx context.Context) error {
	var err error
	cli.client, err = newHTTPClientByBackendID(ctx, cli.backendID)
	if err != nil {
		log.AddContext(ctx).Errorf("get http client by backend %s failed, err is %v", cli.backendID, err)
		return err
	}

	log.AddContext(ctx).Infof("Try to login %s.", cli.url)

	authInfo, err := pkgUtils.GetAuthInfoFromBackendID(ctx, cli.backendID)
	if err != nil {
		return err
	}
	cli.user = authInfo.User

	data := map[string]interface{}{
		"userName": authInfo.User,
		"password": authInfo.Password,
	}

	respHeader, resp, err := cli.baseCall(ctx, "POST", "/dsware/service/v1.3/sec/login", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Login %s error: %+v", cli.url, resp)
		errorCode, ok := resp["errorCode"].(float64)
		if !ok {
			return errors.New(msg)
		}

		// If the password is incorrect, set sbct to offline.
		code := int64(errorCode)
		if code == loginFailed || code == loginFailedWithArg || code == userPasswordInvalid || code == IPLock {
			setErr := pkgUtils.SetStorageBackendContentOnlineStatus(ctx, cli.backendID, false)
			if setErr != nil {
				msg = msg + fmt.Sprintf("\nSetStorageBackendContentOffline [%s] failed. error: %v", cli.backendID,
					setErr)
			}
		}

		return errors.New(msg)
	}

	if respHeader["X-Auth-Token"] == nil || len(respHeader["X-Auth-Token"]) == 0 {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("get respHeader[\"X-Auth-Token\"]: %v failed.",
			respHeader["X-Auth-Token"]))
	}

	cli.authToken = respHeader["X-Auth-Token"][0]

	log.AddContext(ctx).Infof("Login %s success", cli.url)
	return nil
}

// SetAccountId used to set account id of the client
func (cli *RestClient) SetAccountId(ctx context.Context) error {
	log.AddContext(ctx).Debugf("setAccountId start. account name: %s", cli.accountName)
	if cli.accountName == "" {
		cli.accountName = types.DefaultAccountName
		cli.accountId = types.DefaultAccountId
		return nil
	}

	accountId, err := cli.GetAccountIdByName(ctx, cli.accountName)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("Get account id by name: [%s] failed, error: %v",
			cli.accountName, err))
	}
	id, err := strconv.Atoi(accountId)
	if err != nil {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("Convert account id: [%s] to int failed", accountId))
	}
	cli.accountId = id
	log.AddContext(ctx).Infof("setAccountId finish, account name: %s, account id: %d", cli.accountName, cli.accountId)
	return nil
}

// Logout used to log out
func (cli *RestClient) Logout(ctx context.Context) {
	defer func() {
		cli.authToken = ""
		cli.client = nil
	}()

	if cli.client == nil {
		return
	}

	_, resp, err := cli.baseCall(ctx, "POST", "/dsware/service/v1.3/sec/logout", nil)
	if err != nil {
		log.AddContext(ctx).Warningf("Logout %s error: %v", cli.url, err)
		return
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		log.AddContext(ctx).Warningf("Logout %s error: %d", cli.url, result)
		return
	}

	log.AddContext(ctx).Infof("Logout %s success.", cli.url)
}

// KeepAlive used to keep connection token alive
func (cli *RestClient) KeepAlive(ctx context.Context) {
	_, err := cli.post(ctx, "/dsware/service/v1.3/sec/keepAlive", nil)
	if err != nil {
		log.AddContext(ctx).Warningf("Keep token alive error: %v", err)
	}
}

func (cli *RestClient) reLoginLock(ctx context.Context) {
	log.AddContext(ctx).Debugln("Try to reLoginLock.")
	cli.reloginMutex.Lock()
	log.AddContext(ctx).Debugln("ReLoginLock success.")
}

func (cli *RestClient) reLoginUnlock(ctx context.Context) {
	log.AddContext(ctx).Debugln("Try to reLoginUnlock.")
	cli.reloginMutex.Unlock()
	log.AddContext(ctx).Debugln("ReLoginUnlock success.")
}

func (cli *RestClient) doCall(ctx context.Context, method string, url string, data any) (http.Header, []byte, error) {
	var err error
	var reqUrl string
	var reqBody io.Reader
	var respBody []byte

	if cli.client == nil {
		errMsg := "http client is nil"
		log.AddContext(ctx).Errorf("Failed to send request method: %s, url: %s, error: %s", method, url, errMsg)
		return nil, nil, errors.New(errMsg)
	}

	if !utils.IsNil(data) {
		reqBytes, err := json.Marshal(data)
		if err != nil {
			log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
				fmt.Sprintf("json.Marshal data %v error: %v", data, err))
			return nil, nil, err
		}

		reqBody = bytes.NewReader(reqBytes)
	}
	reqUrl = cli.url + url

	req, err := http.NewRequest(method, reqUrl, reqBody)
	if err != nil {
		log.AddContext(ctx).Errorf("Construct http request error: %v", err)
		return nil, nil, err
	}
	cli.setRequestHeader(ctx, req, url)

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Request method: %s, url: %s, body: %+v", method, req.URL, data))

	cli.RequestSemaphore.Acquire()
	defer cli.RequestSemaphore.Release()

	resp, err := cli.client.Do(req)
	if err != nil {
		log.AddContext(ctx).Errorf("Send request method: %s, url: %s, error: %v", method, req.URL, err)
		return nil, nil, errors.New(unconnectedError)
	}

	defer resp.Body.Close()

	respBody, err = io.ReadAll(resp.Body)
	if err != nil {
		log.AddContext(ctx).Errorf("Read response data error: %v", err)
		return nil, nil, err
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Response method: %s, url: %s, body: %s", method, req.URL, respBody))

	return resp.Header, respBody, nil
}

func (cli *RestClient) setRequestHeader(ctx context.Context, req *http.Request, url string) {
	req.Header.Set("Referer", cli.url)
	req.Header.Set("Content-Type", "application/json")

	// When the non-login/logout interface is invoked, if a thread is relogin, the new token is used after the relogin
	// is complete. This prevents the relogin interface from being invoked for multiple times.
	if url != "/dsware/service/v1.3/sec/login" && url != "/dsware/service/v1.3/sec/logout" {
		cli.reLoginLock(ctx)
		if cli.authToken != "" {
			req.Header.Set("X-Auth-Token", cli.authToken)
		}
		cli.reLoginUnlock(ctx)
	} else {
		if cli.authToken != "" {
			req.Header.Set("X-Auth-Token", cli.authToken)
		}
	}
}

func (cli *RestClient) baseCall(ctx context.Context, method string, url string, data map[string]any) (http.Header,
	map[string]any, error) {
	var body map[string]any
	respHeader, respBody, err := cli.doCall(ctx, method, url, data)
	if err != nil {
		return nil, nil, err
	}
	err = json.Unmarshal(respBody, &body)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal response body %s error: %v", respBody, err)
		return nil, nil, err
	}
	return respHeader, body, nil
}

func (cli *RestClient) retryCall(ctx context.Context, method string, url string, data map[string]any) (
	http.Header, map[string]any, error) {

	log.AddContext(ctx).Debugf("retry call: method: %s, url: %s, data: %v.", method, url, data)
	var err error
	var respHeader http.Header
	var respBody []byte

	err = cli.ReLogin(ctx)
	if err != nil {
		return nil, nil, err
	}

	respHeader, respBody, err = cli.doCall(ctx, method, url, data)
	if err != nil {
		return nil, nil, err
	}

	var body map[string]any
	err = json.Unmarshal(respBody, &body)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal response body %s error: %v", respBody, err)
		return nil, nil, err
	}

	return respHeader, body, nil
}

func (cli *RestClient) call(ctx context.Context, method string, url string, data map[string]any) (
	http.Header, map[string]any, error) {

	var body map[string]any
	respHeader, respBody, err := cli.doCall(ctx, method, url, data)
	if err != nil {
		if err.Error() == unconnectedError {
			return cli.retryCall(ctx, method, url, data)
		}
		return nil, nil, err
	}

	err = json.Unmarshal(respBody, &body)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal response body %s error: %v", respBody, err)
		return nil, nil, err
	}

	errCode, err := getErrorCode(body)
	if err != nil {
		log.AddContext(ctx).Errorf("Get error code failed. error: %v", err)
		return nil, nil, err
	}

	if int64(errCode) == offLineCodeInt || int64(errCode) == noAuthenticated {
		log.AddContext(ctx).Warningf("User offline, try to relogin %s", cli.url)
		return cli.retryCall(ctx, method, url, data)
	}

	return respHeader, body, nil
}

// ReLogin logout and login again
func (cli *RestClient) ReLogin(ctx context.Context) error {
	cli.reLoginLock(ctx)
	defer cli.reLoginUnlock(ctx)

	oldToken := cli.authToken
	if cli.authToken != "" && oldToken != cli.authToken {
		// Coming here indicates other thread had already done relogin, so no need to relogin again
		return nil
	} else if cli.authToken != "" {
		cli.Logout(ctx)
	}

	err := cli.Login(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Try to relogin error: %v", err)
		return err
	}

	return nil
}

func (cli *RestClient) get(ctx context.Context,
	url string,
	data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call(ctx, "GET", url, data)
	return body, err
}

// Post used to send post request to storage client
func (cli *RestClient) Post(ctx context.Context, url string, data map[string]any) (map[string]any, error) {
	return cli.post(ctx, url, data)
}

func (cli *RestClient) post(ctx context.Context,
	url string,
	data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call(ctx, "POST", url, data)
	return body, err
}

func (cli *RestClient) put(ctx context.Context,
	url string,
	data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call(ctx, "PUT", url, data)
	return body, err
}

func (cli *RestClient) delete(ctx context.Context,
	url string,
	data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call(ctx, "DELETE", url, data)
	return body, err
}

func (cli *RestClient) checkErrorCode(ctx context.Context, resp map[string]interface{}, errorCode int64) bool {
	details, exist := resp["detail"].([]interface{})
	if !exist || len(details) == 0 {
		return false
	}

	for _, i := range details {
		detail, ok := i.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The detail %v's format is not map[string]interface{}", i)
			log.AddContext(ctx).Errorln(msg)
			return false
		}
		detailErrorCode := int64(detail["errorCode"].(float64))
		if detailErrorCode != errorCode {
			return false
		}
	}

	return true
}

func getErrorCode(body map[string]any) (int, error) {
	// demo 1:
	// | "name"     | "type" |
	// | result     | int32  |
	// | errorCode  | int32  |
	// | suggestion | string |
	if errorCodeInterface, exist := body["errorCode"]; exist {
		if errorCodeFloat, ok := errorCodeInterface.(float64); ok {
			return int(errorCodeFloat), nil
		}

		if errorCodeString, ok := errorCodeInterface.(string); ok {
			return strconv.Atoi(errorCodeString)
		}
	}
	// demo 2:
	// | "name"  |  "type" |
	// | data    |  array  |
	// | - xx    |  - xx   |
	// | result  |  json   |
	// | - code  |  int32  |
	if result, exist := body["result"].(map[string]any); exist {
		if errorCodeFloat, ok := result["code"].(float64); ok {
			return int(errorCodeFloat), nil
		}

		if errorCodeString, ok := result["code"].(string); ok {
			return strconv.Atoi(errorCodeString)
		}
	}

	return 0, nil
}

func newHTTPClientByBackendID(ctx context.Context, backendID string) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.AddContext(ctx).Errorf("create jar failed, error: %v", err)
		return defaultHttpClient(), err
	}

	useCert, certMeta, err := pkgUtils.GetCertSecretFromBackendID(ctx, backendID)
	if err != nil {
		log.AddContext(ctx).Errorf("get cert secret from backend [%v] failed, error: %v", backendID, err)
		return defaultHttpClient(), err
	}

	useCert, certPool, err := pkgUtils.GetCertPool(ctx, useCert, certMeta)
	if err != nil {
		log.AddContext(ctx).Errorf("get cert pool failed, error: %v", err)
		return defaultHttpClient(), err
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !useCert, RootCAs: certPool},
		},
		Jar:     jar,
		Timeout: defaultHttpTimeout,
	}, nil
}

func defaultHttpClient() *http.Client {
	var defaultUseCert bool
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !defaultUseCert}},
		Timeout:   defaultHttpTimeout,
	}
}
