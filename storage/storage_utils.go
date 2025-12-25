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

// Package storage provide base operations for storage
package storage

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"

	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	// RequestSemaphoreMap stores the total connection num of each storage
	RequestSemaphoreMap = map[string]*utils.Semaphore{UninitializedStorage: utils.NewSemaphore(MaxStorageThreads)}
)

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

// NewClientConfig stores the information needed to create a new rest client
type NewClientConfig struct {
	Urls            []string
	User            string
	SecretName      string
	SecretNamespace string
	ParallelNum     string
	BackendID       string
	UseCert         bool
	CertSecretMeta  string
	Storage         string
	Name            string
}
