package runloop

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	timeutil "github.com/jblebrun/go-timeutil"
)

type RunloopTest struct {
	tm      timeutil.TestTimeModule
	rl      *RunLoop
	timeout chan time.Time
}

func newTest(validate func(funcName string, info []KVP)) *RunloopTest {
	timeout := make(chan time.Time, 10)
	return &RunloopTest{
		tm:      timeutil.NewTest(),
		timeout: timeout,
		rl: NewWatched(5*time.Millisecond, func(t time.Time, funcName string, info []KVP) {
			validate(funcName, info)
			timeout <- t
		}),
	}
}

func (rlt *RunloopTest) dispatchBlocker(kvp []KVP) chan struct{} {
	done := make(chan struct{})
	inside := make(chan struct{})
	rlt.rl.Async(func() {
		close(inside)
		<-done
	}, kvp...)
	<-inside
	return done
}

func (rlt *RunloopTest) awaitTimeout() error {
	select {
	case <-rlt.timeout:
		return nil
	case <-time.After(1000 * time.Millisecond):
		return fmt.Errorf("No timeout")
	}
}

func (rlt *RunloopTest) expectNoTimeout() error {
	select {
	case <-rlt.timeout:
		return fmt.Errorf("Got unexpected timeout")
	default:
		return nil
	}
}

func (rlt *RunloopTest) tickBy(total time.Duration, divs int) {
	for i := 0; i < divs; i++ {
		rlt.tm.Advance(total / time.Duration(divs))
		runtime.Gosched()
	}
}

func (rlt *RunloopTest) flush() {
	rlt.rl.Sync(func() {})
}

func (rlt *RunloopTest) noTimeoutCase() error {
	done := rlt.dispatchBlocker(nil)
	rlt.tickBy(3*time.Millisecond, 100)
	if e := rlt.expectNoTimeout(); e != nil {
		return e
	}
	close(done)
	rlt.flush()
	rlt.tickBy(50*time.Millisecond, 100)
	return rlt.expectNoTimeout()
}

func (rlt *RunloopTest) timeoutCase(kvp []KVP) error {
	done := rlt.dispatchBlocker(kvp)
	rlt.tickBy(10*time.Millisecond, 100)
	if e := rlt.awaitTimeout(); e != nil {
		return e
	}
	close(done)
	rlt.flush()
	rlt.tickBy(50*time.Millisecond, 100)
	return rlt.expectNoTimeout()
}

func TestRunloopNoTimeout(t *testing.T) {
	for i := 0; i < 20; i++ {
		rlt := newTest(func(funcName string, info []KVP) {
			t.Fatalf("shouldn't have timed out (%s) (%v)", funcName, info)
		})
		if err := rlt.noTimeoutCase(); err != nil {
			t.Fatal(err)
		}
	}
}

func checkKvpAndFunc(t *testing.T, kvp []KVP, funcName string, info []KVP) {
	if !strings.HasSuffix(funcName, "(*RunloopTest).dispatchBlocker.func1") {
		t.Fatalf("Wrong function name in timeout: %v", funcName)
	}
	if len(info) != 1 {
		t.Fatal("No info in timeout")
	}
	if info[0] != kvp[0] {
		t.Fatalf("Wrong info in timeout: %v is not %v", info[0], kvp[0])
	}
}

func TestRunloopTimeout(t *testing.T) {
	for i := 0; i < 20; i++ {
		kvp := []KVP{KVP{"a", fmt.Sprintf("%d", i)}}
		rlt := newTest(func(funcName string, info []KVP) {
			checkKvpAndFunc(t, kvp, funcName, info)
		})
		if err := rlt.timeoutCase(kvp); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunloopRecovery(t *testing.T) {
	for i := 0; i < 20; i++ {
		kvp := []KVP{KVP{"b", fmt.Sprintf("%d", i)}}
		rlt := newTest(func(funcName string, info []KVP) {
			checkKvpAndFunc(t, kvp, funcName, info)
		})
		if err := rlt.timeoutCase(kvp); err != nil {
			t.Fatal(err)
		}
		if err := rlt.noTimeoutCase(); err != nil {
			t.Fatal(err)
		}
	}
}
