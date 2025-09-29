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

// Package client provides fusion storage client
package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRestClient_IsSupportDynamicLinks_Success(t *testing.T) {
	// arrange
	respBody := `{"result":0,"hostList":[],"newIscsi":true}`

	// mock
	mockClient := getMockClient(200, respBody)

	// act
	resp, err := mockClient.IsSupportDynamicLinks(context.Background(), "test")

	// assert
	assert.Equal(t, true, resp)
	assert.Nil(t, err)
}

func TestRestClient_IsSupportDynamicLinks_Error(t *testing.T) {
	// arrange
	respBody := `{"result":2,"suggestion":"Check whether the input parameters are correct.","errorCode":37100170,
"description":"The input parameters are incorrect.","i18n_description":"The input parameters are incorrect."}`

	// mock
	mockClient := getMockClient(200, respBody)

	// act
	resp, err := mockClient.IsSupportDynamicLinks(context.Background(), "test")

	// assert
	assert.Equal(t, false, resp)
	assert.ErrorContains(t, err, "The input parameters are incorrect")
}

func TestRestClient_QueryDynamicLinks_Success(t *testing.T) {
	// arrange
	respBody := `{"result":0,"iscsiLinks":[{"ip":"test","iscsiLinksNum":1,
"targetName":"test_target","iscsiPortal":"portal_ip"}]}`

	// mock
	mockClient := getMockClient(200, respBody)

	// act
	resp, err := mockClient.QueryDynamicLinks(context.Background(), "pool", "host", 1)

	// assert
	assert.Equal(t, 1, len(resp))
	assert.Equal(t, "test", resp[0].IP)
	assert.Nil(t, err)
}

func TestRestClient_QueryDynamicLinks_Error(t *testing.T) {
	// arrange
	respBody := `{"result":2,"suggestion":"Check whether the input parameters are correct.","errorCode":37100170,
"description":"The input parameters are incorrect.","i18n_description":"The input parameters are incorrect."}`

	// mock
	mockClient := getMockClient(200, respBody)

	// act
	resp, err := mockClient.QueryDynamicLinks(context.Background(), "pool", "host", 1)

	// assert
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "The input parameters are incorrect")
}
