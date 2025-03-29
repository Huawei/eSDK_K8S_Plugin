/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2025. All rights reserved.
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

package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/base"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
)

func mockCli() *client.OceanstorClient {
	restClient := &client.RestClient{
		Client:     http.DefaultClient,
		Url:        "https://localhost:8088",
		Urls:       []string{"localhost"},
		User:       "user",
		VStoreName: "testVStore",
		BackendID:  "backend-test",
	}
	return &client.OceanstorClient{
		RestClient: restClient,
	}
}

var (
	rawSuccessResp = `{
    "data": [
       {
	  "HOMESITEWWN": "testWwn",
	  "vstoreId": "0",
	  "vstoreName": "System_vStore"
	}
    ],
    "error": {
        "code": 0,
        "description": "0"
    }
}`

	notFoundResp = `{
    "error": {
        "code": 0,
        "description": "0"
    }
}`

	wrongDataResp = `{
    "data": "wrong data of lif",
    "error": {
        "code": 0,
        "description": "0"
    }
}`

	errorCodeResp = `{
    "data": [],
    "error": {
        "code": 1077949006,
        "description": "The system is busy."
    }
}`
)

func responseOfLif(rawResp string) base.Response {
	var resp base.Response
	if err := json.Unmarshal([]byte(rawResp), &resp); err != nil {
		fmt.Println(err)
	}
	return resp
}

func TestBaseClient_GetLogicPort(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		// arrange
		cli := mockCli()
		resp := responseOfLif(rawSuccessResp)

		// mock
		patches := gomonkey.ApplyMethodReturn(cli.RestClient, "Get", resp, nil)
		defer patches.Reset()

		// act
		lif, err := cli.GetLogicPort(ctx, cli.GetCurrentLif(ctx))

		// assert
		require.NoError(t, err)
		require.NotEmpty(t, lif)
		require.Equal(t, "testWwn", lif.HomeSiteWwn)
	})

	t.Run("with invalid url", func(t *testing.T) {
		// arrange
		cli := mockCli()
		cli.Url = "an invalid url"

		// act
		lif := cli.GetCurrentLif(ctx)

		// assert
		require.Equal(t, lif, "")
	})

	t.Run("get error from http", func(t *testing.T) {
		// arrange
		cli := mockCli()

		// mock
		patches := gomonkey.ApplyMethodReturn(cli.RestClient, "Get", nil, assert.AnError)
		defer patches.Reset()

		// act
		lif, err := cli.GetLogicPort(ctx, cli.GetCurrentLif(ctx))

		// assert
		require.ErrorIs(t, err, assert.AnError)
		require.Nil(t, lif)
	})

	t.Run("error code of response", func(t *testing.T) {
		// arrange
		cli := mockCli()
		resp := responseOfLif(errorCodeResp)

		// mock
		patches := gomonkey.ApplyMethodReturn(cli.RestClient, "Get", resp, nil)
		defer patches.Reset()

		// act
		lif, err := cli.GetLogicPort(ctx, cli.GetCurrentLif(ctx))

		// assert
		require.ErrorContains(t, err, "The system is busy")
		require.Nil(t, lif)
	})

	t.Run("unmarshal data error", func(t *testing.T) {
		// arrange
		cli := mockCli()
		resp := responseOfLif(wrongDataResp)

		// mock
		patches := gomonkey.ApplyMethodReturn(cli.RestClient, "Get", resp, nil)
		defer patches.Reset()

		// act
		lif, err := cli.GetLogicPort(ctx, cli.GetCurrentLif(ctx))

		// assert
		require.ErrorContains(t, err, "get logic port error")
		require.Nil(t, lif)
	})

	t.Run("logic port not found", func(t *testing.T) {
		// arrange
		cli := mockCli()
		resp := responseOfLif(notFoundResp)

		// mock
		patches := gomonkey.ApplyMethodReturn(cli.RestClient, "Get", resp, nil)
		defer patches.Reset()

		// act
		_, err := cli.GetLogicPort(ctx, cli.GetCurrentLif(ctx))

		// assert
		require.Nil(t, err)
	})
}
