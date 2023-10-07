/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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
	"testing"

	"github.com/prashantv/gostub"
	. "github.com/smartystreets/goconvey/convey"

	"huawei-csi-driver/csi/app"
	cfg "huawei-csi-driver/csi/app/config"
	"huawei-csi-driver/utils/log"
)

var (
	testClient *Client
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
		ParallelNum:     "",
		BackendID:       "mock-backend-id",
		AccountName:     "dev-account",
		UseCert:         false,
		CertSecretMeta:  "",
	}

	testClient = NewClient(clientConfig)

	m.Run()
}

func TestGetErrorCode(t *testing.T) {
	Convey("Normal case", t, func() {
		errCode, err := getErrorCode(map[string]any{
			"name":       "mock-name",
			"errorCode":  12345.0,
			"suggestion": "mock-suggestion",
		})
		So(errCode, ShouldEqual, 12345)
		So(err, ShouldBeNil)
	})

	Convey("Error code is string ", t, func() {
		errCode, err := getErrorCode(map[string]any{
			"name":       "mock-name",
			"errorCode":  "12345",
			"suggestion": "mock-suggestion",
		})
		So(errCode, ShouldEqual, 12345)
		So(err, ShouldBeNil)
	})

	Convey("Error code in result", t, func() {
		errCode, err := getErrorCode(map[string]any{
			"result": map[string]any{
				"code": 12345.0,
			},
			"data": map[string]any{
				"name": "mock-name",
			},
		})
		So(errCode, ShouldEqual, 12345)
		So(err, ShouldBeNil)
	})

	Convey("Can not convert to int", t, func() {
		_, err := getErrorCode(map[string]any{
			"result": map[string]any{
				"code": "a12345",
			},
			"data": map[string]any{
				"name": "mock-name",
			},
		})
		So(err, ShouldBeError)
	})
}
