/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2022. All rights reserved.
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

package taskflow

import (
	"context"

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
)

type TaskRunFunc func(ctx context.Context, params map[string]interface{}, result map[string]interface{}) (map[string]interface{}, error)
type TaskWithoutRevert func(ctx context.Context, params map[string]interface{}) error
type TaskRevertFunc func(ctx context.Context, result map[string]interface{}) error

type Task struct {
	name   string
	finish bool
	run    TaskRunFunc
	revert TaskRevertFunc
}

type TaskFlow struct {
	name   string
	tasks  []*Task
	result map[string]interface{}
	ctx    context.Context
}

func NewTaskFlow(ctx context.Context, name string) *TaskFlow {
	return &TaskFlow{
		name:   name,
		result: make(map[string]interface{}),
		ctx:    ctx,
	}
}

func (p *TaskFlow) AddTask(name string, run TaskRunFunc, revert TaskRevertFunc) {
	p.tasks = append(p.tasks, &Task{
		name:   name,
		finish: false,
		run:    run,
		revert: revert,
	})
}

func (p *TaskFlow) Run(params map[string]interface{}) (map[string]interface{}, error) {
	log.AddContext(p.ctx).Infof("Start to run taskflow %s", p.name)

	for _, task := range p.tasks {
		result, err := task.run(p.ctx, params, p.result)
		if err != nil {
			log.AddContext(p.ctx).Errorf("Run task %s of taskflow %s error: %v", task.name, p.name, err)
			return nil, err
		}

		task.finish = true

		if result != nil {
			p.result = utils.MergeMap(p.result, result)
		}
	}

	log.AddContext(p.ctx).Infof("Taskflow %s is finished", p.name)
	return p.result, nil
}

func (p *TaskFlow) GetResult() map[string]interface{} {
	return p.result
}

func (p *TaskFlow) Revert() {
	log.AddContext(p.ctx).Infof("Start to revert taskflow %s", p.name)

	for i := len(p.tasks) - 1; i >= 0; i-- {
		task := p.tasks[i]

		if task.finish && task.revert != nil {
			err := task.revert(p.ctx, p.result)
			if err != nil {
				log.AddContext(p.ctx).Warningf("Revert task %s of taskflow %s error: %v", task.name, p.name, err)
			}
		}
	}

	log.AddContext(p.ctx).Infof("Taskflow %s is reverted", p.name)
}

// AddTaskWithOutRevert be used when the task does not need revert function
func (p *TaskFlow) AddTaskWithOutRevert(run TaskWithoutRevert) *TaskFlow {
	var buildFun = func(ctx context.Context, params map[string]interface{},
		_ map[string]interface{}) (map[string]interface{}, error) {
		if err := run(ctx, params); err != nil {
			return nil, err
		}
		return nil, nil
	}
	p.AddTask("", buildFun, nil)
	return p
}

// RunWithOutRevert run task without revert function and return only error
func (p *TaskFlow) RunWithOutRevert(params map[string]interface{}) error {
	if _, err := p.Run(params); err != nil {
		return err
	}
	return nil
}
