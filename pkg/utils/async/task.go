package async

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
