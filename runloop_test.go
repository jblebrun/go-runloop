package runloop

import (
	"fmt"
	"runtime"
	"testing"
)

func assertEqual(t *testing.T, expected interface{}, actual interface{}) {
	if expected != actual {
		_, file, line, ok := runtime.Caller(1)
		src := ""
		if ok {
			src = fmt.Sprintf("From %s:%d -- ", file, line)
		}
		t.Errorf("%sAssert failed. Expected\n%s\nbut got\n%s\n", src, expected, actual)
	}
}

func TestRunloopSync(t *testing.T) {

	rl := New()

	var n = 0

	rl.Sync(func() {
		n++
	})

	rl.Wait()

	assertEqual(t, 1, n)

}

func TestRunloopAsync(t *testing.T) {

	rl := New()

	var n = 0
	wait := make(chan struct{})
	done := make(chan struct{})
	rl.Async(func() {
		<-wait
		n++
		close(done)
	})

	assertEqual(t, 0, n)
	close(wait)
	<-done

	assertEqual(t, 1, n)
}

func TestRunloopSyncE(t *testing.T) {
	rl := New()

	var n = 0

	err := rl.SyncE(func() error {
		n++
		return fmt.Errorf("Yup")
	})

	assertEqual(t, 1, n)
	if err.Error() != "Yup" {
		t.Fatalf("Didn't get the error")
	}
}

func TestRunloopQueuing(t *testing.T) {
	rl := New()

	wait := make(chan struct{})
	done := make(chan struct{})

	var n = 0
	for i := 0; i < 50; i++ {
		rl.Async(func() {
			<-wait
			n++
			done <- struct{}{}
		})
	}

	go func() {
		assertEqual(t, 0, n)

		for i := 0; i < 50; i++ {
			wait <- struct{}{}
			<-done
			assertEqual(t, i+1, n)
		}
	}()
	rl.Wait()
}

func TestRunloopPriority(t *testing.T) {
	rl := New()

	first := make(chan struct{})
	next := make(chan struct{})
	second := make(chan struct{})

	// Queue up some tasks
	rl.Async(func() { <-next })
	rl.Async(func() { close(second); <-next })
	rl.Priority().Async(func() { close(first); <-next })

	next <- struct{}{}
	<-first
	next <- struct{}{}
	<-second
	next <- struct{}{}

	rl.Wait()

}
