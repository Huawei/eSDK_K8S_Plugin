/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.
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
	"context"
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var (
	testClient *RestClient
)

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	clientConfig := &NewClientConfig{
		Url:             "https://127.0.0.1:8088",
		User:            "dev-account",
		SecretName:      "mock-sec-name",
		SecretNamespace: "mock-sec-namespace",
		ParallelNum:     "30",
		BackendID:       "mock-backend-id",
		AccountName:     "dev-account",
		UseCert:         false,
		CertSecretMeta:  "",
	}
	testClient = NewClient(context.Background(), clientConfig)

	m.Run()
}

func TestGetErrorCode(t *testing.T) {
	t.Run("Normal case", func(t *testing.T) {
		errCode, err := getErrorCode(map[string]any{
			"name":       "mock-name",
			"errorCode":  12345.0,
			"suggestion": "mock-suggestion",
		})
		require.NoError(t, err)
		require.Equal(t, 12345, errCode)
	})

	t.Run("Error code is string ", func(t *testing.T) {
		errCode, err := getErrorCode(map[string]any{
			"name":       "mock-name",
			"errorCode":  "12345",
			"suggestion": "mock-suggestion",
		})
		require.NoError(t, err)
		require.Equal(t, 12345, errCode)
	})

	t.Run("Error code in result", func(t *testing.T) {
		errCode, err := getErrorCode(map[string]any{
			"result": map[string]any{
				"code": 12345.0,
			},
			"data": map[string]any{
				"name": "mock-name",
			},
		})
		require.NoError(t, err)
		require.Equal(t, 12345, errCode)
	})

	t.Run("Can not convert to int", func(t *testing.T) {
		_, err := getErrorCode(map[string]any{
			"result": map[string]any{
				"code": "a12345",
			},
			"data": map[string]any{
				"name": "mock-name",
			},
		})
		require.Error(t, err)
	})
}
