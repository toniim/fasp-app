package countdown

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

func fast() *Timer { return &Timer{Interval: 5 * time.Millisecond} }

// collect gathers what a countdown reports.
type collect struct {
	mu    sync.Mutex
	ticks []int
	ends  []bool
	done  chan struct{}
}

func newCollect() *collect { return &collect{done: make(chan struct{}, 4)} }

func (c *collect) tick(n int) {
	c.mu.Lock()
	c.ticks = append(c.ticks, n)
	c.mu.Unlock()
}

func (c *collect) end(fired bool) {
	c.mu.Lock()
	c.ends = append(c.ends, fired)
	c.mu.Unlock()
	c.done <- struct{}{}
}

func (c *collect) wait(t *testing.T) {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		t.Fatal("countdown did not finish")
	}
}

func TestCountsDownAndFiresOnce(t *testing.T) {
	c := newCollect()
	tm := fast()
	if !tm.Start(3, c.tick, c.end) {
		t.Fatal("Start refused an idle timer")
	}
	c.wait(t)
	time.Sleep(30 * time.Millisecond) // a second fire would show up here

	if want := []int{3, 2, 1}; !reflect.DeepEqual(c.ticks, want) {
		t.Errorf("ticks = %v, want %v", c.ticks, want)
	}
	if want := []bool{true}; !reflect.DeepEqual(c.ends, want) {
		t.Errorf("ends = %v, want exactly one fire", c.ends)
	}
	if tm.Running() {
		t.Error("timer still running after it fired")
	}
}

func TestCancelStopsBeforeFiring(t *testing.T) {
	c := newCollect()
	tm := &Timer{Interval: 200 * time.Millisecond}
	tm.Start(5, c.tick, c.end)
	if !tm.Cancel() {
		t.Fatal("Cancel found no running countdown")
	}
	if tm.Cancel() {
		t.Error("a second Cancel must report nothing to cancel")
	}
	c.wait(t)

	if want := []bool{false}; !reflect.DeepEqual(c.ends, want) {
		t.Errorf("ends = %v, want one cancelled end", c.ends)
	}
	if len(c.ticks) != 1 || c.ticks[0] != 5 {
		t.Errorf("ticks = %v, want only the first", c.ticks)
	}
}

func TestOnlyOneCountdownAtATime(t *testing.T) {
	c := newCollect()
	tm := &Timer{Interval: 200 * time.Millisecond}
	if !tm.Start(3, c.tick, c.end) {
		t.Fatal("first Start refused")
	}
	if tm.Start(3, c.tick, c.end) {
		t.Error("a second Start must be refused while one runs")
	}
	tm.Cancel()
	c.wait(t)

	if tm.Start(0, c.tick, c.end) {
		t.Error("a countdown of 0 seconds must be refused")
	}
	// Idle again: a new countdown starts and fires.
	tm.Interval = 5 * time.Millisecond
	if !tm.Start(1, c.tick, c.end) {
		t.Fatal("Start refused after the previous one ended")
	}
	c.wait(t)
	if want := []bool{false, true}; !reflect.DeepEqual(c.ends, want) {
		t.Errorf("ends = %v, want %v", c.ends, want)
	}
}
