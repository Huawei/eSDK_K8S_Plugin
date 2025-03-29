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

// Package backend includes the integration tests of backend
package backend

import (
	"testing"

	"github.com/agiledragon/gomonkey/v2"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

func TestMain(m *testing.M) {
	log.MockInitLogging("test")
	defer log.MockStopLogging("test")

	cfg := config.MockCompletedConfig()
	p := gomonkey.NewPatches().ApplyFuncReturn(app.GetGlobalConfig, cfg)
	defer p.Reset()

	m.Run()
}
