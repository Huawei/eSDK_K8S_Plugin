/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2025. All rights reserved.
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

// Package base provide base operations for oceanstor and oceandisk storage
package base

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"slices"
	"time"

	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	// SuccessCode defines error code of success
	SuccessCode = int64(0)

	// QueryCountPerBatch defines query count for each circle of batch operation
	QueryCountPerBatch int = 100

	// UserOffline defines error code of user off line
	UserOffline = 1077949069

	// IPLockErrorCode defines error code of ip lock
	IPLockErrorCode = 1077949071

	// UserUnauthorized defines error code of user unauthorized
	UserUnauthorized = -401

	// Unconnected defines the error msg of unconnected
	Unconnected = "unconnected"

	// LocalUserType defines the user type of local
	LocalUserType = "0"

	// MaxStorageThreads defines max threads of each storage
	MaxStorageThreads = 100

	// UninitializedStorage defines uninitialized storage
	UninitializedStorage = "UninitializedStorage"

	defaultHttpTimeout = 60 * time.Second
)

var (
	// WrongPasswordErrorCodes user or password is incorrect
	WrongPasswordErrorCodes = []int64{1077987870, 1077949081, 1077949061}
	// AccountBeenLocked account been locked
	AccountBeenLocked = []int64{1077949070, 1077987871}
	// RequestSemaphoreMap stores the total connection num of each storage
	RequestSemaphoreMap = map[string]*utils.Semaphore{UninitializedStorage: utils.NewSemaphore(MaxStorageThreads)}
)

// Response defines response of request
type Response struct {
	Error map[string]interface{} `json:"error"`
	Data  interface{}            `json:"data,omitempty"`
}

// AssertErrorCode asserts if error code represents success
func (resp *Response) AssertErrorCode() error {
	code, err := resp.getInt64Code()
	if err != nil {
		return err
	}

	if code != SuccessCode {
		return fmt.Errorf("error code %d: [%v]", code, resp.Error["description"])
	}

	return nil
}

// ResponseToleration defines the error code that can be tolerant and its reason
type ResponseToleration struct {
	Code   int64
	Reason string
}

// AssertErrorWithTolerations asserts if error code represents success
func (resp *Response) AssertErrorWithTolerations(ctx context.Context, tolerations ...ResponseToleration) error {
	code, err := resp.getInt64Code()
	if err != nil {
		return err
	}

	if code != SuccessCode {
		tolerantIndex := slices.IndexFunc(tolerations, func(toleration ResponseToleration) bool {
			return code == toleration.Code
		})
		if tolerantIndex != -1 {
			log.AddContext(ctx).Infof(tolerations[tolerantIndex].Reason)
			return nil
		}

		return fmt.Errorf("error code %d: [%v]", code, resp.Error["description"])
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

func (resp *Response) getInt64Code() (int64, error) {
	val, exists := resp.Error["code"]
	if !exists {
		return 0, fmt.Errorf("error code not exists, response: %+v", *resp)
	}

	codeRaw, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("code is not float64, response: %+v", *resp)
	}

	code, accuracy := big.NewFloat(codeRaw).Int64()
	if accuracy != big.Exact {
		return 0, fmt.Errorf("code is not accuracy, response: %+v", *resp)
	}

	return code, nil
}

// RestClientInterface defines interfaces for base restful call
type RestClientInterface interface {
	Call(ctx context.Context, method string, url string, data map[string]interface{}) (Response, error)
	BaseCall(ctx context.Context, method string, url string, data map[string]interface{}) (Response, error)
	Get(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Post(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Put(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	Delete(ctx context.Context, url string, data map[string]interface{}) (Response, error)
	GetRequest(ctx context.Context, method string, url string, data map[string]interface{}) (*http.Request, error)
	Login(ctx context.Context) error
	Logout(ctx context.Context)
	ReLogin(ctx context.Context) error
	GetSystem(ctx context.Context) (map[string]interface{}, error)
}

// HTTP defines for http request process
type HTTP interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewHTTPClientByBackendID provides a new http client by backend id
func NewHTTPClientByBackendID(ctx context.Context, backendID string) (HTTP, error) {
	var defaultUseCert bool
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !defaultUseCert}},
		Timeout:   defaultHttpTimeout,
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

// NewHTTPClientByCertMeta provides a new http client by cert meta
func NewHTTPClientByCertMeta(ctx context.Context, useCert bool, certMeta string) (HTTP, error) {
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
		Timeout: defaultHttpTimeout,
	}, nil
}

// MaskRequestData masks the sensitive data
func MaskRequestData(data map[string]any) map[string]any {
	sensitiveKey := []string{"user", "password", "iqn", "tgt", "tgtname", "initiatorname"}

	maskedData := make(map[string]any)
	for k, v := range data {
		if slices.Contains(sensitiveKey, k) {
			maskedData[k] = "***"
		} else {
			maskedData[k] = v
		}
	}

	return maskedData
}

// NeedReLogin determine if it is necessary to log in to the storage again
func NeedReLogin(r Response, err error) bool {
	var unconnected, unauthorized, offline bool
	if err != nil && err.Error() == Unconnected {
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

// GetBatchObjs used to get batch objs by url
func GetBatchObjs(ctx context.Context, cli RestClientInterface, url string) ([]map[string]interface{}, error) {
	rangeStart := 0
	var objList []map[string]interface{}
	for {
		rangeEnd := rangeStart + QueryCountPerBatch
		objs, err := getObj(ctx, cli, url, rangeStart, rangeEnd)
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
		rangeStart = rangeEnd
	}
	return objList, nil
}

func getObj(ctx context.Context, cli RestClientInterface,
	url string, start, end int) ([]map[string]interface{}, error) {
	objUrl := fmt.Sprintf("%s?range=[%d-%d]", url, start, end)
	resp, err := cli.Get(ctx, objUrl, nil)
	if err != nil {
		return nil, err
	}

	code, msg, err := utils.FormatRespErr(resp.Error)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, fmt.Errorf("get batch obj list failed, error code: %d, error msg: %s", code, msg)
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
		obj, ok := i.(map[string]interface{})
		if !ok {
			log.AddContext(ctx).Warningf("convert resp.Data to map[string]interface{} failed")
			continue
		}
		objList = append(objList, obj)
	}
	return objList, nil
}
