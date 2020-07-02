package taskflow

import (
	"utils"
	"utils/log"
)

type TaskRunFunc func(params map[string]interface{}, result map[string]interface{}) (map[string]interface{}, error)
type TaskRevertFunc func(result map[string]interface{}) error

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
}

func NewTaskFlow(name string) *TaskFlow {
	return &TaskFlow{
		name:   name,
		result: make(map[string]interface{}),
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
	log.Infof("Start to run taskflow %s", p.name)

	for _, task := range p.tasks {
		result, err := task.run(params, p.result)
		if err != nil {
			log.Errorf("Run task %s of taskflow %s error: %v", task.name, p.name, err)
			return nil, err
		}

		task.finish = true

		if result != nil {
			p.result = utils.MergeMap(p.result, result)
		}
	}

	log.Infof("Taskflow %s is finished", p.name)
	return p.result, nil
}

func (p *TaskFlow) GetResult() map[string]interface{} {
	return p.result
}

func (p *TaskFlow) Revert() {
	log.Infof("Start to revert taskflow %s", p.name)

	for i := len(p.tasks) - 1; i >= 0; i-- {
		task := p.tasks[i]

		if task.finish && task.revert != nil {
			err := task.revert(p.result)
			if err != nil {
				log.Warningf("Revert task %s of taskflow %s error: %v", task.name, p.name, err)
			}
		}
	}

	log.Infof("Taskflow %s is reverted", p.name)
}
