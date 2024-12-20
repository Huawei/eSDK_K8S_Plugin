/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

package connector

import (
	"context"
	"testing"

	"github.com/prashantv/gostub"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
)

var showArrayOutput = `
-----------------------------------------------------------------------------
Array ID    Array Name          Array SN        Vendor Name  Product Name
0           xx.Storage            xxxx              xx        xx1
1           xx.Storage            xxxxx             xx        xx1
-----------------------------------------------------------------------------
`

func TestFindAllArrays(t *testing.T) {
	// arrange mock
	stub := gostub.New()
	defer stub.Reset()
	stub.StubFunc(&utils.ExecShellCmd, showArrayOutput, nil)

	// action
	arrays, err := FindAllArrays(context.Background())

	// assert
	if err != nil {
		t.Errorf("TestFindAllArrays() failed, error = %v", err)
	}
	if len(arrays) != 2 {
		t.Errorf("TestFindAllArrays() failed, arrays want 2 but got = %d", len(arrays))
	}
}
