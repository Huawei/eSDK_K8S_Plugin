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

package concurrent

import (
	"context"
	"sync"
)

// Result is the results of task.
type Result[T any] struct {
	Value T
	Err   error
}

// ForEach iterates over elements of list and invokes iter function for each element.
func ForEach[T any, R any](ctx context.Context, items []T, maxWorker int,
	fn func(context.Context, T) (R, error)) []Result[R] {
	results := make([]Result[R], len(items))
	if maxWorker <= 0 {
		maxWorker = len(items)
	}
	sem := make(chan struct{}, maxWorker)

	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i].Err = ctx.Err()
				return
			default:
			}

			results[i].Value, results[i].Err = fn(ctx, item)
		}()
	}

	wg.Wait()
	return results
}
