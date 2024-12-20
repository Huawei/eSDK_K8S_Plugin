/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

package retry_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/retry"
)

func TestRetry_Success(t *testing.T) {
	// arrange
	attempts := 3
	period := time.Millisecond
	count := 0

	// action
	err := retry.Attempts(attempts).
		Period(period).
		Do(func() error {
			if count == attempts-1 {
				return nil
			}
			count++
			return errors.New("error")
		})

	// assert
	assert.NoError(t, err)
	assert.Equal(t, count, attempts-1)
}

func TestRetry_Error(t *testing.T) {
	// arrange
	attempts := 3
	period := time.Millisecond
	count := 0

	// action
	now := time.Now()
	err := retry.Attempts(attempts).
		Period(period).
		Do(func() error {
			count++
			return errors.New("error")
		})

	// assert
	assert.ErrorContains(t, err, "error")
	assert.Equal(t, count, attempts)
	assert.True(t, time.Since(now) > time.Duration(attempts)*period)
}
