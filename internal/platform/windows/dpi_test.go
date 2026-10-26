package winplat

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestEnsureOnceRunsExactlyOnce(t *testing.T) {
	var n atomic.Int32
	run := ensureOnce(func() { n.Add(1) })
	if n.Load() != 0 {
		t.Fatalf("ensureOnce must not call fn eagerly")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); run() }()
	}
	wg.Wait()
	run()
	if got := n.Load(); got != 1 {
		t.Fatalf("fn ran %d times, want 1", got)
	}
}

func TestEnsureOnceIndependentInstances(t *testing.T) {
	var a, b int
	ra, rb := ensureOnce(func() { a++ }), ensureOnce(func() { b++ })
	ra()
	ra()
	rb()
	if a != 1 || b != 1 {
		t.Fatalf("a=%d b=%d, want 1 1", a, b)
	}
}
