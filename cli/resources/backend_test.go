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

// Package resources defines resources handle
package resources

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/cli/config"
	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
)

func Test_createSecretWithUid_Success(t *testing.T) {
	// arrange
	claim := xuanwuv1.StorageBackendClaim{Spec: xuanwuv1.StorageBackendClaimSpec{SecretMeta: "huawei-csi/old-secret"}}
	newSecretName := "new-secret"
	oldSecret := v1.Secret{Data: map[string][]byte{constants.AuthenticationModeKey: []byte("0")}}
	bytes, marshalErr := json.Marshal(oldSecret)
	if marshalErr != nil {
		t.Errorf("Test_createSecretWithUid_Success failed, marshal failed, err is [%v]", marshalErr)
	}

	// mock
	originClient := config.Client
	config.Client = &client.KubernetesCLI{}
	mock := gomonkey.ApplyMethodReturn(config.Client, "GetResource", bytes, nil)
	mock.ApplyMethodReturn(&BackendConfiguration{}, "ToSecretConfig", SecretConfig{}, nil)
	config.AuthenticationMode = constants.AuthModeLDAP
	mock.ApplyMethodReturn(config.Client, "OperateResourceByYaml", nil)

	// act
	err := createSecretWithUid(claim, newSecretName)

	// assert
	if err != nil {
		t.Errorf("Test_createSecretWithUid_Success failed, err is [%v]", err)
	}

	// clean
	t.Cleanup(func() {
		config.AuthenticationMode = ""
		config.Client = originClient
		mock.Reset()
	})
}

func Test_createSecretWithUid_GetSecretFailed(t *testing.T) {
	// arrange
	claim := xuanwuv1.StorageBackendClaim{Spec: xuanwuv1.StorageBackendClaimSpec{SecretMeta: "huawei-csi/old-secret"}}
	newSecretName := "new-secret"
	wantErr := errors.New("simulated API failure")

	// mock
	mock := gomonkey.ApplyMethodReturn(config.Client, "GetResource", []byte{}, wantErr)
	mock.ApplyMethodReturn(&BackendConfiguration{}, "ToSecretConfig", SecretConfig{}, nil)

	// act
	err := createSecretWithUid(claim, newSecretName)

	// assert
	if !reflect.DeepEqual(err, wantErr) {
		t.Errorf("Test_createSecretWithUid_GetSecretFailed failed, err is [%v], want %v", err, wantErr)
	}

	// clean
	t.Cleanup(func() {
		mock.Reset()
	})
}

func Test_ValidateBackend_NfsAutoAuthClient(t *testing.T) {
	// arrange
	backend := &BackendConfiguration{
		Parameters: struct {
			Protocol   string                            `json:"protocol,omitempty" yaml:"protocol"`
			ParentName string                            `json:"parentname,omitempty" yaml:"parentname"`
			DeviceWWN  string                            `json:"deviceWWN,omitempty" yaml:"deviceWWN"`
			Portals    interface{}                       `json:"portals,omitempty" yaml:"portals"`
			IscsiLinks string                            `json:"iscsiLinks,omitempty" yaml:"iscsiLinks"`
			Alua       map[string]map[string]interface{} `json:"ALUA,omitempty" yaml:"ALUA"`

			NfsAutoAuthClient      bool     `json:"nfsAutoAuthClient,omitempty" yaml:"nfsAutoAuthClient"`
			NfsAutoAuthClientCIDRs []string `json:"nfsAutoAuthClientCIDRs,omitempty" yaml:"nfsAutoAuthClientCIDRs"`
		}{
			NfsAutoAuthClient:      true,
			NfsAutoAuthClientCIDRs: []string{"127.0.0.0/24"},
		},
	}

	// action
	gotErr := validateBackend(backend)

	// assert
	assert.NoError(t, gotErr)
}

func Test_ValidateBackend_NfsAutoAuthClientCIDRsInvalid(t *testing.T) {
	// arrange
	backend := &BackendConfiguration{
		Parameters: struct {
			Protocol   string                            `json:"protocol,omitempty" yaml:"protocol"`
			ParentName string                            `json:"parentname,omitempty" yaml:"parentname"`
			DeviceWWN  string                            `json:"deviceWWN,omitempty" yaml:"deviceWWN"`
			Portals    interface{}                       `json:"portals,omitempty" yaml:"portals"`
			IscsiLinks string                            `json:"iscsiLinks,omitempty" yaml:"iscsiLinks"`
			Alua       map[string]map[string]interface{} `json:"ALUA,omitempty" yaml:"ALUA"`

			NfsAutoAuthClient      bool     `json:"nfsAutoAuthClient,omitempty" yaml:"nfsAutoAuthClient"`
			NfsAutoAuthClientCIDRs []string `json:"nfsAutoAuthClientCIDRs,omitempty" yaml:"nfsAutoAuthClientCIDRs"`
		}{
			NfsAutoAuthClient:      true,
			NfsAutoAuthClientCIDRs: []string{"invalid_cidrs"},
		},
	}

	// action
	gotErr := validateBackend(backend)

	// assert
	assert.Error(t, gotErr)
}

func TestBackend_setMaxClients_DMESuccess(t *testing.T) {
	// arrange
	backend := &Backend{}
	backendConfig := &BackendConfiguration{StorageDeviceSN: "testSN", Storage: constants.OceanStorASeriesNas}

	// act
	backend.setMaxClients(backendConfig)

	// assert
	assert.Equal(t, config.DMEDefaultMaxClientThreads, backendConfig.MaxClientThreads)
}

func TestBackend_setMaxClients_Success(t *testing.T) {
	// arrange
	backend := &Backend{}
	backendConfig := &BackendConfiguration{Storage: constants.OceanStorASeriesNas}

	// act
	backend.setMaxClients(backendConfig)

	// assert
	assert.Equal(t, config.DefaultMaxClientThreads, backendConfig.MaxClientThreads)
}
