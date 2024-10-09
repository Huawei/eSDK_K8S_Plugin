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

package flow

type transactionStep struct {
	exec       func() error
	onRollback func()
}

// Transaction implements a TCC (Try Confirm Cancel) pattern.
type Transaction struct {
	stepAt int
	steps  []transactionStep
}

// NewTransaction instantiate a new transaction.
func NewTransaction() *Transaction {
	return &Transaction{
		steps: []transactionStep{},
	}
}

// Then adds a step to the steps chain, and returns the same Transaction,
func (t *Transaction) Then(exec func() error, onRollback func()) *Transaction {
	t.steps = append(t.steps, transactionStep{
		exec:       exec,
		onRollback: onRollback,
	})
	return t
}

// Commit executes the Transaction steps and returns error if any one step returns error.
func (t *Transaction) Commit() error {
	var err error

	for t.stepAt < len(t.steps) {
		if t.stepAt >= 0 && t.stepAt < len(t.steps) && t.steps[t.stepAt].exec != nil {
			if err = t.steps[t.stepAt].exec(); err != nil {
				break
			}
		}

		t.stepAt++
	}

	return err
}

// Rollback executes the Transaction rollbacks.
func (t *Transaction) Rollback() {
	for i := t.stepAt - 1; i >= 0; i-- {
		if i < len(t.steps) && t.steps[i].onRollback != nil {
			t.steps[i].onRollback()
		}
	}
}
