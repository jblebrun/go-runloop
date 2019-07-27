package runloop

import (
	"testing"
)

func BenchmarkRunloop(b *testing.B) {
	rl := New()
	var x = 0
	for n := 0; n < b.N; n++ {
		rl.Async(func() {
			x++
		})
	}
	rl.Wait()
	if x != b.N {
		b.Fatalf("Wrong value %d", x)
	}
}

func BenchmarkRunloopSync(b *testing.B) {
	rl := New()
	var x = 0
	for n := 0; n < b.N; n++ {
		rl.Sync(func() {
			x++
		})
	}
	rl.Wait()
	if x != b.N {
		b.Fatalf("Wrong value %d", x)
	}
}

type action struct {
	n *int
}

func (a *action) Inc() {
	(*a.n)++
}
func BenchmarkDispatchStruct(b *testing.B) {
	c := make(chan *action, 50)
	done := make(chan struct{})
	go func() {
		for a := range c {
			a.Inc()
		}
		close(done)
	}()
	var x = 0
	for n := 0; n < b.N; n++ {
		c <- &action{&x}
	}
	close(c)
	<-done
	if x != b.N {
		b.Fatalf("Wrong value %d", x)
	}
}

func BenchmarkCallFunc(b *testing.B) {
	var x = 0
	for n := 0; n < b.N; n++ {
		f := func(n *int) {
			(*n)++
		}
		f(&x)
	}
	if x != b.N {
		b.Fatalf("Wrong value %d", x)
	}
}

type Iface interface {
	Incr(n *int)
}

type impl struct{}

func (i impl) Incr(n *int) {
	(*n)++
}

func BenchmarkCallVirtual(b *testing.B) {
	var x = 0
	for n := 0; n < b.N; n++ {
		var i Iface = impl{}
		i.Incr(&x)
	}
	if x != b.N {
		b.Fatalf("Wrong value %d", x)
	}
}
