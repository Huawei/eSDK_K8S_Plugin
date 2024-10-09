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

package concurrent

import (
	"fmt"
	"sync"
)

// Slice is a slice wrapper for safe concurrency
type Slice[T any] struct {
	mu    sync.RWMutex
	slice []T
}

// Len gets the length of Slice
func (s *Slice[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.slice)
}

// Get gets the value of given index,
// NOTE that if index is out of range, will return zero value of T.
func (s *Slice[T]) Get(index int) T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if index < 0 || index >= len(s.slice) {
		var zero T
		return zero
	}

	return s.slice[index]
}

// Values gets a copy values of the base slice
func (s *Slice[T]) Values() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copySlice := make([]T, 0, len(s.slice))
	for _, v := range s.slice {
		copySlice = append(copySlice, v)
	}

	return copySlice
}

// Append appends elements to the end of the Slice
func (s *Slice[T]) Append(e ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.slice = append(s.slice, e...)
}

// Cut cuts the Slice by the given low and high index
func (s *Slice[T]) Cut(low, high int) error {
	if low > high {
		return fmt.Errorf("low index is greater than high index")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if low < 0 || low > len(s.slice) || high < 0 || high > len(s.slice) {
		return fmt.Errorf("index out of range [%d,%d] with length: %d", low, high, len(s.slice))
	}
	s.slice = s.slice[low:high]
	return nil
}

// Reset sets the base slice to nil
func (s *Slice[T]) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.slice = nil
}
