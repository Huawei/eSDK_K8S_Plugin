/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

// Package volume is used for OceanDisk san test
package volume

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceandisk/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "OceanDisk_san_test.log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestSAN_Create_Success(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientConfig := client.NewClientConfig{
		Urls:            []string{"127.0.0.1"},
		User:            "testUser",
		SecretName:      "testSecretName",
		SecretNamespace: "testSecretNamespace",
		ParallelNum:     "30",
		BackendID:       "BackendID",
		UseCert:         false,
		CertSecretMeta:  "test/test",
		Storage:         "testStorage",
		Name:            "testName",
	}
	cli, _ := client.NewClient(ctx, &clientConfig)
	san := NewSAN(cli)
	param := map[string]interface{}{
		"name":        "testVolume",
		"storagepool": "testPool",
	}

	// mock
	m := gomonkey.NewPatches()
	defer m.Reset()
	m.ApplyPrivateMethod(&SAN{}, "preCreate",
		func(ctx context.Context, params, taskResult map[string]interface{}) (map[string]interface{}, error) {
			return nil, nil
		})
	m.ApplyPrivateMethod(&SAN{}, "createLocalNamespace",
		func(ctx context.Context, params, taskResult map[string]interface{}) (map[string]interface{}, error) {
			return nil, nil
		})
	m.ApplyPrivateMethod(&SAN{}, "createLocalQoS",
		func(ctx context.Context, params, taskResult map[string]interface{}) (map[string]interface{}, error) {
			return nil, nil
		})

	// action
	_, gotErr := san.Create(ctx, param)

	// assert
	if gotErr != nil {
		t.Errorf("TestCreate_Success failed, gotErr = %v, wantErr = %v.", gotErr, nil)
	}
}

func TestSAN_Create_PrepareFailed(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientConfig := client.NewClientConfig{
		Urls:            []string{"127.0.0.1"},
		User:            "testUser",
		SecretName:      "testSecretName",
		SecretNamespace: "testSecretNamespace",
		ParallelNum:     "30",
		BackendID:       "BackendID",
		UseCert:         false,
		CertSecretMeta:  "test/test",
		Storage:         "testStorage",
		Name:            "testName",
	}
	cli, _ := client.NewClient(ctx, &clientConfig)
	san := NewSAN(cli)
	param := map[string]interface{}{
		"name": "testVolume",
	}
	wantErr := fmt.Errorf("test error")

	// mock
	m := gomonkey.NewPatches()
	defer m.Reset()
	m.ApplyPrivateMethod(&SAN{}, "preCreate",
		func(ctx context.Context, params map[string]interface{}) error {
			return wantErr
		})
	m.ApplyPrivateMethod(&SAN{}, "createLocalNamespace",
		func(ctx context.Context, params, taskResult map[string]interface{}) (map[string]interface{}, error) {
			return nil, nil
		})

	// action
	_, gotErr := san.Create(ctx, param)

	// assert
	if gotErr == nil {
		t.Errorf("TestSAN_Create_PrepareFailed failed, gotErr = %v, wantErr = %v.", gotErr, wantErr)
	}
}

func TestSAN_deleteNamespace_success(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientConfig := client.NewClientConfig{
		Urls:            []string{"127.0.0.1"},
		User:            "testUser",
		SecretName:      "testSecretName",
		SecretNamespace: "testSecretNamespace",
		ParallelNum:     "30",
		BackendID:       "BackendID",
		UseCert:         false,
		CertSecretMeta:  "test/test",
		Storage:         "testStorage",
		Name:            "testName",
	}
	cli, _ := client.NewClient(ctx, &clientConfig)
	san := NewSAN(cli)
	param := map[string]interface{}{
		"namespaceName": "testName",
	}
	taskResult := map[string]interface{}{}

	// mock
	m := gomonkey.NewPatches()
	defer m.Reset()
	m.ApplyMethodReturn(&client.OceandiskClient{}, "GetNamespaceByName", nil, nil)

	// action
	_, gotErr := san.deleteNamespace(ctx, param, taskResult)

	// assert
	if gotErr != nil {
		t.Errorf("TestSAN_deleteNamespace_success failed, gotErr = %v, wantErr = %v.", gotErr, nil)
	}
}

func TestSAN_expandLocalNamespace_poolNotExist(t *testing.T) {
	// arrange
	ctx := context.Background()
	clientConfig := client.NewClientConfig{
		Urls:            []string{"127.0.0.1"},
		User:            "testUser",
		SecretName:      "testSecretName",
		SecretNamespace: "testSecretNamespace",
		ParallelNum:     "30",
		BackendID:       "BackendID",
		UseCert:         false,
		CertSecretMeta:  "test/test",
		Storage:         "testStorage",
		Name:            "testName",
	}
	cli, _ := client.NewClient(ctx, &clientConfig)
	san := NewSAN(cli)
	localParentName := "testPool01"
	param := map[string]interface{}{
		"localParentName": localParentName,
	}
	taskResult := map[string]interface{}{}
	wantErr := fmt.Errorf("storage pool: [%s] dose not exist", localParentName)

	// mock
	m := gomonkey.NewPatches()
	defer m.Reset()
	m.ApplyMethodReturn(&client.OceandiskClient{}, "GetPoolByName", nil, nil)

	// action
	_, gotErr := san.expandLocalNamespace(ctx, param, taskResult)

	// assert
	if !reflect.DeepEqual(gotErr, wantErr) {
		t.Errorf("TestSAN_expandLocalNamespace_poolNotExist failed, gotErr = %v, wantErr = %v.", gotErr, wantErr)
	}
}
