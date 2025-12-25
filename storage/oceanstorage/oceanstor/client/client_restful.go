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

// Package client provides oceanstor storage client
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	netUrl "net/url"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// RestClient defines client implements the rest interface
type RestClient struct {
	Client storage.HTTP
	Url    string
	Urls   []string

	User               string
	SecretNamespace    string
	SecretName         string
	VStoreName         string
	VStoreID           string
	StorageVersion     string
	BackendID          string
	Storage            string
	CurrentSiteWwn     string
	CurrentLifWwn      string
	LastLif            string
	Product            constants.OceanstorVersion
	DeviceId           string
	Token              string
	AuthenticationMode string

	SystemInfoRefreshing uint32
	ReLoginMutex         sync.Mutex
	RequestSemaphore     *utils.Semaphore
}

// NewRestClient inits a new rest client
func NewRestClient(ctx context.Context, param *NewClientConfig) (*RestClient, error) {
	var err error
	var parallelCount int

	parallelCount, err = strconv.Atoi(param.ParallelNum)
	if err != nil || parallelCount > MaxParallelCount || parallelCount < MinParallelCount {
		log.Infof("the config parallelNum %d is invalid, set it to the default value %d",
			parallelCount, DefaultParallelCount)
		parallelCount = DefaultParallelCount
	}

	log.AddContext(ctx).Infof("Init parallel count is %d", parallelCount)
	httpClient, err := storage.NewHTTPClientByCertMeta(ctx, param.UseCert, param.CertSecretMeta)
	if err != nil {
		log.AddContext(ctx).Errorf("new http client by cert meta failed, err is %v", err)
		return nil, err
	}

	return &RestClient{
		Urls:             param.Urls,
		User:             param.User,
		Storage:          param.Storage,
		SecretName:       param.SecretName,
		SecretNamespace:  param.SecretNamespace,
		VStoreName:       param.VstoreName,
		Client:           httpClient,
		BackendID:        param.BackendID,
		RequestSemaphore: utils.NewSemaphore(parallelCount),
	}, nil
}

