/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2024-2024. All rights reserved.
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

package taskflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"huawei-csi-driver/utils/taskflow"
)

func TestTransaction_NoError(t *testing.T) {
	// arrange
	var i int
	transaction := taskflow.NewTransaction().
		Then(
			func() error {
				i++
				return nil
			},
			func() { i-- },
		).
		Then(
			func() error {
				i += 2
				return nil
			},
			func() { i -= 2 },
		)

	// act
	err := transaction.Commit()

	// assert
	require.NoError(t, err)
	require.Equal(t, 3, i)
}

func TestTransaction_WithError(t *testing.T) {
	// arrange
	var i int
	transaction := taskflow.NewTransaction().
		Then(
			func() error {
				return assert.AnError
			},
			func() { i-- },
		)

	// act
	err := transaction.Commit()

	// assert
	require.ErrorIs(t, err, assert.AnError)
}

func TestTransaction_Rollback(t *testing.T) {
	// arrange
	var i int
	transaction := taskflow.NewTransaction().
		Then(
			func() error {
				i++
				return nil
			},
			func() { i-- },
		).
		Then(
			func() error {
				i += 2
				return nil
			},
			func() { i -= 2 },
		).
		Then(
			func() error {
				return assert.AnError
			},
			func() { i-- },
		)

	// act
	err := transaction.Commit()
	transaction.Rollback()

	// assert
	require.ErrorIs(t, err, assert.AnError)
	require.Equal(t, 0, i)
}
