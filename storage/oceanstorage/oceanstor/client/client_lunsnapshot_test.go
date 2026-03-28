/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2026-2026. All rights reserved.
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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateHyperMetroSnapshot_Succeed(t *testing.T) {
	// arrange
	snapName := "mock-snapName"
	pairID := "mock-pairID"
	successRespBody := `
		{ 
			"data": { "localSnapId": "1", "remoteSnapId": "1" }, 
			"error": { "code": 0, "description": "0" }
		}`

	// mock
	mockClient := getMockClient(http.StatusOK, successRespBody)

	// action
	snap, err := mockClient.CreateHyperMetroSnap(context.Background(), snapName, pairID)

	// assert
	assert.NoError(t, err)
	assert.Contains(t, snap, "localSnapId")
	assert.Contains(t, snap, "remoteSnapId")
}

func TestCreateHyperMetroSnapshot_Failed(t *testing.T) {
	// arrange
	snapName := "mock-snapName"
	pairID := "mock-pairID"
	successRespBody := `
		{ 
			"data": { "localSnapId": "1", "remoteSnapId": "1" }, 
			"error": { "code": -1, "description": "0" }
		}`

	// mock
	mockClient := getMockClient(http.StatusOK, successRespBody)

	// action
	_, err := mockClient.CreateHyperMetroSnap(context.Background(), snapName, pairID)

	// assert
	assert.Error(t, err)
}
