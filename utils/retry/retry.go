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

// Package retry provides a simple retry mechanism
package retry

import (
	"time"
)

type attempt struct {
	attempts int
	period   time.Duration
}

// Attempts sets the number of retry attempts
func Attempts(attempts int) *attempt {
	return &attempt{
		attempts: attempts,
	}
}

// Period sets the period of each retry attempt
func (r *attempt) Period(period time.Duration) *attempt {
	r.period = period
	return r
}

// Do run the retry function
func (r *attempt) Do(do func() error) error {
	var err error
	for i := 0; i < r.attempts; i++ {
		if err = do(); err == nil {
			return nil
		}

		time.Sleep(r.period)
	}

	return err
}
