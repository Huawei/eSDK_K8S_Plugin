/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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
	"strconv"
	"sync"
	"time"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

const (
	noAuthenticated int64 = 10000003
	offLineCodeInt  int64 = 1077949069
	offLineCode           = "1077949069"

	defaultParallelCount int = 50
	maxParallelCount     int = 1000
	minParallelCount     int = 20
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
	clientSemaphore *utils.Semaphore
)

func isFilterLog(method, url string) bool {
	filter, exist := filterLog[method]
	return exist && filter[url]
}

type Client struct {
	url       string
	user      string
	password  string
	authToken string
	client    *http.Client

	reloginMutex sync.Mutex
}

func NewClient(url, user, password, parallelNum string) *Client {
	var err error
	var parallelCount int

	if len(parallelNum) > 0 {
		parallelCount, err = strconv.Atoi(parallelNum)
		if err != nil || parallelCount > maxParallelCount || parallelCount < minParallelCount {
			log.Warningf("The config parallelNum %d is invalid, set it to the default value %d", parallelCount, defaultParallelCount)
			parallelCount = defaultParallelCount
		}
	} else {
		parallelCount = defaultParallelCount
	}

	log.Infof("Init parallel count is %d", parallelCount)
	clientSemaphore = utils.NewSemaphore(parallelCount)
	return &Client{
		url:      url,
		user:     user,
		password: password,
	}
}

func (cli *Client) DuplicateClient() *Client {
	dup := *cli
	dup.client = nil

	return &dup
}

func (cli *Client) Login(ctx context.Context) error {
	jar, _ := cookiejar.New(nil)
	cli.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar:     jar,
		Timeout: 60 * time.Second,
	}

	log.AddContext(ctx).Infof("Try to login %s.", cli.url)

	data := map[string]interface{}{
		"userName": cli.user,
		"password": cli.password,
	}

	respHeader, resp, err := cli.baseCall(ctx, "POST", "/dsware/service/v1.3/sec/login", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("login %s error: %+v", cli.url, resp)
	}

	cli.authToken = respHeader["X-Auth-Token"][0]
	log.AddContext(ctx).Infof("Login %s success", cli.url)

	return nil
}

func (cli *Client) Logout(ctx context.Context) {
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

func (cli *Client) KeepAlive(ctx context.Context) {
	_, err := cli.post(ctx, "/dsware/service/v1.3/sec/keepAlive", nil)
	if err != nil {
		log.AddContext(ctx).Warningf("Keep token alive error: %v", err)
	}
}

func (cli *Client) doCall(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (http.Header, []byte, error) {
	var err error
	var reqUrl string
	var reqBody io.Reader
	var respBody []byte

	if data != nil {
		reqBytes, err := json.Marshal(data)
		if err != nil {
			log.AddContext(ctx).Errorf("json.Marshal data %v error: %v", data, err)
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

	req.Header.Set("Referer", cli.url)
	req.Header.Set("Content-Type", "application/json")

	if url != "/dsware/service/v1.3/sec/login" && url != "/dsware/service/v1.3/sec/logout" {
		cli.reloginMutex.Lock()
		if cli.authToken != "" {
			req.Header.Set("X-Auth-Token", cli.authToken)
		}
		cli.reloginMutex.Unlock()
	} else {
		if cli.authToken != "" {
			req.Header.Set("X-Auth-Token", cli.authToken)
		}
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog),
		fmt.Sprintf("Request method: %s, url: %s, body: %v", method, reqUrl, data))

	clientSemaphore.Acquire()
	defer clientSemaphore.Release()

	resp, err := cli.client.Do(req)
	if err != nil {
		log.AddContext(ctx).Errorf("Send request method: %s, url: %s, error: %v", method, reqUrl, err)
		return nil, nil, errors.New("unconnected")
	}

	defer resp.Body.Close()

	respBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.AddContext(ctx).Errorf("Read response data error: %v", err)
		return nil, nil, err
	}

	log.FilteredLog(ctx, isFilterLog(method, url), utils.IsDebugLog(method, url, debugLog),
		fmt.Sprintf("Response method: %s, url: %s, body: %s", method, reqUrl, respBody))

	return resp.Header, respBody, nil
}

func (cli *Client) baseCall(ctx context.Context, method string, url string, data map[string]interface{}) (http.Header,
	map[string]interface{}, error) {
	var body map[string]interface{}
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

func (cli *Client) call(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (http.Header, map[string]interface{}, error) {
	var body map[string]interface{}

	respHeader, respBody, err := cli.doCall(ctx, method, url, data)

	if err != nil {
		if err.Error() == "unconnected" {
			goto RETRY
		}

		return nil, nil, err
	}

	err = json.Unmarshal(respBody, &body)
	if err != nil {
		log.AddContext(ctx).Errorf("Unmarshal response body %s error: %v", respBody, err)
		return nil, nil, err
	}

	if errorCode, ok := body["errorCode"].(string); ok && errorCode == offLineCode {
		log.AddContext(ctx).Warningf("User offline, try to relogin %s", cli.url)
		goto RETRY
	}

	// Compatible with int error code 1077949069
	if errorCode, ok := body["errorCode"].(float64); ok && int64(errorCode) == offLineCodeInt {
		log.AddContext(ctx).Warningf("User offline, try to relogin %s", cli.url)
		goto RETRY
	}

	// Compatible with FusionStorage 6.3
	if errorCode, ok := body["errorCode"].(float64); ok && int64(errorCode) == noAuthenticated {
		log.AddContext(ctx).Warningf("User offline, try to relogin %s", cli.url)
		goto RETRY
	}

	return respHeader, body, nil

RETRY:
	err = cli.reLogin(ctx)
	if err == nil {
		respHeader, respBody, err = cli.doCall(ctx, method, url, data)
	}

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

func (cli *Client) reLogin(ctx context.Context) error {
	oldToken := cli.authToken

	cli.reloginMutex.Lock()
	defer cli.reloginMutex.Unlock()
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

func (cli *Client) get(ctx context.Context,
	url string,
	data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call(ctx, "GET", url, data)
	return body, err
}

func (cli *Client) Post(ctx context.Context, url string, data map[string]interface{}) (map[string]interface{}, error) {
	return cli.post(ctx, url, data)
}

func (cli *Client) post(ctx context.Context,
	url string,
	data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call(ctx, "POST", url, data)
	return body, err
}

func (cli *Client) put(ctx context.Context,
	url string,
	data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call(ctx, "PUT", url, data)
	return body, err
}

func (cli *Client) delete(ctx context.Context,
	url string,
	data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call(ctx, "DELETE", url, data)
	return body, err
}

func (cli *Client) checkErrorCode(ctx context.Context, resp map[string]interface{}, errorCode int64) bool {
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
