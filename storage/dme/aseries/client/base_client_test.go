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
	"errors"
	"fmt"
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

func TestIsCredentialError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "ErrUnconnected is not credential error",
			err:      storage.ErrUnconnected,
			expected: false,
		},
		{
			name:     "wrapped ErrUnconnected is not credential error",
			err:      fmt.Errorf("wrapped: %w", storage.ErrUnconnected),
			expected: false,
		},
		{
			name:     "context.Canceled is not credential error",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context.DeadlineExceeded is not credential error",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "generic error is not credential error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "BusinessError is not credential error",
			err:      BusinessError{ErrorCode: "12345", ErrorMessage: "some error"},
			expected: false,
		},
		{
			name:     "AuthError 4012 (session expired) is not credential error",
			err:      AuthError{Code: offLineCode, Description: "offline"},
			expected: false,
		},
		{
			name:     "AuthError 4011 (not authenticated) is not credential error",
			err:      AuthError{Code: noAuthenticated, Description: "not authenticated"},
			expected: false,
		},
		{
			name:     "AuthError 400 (wrong password) is credential error",
			err:      AuthError{Code: "400", Description: "user name or password error"},
			expected: true,
		},
		{
			name:     "AuthError 403 (account disabled) is credential error",
			err:      AuthError{Code: "403", Description: "login restricted"},
			expected: true,
		},
		{
			name: "LoginError with credential exceptionId (user_or_value_invalid)",
			err: fmt.Errorf("login failed: %w",
				LoginError{ExceptionId: "user.login.user_or_value_invalid", ExceptionType: "ROA_EXFRAME_EXCEPTION"}),
			expected: true,
		},
		{
			name: "LoginError with credential exceptionId (policy_violation_lock)",
			err: fmt.Errorf("login failed: %w",
				LoginError{ExceptionId: "user.user.policy_violation_lock", ExceptionType: "ROA_EXFRAME_EXCEPTION"}),
			expected: true,
		},
		{
			name: "LoginError with credential exceptionId (pwd_expired)",
			err: fmt.Errorf("login failed: %w",
				LoginError{ExceptionId: "user.pwd.expired", ExceptionType: "ROA_EXFRAME_EXCEPTION"}),
			expected: true,
		},
		{
			name: "LoginError with non-credential exceptionId",
			err: fmt.Errorf("login failed: %w",
				LoginError{ExceptionId: "unknown.exception", ExceptionType: "ROA_EXFRAME_EXCEPTION"}),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCredentialError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBaseClient_Login_TransientError_SkipsOffline(t *testing.T) {
	// When login fails with Unconnected (network error), SetOffline and Logout should NOT be called
	cli := &BaseClient{urls: []string{sessionUrl}}
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyFuncReturn(storage.NewHTTPClientByBackendID, &http.Client{}, nil)
	patch.ApplyFuncReturn(pkgUtils.GetAuthInfoFromBackendID,
		&pkgUtils.BackendAuthInfo{User: "1", Password: "1"}, nil)

	// Mock Call to return Unconnected error (simulating network failure)
	patch.ApplyMethod((*BaseClient)(nil), "Call",
		func(_ *BaseClient, _ context.Context, _ string, _ string, _ any) ([]byte, error) {
			return nil, storage.ErrUnconnected
		})

	logoutCalled := false
	patch.ApplyMethod((*BaseClient)(nil), "Logout",
		func(_ *BaseClient, _ context.Context) {
			logoutCalled = true
		})

	setOfflineCalled := false
	patch.ApplyFunc(pkgUtils.SetStorageBackendContentOnlineStatus,
		func(_ context.Context, _ string, _ bool) error {
			setOfflineCalled = true
			return nil
		})

	err := cli.Login(context.Background())
	assert.NotNil(t, err)
	assert.False(t, logoutCalled, "Logout should not be called for transient errors")
	assert.False(t, setOfflineCalled, "SetStorageBackendContentOnlineStatus should not be called for transient errors")
}

func TestBaseClient_Login_CredentialError_SetsOffline(t *testing.T) {
	// When login fails with a credential error (AuthError 400), SetOffline and Logout SHOULD be called
	cli := &BaseClient{urls: []string{sessionUrl}}
	patch := gomonkey.NewPatches()
	defer patch.Reset()
	patch.ApplyFuncReturn(storage.NewHTTPClientByBackendID, &http.Client{}, nil)
	patch.ApplyFuncReturn(pkgUtils.GetAuthInfoFromBackendID,
		&pkgUtils.BackendAuthInfo{User: "1", Password: "1"}, nil)

	// Mock Call to return AuthError with non-retriable code (credential failure)
	patch.ApplyMethod((*BaseClient)(nil), "Call",
		func(_ *BaseClient, _ context.Context, _ string, _ string, _ any) ([]byte, error) {
			return nil, AuthError{Code: "400", Description: "user name or password error"}
		})

	logoutCalled := false
	patch.ApplyMethod((*BaseClient)(nil), "Logout",
		func(_ *BaseClient, _ context.Context) {
			logoutCalled = true
		})

	setOfflineCalled := false
	patch.ApplyFunc(pkgUtils.SetStorageBackendContentOnlineStatus,
		func(_ context.Context, _ string, _ bool) error {
			setOfflineCalled = true
			return nil
		})

	err := cli.Login(context.Background())
	assert.NotNil(t, err)
	assert.True(t, logoutCalled, "Logout should be called for credential errors")
	assert.True(t, setOfflineCalled, "SetStorageBackendContentOnlineStatus should be called for credential errors")
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

func TestBaseClient_Zone_Defaults(t *testing.T) {
	// Arrange
	cli := &BaseClient{storageID: "test-storage-id"}

	// Act
	zoneID := cli.GetZoneID()

	// Assert
	assert.Equal(t, "test-storage-id", zoneID)
}

func TestBaseClient_SetZoneInfo(t *testing.T) {
	// Arrange
	cli := &BaseClient{storageID: "test-storage-id"}

	// Act
	cli.SetZoneID("zone-id-456")

	// Assert
	assert.Equal(t, "zone-id-456", cli.GetZoneID())
	assert.True(t, cli.IsLocalMode())
}

func TestBaseClient_IsLocalMode_False(t *testing.T) {
	// Arrange
	cli := &BaseClient{storageID: "test-storage-id"}

	// Act
	isLocal := cli.IsLocalMode()

	// Assert
	assert.False(t, isLocal)
}

func TestFormatRequestBody_NilData(t *testing.T) {
	// Arrange
	var data any = nil

	// Act
	result := formatRequestBody(data)

	// Assert
	assert.Equal(t, "", result)
}

func TestFormatRequestBody_BasicString(t *testing.T) {
	// Arrange
	data := "hello"

	// Act
	result := formatRequestBody(data)

	// Assert
	assert.Equal(t, `"hello"`, result)
}

func TestFormatRequestBody_StructWithPointerFields(t *testing.T) {
	// Arrange
	inner := &DTreeCreateQuotaParam{
		QuotaType:      "directory",
		SpaceHardQuota: 10240,
	}
	params := &CreateDTreeParams{
		CreateDtreesParam: []*CreateDtreeParam{
			{DtreeName: "dtree-01", Count: 1},
		},
		QuotaSwitch:      true,
		StorageID:        "storage-001",
		FsID:             "fs-001",
		CreateQuotaParam: []*DTreeCreateQuotaParam{inner},
		CreateNfsShareParam: &DTreeNfsShareParam{
			SharePath:   "/fs/dtree-01",
			Description: "test share",
		},
		DataturboShare: &DTreeDpcShareParam{
			Description: "dpc share",
			Charset:     "UTF-8",
		},
	}

	// Act
	result := formatRequestBody(params)

	// Assert
	assert.NotContains(t, result, "0x", "body should not contain hex pointer addresses")
	assert.Contains(t, result, `"dtree_name":"dtree-01"`, "slice-of-pointer field should be serialized")
	assert.Contains(t, result, `"quota_switch":true`, "bool field should be serialized")
	assert.Contains(t, result, `"storage_id":"storage-001"`, "string field should be serialized")
	assert.Contains(t, result, `"quota_type":"directory"`, "nested pointer slice field should be serialized")
	assert.Contains(t, result, `"share_path":"/fs/dtree-01"`, "pointer struct field should be serialized")
	assert.Contains(t, result, `"description":"dpc share"`, "pointer struct field should be serialized")
}

func TestFormatRequestBody_EmptyStruct(t *testing.T) {
	// Arrange
	params := &QueryKVCacheParams{
		ID:        "test-id",
		StorageID: "storage-001",
	}

	// Act
	result := formatRequestBody(params)

	// Assert
	assert.Contains(t, result, `"id":"test-id"`)
	assert.Contains(t, result, `"storage_id":"storage-001"`)
}

func TestFormatRequestBody_FallbackToSprintf(t *testing.T) {
	// Arrange
	data := make(chan int)

	// Act
	result := formatRequestBody(data)

	// Assert
	assert.Contains(t, result, "0x", "should fall back to %+v formatting for unmarshallable types")
}
