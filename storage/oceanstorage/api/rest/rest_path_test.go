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

// Package rest provides operations for rest path
package rest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestPath_Encode_InvalidPath(t *testing.T) {
	// arrange
	rp := NewRequestPath("invalid%%path")

	// act
	gotURL, gotErr := rp.Encode()

	// assert
	assert.Equal(t, "", gotURL)
	assert.ErrorContains(t, gotErr, "invalid")
}

func TestRequestPath_Encode_Success(t *testing.T) {
	// arrange
	path := NewRequestPath("/base/path")
	path.SetQuery("query-key", "query-value")
	path.AddFilter("filter-key", "filter-value")
	path.SetDefaultListRange()
	expectedEncodedPath := "/base/path?filter=filter-key%3A%3Afilter-value&query-key=query-value&range=%5B0-100%5D"

	// act
	encodedPath, err := path.Encode()

	// assert
	require.NoError(t, err)
	require.Equal(t, expectedEncodedPath, encodedPath)
}
