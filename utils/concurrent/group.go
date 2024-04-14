/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
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

// Package concurrent used to process concurrent request
package concurrent

import (
	"sync"
)

type singleEntry[T any] struct {
	value T
	err   error
	wg    sync.WaitGroup
}

type singleGroup[T any] struct {
	locker sync.Mutex
	cache  map[string]*singleEntry[T]
}

// NewSingleGroup instance singleGroup
func NewSingleGroup[T any]() *singleGroup[T] {
	return &singleGroup[T]{}
}

// Do the same key can be invoked only once at a time.
func (sg *singleGroup[T]) Do(key string, fn func() (T, error)) (T, error) {
	sg.locker.Lock()
	// lazily initialized
	if sg.cache == nil {
		sg.cache = map[string]*singleEntry[T]{}
	}

	if call, ok := sg.cache[key]; ok {
		sg.locker.Unlock()
		call.wg.Wait()
		return call.value, call.err
	}

	call := new(singleEntry[T])
	call.wg.Add(1)
	sg.cache[key] = call
	sg.locker.Unlock()

	call.value, call.err = fn()
	call.wg.Done()

	sg.locker.Lock()
	defer sg.locker.Unlock()
	delete(sg.cache, key)

	return call.value, call.err
}
