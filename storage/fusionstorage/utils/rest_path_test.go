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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFusionPath_Encode(t *testing.T) {
	// arrange
	dtreePath := NewFusionRestPath("/base/path")
	dtreePath.SetQuery("query-key", "query-value")
	dtreePath.AddFilter("filter-key", "filter-value")
	dtreePath.SetDefaultRange()
	expectedEncodedPath := "/base/path?filter=%5B%7B%22filter-key%22%3A%22filter-value%22%7D%5D&" +
		"query-key=query-value&range=%7B%22offset%22%3A0%2C%22limit%22%3A100%7D"

	// action
	encodedPath, err := dtreePath.Encode()

	// assert
	require.NoError(t, err)
	require.Equal(t, encodedPath, expectedEncodedPath)
}
