package internal

import (
	"fmt"
	"sync"

	"github.com/renstrom/shortuuid"
)

type BaseTask struct {
	mu    sync.RWMutex
	state TaskState
	data  TaskData
	err   error
	id    string
}

func NewBaseTask() *BaseTask {
	return &BaseTask{
		data: make(TaskData),
		id:   shortuuid.New(),
	}
}

func (t *BaseTask) SetState(state TaskState) {
	t.mu.Lock()
	t.state = state
	t.mu.Unlock()
}

func (t *BaseTask) SetData(key, val string) {
	t.mu.Lock()
	if t.data == nil {
		t.data = make(TaskData)
	}
	t.data[key] = val
	t.mu.Unlock()
}

func (t *BaseTask) Done() {
	if t.err != nil {
		t.SetState(TaskStateFailed)
	} else {
		t.SetState(TaskStateComplete)
	}
}

func (t *BaseTask) Fail(err error) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.err = err
	return err
}

func (t *BaseTask) Result() TaskResult {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stateStr := t.state.String()
	errStr := ""
	if t.err != nil {
		errStr = t.err.Error()
	}

	return TaskResult{
		State: stateStr,
		Error: errStr,
		Data:  t.data,
	}
}

func (t *BaseTask) String() string {
	return fmt.Sprintf("%T: %s", t, t.ID())
}

func (t *BaseTask) ID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.id
}

func (t *BaseTask) State() TaskState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.state
}

func (t *BaseTask) Error() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.err
}