// Call provides call for restful request
func (cli *RestClient) Call(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (base.Response, error) {
	var r base.Response
	var err error

	r, err = cli.BaseCall(ctx, method, url, data)
	if !base.NeedReLogin(r, err) {
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

// BaseCall provides base call for request
func (cli *RestClient) BaseCall(ctx context.Context, method string, url string,
	data map[string]interface{}) (base.Response, error) {
	var r base.Response
	var req *http.Request
	var err error

	if cli.Client == nil {
		errMsg := "http client is nil"
		log.AddContext(ctx).Errorf("Failed to send request method: %s, url: %s, error: %s", method, url, errMsg)
		return base.Response{}, errors.New(errMsg)
	}

	if url != "/xx/sessions" && url != "/sessions" {
		cli.ReLoginMutex.Lock()
		req, err = cli.GetRequest(ctx, method, url, data)
		cli.ReLoginMutex.Unlock()
	} else {
		req, err = cli.GetRequest(ctx, method, url, data)
	}

	if err != nil {
		return base.Response{}, err
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Request method: %s, Url: %s, body: %v", method, req.URL, data))

	if cli.RequestSemaphore == nil {
		return base.Response{}, errors.New("request semaphore is nil")
	}

	cli.RequestSemaphore.Acquire()
	defer cli.RequestSemaphore.Release()

	if storage.RequestSemaphoreMap[cli.GetDeviceSN()] != nil {
		storage.RequestSemaphoreMap[cli.GetDeviceSN()].Acquire()
		defer storage.RequestSemaphoreMap[cli.GetDeviceSN()].Release()
	} else {
		storage.RequestSemaphoreMap[storage.UninitializedStorage].Acquire()
		defer storage.RequestSemaphoreMap[storage.UninitializedStorage].Release()
	}

	resp, err := cli.Client.Do(req)
	if err != nil {
		log.AddContext(ctx).Errorf("Send request method: %s, Url: %s, error: %v", method, req.URL, err)
		return base.Response{}, errors.New(storage.Unconnected)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.AddContext(ctx).Errorf("Read response data error: %v", err)
		return base.Response{}, err
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog, debugLogRegex),
		fmt.Sprintf("Response method: %s, Url: %s, body: %s", method, req.URL, body))

	err = json.Unmarshal(body, &r)
	if err != nil {
		log.AddContext(ctx).Errorf("json.Unmarshal data %s error: %v", body, err)
		return base.Response{}, err
	}

	return r, nil
}

// Get provides http request of GET method
func (cli *RestClient) Get(ctx context.Context, url string, data map[string]interface{}) (base.Response, error) {
	return cli.Call(ctx, "GET", url, data)
}

// Post provides http request of POST method
func (cli *RestClient) Post(ctx context.Context, url string, data map[string]interface{}) (base.Response, error) {
	return cli.Call(ctx, "POST", url, data)
}

// Put provides http request of PUT method
func (cli *RestClient) Put(ctx context.Context, url string, data map[string]interface{}) (base.Response, error) {
	return cli.Call(ctx, "PUT", url, data)
}

// Delete provides http request of DELETE method
func (cli *RestClient) Delete(ctx context.Context, url string, data map[string]interface{}) (base.Response, error) {
	return cli.Call(ctx, "DELETE", url, data)
}

// GetRequest return the request info
func (cli *RestClient) GetRequest(ctx context.Context,
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
			log.AddContext(ctx).Errorf("json.Marshal data %v error: %v", base.MaskRequestData(data), err)
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

// Login login and set data from response
func (cli *RestClient) Login(ctx context.Context) error {
	var resp base.Response
	var err error

	cli.Client, err = storage.NewHTTPClientByBackendID(ctx, cli.BackendID)
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
		} else if err.Error() != storage.Unconnected {
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
		if utils.Contains(base.WrongPasswordErrorCodes, code) || utils.Contains(base.AccountBeenLocked, code) ||
			code == storage.IPLockErrorCode {
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

func (cli *RestClient) setDataFromRespData(ctx context.Context, resp base.Response) error {
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

	if storage.RequestSemaphoreMap[cli.DeviceId] == nil {
		storage.RequestSemaphoreMap[cli.DeviceId] = utils.NewSemaphore(storage.MaxStorageThreads)
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
			storage.DefaultVStore)
		cli.VStoreName = storage.DefaultVStore
	} else if exist {
		cli.VStoreName = vStoreName
	}

	if !idExist {
		log.AddContext(ctx).Infof("storage client login response vstoreID is empty, set it to default %s",
			storage.DefaultVStoreID)
		cli.VStoreID = storage.DefaultVStoreID
	} else {
		cli.VStoreID = vStoreID
	}

	log.AddContext(ctx).Infof("Login %s success", cli.Url)
	return nil
}

// Logout logout
func (cli *RestClient) Logout(ctx context.Context) {
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
func (cli *RestClient) ReLogin(ctx context.Context) error {
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

func (cli *RestClient) getRequestParams(ctx context.Context, backendID string) (map[string]interface{}, error) {
	params, err := pkgUtils.GetAuthInfoFromBackendID(ctx, backendID)
	if err != nil {
		return nil, err
	}
	cli.User = params.User

	data := map[string]interface{}{
		"username": params.User,
		"password": params.Password,
		"scope":    params.Scope,
	}
	params.Password = ""

	if len(cli.VStoreName) > 0 && cli.VStoreName != storage.DefaultVStore {
		data["vstorename"] = cli.VStoreName
	}

	return data, err
}

// SetSystemInfo set system info
// the mutex lock is required for re-login. Therefore, the internal query of the login interface cannot be performed.
func (cli *RestClient) SetSystemInfo(ctx context.Context) error {
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
		"current site wwn [%s], current lif [%s], current lif wwn [%s]",
		cli.Storage, cli.BackendID, cli.Product, cli.GetStorageVersion(),
		cli.CurrentSiteWwn, cli.GetCurrentLif(ctx), cli.CurrentLifWwn)
	return nil
}

func (cli *RestClient) setBaseInfo(ctx context.Context) error {
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

// GetSystem used for get system info
func (cli *RestClient) GetSystem(ctx context.Context) (map[string]interface{}, error) {
	resp, err := cli.Get(ctx, "/system/", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("get system info error: %d", code)
		return nil, errors.New(msg)
	}

	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, pkgUtils.Errorf(ctx, "convert respData to map failed, data: %v", resp.Data)
	}

	return respData, nil
}

func (cli *RestClient) setLifInfo(ctx context.Context) {
	if !cli.Product.IsDoradoV6OrV7() || cli.Storage != constants.OceanStorNas {
		log.AddContext(ctx).Infof("backend type [%s], name [%s], version [%s], not need query lif", cli.Storage,
			cli.BackendID, cli.GetStorageVersion())
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

// GetBackendID get backend id of client
func (cli *RestClient) GetBackendID() string {
	return cli.BackendID
}

// GetDeviceSN used for get device sn
func (cli *RestClient) GetDeviceSN() string {
	return cli.DeviceId
}

// GetStorageVersion used for get storage version
func (cli *RestClient) GetStorageVersion() string {
	return cli.StorageVersion
}

// GetCurrentSiteWwn used for get current site wwn
func (cli *RestClient) GetCurrentSiteWwn() string {
	return cli.CurrentSiteWwn
}

// GetCurrentLif used for get current lif wwn
func (cli *RestClient) GetCurrentLif(ctx context.Context) string {
	u, err := netUrl.Parse(cli.Url)
	if err != nil {
		log.AddContext(ctx).Errorf("parse url failed, error: %v", err)
		return ""
	}
	return u.Hostname()
}

// GetLogicPort gets logic port information by port address
func (cli *RestClient) GetLogicPort(ctx context.Context, addr string) (*Lif, error) {
	url := fmt.Sprintf("/lif?filter=IPV4ADDR:%s&range=[0-100]", addr)
	resp, err := cli.Get(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	if err := resp.AssertErrorCode(); err != nil {
		return nil, err
	}

	var lifs []*Lif
	if err := resp.GetData(&lifs); err != nil {
		return nil, fmt.Errorf("get logic port error: %w", err)
	}

	if len(lifs) == 0 {
		// because manage lif is not exist lif list
		log.AddContext(ctx).Infof("return lis list not contains [%s], it is considered as the management LIF", addr)
		return &Lif{}, nil
	}

	return lifs[0], nil
}

// ValidateLogin validates the login info
func (cli *RestClient) ValidateLogin(ctx context.Context) error {
	var resp base.Response
	var err error

	params, err := pkgUtils.GetAuthInfoFromSecret(ctx, cli.SecretName, cli.SecretNamespace)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"username": cli.User,
		"password": params.Password,
		"scope":    params.Scope,
	}
	params.Password = ""

	if len(cli.VStoreName) > 0 && cli.VStoreName != storage.DefaultVStore {
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
		} else if err.Error() != storage.Unconnected {
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

func (cli *RestClient) setDeviceIdFromRespData(ctx context.Context, resp base.Response) {
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
