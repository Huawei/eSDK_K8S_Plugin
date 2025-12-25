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
	"io"
	"net/http"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "clientTest.log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	m.Run()
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.Response, m.Err
}

type MockTransport struct {
	Response *http.Response
	Err      error
}

func getMockClient(statusCode int, body string) *BaseClient {
	cli, _ := NewBaseClient(context.Background(), &storage.NewClientConfig{})
	cli.client = &http.Client{
		Transport: &MockTransport{
			Response: &http.Response{
				StatusCode: statusCode,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		},
	}

	return cli
}

func Test_NewBaseClient_Success(t *testing.T) {
	param := &storage.NewClientConfig{}
	_, err := NewBaseClient(context.Background(), param)
	assert.Nil(t, err)
}

func TestBaseClient_ReLogin_Success(t *testing.T) {
	successResp := `
		{
			"accessSession": "xxx",
			"roaRand": "yyy",
			"expires": 60,
			"additionalInfo": null
		}
	`
	cli := getMockClient(200, successResp)

	patch1 := gomonkey.ApplyFuncReturn(pkgUtils.GetCertSecretFromBackendID, false, "", nil)
	defer patch1.Reset()

	patch2 := gomonkey.ApplyFuncReturn(pkgUtils.GetAuthInfoFromBackendID,
		&pkgUtils.BackendAuthInfo{User: "1", Password: "1"}, nil)
	defer patch2.Reset()
	err := cli.ReLogin(context.Background())
	assert.Nil(t, err)
}

func TestBaseClient_ReLogin_Fail(t *testing.T) {
	cli := &BaseClient{urls: []string{sessionUrl}}
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyFuncReturn(pkgUtils.GetCertSecretFromBackendID, false, "", nil)
	patch.ApplyFuncReturn(pkgUtils.GetAuthInfoFromBackendID,
		&pkgUtils.BackendAuthInfo{User: "1", Password: "1"}, nil)
	patch.ApplyFuncReturn(pkgUtils.SetStorageBackendContentOnlineStatus, nil)
	err := cli.ReLogin(context.Background())
	assert.NotNil(t, err)
}

func TestBaseClient_SetSystemInfo_Success(t *testing.T) {
	systemResp := `
		{
			"version": "aaa",
			"sn": "bbb"
		}
	`

	storageResp := `
		{
			"total": 1,
			"datas": [
				{
					"id": "ccc",
					"sn": "ddd"
				}
			]
		}
	`
	patch := gomonkey.ApplyMethod((*MockTransport)(nil), "RoundTrip",
		func(t *MockTransport, req *http.Request) (*http.Response, error) {
			if req.URL.String() == systemInfoUrl {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(systemResp)),
				}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(storageResp)),
			}, nil
		})
	defer patch.Reset()
	cli := getMockClient(200, storageResp)
	err := cli.SetSystemInfo(context.Background(), "ddd")
	assert.Nil(t, err)
	assert.Equal(t, cli.deviceSN, "bbb")
	assert.Equal(t, cli.storageID, "ccc")
}

func TestBaseClient_ValidateLogin_Success(t *testing.T) {
	successResp := `
		{
			"accessSession": "xxx",
			"roaRand": "yyy",
			"expires": 60,
			"additionalInfo": null
		}
	`
	cli := getMockClient(200, successResp)
	patch := gomonkey.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret,
		&pkgUtils.BackendAuthInfo{User: "1", Password: "1"}, nil)
	defer patch.Reset()
	err := cli.ValidateLogin(context.Background())
	assert.Nil(t, err)
}

func TestBaseClient_Login_Fail(t *testing.T) {
	cli := &BaseClient{urls: []string{sessionUrl}}
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyFuncReturn(pkgUtils.GetCertSecretFromBackendID, false, "", nil)
	patch.ApplyFuncReturn(pkgUtils.GetAuthInfoFromBackendID,
		&pkgUtils.BackendAuthInfo{User: "1", Password: "1"}, nil)
	patch.ApplyFuncReturn(pkgUtils.SetStorageBackendContentOnlineStatus, nil)
	err := cli.Login(context.Background())
	assert.NotNil(t, err)
}

func TestBaseClient_Call_Success(t *testing.T) {
	cli := getMockClient(200, `{"accessSession": "xxx"}`)
	_, err := cli.Call(context.Background(), "GET", sessionUrl, nil)
	assert.Nil(t, err)
}

func TestBaseClient_Call_Fail(t *testing.T) {
	cli := &BaseClient{}
	_, err := cli.Call(context.Background(), "GET", "http://localhost", nil)
	assert.NotNil(t, err)
}

var taskResp = `
[
    {
        "id": "bbca21d3-cdd3-4de1-af3e-1407a07c7e50",
        "name_en": "Modify File System",
        "parent_id": "bbca21d3-cdd3-4de1-af3e-1407a07c7e50",
        "status": 4,
        "detail_en": "The device failed to process the request."
    },
    {
        "id": "b1239b91-ce1c-46df-82ba-19094e9a0f17",
        "name_en": "Modify File System Pre-check",
        "parent_id": "bbca21d3-cdd3-4de1-af3e-1407a07c7e50",
        "status": 3,
        "detail_en": ""
    },
    {
        "id": "dcad2334-80a0-41dc-a46f-5e257df98b41",
        "name_en": "Check file system names",
        "parent_id": "b1239b91-ce1c-46df-82ba-19094e9a0f17",
        "status": 3,
        "detail_en": ""
    }
]
`

func TestBaseClient_GetTaskInfos_Success(t *testing.T) {
	cli := getMockClient(200, taskResp)
	tasks, err := cli.GetTaskInfos(context.Background(), "xxx")
	assert.Nil(t, err)
	assert.Equal(t, 3, len(tasks))
}

func TestBaseClient_GetStorageID_Success(t *testing.T) {
	// arrange
	wantStorageID := "storageID"
	cli := &BaseClient{storageID: wantStorageID}

	// act
	gotStorageID := cli.GetStorageID()

	// assert
	assert.Equal(t, wantStorageID, gotStorageID)
}

func TestBaseClient_GetBackendID_Success(t *testing.T) {
	// arrange
	wantBackendID := "backendID"
	cli := &BaseClient{backendID: wantBackendID}

	// act
	gotBackendID := cli.GetBackendID()

	// assert
	assert.Equal(t, wantBackendID, gotBackendID)
}
