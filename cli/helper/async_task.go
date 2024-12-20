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

	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Task Define interfaces required by asynchronous tasks.
type Task interface {
	Do()
}

// TaskHandler Define asynchronous task processing configuration
type TaskHandler struct {
	task            chan Task
	maxHandlerLimit int
	wg              *sync.WaitGroup
}

// NewTransmitter initialize a TaskHandler instance
func NewTransmitter(maxHandlerNum, taskCapacity int) *TaskHandler {
	return &TaskHandler{
		task:            make(chan Task, taskCapacity),
		maxHandlerLimit: maxHandlerNum,
		wg:              &sync.WaitGroup{},
	}
}

// AddTask add an asynchronous Task
func (t *TaskHandler) AddTask(task Task) {
	t.task <- task
}

// Start Create maxHandlerLimit goroutines to execute the task.
func (t *TaskHandler) Start() {
	t.wg.Add(t.maxHandlerLimit)
	for i := 0; i < t.maxHandlerLimit; i++ {
		go t.handle()
	}
}

func (t *TaskHandler) handle() {
	defer func() {
		if e := recover(); e != nil {
			log.Errorf("an error occurred when executing the async task, error: %v", e)
			go t.handle()
		} else {
			t.wg.Done()
		}
	}()
	for task := range t.task {
		task.Do()
	}
}

// End the adding task.
func (t *TaskHandler) End() {
	close(t.task)
}

// Wait for all tasks to complete
func (t *TaskHandler) Wait() {
	t.End()
	t.wg.Wait()
}
