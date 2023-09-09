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
	"time"

	pkgUtils "huawei-csi-driver/pkg/utils"
	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	DefaultParallelCount int = 50
	MaxParallelCount     int = 1000
	MinParallelCount     int = 20
	GetInfoWaitInternal      = 10
	QueryCountPerBatch   int = 100

	description string = "Created from huawei-csi for Kubernetes"

	defaultVStore   string = "System_vStore"
	defaultVStoreID string = "0"

	IPLockErrorCode        = 1077949071
	WrongPasswordErrorCode = 1077987870
)

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

	Call(ctx context.Context, method string, url string, data map[string]interface{}) (Response, error)
	BaseCall(ctx context.Context, method string, url string, data map[string]interface{}) (Response, error)
	Get(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Post(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Put(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Delete(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	GetRequest(ctx context.Context, method string, url string, data map[string]interface{}) (*http.Request, error)
	DuplicateClient() *BaseClient
	Login(ctx context.Context) error
	Logout(ctx context.Context)
	ReLogin(ctx context.Context) error
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
			"/license/feature": true,
			"/nfsservice":      true,
			"/storagepool":     true,
		},
	}

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

	DeviceId string
	Token    string

	ReLoginMutex sync.Mutex
}

type HTTP interface {
	Do(req *http.Request) (*http.Response, error)
}

var newHTTPClient = func() HTTP {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar:     jar,
		Timeout: 60 * time.Second,
	}
}

type Response struct {
	Error map[string]interface{} `json:"error"`
	Data  interface{}            `json:"data,omitempty"`
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
}

func NewClient(param *NewClientConfig) *BaseClient {
	var err error
	var parallelCount int

	if len(param.ParallelNum) > 0 {
		parallelCount, err = strconv.Atoi(param.ParallelNum)
		if err != nil || parallelCount > MaxParallelCount || parallelCount < MinParallelCount {
			log.Warningf("The config parallelNum %d is invalid, set it to the default value %d",
				parallelCount, DefaultParallelCount)
			parallelCount = DefaultParallelCount
		}
	} else {
		parallelCount = DefaultParallelCount
	}

	log.Infof("Init parallel count is %d", parallelCount)
	ClientSemaphore = utils.NewSemaphore(parallelCount)
	return &BaseClient{
		Urls:            param.Urls,
		User:            param.User,
		SecretName:      param.SecretName,
		SecretNamespace: param.SecretNamespace,
		VStoreName:      param.VstoreName,
		Client:          newHTTPClient(),
		BackendID:       param.BackendID,
	}
}

func (cli *BaseClient) Call(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (Response, error) {
	var r Response
	var err error

	r, err = cli.BaseCall(ctx, method, url, data)
	if (err != nil && err.Error() == "unconnected") ||
		(r.Error != nil && int64(r.Error["code"].(float64)) == -401) {
		// Current connection fails, try to relogin to other Urls if exist,
		// if relogin success, resend the request again.
		log.AddContext(ctx).Infof("Try to relogin and resend request method: %s, Url: %s", method, url)

		err = cli.ReLogin(ctx)
		if err == nil {
			r, err = cli.BaseCall(ctx, method, url, data)
		}
	}

	return r, err
}

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

func (cli *BaseClient) BaseCall(ctx context.Context,
	method string,
	url string,
	data map[string]interface{}) (Response, error) {
	var r Response
	var req *http.Request
	var err error

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

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog),
		fmt.Sprintf("Request method: %s, Url: %s, body: %v", method, reqUrl, data))

	ClientSemaphore.Acquire()
	defer ClientSemaphore.Release()

	resp, err := cli.Client.Do(req)
	if err != nil {
		log.AddContext(ctx).Errorf("Send request method: %s, Url: %s, error: %v", method, reqUrl, err)
		return r, errors.New("unconnected")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.AddContext(ctx).Errorf("Read response data error: %v", err)
		return r, err
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog),
		fmt.Sprintf("Response method: %s, Url: %s, body: %s", method, reqUrl, body))

	err = json.Unmarshal(body, &r)
	if err != nil {
		log.AddContext(ctx).Errorf("json.Unmarshal data %s error: %v", body, err)
		return r, err
	}

	return r, nil
}

func (cli *BaseClient) Get(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.Call(ctx, "GET", url, data)
}

func (cli *BaseClient) Post(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.Call(ctx, "POST", url, data)
}

func (cli *BaseClient) Put(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.Call(ctx, "PUT", url, data)
}

func (cli *BaseClient) Delete(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.Call(ctx, "DELETE", url, data)
}

func (cli *BaseClient) DuplicateClient() *BaseClient {
	dup := *cli

	dup.Urls = make([]string, len(cli.Urls))
	copy(dup.Urls, cli.Urls)

	dup.Client = nil

	return &dup
}

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
		resp, err = cli.BaseCall(context.Background(), "POST", "/xx/sessions", data)
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

	log.AddContext(ctx).Infof("Validate login %s success", cli.Url)
	return nil
}

func (cli *BaseClient) Login(ctx context.Context) error {
	var resp Response
	var err error

	password, err := pkgUtils.GetPasswordFromBackendID(ctx, cli.BackendID)
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
		resp, err = cli.BaseCall(context.Background(), "POST", "/xx/sessions", data)
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
		msg := fmt.Sprintf("Login %s error: %+v", cli.Url, resp)
		if code == WrongPasswordErrorCode || code == IPLockErrorCode {
			err := pkgUtils.SetStorageBackendContentOnlineStatus(ctx, cli.BackendID, false)
			if err != nil {
				msg = msg + fmt.Sprintf("\nSetStorageBackendContentOffline [%s] failed. error: %v", cli.BackendID, err)
			}
		}

		return errors.New(msg)
	}

	err = cli.setDataFromRespData(ctx, resp)
	if err != nil {
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
		return pkgUtils.Errorln(ctx, fmt.Sprintf("convert resp.Data: [%v] to map[string]interface{} failed",
			resp.Data))
	}
	cli.DeviceId, ok = respData["deviceid"].(string)
	if !ok {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("convert respData[\"deviceid\"]: [%v] to string failed",
			respData["deviceid"]))
	}
	cli.Token, ok = respData["iBaseToken"].(string)
	if !ok {
		return pkgUtils.Errorln(ctx, fmt.Sprintf("convert respData[\"iBaseToken\"]: [%v] to string failed",
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
	respData := resp.Data.([]interface{})
	for _, i := range respData {
		device := i.(map[string]interface{})
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
