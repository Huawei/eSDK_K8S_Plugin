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

// Package client provides oceanstor storage client
package client

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"

	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
)

func TestRestClient_ValidateLogin_GetPasswordError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	wantErr := errors.New("password error")

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, wantErr)

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	require.ErrorContains(t, gotErr, wantErr.Error())
}

func TestRestClient_ValidateLogin_AllUrlUnconnected(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	cli.Urls = []string{"url1", "url2"}

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, nil).
		ApplyMethodReturn(cli, "BaseCall", base.Response{}, errors.New(base.Unconnected))

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	require.ErrorContains(t, gotErr, base.Unconnected)
}

func TestRestClient_ValidateLogin_NonConnectionError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	cli.Urls = []string{"url1", "url2"}
	wantErr := errors.New("login error")

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, nil).
		ApplyMethodReturn(cli, "BaseCall", base.Response{}, wantErr)

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	require.ErrorIs(t, gotErr, wantErr)
}

func TestRestClient_ValidateLogin_StatusCodeError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	cli.Urls = []string{"url1", "url2"}
	wantCode := float64(1)
	wantMsg := "internal error"

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFuncReturn(pkgUtils.GetAuthInfoFromSecret, &pkgUtils.BackendAuthInfo{}, nil).
		ApplyMethodReturn(cli, "BaseCall", base.Response{
			Error: map[string]interface{}{
				"code":        wantCode,
				"description": wantMsg,
			}}, nil)

	// act
	gotErr := cli.ValidateLogin(context.Background())

	// assert
	require.ErrorContains(t, gotErr, "code:1")
	require.ErrorContains(t, gotErr, wantMsg)
}

func TestRestClient_setDeviceIdFromRespData_TypeConversionError(t *testing.T) {
	// arrange
	cli, _ := NewRestClient(context.Background(), &NewClientConfig{})
	resp := base.Response{
		Data: map[string]interface{}{
			"deviceid":   123,
			"iBaseToken": true,
		},
	}

	// act
	cli.setDeviceIdFromRespData(context.Background(), resp)

	// assert
	require.Empty(t, cli.DeviceId)
	require.Empty(t, cli.Token)
}
