/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2025. All rights reserved.
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

package volume

import (
	"testing"

	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	cfg "github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app/config"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	logName = "volume_dTree_test.log"
)

func TestMain(m *testing.M) {
	getGlobalConfig := gostub.StubFunc(&app.GetGlobalConfig, cfg.MockCompletedConfig())
	defer getGlobalConfig.Reset()

	log.MockInitLogging(logName)
	defer log.MockStopLogging(logName)

	m.Run()
}

func TestFormatKerberosParam(t *testing.T) {
	type TestFormatKerberosParam struct {
		target   interface{}
		expected int
	}
	var testCases = []TestFormatKerberosParam{
		{
			target:   "read_only",
			expected: 0,
		},
		{
			target:   "read_write",
			expected: 1,
		},
		{
			target:   "none",
			expected: 5,
		},
		{
			target:   "",
			expected: -1,
		},
		{
			target:   nil,
			expected: -1,
		},
	}

	for _, c := range testCases {
		assert.Equal(t, c.expected, formatKerberosParam(c.target))
	}
}
