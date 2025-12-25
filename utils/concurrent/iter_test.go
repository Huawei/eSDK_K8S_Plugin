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

// Package concurrent
package concurrent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ForEach_NormalCase(t *testing.T) {
	// arrange
	ctx := context.Background()
	items := []int{1, 2, 3}
	expectedResults := []Result[int]{
		{Value: 2, Err: nil},
		{Value: 4, Err: nil},
		{Value: 6, Err: nil},
	}

	// action
	gotResults := ForEach(ctx, items, 0, func(ctx context.Context, item int) (int, error) {
		return item * 2, nil
	})

	// assert
	assert.Len(t, gotResults, len(expectedResults))
	for i, expected := range expectedResults {
		assert.Equal(t, expected.Value, gotResults[i].Value)
		assert.Equal(t, expected.Err, gotResults[i].Err)
	}
}

func Test_ForEach_FunctionError(t *testing.T) {
	// arrange
	ctx := context.Background()
	items := []int{1, 2, 3}
	wantErr := errors.New("function error")

	// action
	gotResults := ForEach(ctx, items, 0, func(ctx context.Context, item int) (int, error) {
		return 0, wantErr
	})

	// assert
	assert.Len(t, gotResults, len(items))
	for _, result := range gotResults {
		assert.Equal(t, wantErr, result.Err)
	}
}
