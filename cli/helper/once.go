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

package helper

import (
	"sync"
	"sync/atomic"
)

// Once same as sync.Once, but the Do() method is overridden.
type Once struct {
	done uint32
	m    sync.Mutex
}

// Do same as sync.Once.Do(), but you can determine whether the execution is successful by returning error.
func (o *Once) Do(f func() error) {
	if atomic.LoadUint32(&o.done) == 0 {
		o.doSlow(f)
	}
}

func (o *Once) doSlow(f func() error) {
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		if f() == nil {
			atomic.StoreUint32(&o.done, 1)
		}
	}
}
