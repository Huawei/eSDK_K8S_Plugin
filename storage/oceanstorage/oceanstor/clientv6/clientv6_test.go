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

package clientv6

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

var testClient *V6Client

const (
	logName = "clientV6_test.log"

	successStatus int = 200
)

func TestSplitCloneFS(t *testing.T) {
	var cases = []struct {
		name         string
		responseBody string
		wantErr      bool
	}{
		{
			"Normal",
			`{"data": {}, "error": {"code": 0, "description": "0"}}`,
			false,
		},
		{
			"SplitCloneFailed",
			`{"data": {}, "error": {"code": 1, "description": "failed case"}}`,
			true,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBaseClient := client.NewMockHTTPClient(ctrl)

	temp := testClient.OceanstorClient.Client
	defer func() { testClient.OceanstorClient.Client = temp }()
	testClient.OceanstorClient.Client = mockBaseClient

	for _, c := range cases {
		r := ioutil.NopCloser(bytes.NewReader([]byte(c.responseBody)))
		mockBaseClient.EXPECT().Do(gomock.Any()).AnyTimes().Return(&http.Response{
			StatusCode: successStatus,
			Body:       r,
		}, nil)

		err := testClient.SplitCloneFS(context.TODO(), "test", "0", 2, true)
		assert.Equal(t, c.wantErr, err != nil, "Test SplitCloneFSV6 failed, error: %v", err)
	}
}

func TestMain(m *testing.M) {
	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	testClient, _ = NewClientV6(context.Background(), &client.NewClientConfig{
		Urls:            []string{"https://192.168.125.*:8088"},
		User:            "dev-account",
		SecretName:      "mock-sec-name",
		SecretNamespace: "mock-sec-namespace",
		VstoreName:      "dev-vStore",
		ParallelNum:     "",
		BackendID:       "mock-backend-id",
	})

	m.Run()
}
