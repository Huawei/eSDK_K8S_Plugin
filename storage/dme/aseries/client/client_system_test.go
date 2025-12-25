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

// Package client provides DME A-series storage client
package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemClient_GetHyperScalePoolByName_Success(t *testing.T) {
	// arrange
	successResp := `
		{
			"total": 1,
			"data": [
				{
					"id": "aaa",
					"name": "bbb",
					"total_capacity": 4194304,
					"capacity_usage": 0.13,
					"free_capacity": 4188568.95488
				}
			]
		}
	`

	// Mock
	systemCli1 := SystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	pool1, err := systemCli1.GetHyperScalePoolByName(context.Background(), "bbb")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, pool1.ID, "aaa")

	// Mock
	systemCli2 := SystemClient{BaseClientInterface: getMockClient(200, successResp)}

	// Action
	pool2, err := systemCli2.GetHyperScalePoolByName(context.Background(), "ccc")

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, pool2)
}

func TestSystemClient_GetHyperScalePoolByName_Error(t *testing.T) {
	// Mock
	systemCli := SystemClient{BaseClientInterface: getMockClient(200, "")}

	// Action
	pool, err := systemCli.GetHyperScalePoolByName(context.Background(), "aaa")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, pool)
}
