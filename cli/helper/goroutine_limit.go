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

	"huawei-csi-driver/utils/log"
)

// GlobalGoroutineLimit Configure the total goroutine limit
type GlobalGoroutineLimit struct {
	localNum        int32
	maxGoroutineNum int32
	limit           int32
	allCond         []*sync.Cond
}

// LocalGoroutineLimit Record local goroutine limit under total limit
type LocalGoroutineLimit struct {
	cond                *sync.Cond
	currentGoroutineNum int32
	limit               *int32
}

// NewGlobalGoroutineLimit initialize a GlobalGoroutineLimit instance
func NewGlobalGoroutineLimit(maxGoroutineNum int32) *GlobalGoroutineLimit {
	res := &GlobalGoroutineLimit{
		allCond:         make([]*sync.Cond, maxGoroutineNum),
		maxGoroutineNum: maxGoroutineNum,
		limit:           maxGoroutineNum,
	}
	return res
}

// NewLocalGoroutineLimit initialize a LocalGoroutineLimit instance
func NewLocalGoroutineLimit(global *GlobalGoroutineLimit) *LocalGoroutineLimit {
	res := &LocalGoroutineLimit{
		limit: &global.limit,
		cond:  sync.NewCond(&sync.Mutex{}),
	}
	atomic.AddInt32(&global.localNum, 1)
	atomic.StoreInt32(&global.limit, global.maxGoroutineNum/global.localNum)
	global.allCond = append(global.allCond, res.cond)
	return res
}

// Do create a Goroutine to Execution function
func (l *LocalGoroutineLimit) Do(f func()) {
	l.cond.L.Lock()
	for atomic.LoadInt32(&l.currentGoroutineNum) >= atomic.LoadInt32(l.limit) {
		l.cond.Wait()
	}
	l.cond.L.Unlock()
	atomic.AddInt32(&l.currentGoroutineNum, 1)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("an error occurred when executing the sub-goroutine, error: %v", e)
			}
			atomic.AddInt32(&l.currentGoroutineNum, -1)
			l.cond.Signal()
		}()
		f()
	}()
}

// Update LocalGoroutine number to Decrease the limit per LocalGoroutine
func (g *GlobalGoroutineLimit) Update() {
	atomic.AddInt32(&g.localNum, -1)
	if atomic.LoadInt32(&g.localNum) == 0 {
		return
	}
	atomic.StoreInt32(&g.limit, g.maxGoroutineNum/atomic.LoadInt32(&g.localNum))
	for _, cond := range g.allCond {
		if cond != nil {
			cond.Signal()
		}
	}
}
