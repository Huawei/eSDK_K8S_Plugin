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

// Package provider is related with storage provider
package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/handler"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/model"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend/plugin"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/lib/drcsi"
	pkgUtils "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/test/mocks/mock_client"
)

func TestStorageProvider_UpdateStorageBackend_SplitMetaError(t *testing.T) {
	// arrange
	provider := &StorageProvider{}
	req := &drcsi.UpdateStorageBackendRequest{BackendId: "invalid_id"}
	wantErr := errors.New("split error")

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(pkgUtils.SplitMetaNamespaceKey, func(string) (string, string, error) {
		return "", "", wantErr
	})

	// act
	gotResp, gotErr := provider.UpdateStorageBackend(context.Background(), req)

	// assert
	require.ErrorContains(t, gotErr, wantErr.Error())
	require.Nil(t, gotResp)
}

func TestStorageProvider_UpdateStorageBackend_FetchRegisterError(t *testing.T) {
	// arrange
	provider := &StorageProvider{register: handler.NewBackendRegister()}
	req := &drcsi.UpdateStorageBackendRequest{BackendId: "ns/valid"}
	wantErr := errors.New("register failed")

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(pkgUtils.SplitMetaNamespaceKey,
		func(string) (string, string, error) { return "ns", "valid", nil })
	patches.ApplyMethod(provider.register, "FetchAndRegisterOneBackend",
		func(_ *handler.BackendRegister, ctx context.Context, name string, checkOnline bool) (*model.Backend,
			error) {
			return nil, wantErr
		})

	// act
	gotResp, gotErr := provider.UpdateStorageBackend(context.Background(), req)

	// assert
	require.ErrorContains(t, gotErr, wantErr.Error())
	require.Nil(t, gotResp)
}

func TestStorageProvider_UpdateStorageBackend_ReLoginError(t *testing.T) {
	// arrange
	provider := &StorageProvider{register: handler.NewBackendRegister()}
	req := &drcsi.UpdateStorageBackendRequest{BackendId: "ns/valid"}
	wantErr := errors.New("relogin failed")
	mockCtrl := gomock.NewController(t)
	cli := mock_client.NewMockIRestClient(mockCtrl)
	fakePlugin := &plugin.FusionStorageNasPlugin{}
	fakePlugin.SetCli(cli)

	// mock
	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyFunc(pkgUtils.SplitMetaNamespaceKey, func(string) (string, string, error) {
		return "ns", "valid", nil
	})
	patches.ApplyMethod(provider.register, "FetchAndRegisterOneBackend",
		func(_ *handler.BackendRegister, ctx context.Context, name string, checkOnline bool) (*model.Backend,
			error) {
			return &model.Backend{Plugin: fakePlugin}, nil
		})
	cli.EXPECT().ReLogin(context.Background()).Return(wantErr)

	// act
	gotResp, gotErr := provider.UpdateStorageBackend(context.Background(), req)

	// assert
	require.ErrorContains(t, gotErr, wantErr.Error())
	require.Nil(t, gotResp)
}
