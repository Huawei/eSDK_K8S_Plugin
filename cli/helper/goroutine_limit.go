/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
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

	"huawei-csi-driver/utils/log"
)

// GlobalGoroutineLimit is used to limit concurrency of goroutine
type GlobalGoroutineLimit struct {
	sem chan struct{}
	wg  *sync.WaitGroup
}

// NewGlobalGoroutineLimit initialize a GlobalGoroutineLimit instance
func NewGlobalGoroutineLimit(maxGoroutineNum int) *GlobalGoroutineLimit {
	res := &GlobalGoroutineLimit{
		sem: make(chan struct{}, maxGoroutineNum),
		wg:  &sync.WaitGroup{},
	}
	return res
}

// HandleWork handle the work func with limit
func (n *GlobalGoroutineLimit) HandleWork(work func()) {
	go func() {
		n.sem <- struct{}{}
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("an error occurred when executing the work goroutine, error: %v", e)
			}

			<-n.sem
			n.wg.Done()
		}()
		work()
	}()
}

// AddWork add the work num for wait
func (n *GlobalGoroutineLimit) AddWork(num int) {
	n.wg.Add(num)
}

// Wait wait all works done
func (n *GlobalGoroutineLimit) Wait() {
	n.wg.Wait()
}
