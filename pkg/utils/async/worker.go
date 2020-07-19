package async

import (
	"github.com/gammazero/workerpool"
)

type WorkerPool struct {
	wp    *workerpool.WorkerPool
	tasks []*Task
}

func NewWorkerPool() *WorkerPool {
	return NewWorkerPoolWithSize(10)
}

func NewWorkerPoolWithSize(size int) *WorkerPool {
	return &WorkerPool{
		wp: workerpool.New(size),
	}
}

func (p *WorkerPool) AddTask(taskFunc func() (interface{}, error)) {
	task := NewTask(taskFunc)
	p.tasks = append(p.tasks, task)
	p.wp.Submit(task.Run)
}

func (p *WorkerPool) GetTasks() []*Task {
	return p.tasks
}

func (p *WorkerPool) StopWait() {
	p.wp.StopWait()
}

// a task wraps a function and its response
type Task struct {
	f      func() (interface{}, error)
	Err    error
	Result interface{}
}

func NewTask(f func() (interface{}, error)) *Task {
	return &Task{
		f: f,
	}
}

func (t *Task) Run() {
	t.Result, t.Err = t.f()
}
