package runloop

import (
	"reflect"
	"runtime"
	"sync"
	"time"

	timeutil "github.com/jblebrun/go-timeutil"
)

type TimeoutFunc func(timeoutAt time.Time, funcName string, info []KVP)

type Watchdog struct {
	sync.Mutex
	ticks       uint32
	divs        uint32
	running     bool
	period      time.Duration
	timer       timeutil.Timer
	onTimeout   TimeoutFunc
	currentFunc func()
	currentInfo []KVP
}

func fname(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// When you call `Enter` the watchdog starts
// If it ticks `divs` times during `timeout` before `Exit` is called
// `onTimeout` is called.
func NewWatchdog(onTimeout TimeoutFunc, timeout time.Duration, divs uint32) *Watchdog {

	wd := &Watchdog{
		divs:      divs,
		period:    timeout / time.Duration(divs),
		onTimeout: onTimeout,
	}
	wd.timer = timeutil.AfterFunc(-1, wd.Check)
	return wd
}

func (wd *Watchdog) Enter(f func(), info []KVP) {
	wd.Lock()
	wd.running = true
	wd.currentFunc = f
	wd.currentInfo = info
	wd.ticks = 0
	wd.timer.Reset(wd.period)
	wd.Unlock()
}

func (wd *Watchdog) Exit() {
	wd.Lock()
	wd.running = false
	wd.currentFunc = nil
	wd.ticks = 0
	wd.timer.Stop()
	wd.Unlock()
}

func (wd *Watchdog) Check() {
	wd.Lock()
	defer wd.Unlock()
	if !wd.running {
		// We don't want to reset the timer in this case, if we get here
		// then the timer had already fired after a stop was called.
		return
	}
	wd.ticks++
	if wd.ticks == wd.divs {
		// Don't reset the timer: only fire once per operation.
		wd.onTimeout(time.Now(), fname(wd.currentFunc), wd.currentInfo)
	} else {
		wd.timer.Reset(wd.period)
	}
}
