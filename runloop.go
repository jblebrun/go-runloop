package runloop

import (
	"container/list"
	"sync"
	"time"

	timeutil "github.com/jblebrun/go-timeutil"
)

type KVP struct {
	Key   string
	Value string
}

type task struct {
	op   func()
	info []KVP
}

type rlCore struct {
	cond      *sync.Cond
	runningWg *sync.WaitGroup
	tasks     *list.List
	ptasks    *list.List
	running   bool
	watcher   interface {
		Enter(func(), []KVP)
		Exit()
	}
}

// This structure is how we handle multiple priorities.
// To change the priority that a task is issued to,
// call a function that returns a RunLoop w/ a different
// target list exposed.
type RunLoop struct {
	*rlCore
	target *list.List
}

func New() *RunLoop {
	tasks := list.New()
	ptasks := list.New()
	r := &RunLoop{
		rlCore: &rlCore{
			cond:      sync.NewCond(&sync.Mutex{}),
			runningWg: &sync.WaitGroup{},
			tasks:     tasks,
			ptasks:    ptasks,
		},
		target: tasks,
	}
	go r.run()
	return r
}

func NewWatched(timeout time.Duration, onTimeout TimeoutFunc) *RunLoop {
	r := New()
	t := time.NewTimer(timeout)
	timeutil.StopTimerSafely(t)
	r.watcher = NewWatchdog(onTimeout, timeout, 4)
	return r
}

func (rl *RunLoop) getTask() *task {
	action := rl.ptasks.Front()
	if action != nil {
		return rl.ptasks.Remove(action).(*task)
	}
	action = rl.tasks.Front()
	if action != nil {
		return rl.tasks.Remove(action).(*task)
	}
	return nil

}

func (rl *RunLoop) run() {
	for {
		rl.cond.L.Lock()
		var t *task
		for t = rl.getTask(); t == nil; t = rl.getTask() {
			rl.cond.Wait()
		}
		rl.cond.L.Unlock()
		if rl.watcher != nil {
			rl.watcher.Enter(t.op, t.info)
		}
		t.op()
		rl.runningWg.Done()
		if rl.watcher != nil {
			rl.watcher.Exit()
		}
	}
}

func (rl *RunLoop) Wait() {
	rl.runningWg.Wait()
}

func (rl *RunLoop) submit(f func(), info []KVP) {
	rl.cond.L.Lock()
	rl.target.PushBack(&task{f, info})
	rl.runningWg.Add(1)
	rl.cond.Signal()
	rl.cond.L.Unlock()
}

// Dispatch API
func (rl *RunLoop) Priority() *RunLoop {
	return &RunLoop{rl.rlCore, rl.ptasks}
}

func (rl *RunLoop) Async(op func(), info ...KVP) {
	rl.submit(op, info)
}

func (rl *RunLoop) Sync(op func(), info ...KVP) {
	done := make(chan struct{})
	rl.submit(func() { op(); close(done) }, info)
	<-done
}

func (rl *RunLoop) SyncE(op func() error, info ...KVP) (err error) {
	done := make(chan error)
	rl.submit(func() { done <- op() }, info)
	return <-done
}

func (rl *RunLoop) Expecting() {
	rl.cond.L.Lock()
	rl.runningWg.Add(1)
	rl.cond.L.Unlock()
}

func (rl *RunLoop) DoneExpecting() {
	rl.runningWg.Done()
}
